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
	got := unknownRangeReply("NOW", "2Y")
	assert.Contains(t, got, "I don't recognize `2Y`")
	assert.Contains(t, got, "`5D`")
	assert.Contains(t, got, "`1M`")
	assert.Contains(t, got, "`3M`")
	assert.Contains(t, got, "`6M`")
	assert.Contains(t, got, "`1Y`")
	assert.Contains(t, got, "`YTD`")
	assert.Contains(t, got, "`$NOW YTD`")
	assert.Contains(t, got, "today's session")
}

func TestQuoteFileName(t *testing.T) {
	assert.Equal(t, "NOW.png", quoteFileName(&yahoo.Quote{Symbol: "NOW"}))
	assert.Equal(t, "NOW-YTD.png", quoteFileName(&yahoo.Quote{Symbol: "NOW", Range: yahoo.RangeYTD}))
}

func TestQuoteImageFallback(t *testing.T) {
	got := quoteImageFallback(&yahoo.Quote{Symbol: "AAPL", Price: 1, PriceHint: 2, Currency: "USD"})
	assert.Contains(t, got, "couldn't attach the quote card")
	assert.Contains(t, got, "`$AAPL`")
	assert.Contains(t, got, "**AAPL**")
}
