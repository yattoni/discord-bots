package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTicker(t *testing.T) {
	cases := []struct {
		name    string
		message string
		ticker  string
		ok      bool
	}{
		{name: "service now", message: "$NOW", ticker: "NOW", ok: true},
		{name: "single letter", message: "$F", ticker: "F", ok: true},
		{name: "four letters", message: "$AAPL", ticker: "AAPL", ok: true},
		{name: "five letters", message: "$GOOGL", ticker: "GOOGL", ok: true},
		{name: "lowercase", message: "$now", ticker: "NOW", ok: true},
		{name: "whitespace", message: "  $TSLA  \n", ticker: "TSLA", ok: true},
		{name: "share class", message: "$BRK.B", ticker: "BRK.B", ok: true},
		{name: "mixed text", message: "buy $NOW please", ok: false},
		{name: "too long", message: "$GOOGLX", ok: false},
		{name: "missing dollar", message: "NOW", ok: false},
		{name: "empty", message: "", ok: false},
		{name: "just dollar", message: "$", ok: false},
		{name: "numbers only not a ticker", message: "$123", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseTicker(tc.message)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.ticker, got)
		})
	}
}
