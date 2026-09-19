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
	assert.InDelta(t, 8.20, quote.Change, 0.0001)
	assert.InDelta(t, 6.00, quote.ChangePercent, 0.01)
	assert.Equal(t, 144.92, quote.RegularPrice)
	assert.InDelta(t, 8.20, quote.RegularChange, 0.01)
	assert.Equal(t, 3, len(quote.Points))
	assert.Equal(t, 140.00, quote.Points[0].Price)
	assert.Equal(t, 144.92, quote.Points[2].Price)
	assert.Equal(t, time.Unix(1000, 0).UTC(), quote.PreStart)
	assert.Equal(t, time.Unix(2000, 0).UTC(), quote.RegularStart)
	assert.Equal(t, "EQUITY", quote.InstrumentType)
	assert.True(t, quote.HasExtendedHours())
	assert.False(t, quote.IsCrypto())
	assert.Equal(t, "After hours", quote.SessionLabel())
	assert.False(t, quote.ShowRegularClose())
	assert.Equal(t, "After hours", quote.HeadlineSessionBadge())
}

const sampleAfterHoursChart = `{
  "chart": {
    "result": [{
      "meta": {
        "currency": "USD",
        "symbol": "GNRC",
        "exchangeTimezoneName": "America/New_York",
        "regularMarketPrice": 175.11,
        "regularMarketChangePercent": 0.046,
        "chartPreviousClose": 175.03,
        "previousClose": 175.03,
        "priceHint": 2,
        "shortName": "Generac Holdings Inc.",
        "longName": "Generac Holdings Inc.",
        "instrumentType": "EQUITY",
        "fulldayPrice": 232.80,
        "fulldayChange": 0.08,
        "fulldayChangePercent": 0.046,
        "currentTradingPeriod": {
          "pre": {"timezone": "EDT", "start": 1000, "end": 2000, "gmtoffset": -14400},
          "regular": {"timezone": "EDT", "start": 2000, "end": 3000, "gmtoffset": -14400},
          "post": {"timezone": "EDT", "start": 3000, "end": 4000, "gmtoffset": -14400}
        }
      },
      "timestamp": [1100, 2100, 2900, 3500],
      "indicators": {
        "quote": [{
          "close": [174.50, 175.00, 175.11, 232.80]
        }]
      }
    }],
    "error": null
  }
}`

func TestParseChartAfterHoursUsesSessionChange(t *testing.T) {
	quote, err := parseChart([]byte(sampleAfterHoursChart), RangeToday)
	require.NoError(t, err)
	assert.Equal(t, "GNRC", quote.Symbol)
	assert.Equal(t, 232.80, quote.Price)
	assert.Equal(t, 175.11, quote.RegularPrice)
	assert.InDelta(t, 0.08, quote.RegularChange, 0.0001)
	assert.Equal(t, 0.046, quote.RegularChangePercent)
	assert.InDelta(t, 57.69, quote.Change, 0.0001)
	assert.InDelta(t, 32.95, quote.ChangePercent, 0.01)
	assert.Equal(t, "After hours", quote.SessionLabel())
	assert.True(t, quote.ShowRegularClose())
	assert.Equal(t, "After hours", quote.HeadlineSessionBadge())
}

const samplePremarketChart = `{
  "chart": {
    "result": [{
      "meta": {
        "currency": "USD",
        "symbol": "GNRC",
        "exchangeTimezoneName": "America/New_York",
        "regularMarketPrice": 175.11,
        "regularMarketChangePercent": 0.046,
        "previousClose": 175.03,
        "priceHint": 2,
        "shortName": "Generac Holdings Inc.",
        "instrumentType": "EQUITY",
        "fulldayPrice": 178.00,
        "fulldayChange": 0.08,
        "fulldayChangePercent": 0.046,
        "currentTradingPeriod": {
          "pre": {"timezone": "EDT", "start": 1000, "end": 2000, "gmtoffset": -14400},
          "regular": {"timezone": "EDT", "start": 2000, "end": 3000, "gmtoffset": -14400},
          "post": {"timezone": "EDT", "start": 3000, "end": 4000, "gmtoffset": -14400}
        }
      },
      "timestamp": [1100, 1500],
      "indicators": {
        "quote": [{
          "close": [176.00, 178.00]
        }]
      }
    }],
    "error": null
  }
}`

func TestParseChartPremarketUsesChangeFromPreviousClose(t *testing.T) {
	quote, err := parseChart([]byte(samplePremarketChart), RangeToday)
	require.NoError(t, err)
	assert.Equal(t, 178.00, quote.Price)
	assert.InDelta(t, 2.97, quote.Change, 0.0001)
	assert.Equal(t, "Premarket", quote.SessionLabel())
	assert.False(t, quote.ShowRegularClose())
	assert.Equal(t, "Premarket", quote.HeadlineSessionBadge())
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

const sampleIndexChart = `{
  "chart": {
    "result": [{
      "meta": {
        "currency": "USD",
        "symbol": "^TNX",
        "exchangeTimezoneName": "America/Chicago",
        "instrumentType": "INDEX",
        "regularMarketPrice": 4.961,
        "chartPreviousClose": 4.975,
        "previousClose": 4.975,
        "priceHint": 4,
        "shortName": "CBOE Interest Rate 10 Year T No",
        "longName": "CBOE Interest Rate 10 Year T No",
        "fulldayPrice": 4.961,
        "fulldayChange": -0.014,
        "fulldayChangePercent": -0.281,
        "currentTradingPeriod": {
          "pre": {"timezone": "CDT", "start": 1000, "end": 1000, "gmtoffset": -18000},
          "regular": {"timezone": "CDT", "start": 1000, "end": 25000, "gmtoffset": -18000},
          "post": {"timezone": "CDT", "start": 25000, "end": 25000, "gmtoffset": -18000}
        }
      },
      "timestamp": [2000, 4000, 6000],
      "indicators": {
        "quote": [{
          "close": [4.980, 4.970, 4.961]
        }]
      }
    }],
    "error": null
  }
}`

func TestParseIndexChart(t *testing.T) {
	quote, err := parseChart([]byte(sampleIndexChart), RangeToday)
	require.NoError(t, err)
	assert.Equal(t, "^TNX", quote.Symbol)
	assert.Equal(t, "CBOE Interest Rate 10 Year T No", quote.ShortName)
	assert.Equal(t, "INDEX", quote.InstrumentType)
	assert.True(t, quote.IsIndex())
	assert.False(t, quote.IsCrypto())
	assert.False(t, quote.HasExtendedHours())
	assert.Equal(t, 4.961, quote.Price)
	assert.Equal(t, 4.975, quote.PreviousClose)
	assert.InDelta(t, -0.014, quote.Change, 0.0001)
	assert.Equal(t, 3, len(quote.Points))
	assert.Equal(t, "Market hours", quote.SessionLabel())
	assert.True(t, quote.PreStart.IsZero())
	assert.True(t, quote.PostEnd.IsZero())
}

func TestIndexHasNoExtendedHoursEvenWhenYahooReportsSessions(t *testing.T) {
	quote := &Quote{
		InstrumentType: "INDEX",
		Range:          RangeToday,
		PreStart:       time.Unix(1000, 0).UTC(),
		RegularStart:   time.Unix(2000, 0).UTC(),
		RegularEnd:     time.Unix(3000, 0).UTC(),
		PostEnd:        time.Unix(4000, 0).UTC(),
		LastTradeTime:  time.Unix(3500, 0).UTC(),
	}
	assert.True(t, quote.IsIndex())
	assert.False(t, quote.HasExtendedHours())
	assert.Equal(t, "Market hours", quote.SessionLabel())
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

func TestFetchQuoteEncodesCaretIndex(t *testing.T) {
	var gotURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleIndexChart))
	}))
	defer server.Close()

	client := NewClient()
	client.baseURL = server.URL
	quote, err := client.FetchQuote("^TNX")
	require.NoError(t, err)
	assert.Equal(t, "^TNX", quote.Symbol)
	assert.True(t, quote.IsIndex())
	assert.Contains(t, gotURL, "/v8/finance/chart/%5ETNX")
	assert.NotContains(t, gotURL, "/chart/^TNX")
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

func TestFetchQuoteLiveIndex(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("live Yahoo Finance test disabled")
	}
	quote, err := NewClient().FetchQuote("^TNX")
	require.NoError(t, err)
	assert.Equal(t, "^TNX", quote.Symbol)
	assert.True(t, quote.IsIndex())
	assert.Greater(t, quote.Price, 0.0)
	assert.Greater(t, len(quote.Points), 10)
	assert.Contains(t, strings.ToLower(quote.ShortName), "10 year")
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
	cases := []struct {
		rng   Range
		value string
	}{
		{rng: RangeYTD, value: "ytd"},
		{rng: Range2Y, value: "2y"},
		{rng: Range5Y, value: "5y"},
	}
	for _, tc := range cases {
		t.Run(string(tc.rng), func(t *testing.T) {
			var gotURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotURL = r.URL.String()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(sampleChart))
			}))
			defer server.Close()

			client := NewClient()
			client.baseURL = server.URL
			quote, err := client.FetchQuoteRange("NOW", tc.rng)
			require.NoError(t, err)
			assert.Equal(t, tc.rng, quote.Range)
			assert.Contains(t, gotURL, "range="+tc.value)
			assert.Contains(t, gotURL, "interval=1d")
			assert.NotContains(t, gotURL, "includePrePost=true")
		})
	}
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

func TestFetchQuoteLive2Y(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("live Yahoo Finance test disabled")
	}
	quote, err := NewClient().FetchQuoteRange("NOW", Range2Y)
	require.NoError(t, err)
	assert.Equal(t, "NOW", quote.Symbol)
	assert.Equal(t, Range2Y, quote.Range)
	assert.Greater(t, quote.Price, 0.0)
	assert.Greater(t, len(quote.Points), 200)
	assert.True(t, quote.MultiDay())
	assert.Equal(t, "2Y", quote.SessionLabel())
}

func TestFetchQuoteLive5Y(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("live Yahoo Finance test disabled")
	}
	quote, err := NewClient().FetchQuoteRange("NOW", Range5Y)
	require.NoError(t, err)
	assert.Equal(t, "NOW", quote.Symbol)
	assert.Equal(t, Range5Y, quote.Range)
	assert.Greater(t, quote.Price, 0.0)
	assert.Greater(t, len(quote.Points), 400)
	assert.True(t, quote.MultiDay())
	assert.Equal(t, "5Y", quote.SessionLabel())
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
