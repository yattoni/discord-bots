package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yattoni/discord-bots/stock/yahoo"
)

func TestQuoteErrorReply(t *testing.T) {
	cases := []struct {
		name   string
		ticker string
		err    error
		substr string
	}{
		{
			name:   "unknown ticker",
			ticker: "ZZZZZ",
			err:    fmt.Errorf("%w: No data found, symbol may be delisted", yahoo.ErrNotFound),
			substr: "I couldn't find `$ZZZZZ`",
		},
		{
			name:   "unknown index ticker",
			ticker: "^ZZZZ",
			err:    fmt.Errorf("%w: No data found, symbol may be delisted", yahoo.ErrNotFound),
			substr: "I couldn't find `$^ZZZZ`",
		},
		{
			name:   "no chart data",
			ticker: "NOW",
			err:    fmt.Errorf("%w for NOW", yahoo.ErrNoData),
			substr: "no recent price data",
		},
		{
			name:   "yahoo down",
			ticker: "NOW",
			err:    fmt.Errorf("%w: yahoo finance returned 503", yahoo.ErrUnavailable),
			substr: "didn't respond for `$NOW`",
		},
		{
			name:   "unexpected render error",
			ticker: "NOW",
			err:    errors.New("encode png: boom"),
			substr: "Something went wrong fetching `$NOW`",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := quoteErrorReply(tc.ticker, tc.err)
			assert.Contains(t, got, tc.substr)
			assert.NotContains(t, got, "boom")
			assert.NotContains(t, got, "503")
			assert.NotContains(t, got, "No data found")
		})
	}
}

func TestFormatQuoteText(t *testing.T) {
	got := formatQuoteText(&yahoo.Quote{
		Symbol:        "NOW",
		ShortName:     "ServiceNow, Inc.",
		Currency:      "USD",
		Price:         144.92,
		Change:        8.2,
		ChangePercent: 6,
		PriceHint:     2,
	})
	assert.Contains(t, got, "**NOW** · ServiceNow, Inc.")
	assert.Contains(t, got, "$144.92")
	assert.Contains(t, got, "+$8.20")
	assert.Contains(t, got, "+6.00%")
}

func TestFormatQuoteTextAfterHours(t *testing.T) {
	got := formatQuoteText(&yahoo.Quote{
		Symbol:               "GNRC",
		ShortName:            "Generac Holdings Inc.",
		Currency:             "USD",
		Price:                232.80,
		Change:               57.69,
		ChangePercent:        32.95,
		RegularPrice:         175.11,
		RegularChange:        0.08,
		RegularChangePercent: 0.046,
		PriceHint:            2,
		PreStart:             time.Unix(1000, 0).UTC(),
		RegularStart:         time.Unix(2000, 0).UTC(),
		RegularEnd:           time.Unix(3000, 0).UTC(),
		PostEnd:              time.Unix(4000, 0).UTC(),
		LastTradeTime:        time.Unix(3500, 0).UTC(),
	})
	assert.Contains(t, got, "**GNRC** · Generac Holdings Inc.")
	assert.Contains(t, got, "$232.80")
	assert.Contains(t, got, "+$57.69")
	assert.Contains(t, got, "+32.95%")
	assert.Contains(t, got, "After hours")
	assert.Contains(t, got, "Close $175.11")
	assert.Contains(t, got, "+$0.08")
	assert.Contains(t, got, "+0.05%")
	assert.NotContains(t, got, "+$0.08 (+32.95%)")
}

func TestFormatQuoteTextCrypto(t *testing.T) {
	got := formatQuoteText(&yahoo.Quote{
		Symbol:         "BTC-USD",
		ShortName:      "Bitcoin USD",
		Currency:       "USD",
		InstrumentType: "CRYPTOCURRENCY",
		Price:          81500.25,
		Change:         4190.08,
		ChangePercent:  5.42,
		PriceHint:      2,
		LastTradeTime:  time.Unix(10800, 0).UTC(),
	})
	assert.Contains(t, got, "**BTC-USD** · Bitcoin USD")
	assert.Contains(t, got, "$81500.25")
	assert.Contains(t, got, "24h")
}

func TestFormatQuoteTextIndex(t *testing.T) {
	got := formatQuoteText(&yahoo.Quote{
		Symbol:         "^TNX",
		ShortName:      "CBOE Interest Rate 10 Year T No",
		Currency:       "USD",
		InstrumentType: "INDEX",
		Price:          4.961,
		Change:         -0.014,
		ChangePercent:  -0.281,
		PriceHint:      4,
		LastTradeTime:  time.Unix(6000, 0).UTC(),
	})
	assert.Contains(t, got, "**^TNX** · CBOE Interest Rate 10 Year T No")
	assert.Contains(t, got, "4.9610")
	assert.Contains(t, got, "-0.0140")
	assert.NotContains(t, got, "$4.9610")
	assert.NotContains(t, got, "-$0.0140")
}

func TestFormatQuoteTextRange(t *testing.T) {
	got := formatQuoteText(&yahoo.Quote{
		Symbol:        "NOW",
		ShortName:     "ServiceNow, Inc.",
		Currency:      "USD",
		Price:         144.92,
		Change:        20,
		ChangePercent: 16,
		PriceHint:     2,
		Range:         yahoo.RangeYTD,
	})
	assert.Contains(t, got, "**NOW** · ServiceNow, Inc.")
	assert.Contains(t, got, "+$20.00")
	assert.Contains(t, got, "YTD")
}

func TestUnknownRangeReply(t *testing.T) {
	got := unknownRangeReply("NOW", "10Y")
	assert.Contains(t, got, "I don't recognize `10Y`")
	assert.Contains(t, got, "`5D`")
	assert.Contains(t, got, "`1M`")
	assert.Contains(t, got, "`3M`")
	assert.Contains(t, got, "`6M`")
	assert.Contains(t, got, "`1Y`")
	assert.Contains(t, got, "`2Y`")
	assert.Contains(t, got, "`5Y`")
	assert.Contains(t, got, "`YTD`")
	assert.Contains(t, got, "`$NOW YTD`")
	assert.Contains(t, got, "today's session")
}

func TestQuoteFileName(t *testing.T) {
	assert.Equal(t, "NOW.png", quoteFileName(&yahoo.Quote{Symbol: "NOW"}))
	assert.Equal(t, "NOW-YTD.png", quoteFileName(&yahoo.Quote{Symbol: "NOW", Range: yahoo.RangeYTD}))
	assert.Equal(t, "NOW-2Y.png", quoteFileName(&yahoo.Quote{Symbol: "NOW", Range: yahoo.Range2Y}))
	assert.Equal(t, "NOW-5Y.png", quoteFileName(&yahoo.Quote{Symbol: "NOW", Range: yahoo.Range5Y}))
	assert.Equal(t, "^TNX.png", quoteFileName(&yahoo.Quote{Symbol: "^TNX"}))
	assert.Equal(t, "^TNX-YTD.png", quoteFileName(&yahoo.Quote{Symbol: "^TNX", Range: yahoo.RangeYTD}))
}

func TestQuoteImageFallback(t *testing.T) {
	got := quoteImageFallback(&yahoo.Quote{Symbol: "AAPL", Price: 1, PriceHint: 2, Currency: "USD"})
	assert.Contains(t, got, "couldn't attach the quote card")
	assert.Contains(t, got, "`$AAPL`")
	assert.Contains(t, got, "**AAPL**")
}
