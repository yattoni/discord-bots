package main

import (
	"regexp"
	"strings"

	"github.com/yattoni/discord-bots/stock/yahoo"
)

// tickerPrefixPattern matches a message that starts with $TICKER, with optional leftover text.
// US common stocks are 1-5 letters; a single-letter share class (e.g. BRK.B) is allowed.
// Yahoo crypto pairs use a hyphenated quote currency (e.g. BTC-USD for Bitcoin spot).
var tickerPrefixPattern = regexp.MustCompile(`(?i)^\$([A-Z]{1,5}(?:\.[A-Z]|-[A-Z]{1,4})?)(?:\s+(.*))?$`)

const quoteRangeOptions = "`5D`, `1M`, `3M`, `6M`, `1Y`, or `YTD`"

// tickerAliases maps shorthand symbols to the Yahoo Finance quote we actually fetch.
// $BTC is Bitcoin spot, not the Grayscale Mini Trust ETF that Yahoo lists as BTC.
var tickerAliases = map[string]string{
	"BTC": "BTC-USD",
}

type quoteRequest struct {
	Ticker string
	Range  yahoo.Range
}

func (r quoteRequest) cacheKey() string {
	if r.Range == yahoo.RangeToday {
		return r.Ticker
	}
	return r.Ticker + ":" + string(r.Range)
}

func (r quoteRequest) logLabel() string {
	if r.Range == yahoo.RangeToday {
		return "$" + r.Ticker
	}
	return "$" + r.Ticker + " " + string(r.Range)
}

// ResolveTicker uppercases a ticker and applies known aliases.
func ResolveTicker(ticker string) string {
	upper := strings.ToUpper(ticker)
	if alias, ok := tickerAliases[upper]; ok {
		return alias
	}
	return upper
}

func parseTickerPrefix(message string) (ticker, rest string, ok bool) {
	trimmed := strings.TrimSpace(message)
	matches := tickerPrefixPattern.FindStringSubmatch(trimmed)
	if matches == nil {
		return "", "", false
	}
	return ResolveTicker(matches[1]), strings.TrimSpace(matches[2]), true
}

// ParseQuoteRequest returns the ticker and optional range from a message like "$NOW" or "$NOW YTD".
// The message may have leading/trailing whitespace, but no other text.
func ParseQuoteRequest(message string) (quoteRequest, bool) {
	ticker, rest, ok := parseTickerPrefix(message)
	if !ok {
		return quoteRequest{}, false
	}
	rng, valid := yahoo.ParseRange(rest)
	if !valid {
		return quoteRequest{}, false
	}
	return quoteRequest{Ticker: ticker, Range: rng}, true
}

// unknownRangeAfterTicker reports leftover text after a ticker that is not a known range.
func unknownRangeAfterTicker(message string) (ticker, extra string, ok bool) {
	ticker, rest, ok := parseTickerPrefix(message)
	if !ok || rest == "" {
		return "", "", false
	}
	if _, valid := yahoo.ParseRange(rest); valid {
		return "", "", false
	}
	return ticker, rest, true
}

// parsePreviewArg accepts "$NOW YTD", "NOW YTD", or a bare ticker.
func parsePreviewArg(input string) (quoteRequest, bool) {
	trimmed := strings.TrimSpace(input)
	if req, ok := ParseQuoteRequest(trimmed); ok {
		return req, true
	}
	return ParseQuoteRequest("$" + trimmed)
}
