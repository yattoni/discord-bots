package yahoo

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleChart = `{
  "chart": {
    "result": [{
      "meta": {
        "currency": "USD",
        "symbol": "NOW",
        "exchangeTimezoneName": "America/New_York",
        "regularMarketPrice": 144.92,
        "chartPreviousClose": 136.72,
        "previousClose": 136.72,
        "priceHint": 2,
        "shortName": "ServiceNow, Inc.",
        "longName": "ServiceNow, Inc.",
        "fulldayPrice": 144.92,
        "fulldayChange": 8.20,
        "fulldayChangePercent": 6.00,
        "currentTradingPeriod": {
          "pre": {"timezone": "EDT", "start": 1000, "end": 2000, "gmtoffset": -14400},
          "regular": {"timezone": "EDT", "start": 2000, "end": 3000, "gmtoffset": -14400},
          "post": {"timezone": "EDT", "start": 3000, "end": 4000, "gmtoffset": -14400}
        }
      },
      "timestamp": [1100, 2100, 2500, 3100],
      "indicators": {
        "quote": [{
          "close": [140.00, 142.50, null, 144.92]
        }]
      }
    }],
    "error": null
  }
}`

func TestParseChart(t *testing.T) {
	quote, err := parseChart([]byte(sampleChart))
	require.NoError(t, err)
	assert.Equal(t, "NOW", quote.Symbol)
	assert.Equal(t, "ServiceNow, Inc.", quote.ShortName)
	assert.Equal(t, 144.92, quote.Price)
	assert.Equal(t, 136.72, quote.PreviousClose)
	assert.Equal(t, 8.20, quote.Change)
	assert.Equal(t, 6.00, quote.ChangePercent)
	assert.Equal(t, 3, len(quote.Points))
	assert.Equal(t, 140.00, quote.Points[0].Price)
	assert.Equal(t, 144.92, quote.Points[2].Price)
	assert.Equal(t, time.Unix(1000, 0).UTC(), quote.PreStart)
	assert.Equal(t, time.Unix(2000, 0).UTC(), quote.RegularStart)
	assert.Equal(t, "After hours", quote.SessionLabel())
}

func TestFetchQuoteUsesIncludePrePost(t *testing.T) {
	var gotURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleChart))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	quote, err := client.FetchQuote("NOW")
	require.NoError(t, err)
	assert.Equal(t, "NOW", quote.Symbol)
	assert.Contains(t, gotURL, "includePrePost=true")
	assert.Contains(t, gotURL, "interval=1m")
	assert.Contains(t, gotURL, "/v8/finance/chart/NOW")
}

func TestFetchQuoteNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	_, err := client.FetchQuote("ZZZZ")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Contains(t, err.Error(), "No data found, symbol may be delisted")
}

func TestFetchQuoteUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("try again later"))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	_, err := client.FetchQuote("NOW")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestFetchQuoteNoData(t *testing.T) {
	empty := `{
	  "chart": {
	    "result": [{
	      "meta": {
	        "currency": "USD",
	        "symbol": "NOW",
	        "regularMarketPrice": 0,
	        "previousClose": 10,
	        "currentTradingPeriod": {
	          "pre": {"start": 1000, "end": 2000},
	          "regular": {"start": 2000, "end": 3000},
	          "post": {"start": 3000, "end": 4000}
	        }
	      },
	      "timestamp": [],
	      "indicators": {"quote": [{"close": []}]}
	    }],
	    "error": null
	  }
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(empty))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	_, err := client.FetchQuote("NOW")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoData)
}

func TestParseChartNotFound(t *testing.T) {
	_, err := parseChart([]byte(`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestParseChartOtherError(t *testing.T) {
	_, err := parseChart([]byte(`{"chart":{"result":null,"error":{"code":"Unauthorized","description":"Invalid cookie"}}}`))
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	assert.Contains(t, err.Error(), "Invalid cookie")
}

func TestFetchQuoteLive(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("live Yahoo Finance test disabled")
	}
	quote, err := NewClient().FetchQuote("NOW")
	require.NoError(t, err)
	assert.Equal(t, "NOW", quote.Symbol)
	assert.Greater(t, quote.Price, 0.0)
	assert.Greater(t, len(quote.Points), 10)
	assert.False(t, quote.RegularStart.IsZero())
	assert.False(t, quote.PreStart.IsZero())
	assert.NotEmpty(t, quote.ShortName)
}

func TestLastSessionFiltersOlderDays(t *testing.T) {
	quote := &Quote{
		PreStart: time.Unix(2000, 0).UTC(),
		Points: []Point{
			{Time: time.Unix(1000, 0).UTC(), Price: 1},
			{Time: time.Unix(2100, 0).UTC(), Price: 2},
		},
	}
	got := lastSession(quote)
	require.Len(t, got, 1)
	assert.Equal(t, 2.0, got[0].Price)
}
