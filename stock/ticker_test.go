package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yattoni/discord-bots/stock/yahoo"
)

func TestParseQuoteRequest(t *testing.T) {
	cases := []struct {
		name    string
		message string
		ticker  string
		rng     yahoo.Range
		ok      bool
	}{
		{name: "service now", message: "$NOW", ticker: "NOW", ok: true},
		{name: "single letter", message: "$F", ticker: "F", ok: true},
		{name: "four letters", message: "$AAPL", ticker: "AAPL", ok: true},
		{name: "five letters", message: "$GOOGL", ticker: "GOOGL", ok: true},
		{name: "lowercase", message: "$now", ticker: "NOW", ok: true},
		{name: "whitespace", message: "  $TSLA  \n", ticker: "TSLA", ok: true},
		{name: "share class", message: "$BRK.B", ticker: "BRK.B", ok: true},
		{name: "hyphenated share class", message: "$BRK-B", ticker: "BRK-B", ok: true},
		{name: "bitcoin spot", message: "$BTC-USD", ticker: "BTC-USD", ok: true},
		{name: "ethereum lowercase", message: "$eth-usd", ticker: "ETH-USD", ok: true},
		{name: "crypto pair whitespace", message: "  $SOL-USD  \n", ticker: "SOL-USD", ok: true},
		{name: "bitcoin alias", message: "$BTC", ticker: "BTC-USD", ok: true},
		{name: "bitcoin alias lowercase", message: "$btc", ticker: "BTC-USD", ok: true},
		{name: "ytd range", message: "$NOW YTD", ticker: "NOW", rng: yahoo.RangeYTD, ok: true},
		{name: "lowercase ytd", message: "$now ytd", ticker: "NOW", rng: yahoo.RangeYTD, ok: true},
		{name: "mixed case range", message: "$AAPL 5d", ticker: "AAPL", rng: yahoo.Range5D, ok: true},
		{name: "one month", message: "$F 1M", ticker: "F", rng: yahoo.Range1M, ok: true},
		{name: "three months", message: "$BRK.B 3m", ticker: "BRK.B", rng: yahoo.Range3M, ok: true},
		{name: "six months", message: "$BTC 6M", ticker: "BTC-USD", rng: yahoo.Range6M, ok: true},
		{name: "one year extra space", message: "$ETH-USD   1y", ticker: "ETH-USD", rng: yahoo.Range1Y, ok: true},
		{name: "mixed text", message: "buy $NOW please", ok: false},
		{name: "unsupported range", message: "$NOW 2Y", ok: false},
		{name: "unknown range words", message: "$NOW last week", ok: false},
		{name: "range without space", message: "$NOWYTD", ok: false},
		{name: "too long", message: "$GOOGLX", ok: false},
		{name: "crypto base too long", message: "$BITCOIN-USD", ok: false},
		{name: "trailing hyphen", message: "$BTC-", ok: false},
		{name: "slash pair", message: "$BTC/USD", ok: false},
		{name: "missing dollar", message: "NOW", ok: false},
		{name: "empty", message: "", ok: false},
		{name: "just dollar", message: "$", ok: false},
		{name: "numbers only not a ticker", message: "$123", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseQuoteRequest(tc.message)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.ticker, got.Ticker)
			assert.Equal(t, tc.rng, got.Range)
		})
	}
}

func TestUnknownRangeAfterTicker(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		ticker string
		extra  string
		ok     bool
	}{
		{name: "unsupported token", in: "$NOW 2Y", ticker: "NOW", extra: "2Y", ok: true},
		{name: "lowercase unknown", in: "$aapl 1w", ticker: "AAPL", extra: "1w", ok: true},
		{name: "phrase after ticker", in: "$NOW last week", ticker: "NOW", extra: "last week", ok: true},
		{name: "bitcoin alias", in: "$btc 2y", ticker: "BTC-USD", extra: "2y", ok: true},
		{name: "valid ytd is not unknown", in: "$NOW YTD", ok: false},
		{name: "bare ticker is not unknown", in: "$NOW", ok: false},
		{name: "mixed sentence", in: "buy $NOW please", ok: false},
		{name: "range without space", in: "$NOWYTD", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticker, extra, ok := unknownRangeAfterTicker(tc.in)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.ticker, ticker)
			assert.Equal(t, tc.extra, extra)
		})
	}
}

func TestParsePreviewArg(t *testing.T) {
	req, ok := parsePreviewArg("NOW YTD")
	assert.True(t, ok)
	assert.Equal(t, "NOW", req.Ticker)
	assert.Equal(t, yahoo.RangeYTD, req.Range)

	req, ok = parsePreviewArg("$btc 5d")
	assert.True(t, ok)
	assert.Equal(t, "BTC-USD", req.Ticker)
	assert.Equal(t, yahoo.Range5D, req.Range)

	req, ok = parsePreviewArg("AAPL")
	assert.True(t, ok)
	assert.Equal(t, "AAPL", req.Ticker)
	assert.Equal(t, yahoo.RangeToday, req.Range)

	_, ok = parsePreviewArg("NOW 2Y")
	assert.False(t, ok)
}

func TestResolveTicker(t *testing.T) {
	assert.Equal(t, "BTC-USD", ResolveTicker("BTC"))
	assert.Equal(t, "BTC-USD", ResolveTicker("btc"))
	assert.Equal(t, "BTC-USD", ResolveTicker("BTC-USD"))
	assert.Equal(t, "NOW", ResolveTicker("now"))
	assert.Equal(t, "ETH-USD", ResolveTicker("ETH-USD"))
}

func TestQuoteRequestCacheKey(t *testing.T) {
	assert.Equal(t, "NOW", quoteRequest{Ticker: "NOW"}.cacheKey())
	assert.Equal(t, "NOW:YTD", quoteRequest{Ticker: "NOW", Range: yahoo.RangeYTD}.cacheKey())
	assert.Equal(t, "$NOW YTD", quoteRequest{Ticker: "NOW", Range: yahoo.RangeYTD}.logLabel())
}
