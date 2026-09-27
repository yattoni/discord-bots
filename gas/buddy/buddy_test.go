package buddy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleGraphQLBody = `{
  "data": {
    "station": {
      "id": "10870",
      "name": "Chevron",
      "prices": [
        {"fuelProduct":"regular_gas","longName":"Regular","credit":{"formattedPrice":"$4.299","price":4.299},"cash":null},
        {"fuelProduct":"midgrade_gas","longName":"Midgrade","credit":{"formattedPrice":"$4.459","price":4.459},"cash":null},
        {"fuelProduct":"premium_gas","longName":"Premium","credit":{"formattedPrice":"$4.599","price":4.599},"cash":null},
        {"fuelProduct":"diesel","longName":"Diesel","credit":{"formattedPrice":"$5.199","price":5.199},"cash":null}
      ]
    }
  }
}`

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := NewClient()
	require.NoError(t, err)
	client.http = srv.Client()
	client.origin = srv.URL
	client.graphql = srv.URL + "/graphql"
	return client
}

func TestGetFromGasBuddySuccess(t *testing.T) {
	var gotAuth, gotOp, gotID, gotReferer string
	var gotQuery gqlRequest

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`<html><script>window.gbcsrf = "1.test-csrf";</script></html>`))
		case r.URL.Path == "/graphql":
			gotAuth = r.Header.Get("gbcsrf")
			gotReferer = r.Header.Get("Referer")
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &gotQuery))
			gotOp = gotQuery.OperationName
			if id, ok := gotQuery.Variables["id"].(string); ok {
				gotID = id
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sampleGraphQLBody))
		default:
			http.NotFound(w, r)
		}
	}))

	got, err := GetFromGasBuddy(context.Background(), client, client.origin+"/station/10870", "Los Angeles")
	require.NoError(t, err)
	assert.Equal(t, "Los Angeles:\n\tRegular: $4.299\n\tMid: $4.459\n\tPremium: $4.599", got)
	assert.Equal(t, "1.test-csrf", gotAuth)
	assert.Equal(t, "GetStation", gotOp)
	assert.Equal(t, "10870", gotID)
	assert.Contains(t, gotReferer, "/station/10870")
	assert.Contains(t, gotQuery.Query, "station(id: $id)")
}

func TestGetFromGasBuddySkipsUnavailableGrades(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`<html><script>window.gbcsrf = "1.x";</script></html>`))
			return
		}
		_, _ = w.Write([]byte(`{
		  "data": {
		    "station": {
		      "id": "1",
		      "name": "Marathon",
		      "prices": [
		        {"fuelProduct":"regular_gas","longName":"Regular","credit":{"formattedPrice":"$3.19","price":3.19},"cash":null},
		        {"fuelProduct":"midgrade_gas","longName":"Midgrade","credit":{"formattedPrice":"- - -","price":0},"cash":null},
		        {"fuelProduct":"premium_gas","longName":"Premium","credit":{"formattedPrice":"- - -","price":0},"cash":null}
		      ]
		    }
		  }
		}`))
	}))

	got, err := GetFromGasBuddy(context.Background(), client, client.origin+"/station/1", "Chicago")
	require.NoError(t, err)
	assert.Equal(t, "Chicago:\n\tRegular: $3.19\n\tMid: \n\tPremium: ", got)
}

func TestGetFromGasBuddyAPIErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{
			name:   "forbidden",
			status: http.StatusForbidden,
			body:   `{"errors":[{"message":"Invalid CSRF"}]}`,
			want:   ErrUnauthorized,
		},
		{
			name:   "rate limited",
			status: http.StatusTooManyRequests,
			body:   `slow down`,
			want:   ErrRateLimited,
		},
		{
			name:   "server error",
			status: http.StatusBadGateway,
			body:   `upstream down`,
			want:   ErrUnavailable,
		},
		{
			name:   "empty prices",
			status: http.StatusOK,
			body:   `{"data":{"station":{"id":"1","name":"X","prices":[{"fuelProduct":"regular_gas","longName":"Regular","credit":{"formattedPrice":"- - -","price":0}}]}}}`,
			want:   ErrEmpty,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte(`<html><script>window.gbcsrf = "1.x";</script></html>`))
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))

			_, err := GetFromGasBuddy(context.Background(), client, client.origin+"/station/1", "X")
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestGetFromGasBuddyChallengePage(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<title>Attention Required! | Cloudflare</title><h1>Sorry, you have been blocked</h1>`))
	}))

	_, err := GetFromGasBuddy(context.Background(), client, client.origin+"/station/1", "X")
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Contains(t, err.Error(), "challenge")
}

func TestGetFromGasBuddyMissingClient(t *testing.T) {
	_, err := GetFromGasBuddy(context.Background(), nil, "https://www.gasbuddy.com/station/1", "X")
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestStationIDFromURL(t *testing.T) {
	id, err := stationIDFromURL("https://www.gasbuddy.com/station/10870")
	require.NoError(t, err)
	assert.Equal(t, "10870", id)

	_, err = stationIDFromURL("https://www.gasbuddy.com/home")
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestRefreshCSRFOnUnauthorized(t *testing.T) {
	gets := 0
	posts := 0
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			gets++
			_, _ = w.Write([]byte(`<html><script>window.gbcsrf = "1.refresh";</script></html>`))
			return
		}
		posts++
		if posts == 1 {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Invalid CSRF token"}]}`))
			return
		}
		_, _ = w.Write([]byte(sampleGraphQLBody))
	}))

	got, err := GetFromGasBuddy(context.Background(), client, client.origin+"/station/10870", "Los Angeles")
	require.NoError(t, err)
	assert.Contains(t, got, "Regular: $4.299")
	assert.Equal(t, 2, gets)
	assert.Equal(t, 2, posts)
}

func TestNormalizePrice(t *testing.T) {
	assert.Equal(t, "$4.299", normalizePrice("  $4.299  "))
	assert.Equal(t, "$3.19", normalizePrice("3.19 posted 2 hrs ago"))
	assert.Equal(t, "", normalizePrice("- - -"))
	assert.Equal(t, "", normalizePrice(`<div class="loader__loader___3nWcm"></div>`))
}

func TestOfferPricePrefersFormatted(t *testing.T) {
	assert.Equal(t, "$6.49", offerPrice(&gqlOffer{FormattedPrice: "$6.49", Price: 6.49}))
	assert.Equal(t, "$3.190", offerPrice(&gqlOffer{FormattedPrice: "- - -", Price: 3.19}))
	assert.Equal(t, "", offerPrice(&gqlOffer{FormattedPrice: "- - -", Price: 0}))
	assert.Equal(t, "", offerPrice(nil))
}

func TestLiveGasBuddyPrices(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("SKIP_LIVE is set")
	}

	client, err := NewClient()
	require.NoError(t, err)

	got, err := GetFromGasBuddy(context.Background(), client, "https://www.gasbuddy.com/station/10870", "Los Angeles")
	require.NoError(t, err)
	assert.Contains(t, got, "Los Angeles:")
	assert.Regexp(t, `Regular: \$`, got)
}
