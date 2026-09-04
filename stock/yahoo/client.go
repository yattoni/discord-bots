package yahoo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://query1.finance.yahoo.com"

// Client fetches quotes and intraday charts from Yahoo Finance.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    defaultBaseURL,
	}
}

// Quote is a day's worth of price action plus the latest quote.
type Quote struct {
	Symbol         string
	ShortName      string
	LongName       string
	Currency       string
	Price          float64
	PreviousClose  float64
	Change         float64
	ChangePercent  float64
	PriceHint      int
	ExchangeTZ     string
	InstrumentType string
	Points         []Point
	PreStart       time.Time
	RegularStart   time.Time
	RegularEnd     time.Time
	PostEnd        time.Time
	LastTradeTime  time.Time
	Range          Range
}

type Point struct {
	Time  time.Time
	Price float64
}

type chartResponse struct {
	Chart struct {
		Result []chartResult `json:"result"`
		Error  *chartError   `json:"error"`
	} `json:"chart"`
}

type chartError struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type chartResult struct {
	Meta       chartMeta `json:"meta"`
	Timestamp  []int64   `json:"timestamp"`
	Indicators struct {
		Quote []struct {
			Close []*float64 `json:"close"`
		} `json:"quote"`
	} `json:"indicators"`
}

type chartMeta struct {
	Currency             string  `json:"currency"`
	Symbol               string  `json:"symbol"`
	ExchangeTimezoneName string  `json:"exchangeTimezoneName"`
	RegularMarketPrice   float64 `json:"regularMarketPrice"`
	ChartPreviousClose   float64 `json:"chartPreviousClose"`
	PreviousClose        float64 `json:"previousClose"`
	PriceHint            int     `json:"priceHint"`
	ShortName            string  `json:"shortName"`
	LongName             string  `json:"longName"`
	InstrumentType       string  `json:"instrumentType"`
	FulldayPrice         float64 `json:"fulldayPrice"`
	FulldayChange        float64 `json:"fulldayChange"`
	FulldayChangePercent float64 `json:"fulldayChangePercent"`
	CurrentTradingPeriod struct {
		Pre     tradingPeriod `json:"pre"`
		Regular tradingPeriod `json:"regular"`
		Post    tradingPeriod `json:"post"`
	} `json:"currentTradingPeriod"`
}

type tradingPeriod struct {
	Timezone  string `json:"timezone"`
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	GMTOffset int    `json:"gmtoffset"`
}

// FetchQuote loads the latest quote and 1-minute chart, including premarket and after hours.
func (c *Client) FetchQuote(symbol string) (*Quote, error) {
	return c.FetchQuoteRange(symbol, RangeToday)
}

// FetchQuoteRange loads a quote card for today's session or a simple multi-day window.
func (c *Client) FetchQuoteRange(symbol string, rng Range) (*Quote, error) {
	spec := rng.spec()
	raw, err := c.getChart(symbol, spec)
	if err != nil {
		return nil, err
	}
	quote, err := parseChart(raw, rng)
	if err != nil {
		return nil, err
	}
	if len(quote.Points) > 0 {
		return quote, nil
	}
	if rng != RangeToday {
		return nil, fmt.Errorf("%w for %s", ErrNoData, symbol)
	}

	// Weekends and holidays can return an empty 1d series; fall back to the last session in 5d.
	raw, err = c.getChart(symbol, chartSpec{rangeValue: "5d", interval: "1m", includePrePost: true})
	if err != nil {
		return nil, err
	}
	quote, err = parseChart(raw, RangeToday)
	if err != nil {
		return nil, err
	}
	quote.Points = lastSession(quote)
	if len(quote.Points) == 0 {
		return nil, fmt.Errorf("%w for %s", ErrNoData, symbol)
	}
	return quote, nil
}

func (c *Client) getChart(symbol string, spec chartSpec) ([]byte, error) {
	u, err := url.Parse(c.baseURL + "/v8/finance/chart/" + url.PathEscape(symbol))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("interval", spec.interval)
	q.Set("range", spec.rangeValue)
	if spec.includePrePost {
		q.Set("includePrePost", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	// Yahoo Finance rejects requests without a browser-like User-Agent.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; discord-stock-bot/1.0)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: yahoo finance request failed: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read yahoo finance response: %w", ErrUnavailable, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, notFoundError(symbol, body)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: yahoo finance returned %s", ErrUnavailable, resp.Status)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo finance returned %s", resp.Status)
	}
	return body, nil
}

func notFoundError(symbol string, body []byte) error {
	var parsed chartResponse
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Chart.Error != nil && parsed.Chart.Error.Description != "" {
		return fmt.Errorf("%w: %s", ErrNotFound, parsed.Chart.Error.Description)
	}
	return fmt.Errorf("%w: %s", ErrNotFound, symbol)
}

func parseChart(body []byte, rng Range) (*Quote, error) {
	var parsed chartResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode yahoo finance response: %w", err)
	}
	if parsed.Chart.Error != nil {
		return nil, classifyChartError(parsed.Chart.Error)
	}
	if len(parsed.Chart.Result) == 0 {
		return nil, ErrNotFound
	}

	result := parsed.Chart.Result[0]
	meta := result.Meta
	prevClose := meta.PreviousClose
	if prevClose == 0 {
		prevClose = meta.ChartPreviousClose
	}

	var closes []*float64
	if len(result.Indicators.Quote) > 0 {
		closes = result.Indicators.Quote[0].Close
	}

	points := make([]Point, 0, len(result.Timestamp))
	for i, ts := range result.Timestamp {
		if i >= len(closes) || closes[i] == nil {
			continue
		}
		price := *closes[i]
		if price <= 0 {
			continue
		}
		points = append(points, Point{
			Time:  time.Unix(ts, 0).UTC(),
			Price: price,
		})
	}

	price := meta.FulldayPrice
	if price == 0 {
		price = meta.RegularMarketPrice
	}
	if price == 0 && len(points) > 0 {
		price = points[len(points)-1].Price
	}

	change := meta.FulldayChange
	changePct := meta.FulldayChangePercent
	if rng != RangeToday {
		if meta.ChartPreviousClose != 0 {
			prevClose = meta.ChartPreviousClose
		} else if len(points) > 0 {
			prevClose = points[0].Price
		}
		if price != 0 && prevClose != 0 {
			change = price - prevClose
			changePct = (change / prevClose) * 100
		} else {
			change = 0
			changePct = 0
		}
	} else if price != 0 && prevClose != 0 && change == 0 && changePct == 0 && price != prevClose {
		change = price - prevClose
		changePct = (change / prevClose) * 100
	}

	hint := meta.PriceHint
	if hint <= 0 {
		hint = 2
	}

	name := meta.ShortName
	if name == "" {
		name = meta.LongName
	}

	quote := &Quote{
		Symbol:         meta.Symbol,
		ShortName:      name,
		LongName:       meta.LongName,
		Currency:       meta.Currency,
		Price:          price,
		PreviousClose:  prevClose,
		Change:         change,
		ChangePercent:  changePct,
		PriceHint:      hint,
		ExchangeTZ:     meta.ExchangeTimezoneName,
		InstrumentType: meta.InstrumentType,
		Points:         points,
		Range:          rng,
		PreStart:       time.Unix(meta.CurrentTradingPeriod.Pre.Start, 0).UTC(),
		RegularStart:   time.Unix(meta.CurrentTradingPeriod.Regular.Start, 0).UTC(),
		RegularEnd:     time.Unix(meta.CurrentTradingPeriod.Regular.End, 0).UTC(),
		PostEnd:        time.Unix(meta.CurrentTradingPeriod.Post.End, 0).UTC(),
	}
	if rng != RangeToday {
		quote.PreStart = time.Time{}
		quote.RegularStart = time.Time{}
		quote.RegularEnd = time.Time{}
		quote.PostEnd = time.Time{}
	}
	if len(points) > 0 {
		quote.LastTradeTime = points[len(points)-1].Time
	}
	return quote, nil
}

func classifyChartError(err *chartError) error {
	code := strings.ToLower(err.Code)
	desc := strings.ToLower(err.Description)
	if strings.Contains(code, "not found") || strings.Contains(desc, "not found") || strings.Contains(desc, "delisted") {
		if err.Description != "" {
			return fmt.Errorf("%w: %s", ErrNotFound, err.Description)
		}
		return ErrNotFound
	}
	return fmt.Errorf("yahoo finance: %s: %s", err.Code, err.Description)
}

func lastSession(quote *Quote) []Point {
	if len(quote.Points) == 0 {
		return nil
	}
	cutoff := quote.PreStart
	if cutoff.IsZero() {
		last := quote.Points[len(quote.Points)-1].Time
		cutoff = time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, time.UTC)
	}
	filtered := make([]Point, 0, len(quote.Points))
	for _, p := range quote.Points {
		if !p.Time.Before(cutoff) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// IsCrypto reports whether Yahoo classified this quote as a cryptocurrency.
func (q *Quote) IsCrypto() bool {
	return strings.EqualFold(q.InstrumentType, "CRYPTOCURRENCY")
}

// MultiDay reports whether this quote covers a window longer than today's session.
func (q *Quote) MultiDay() bool {
	return q.Range != RangeToday
}

// HasExtendedHours reports whether the quote has distinct premarket and after-hours sessions.
func (q *Quote) HasExtendedHours() bool {
	if q.Range != RangeToday {
		return false
	}
	if q.IsCrypto() {
		return false
	}
	if q.PreStart.IsZero() || q.RegularStart.IsZero() || q.RegularEnd.IsZero() || q.PostEnd.IsZero() {
		return false
	}
	return q.RegularStart.Sub(q.PreStart) > time.Minute && q.PostEnd.Sub(q.RegularEnd) > time.Minute
}

// SessionLabel describes which part of the trading day the latest print belongs to.
func (q *Quote) SessionLabel() string {
	if q.Range != RangeToday {
		return string(q.Range)
	}
	if q.LastTradeTime.IsZero() {
		return "Last session"
	}
	if q.IsCrypto() {
		return "24h"
	}
	if !q.HasExtendedHours() {
		return "Market hours"
	}
	t := q.LastTradeTime
	switch {
	case t.Before(q.RegularStart):
		return "Premarket"
	case !t.Before(q.RegularEnd):
		return "After hours"
	default:
		return "Market hours"
	}
}
