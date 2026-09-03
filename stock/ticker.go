package main

import (
	"regexp"
	"strings"
)

// TickerPattern matches a message that is only a $TICKER symbol.
// US common stocks are 1-5 letters; a single-letter share class (e.g. BRK.B) is allowed.
// Yahoo crypto pairs use a hyphenated quote currency (e.g. BTC-USD for Bitcoin spot).
var tickerPattern = regexp.MustCompile(`(?i)^\$([A-Z]{1,5}(?:\.[A-Z]|-[A-Z]{1,4})?)$`)

// tickerAliases maps shorthand symbols to the Yahoo Finance quote we actually fetch.
// $BTC is Bitcoin spot, not the Grayscale Mini Trust ETF that Yahoo lists as BTC.
var tickerAliases = map[string]string{
	"BTC": "BTC-USD",
}

// ResolveTicker uppercases a ticker and applies known aliases.
func ResolveTicker(ticker string) string {
	upper := strings.ToUpper(ticker)
	if alias, ok := tickerAliases[upper]; ok {
		return alias
	}
	return upper
}

// ParseTicker returns the uppercase ticker from a message like "$NOW".
// The message may have leading/trailing whitespace, but no other text.
func ParseTicker(message string) (string, bool) {
	trimmed := strings.TrimSpace(message)
	matches := tickerPattern.FindStringSubmatch(trimmed)
	if matches == nil {
		return "", false
	}
	return ResolveTicker(matches[1]), true
}
