package main

import (
	"regexp"
	"strings"
)

// TickerPattern matches a message that is only a $TICKER symbol.
// US common stocks are 1-5 letters; a single-letter share class (e.g. BRK.B) is allowed.
// Yahoo crypto pairs use a hyphenated quote currency (e.g. BTC-USD for Bitcoin spot).
var tickerPattern = regexp.MustCompile(`(?i)^\$([A-Z]{1,5}(?:\.[A-Z]|-[A-Z]{1,4})?)$`)

// ParseTicker returns the uppercase ticker from a message like "$NOW".
// The message may have leading/trailing whitespace, but no other text.
func ParseTicker(message string) (string, bool) {
	trimmed := strings.TrimSpace(message)
	matches := tickerPattern.FindStringSubmatch(trimmed)
	if matches == nil {
		return "", false
	}
	return strings.ToUpper(matches[1]), true
}
