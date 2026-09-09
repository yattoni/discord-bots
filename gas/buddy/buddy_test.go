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

const sampleScrapeBody = `{
  "success": true,
  "result": [
    {
      "selector": "[class*=\"GasPriceCollection-module__fuelTypeDisplay\"]",
      "results": [
        {"text": "Regular", "html": "Regular"},
        {"text": "Midgrade", "html": "Midgrade"},
        {"text": "Premium", "html": "Premium"},
        {"text": "Diesel", "html": "Diesel"},
        {"text": "E85", "html": "E85"}
      ]
    },
    {
      "selector": "[class*=\"FuelTypePriceDisplay-module__price\"]",
      "results": [
        {"text": "$4.299", "html": "$4.299"},
        {"text": "$4.459", "html": "$4.459"},
        {"text": "$4.599", "html": "$4.599"},
        {"text": "$5.199", "html": "$5.199"},
        {"text": "$3.999", "html": "$3.999"}
      ]
    }
  ]
}`

func TestScrapeSuccess(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody scrapeRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		assert.Equal(t, "0", r.URL.Query().Get("cacheTTL"))
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleScrapeBody))
	}))
	defer srv.Close()

	client := NewCloudflare("acct-123", "test-token")
	client.baseURL = srv.URL
	client.httpClient = srv.Client()

	got, err := GetFromGasBuddy(context.Background(), client, "https://www.gasbuddy.com/station/10870", "Los Angeles")
	require.NoError(t, err)
	assert.Equal(t, "Los Angeles:\n\tRegular: $4.299\n\tMid: $4.459\n\tPremium: $4.599", got)
	assert.Equal(t, "Bearer test-token", gotAuth)
	assert.Equal(t, "/accounts/acct-123/browser-rendering/scrape", gotPath)
	assert.Equal(t, "https://www.gasbuddy.com/station/10870", gotBody.URL)
	assert.Equal(t, []scrapeSelector{{Selector: labelSelector}, {Selector: priceSelector}}, gotBody.Elements)
	assert.Equal(t, "networkidle2", gotBody.GotoOptions.WaitUntil)
	assert.Equal(t, priceReadySelector, gotBody.WaitForSelector.Selector)
	assert.True(t, gotBody.BestAttempt)
}

func TestScrapeSingleResultObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
		  "success": true,
		  "result": [{
		    "selector": "[class*=\"FuelTypePriceDisplay-module__price\"]",
		    "results": {"text": "$3.19  posted 1 hr ago", "html": "$3.19"}
		  }]
		}`))
	}))
	defer srv.Close()

	client := NewCloudflare("acct", "token")
	client.baseURL = srv.URL
	client.httpClient = srv.Client()

	got, err := GetFromGasBuddy(context.Background(), client, "https://www.gasbuddy.com/station/1", "Chicago")
	require.NoError(t, err)
	assert.Equal(t, "Chicago:\n\tRegular: $3.19\n\tMid: \n\tPremium: ", got)
}

func TestScrapeAPIErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{
			name:   "unauthorized",
			status: http.StatusUnauthorized,
			body:   `{"success":false,"errors":[{"code":10000,"message":"Invalid API Token"}]}`,
			want:   ErrUnauthorized,
		},
		{
			name:   "rate limited",
			status: http.StatusTooManyRequests,
			body:   `{"success":false,"errors":[{"code":2001,"message":"Rate limit exceeded"}]}`,
			want:   ErrRateLimited,
		},
		{
			name:   "server error",
			status: http.StatusBadGateway,
			body:   `{"success":false,"errors":[{"code":500,"message":"upstream down"}]}`,
			want:   ErrUnavailable,
		},
		{
			name:   "empty prices",
			status: http.StatusOK,
			body:   `{"success":true,"result":[{"selector":"[class*=\"FuelTypePriceDisplay-module__price\"]","results":[{"text":"","html":"<div class=\"loader__loader\"></div>"}]}]}`,
			want:   ErrEmpty,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			client := NewCloudflare("acct", "token")
			client.baseURL = srv.URL
			client.httpClient = srv.Client()

			_, err := GetFromGasBuddy(context.Background(), client, "https://www.gasbuddy.com/station/1", "X")
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestScrapeMissingCredentials(t *testing.T) {
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	_, err := NewCloudflareFromEnv()
	assert.ErrorIs(t, err, ErrUnauthorized)

	_, err = GetFromGasBuddy(context.Background(), NewCloudflare("", ""), "https://www.gasbuddy.com/station/1", "X")
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestParseStationPricesByLabel(t *testing.T) {
	prices, err := parseStationPrices([]selectorResult{
		{
			Selector: labelSelector,
			Results:  elementHits{{Text: "Premium"}, {Text: "Regular"}, {Text: "Midgrade"}},
		},
		{
			Selector: priceSelector,
			Results:  elementHits{{Text: "$5.00"}, {Text: "$4.00"}, {Text: "$4.50"}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, stationPrices{Regular: "$4.00", Mid: "$4.50", Premium: "$5.00"}, prices)
}

func TestParseStationPricesFallbackOrder(t *testing.T) {
	prices, err := parseStationPrices([]selectorResult{
		{
			Selector: "div.price",
			Results:  elementHits{{Text: "$4.10"}, {Text: "$4.30"}, {Text: "$4.50"}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, stationPrices{Regular: "$4.10", Mid: "$4.30", Premium: "$4.50"}, prices)
}

func TestParseStationPricesChallengePage(t *testing.T) {
	_, err := parseStationPrices([]selectorResult{
		{
			Selector: priceSelector,
			Results:  elementHits{{Text: "Just a moment..."}, {Text: "Attention Required! | Cloudflare"}},
		},
	})
	assert.ErrorIs(t, err, ErrUnavailable)
}

func TestNormalizePrice(t *testing.T) {
	assert.Equal(t, "$4.299", normalizePrice("  $4.299  "))
	assert.Equal(t, "$3.19", normalizePrice("3.19 posted 2 hrs ago"))
	assert.Equal(t, "", normalizePrice(`<div class="loader__loader___3nWcm"></div>`))
}

func TestRateLimitErrorIncludesRetryAfter(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Retry-After", "10")
	err := rateLimitError(resp, []byte("slow down"))
	assert.ErrorIs(t, err, ErrRateLimited)
	assert.Contains(t, err.Error(), "retry after 10s")
}

func TestElementHitsUnmarshalNull(t *testing.T) {
	var hits elementHits
	require.NoError(t, json.Unmarshal([]byte("null"), &hits))
	assert.Nil(t, hits)
}

func TestLiveGasBuddyScrape(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("SKIP_LIVE is set")
	}
	account := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	token := os.Getenv("CLOUDFLARE_API_TOKEN")
	if account == "" || token == "" {
		t.Skip("CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN not set")
	}

	got, err := GetFromGasBuddy(context.Background(), NewCloudflare(account, token), "https://www.gasbuddy.com/station/10870", "Los Angeles")
	require.NoError(t, err)
	assert.Contains(t, got, "Los Angeles:")
	assert.Regexp(t, `Regular: \$`, got)
}
