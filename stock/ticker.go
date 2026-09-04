package main

import (
	"regexp"
	"strings"

	"github.com/yattoni/discord-bots/stock/yahoo"
)

// quoteRequestPattern matches a message that is only a $TICKER, optionally followed by a range.
// US common stocks are 1-5 letters; a single-letter share class (e.g. BRK.B) is allowed.
// Yahoo crypto pairs use a hyphenated quote currency (e.g. BTC-USD for Bitcoin spot).
var quoteRequestPattern = regexp.MustCompile(`(?i)^\$([A-Z]{1,5}(?:\.[A-Z]|-[A-Z]{1,4})?)(?:\s+(5D|1M|3M|6M|1Y|YTD))?$`)

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

// ParseQuoteRequest returns the ticker and optional range from a message like "$NOW" or "$NOW YTD".
// The message may have leading/trailing whitespace, but no other text.
func ParseQuoteRequest(message string) (quoteRequest, bool) {
	trimmed := strings.TrimSpace(message)
	matches := quoteRequestPattern.FindStringSubmatch(trimmed)
	if matches == nil {
		return quoteRequest{}, false
	}
	rng, _ := yahoo.ParseRange(matches[2])
	return quoteRequest{Ticker: ResolveTicker(matches[1]), Range: rng}, true
}

// parsePreviewArg accepts "$NOW YTD", "NOW YTD", or a bare ticker.
func parsePreviewArg(input string) (quoteRequest, bool) {
	trimmed := strings.TrimSpace(input)
	if req, ok := ParseQuoteRequest(trimmed); ok {
		return req, true
	}
	return ParseQuoteRequest("$" + trimmed)
}
