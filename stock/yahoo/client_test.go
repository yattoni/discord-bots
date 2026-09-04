package yahoo

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
        "instrumentType": "EQUITY",
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
	quote, err := parseChart([]byte(sampleChart), RangeToday)
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
	assert.Equal(t, "EQUITY", quote.InstrumentType)
	assert.True(t, quote.HasExtendedHours())
	assert.False(t, quote.IsCrypto())
	assert.Equal(t, "After hours", quote.SessionLabel())
}

const sampleCryptoChart = `{
  "chart": {
    "result": [{
      "meta": {
        "currency": "USD",
        "symbol": "BTC-USD",
        "exchangeTimezoneName": "UTC",
        "instrumentType": "CRYPTOCURRENCY",
        "regularMarketPrice": 81500.25,
        "chartPreviousClose": 77310.17,
        "previousClose": 77310.17,
        "priceHint": 2,
        "shortName": "Bitcoin USD",
        "longName": "Bitcoin USD",
        "currentTradingPeriod": {
          "pre": {"timezone": "UTC", "start": 1000, "end": 1000, "gmtoffset": 0},
          "regular": {"timezone": "UTC", "start": 1000, "end": 86400, "gmtoffset": 0},
          "post": {"timezone": "UTC", "start": 86400, "end": 86400, "gmtoffset": 0}
        }
      },
      "timestamp": [3600, 7200, 10800],
      "indicators": {
        "quote": [{
          "close": [78000.00, 80000.50, 81500.25]
        }]
      }
    }],
    "error": null
  }
}`

func TestParseCryptoChart(t *testing.T) {
	quote, err := parseChart([]byte(sampleCryptoChart), RangeToday)
	require.NoError(t, err)
	assert.Equal(t, "BTC-USD", quote.Symbol)
	assert.Equal(t, "Bitcoin USD", quote.ShortName)
	assert.Equal(t, "CRYPTOCURRENCY", quote.InstrumentType)
	assert.True(t, quote.IsCrypto())
	assert.False(t, quote.HasExtendedHours())
	assert.Equal(t, 81500.25, quote.Price)
	assert.Equal(t, 77310.17, quote.PreviousClose)
	assert.InDelta(t, 4190.08, quote.Change, 0.01)
	assert.Equal(t, 3, len(quote.Points))
	assert.Equal(t, "24h", quote.SessionLabel())
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

func TestFetchQuoteCryptoPath(t *testing.T) {
	var gotURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleCryptoChart))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	quote, err := client.FetchQuote("BTC-USD")
	require.NoError(t, err)
	assert.Equal(t, "BTC-USD", quote.Symbol)
	assert.Contains(t, gotURL, "/v8/finance/chart/BTC-USD")
	assert.True(t, quote.IsCrypto())
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
	_, err := parseChart([]byte(`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`), RangeToday)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestParseChartOtherError(t *testing.T) {
	_, err := parseChart([]byte(`{"chart":{"result":null,"error":{"code":"Unauthorized","description":"Invalid cookie"}}}`), RangeToday)
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

func TestFetchQuoteLiveCrypto(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("live Yahoo Finance test disabled")
	}
	quote, err := NewClient().FetchQuote("BTC-USD")
	require.NoError(t, err)
	assert.Equal(t, "BTC-USD", quote.Symbol)
	assert.True(t, quote.IsCrypto())
	assert.Greater(t, quote.Price, 0.0)
	assert.Greater(t, len(quote.Points), 10)
	assert.Contains(t, strings.ToLower(quote.ShortName), "bitcoin")
}

func TestParseChartRangeUsesStartOfWindow(t *testing.T) {
	quote, err := parseChart([]byte(sampleChart), RangeYTD)
	require.NoError(t, err)
	assert.Equal(t, RangeYTD, quote.Range)
	assert.True(t, quote.MultiDay())
	assert.False(t, quote.HasExtendedHours())
	assert.Equal(t, "YTD", quote.SessionLabel())
	assert.Equal(t, 136.72, quote.PreviousClose)
	assert.InDelta(t, 8.20, quote.Change, 0.01)
	assert.True(t, quote.PreStart.IsZero())
	assert.True(t, quote.PostEnd.IsZero())
}

func TestParseChartRangeIgnoresTodayChange(t *testing.T) {
	body := `{
	  "chart": {
	    "result": [{
	      "meta": {
	        "currency": "USD",
	        "symbol": "NOW",
	        "regularMarketPrice": 150,
	        "chartPreviousClose": 100,
	        "previousClose": 148,
	        "fulldayPrice": 150,
	        "fulldayChange": 2,
	        "fulldayChangePercent": 1.35,
	        "currentTradingPeriod": {
	          "pre": {"start": 1000, "end": 2000},
	          "regular": {"start": 2000, "end": 3000},
	          "post": {"start": 3000, "end": 4000}
	        }
	      },
	      "timestamp": [1000, 2000],
	      "indicators": {"quote": [{"close": [110.0, 150.0]}]}
	    }],
	    "error": null
	  }
	}`
	quote, err := parseChart([]byte(body), Range1Y)
	require.NoError(t, err)
	assert.Equal(t, 100.0, quote.PreviousClose)
	assert.InDelta(t, 50.0, quote.Change, 0.01)
	assert.InDelta(t, 50.0, quote.ChangePercent, 0.01)
	assert.Equal(t, "1Y", quote.SessionLabel())
}

func TestFetchQuoteRangeUsesYahooWindow(t *testing.T) {
	var gotURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleChart))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	quote, err := client.FetchQuoteRange("NOW", RangeYTD)
	require.NoError(t, err)
	assert.Equal(t, RangeYTD, quote.Range)
	assert.Contains(t, gotURL, "range=ytd")
	assert.Contains(t, gotURL, "interval=1d")
	assert.NotContains(t, gotURL, "includePrePost=true")
}

func TestFetchQuoteRangeNoDataDoesNotFallBackToLastSession(t *testing.T) {
	requests := 0
	empty := `{
	  "chart": {
	    "result": [{
	      "meta": {
	        "currency": "USD",
	        "symbol": "NOW",
	        "regularMarketPrice": 0,
	        "previousClose": 10
	      },
	      "timestamp": [],
	      "indicators": {"quote": [{"close": []}]}
	    }],
	    "error": null
	  }
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(empty))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	_, err := client.FetchQuoteRange("NOW", Range5D)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoData)
	assert.Equal(t, 1, requests)
}

func TestFetchQuoteLiveYTD(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("live Yahoo Finance test disabled")
	}
	quote, err := NewClient().FetchQuoteRange("NOW", RangeYTD)
	require.NoError(t, err)
	assert.Equal(t, "NOW", quote.Symbol)
	assert.Equal(t, RangeYTD, quote.Range)
	assert.Greater(t, quote.Price, 0.0)
	assert.Greater(t, len(quote.Points), 10)
	assert.True(t, quote.MultiDay())
	assert.False(t, quote.HasExtendedHours())
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
