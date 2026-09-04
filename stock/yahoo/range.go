package yahoo

import "strings"

// Range is a user-facing chart window. Empty means today's session.
type Range string

const (
	RangeToday Range = ""
	Range5D    Range = "5D"
	Range1M    Range = "1M"
	Range3M    Range = "3M"
	Range6M    Range = "6M"
	Range1Y    Range = "1Y"
	RangeYTD   Range = "YTD"
)

type chartSpec struct {
	rangeValue     string
	interval       string
	includePrePost bool
}

// ParseRange accepts the simple tokens users type after a ticker.
// Empty input is today's session. Tokens are case insensitive.
func ParseRange(s string) (Range, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "":
		return RangeToday, true
	case "5D":
		return Range5D, true
	case "1M":
		return Range1M, true
	case "3M":
		return Range3M, true
	case "6M":
		return Range6M, true
	case "1Y":
		return Range1Y, true
	case "YTD":
		return RangeYTD, true
	default:
		return "", false
	}
}

func (r Range) spec() chartSpec {
	switch r {
	case Range5D:
		return chartSpec{rangeValue: "5d", interval: "15m"}
	case Range1M:
		// 15-minute bars keep enough texture once nights/weekends are dropped from the axis.
		return chartSpec{rangeValue: "1mo", interval: "15m"}
	case Range3M:
		return chartSpec{rangeValue: "3mo", interval: "1d"}
	case Range6M:
		return chartSpec{rangeValue: "6mo", interval: "1d"}
	case Range1Y:
		return chartSpec{rangeValue: "1y", interval: "1d"}
	case RangeYTD:
		return chartSpec{rangeValue: "ytd", interval: "1d"}
	default:
		return chartSpec{rangeValue: "1d", interval: "1m", includePrePost: true}
	}
}
