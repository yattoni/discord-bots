package yahoo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRange(t *testing.T) {
	cases := []struct {
		in   string
		want Range
		ok   bool
	}{
		{in: "", want: RangeToday, ok: true},
		{in: "  ", want: RangeToday, ok: true},
		{in: "5D", want: Range5D, ok: true},
		{in: "5d", want: Range5D, ok: true},
		{in: "1m", want: Range1M, ok: true},
		{in: "3M", want: Range3M, ok: true},
		{in: "6M", want: Range6M, ok: true},
		{in: "1Y", want: Range1Y, ok: true},
		{in: "2Y", want: Range2Y, ok: true},
		{in: "2y", want: Range2Y, ok: true},
		{in: "5Y", want: Range5Y, ok: true},
		{in: "5y", want: Range5Y, ok: true},
		{in: "ytd", want: RangeYTD, ok: true},
		{in: "YTD", want: RangeYTD, ok: true},
		{in: "10Y", ok: false},
		{in: "1D", ok: false},
		{in: "today", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := ParseRange(tc.in)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRangeSpec(t *testing.T) {
	assert.Equal(t, chartSpec{rangeValue: "1d", interval: "1m", includePrePost: true}, RangeToday.spec())
	assert.Equal(t, chartSpec{rangeValue: "5d", interval: "15m"}, Range5D.spec())
	assert.Equal(t, chartSpec{rangeValue: "1mo", interval: "15m"}, Range1M.spec())
	assert.Equal(t, chartSpec{rangeValue: "3mo", interval: "1d"}, Range3M.spec())
	assert.Equal(t, chartSpec{rangeValue: "6mo", interval: "1d"}, Range6M.spec())
	assert.Equal(t, chartSpec{rangeValue: "1y", interval: "1d"}, Range1Y.spec())
	assert.Equal(t, chartSpec{rangeValue: "2y", interval: "1d"}, Range2Y.spec())
	assert.Equal(t, chartSpec{rangeValue: "5y", interval: "1d"}, Range5Y.spec())
	assert.Equal(t, chartSpec{rangeValue: "ytd", interval: "1d"}, RangeYTD.spec())
}
