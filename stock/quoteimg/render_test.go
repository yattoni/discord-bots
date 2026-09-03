package quoteimg

import (
	"bytes"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yattoni/discord-bots/stock/yahoo"
)

func sampleQuote(change float64) *yahoo.Quote {
	start := time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)
	regular := start.Add(5 * time.Hour)
	regularEnd := regular.Add(6*time.Hour + 30*time.Minute)
	postEnd := regularEnd.Add(4 * time.Hour)
	prev := 100.0
	points := []yahoo.Point{}
	price := prev
	for i := 0; i < 120; i++ {
		if change >= 0 {
			price += 0.15
		} else {
			price -= 0.15
		}
		points = append(points, yahoo.Point{
			Time:  start.Add(time.Duration(i) * 8 * time.Minute),
			Price: price,
		})
	}
	return &yahoo.Quote{
		Symbol:        "NOW",
		ShortName:     "ServiceNow, Inc.",
		Currency:      "USD",
		Price:         price,
		PreviousClose: prev,
		Change:        change,
		ChangePercent: change,
		PriceHint:     2,
		ExchangeTZ:    "America/New_York",
		Points:        points,
		PreStart:      start,
		RegularStart:  regular,
		RegularEnd:    regularEnd,
		PostEnd:       postEnd,
		LastTradeTime: points[len(points)-1].Time,
	}
}

func TestRenderPNGUpDay(t *testing.T) {
	pngBytes, err := RenderPNG(sampleQuote(6.0))
	require.NoError(t, err)
	cfg, err := DecodeSize(pngBytes)
	require.NoError(t, err)
	assert.Equal(t, width, cfg.Width)
	assert.Equal(t, height, cfg.Height)
	assert.True(t, hasColorNear(t, pngBytes, greenColor))
}

func TestRenderPNGDownDay(t *testing.T) {
	pngBytes, err := RenderPNG(sampleQuote(-3.5))
	require.NoError(t, err)
	assert.True(t, hasColorNear(t, pngBytes, redColor))
}

func TestSplitByPreviousClose(t *testing.T) {
	start := time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)
	points := []yahoo.Point{
		{Time: start, Price: 99},
		{Time: start.Add(time.Minute), Price: 99.5},
		{Time: start.Add(2 * time.Minute), Price: 101},
		{Time: start.Add(3 * time.Minute), Price: 102},
		{Time: start.Add(4 * time.Minute), Price: 99},
	}
	segs := splitByPreviousClose(points, 100)
	require.Len(t, segs, 3)
	assert.False(t, segs[0].above)
	assert.Equal(t, 100.0, segs[0].points[len(segs[0].points)-1].Price)
	assert.True(t, segs[1].above)
	assert.Equal(t, 100.0, segs[1].points[0].Price)
	assert.Equal(t, 100.0, segs[1].points[len(segs[1].points)-1].Price)
	assert.False(t, segs[2].above)
	assert.Equal(t, 100.0, segs[2].points[0].Price)
	assert.Equal(t, 99.0, segs[2].points[len(segs[2].points)-1].Price)
}

func TestRenderPNGCrossesPreviousClose(t *testing.T) {
	start := time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)
	regular := start.Add(5 * time.Hour)
	regularEnd := regular.Add(6*time.Hour + 30*time.Minute)
	postEnd := regularEnd.Add(4 * time.Hour)
	points := make([]yahoo.Point, 0, 40)
	for i := 0; i < 40; i++ {
		price := 98.0
		if i >= 10 && i <= 25 {
			price = 103.0
		}
		points = append(points, yahoo.Point{
			Time:  start.Add(time.Duration(i) * 20 * time.Minute),
			Price: price,
		})
	}
	quote := &yahoo.Quote{
		Symbol:        "NFLX",
		ShortName:     "Netflix, Inc.",
		Currency:      "USD",
		Price:         98,
		PreviousClose: 100,
		Change:        -2,
		ChangePercent: -2,
		PriceHint:     2,
		ExchangeTZ:    "America/New_York",
		Points:        points,
		PreStart:      start,
		RegularStart:  regular,
		RegularEnd:    regularEnd,
		PostEnd:       postEnd,
		LastTradeTime: points[len(points)-1].Time,
	}
	pngBytes, err := RenderPNG(quote)
	require.NoError(t, err)
	assert.True(t, hasColorNear(t, pngBytes, greenColor))
	assert.True(t, hasColorNear(t, pngBytes, redColor))
}

func sampleCryptoQuote() *yahoo.Quote {
	start := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	prev := 77000.0
	points := []yahoo.Point{}
	price := prev
	for i := 0; i < 96; i++ {
		price += 80
		points = append(points, yahoo.Point{
			Time:  start.Add(time.Duration(i) * 15 * time.Minute),
			Price: price,
		})
	}
	return &yahoo.Quote{
		Symbol:         "BTC-USD",
		ShortName:      "Bitcoin USD",
		Currency:       "USD",
		InstrumentType: "CRYPTOCURRENCY",
		Price:          price,
		PreviousClose:  prev,
		Change:         price - prev,
		ChangePercent:  ((price - prev) / prev) * 100,
		PriceHint:      2,
		ExchangeTZ:     "UTC",
		Points:         points,
		PreStart:       start,
		RegularStart:   start,
		RegularEnd:     end,
		PostEnd:        end,
		LastTradeTime:  points[len(points)-1].Time,
	}
}

func TestRenderPNGCrypto(t *testing.T) {
	quote := sampleCryptoQuote()
	require.False(t, quote.HasExtendedHours())
	assert.Equal(t, "24h", quote.SessionLabel())
	pngBytes, err := RenderPNG(quote)
	require.NoError(t, err)
	cfg, err := DecodeSize(pngBytes)
	require.NoError(t, err)
	assert.Equal(t, width, cfg.Width)
	assert.Equal(t, height, cfg.Height)
	assert.True(t, hasColorNear(t, pngBytes, greenColor))
}

func TestRenderPNGLive(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("live Yahoo Finance test disabled")
	}
	quote, err := yahoo.NewClient().FetchQuote("NOW")
	require.NoError(t, err)
	pngBytes, err := RenderPNG(quote)
	require.NoError(t, err)
	cfg, err := DecodeSize(pngBytes)
	require.NoError(t, err)
	assert.Equal(t, width, cfg.Width)
	assert.Equal(t, height, cfg.Height)
}

func TestRenderPNGLiveCrypto(t *testing.T) {
	if os.Getenv("SKIP_LIVE") != "" {
		t.Skip("live Yahoo Finance test disabled")
	}
	quote, err := yahoo.NewClient().FetchQuote("BTC-USD")
	require.NoError(t, err)
	pngBytes, err := RenderPNG(quote)
	require.NoError(t, err)
	cfg, err := DecodeSize(pngBytes)
	require.NoError(t, err)
	assert.Equal(t, width, cfg.Width)
	assert.Equal(t, height, cfg.Height)
}

func hasColorNear(t *testing.T, pngBytes []byte, target color.RGBA) bool {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	require.NoError(t, err)
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			r, g, b, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
			if colorNear(got, target, 40) {
				return true
			}
		}
	}
	return false
}

func colorNear(got, want color.RGBA, delta uint8) bool {
	return absDiff(got.R, want.R) <= delta && absDiff(got.G, want.G) <= delta && absDiff(got.B, want.B) <= delta
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
