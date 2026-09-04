package quoteimg

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"github.com/yattoni/discord-bots/stock/yahoo"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed fonts/LiberationSans-Regular.ttf fonts/LiberationSans-Bold.ttf
var fontFS embed.FS

const (
	width  = 900
	height = 520
)

var (
	bgColor        = color.RGBA{R: 18, G: 20, B: 26, A: 255}
	cardColor      = color.RGBA{R: 26, G: 29, B: 38, A: 255}
	textPrimary    = color.RGBA{R: 240, G: 242, B: 248, A: 255}
	textSecondary  = color.RGBA{R: 156, G: 163, B: 180, A: 255}
	gridColor      = color.RGBA{R: 55, G: 60, B: 74, A: 255}
	chartBand      = color.RGBA{R: 22, G: 24, B: 32, A: 255}
	greenColor     = color.RGBA{R: 34, G: 197, B: 94, A: 255}
	redColor       = color.RGBA{R: 239, G: 68, B: 68, A: 255}
	prevCloseColor = color.RGBA{R: 148, G: 163, B: 184, A: 180}
)

type chartSeg struct {
	points []yahoo.Point
	above  bool
}

func loadFace(name string, size float64) (font.Face, error) {
	data, err := fontFS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// RenderPNG draws a quote card with the day's price action, including premarket and after hours.
func RenderPNG(quote *yahoo.Quote) ([]byte, error) {
	dc := gg.NewContext(width, height)
	dc.SetColor(bgColor)
	dc.Clear()

	// Rounded card
	dc.SetColor(cardColor)
	dc.DrawRoundedRectangle(16, 16, float64(width-32), float64(height-32), 20)
	dc.Fill()

	up := quote.Change >= 0
	accent := greenColor
	if !up {
		accent = redColor
	}

	if err := drawHeader(dc, quote, accent, up); err != nil {
		return nil, err
	}
	if err := drawChart(dc, quote); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawHeader(dc *gg.Context, quote *yahoo.Quote, accent color.Color, up bool) error {
	tickerFace, err := loadFace("fonts/LiberationSans-Bold.ttf", 36)
	if err != nil {
		return err
	}
	priceFace, err := loadFace("fonts/LiberationSans-Bold.ttf", 44)
	if err != nil {
		return err
	}
	nameFace, err := loadFace("fonts/LiberationSans-Regular.ttf", 18)
	if err != nil {
		return err
	}
	changeFace, err := loadFace("fonts/LiberationSans-Bold.ttf", 20)
	if err != nil {
		return err
	}
	metaFace, err := loadFace("fonts/LiberationSans-Regular.ttf", 14)
	if err != nil {
		return err
	}

	dc.SetFontFace(tickerFace)
	dc.SetColor(textPrimary)
	dc.DrawString(quote.Symbol, 40, 62)

	symbolWidth, _ := dc.MeasureString(quote.Symbol)
	dc.SetFontFace(nameFace)
	dc.SetColor(textSecondary)
	name := quote.ShortName
	if name == "" {
		name = quote.LongName
	}
	if len(name) > 42 {
		name = name[:39] + "..."
	}
	dc.DrawString(name, 40+symbolWidth+14, 58)

	priceText := formatMoney(quote.Price, quote.Currency, quote.PriceHint)
	dc.SetFontFace(priceFace)
	dc.SetColor(textPrimary)
	dc.DrawString(priceText, 40, 118)

	sign := "+"
	if !up {
		sign = ""
	}
	changeText := fmt.Sprintf("%s%s  (%s%s%%)",
		sign,
		formatMoney(quote.Change, quote.Currency, quote.PriceHint),
		sign,
		formatNumber(quote.ChangePercent, 2),
	)
	priceWidth, _ := dc.MeasureString(priceText)
	dc.SetFontFace(changeFace)
	dc.SetColor(accent)
	dc.DrawString(changeText, 40+priceWidth+16, 112)

	dc.SetFontFace(metaFace)
	dc.SetColor(textSecondary)
	dc.DrawString(sessionCaption(quote), 40, 146)
	return nil
}

func drawChart(dc *gg.Context, quote *yahoo.Quote) error {
	labelFace, err := loadFace("fonts/LiberationSans-Regular.ttf", 13)
	if err != nil {
		return err
	}

	chartX := 40.0
	chartY := 175.0
	chartW := float64(width - 130)
	chartH := 280.0

	sessionStart := quote.PreStart
	sessionEnd := quote.PostEnd
	if sessionStart.IsZero() || sessionEnd.IsZero() || !sessionEnd.After(sessionStart) {
		if len(quote.Points) > 0 {
			sessionStart = quote.Points[0].Time
			sessionEnd = quote.Points[len(quote.Points)-1].Time
		} else {
			return fmt.Errorf("no chart points to draw")
		}
	}

	minPrice, maxPrice := priceRange(quote)
	priceSpan := maxPrice - minPrice
	if priceSpan == 0 {
		priceSpan = math.Max(math.Abs(minPrice)*0.01, 0.01)
		minPrice -= priceSpan / 2
		maxPrice += priceSpan / 2
		priceSpan = maxPrice - minPrice
	}

	xAt := func(t time.Time) float64 {
		total := sessionEnd.Sub(sessionStart).Seconds()
		if total <= 0 {
			return chartX
		}
		frac := t.Sub(sessionStart).Seconds() / total
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		return chartX + frac*chartW
	}
	yAt := func(price float64) float64 {
		frac := (price - minPrice) / priceSpan
		return chartY + chartH - frac*chartH
	}

	dc.SetColor(chartBand)
	dc.DrawRectangle(chartX, chartY, chartW, chartH)
	dc.Fill()

	baseline := quote.PreviousClose
	if baseline <= 0 && len(quote.Points) > 0 {
		baseline = quote.Points[0].Price
	}
	baselineY := chartY + chartH
	if baseline > 0 {
		baselineY = yAt(baseline)
	}

	if len(quote.Points) >= 2 {
		for _, seg := range splitByPreviousClose(quote.Points, baseline) {
			drawSegment(dc, seg, xAt, yAt, baselineY)
		}
		last := quote.Points[len(quote.Points)-1]
		dot := redColor
		if last.Price >= baseline {
			dot = greenColor
		}
		dc.SetColor(dot)
		dc.DrawCircle(xAt(last.Time), yAt(last.Price), 4.5)
		dc.Fill()
	}

	if baseline > 0 {
		dc.SetColor(prevCloseColor)
		dc.SetDash(6, 5)
		dc.SetLineWidth(1)
		dc.DrawLine(chartX, baselineY, chartX+chartW, baselineY)
		dc.Stroke()
		dc.SetDash()
	}

	loc := location(quote.ExchangeTZ)
	dc.SetFontFace(labelFace)
	dc.SetColor(textSecondary)
	if quote.HasExtendedHours() {
		dc.SetColor(gridColor)
		dc.SetLineWidth(1)
		if !quote.RegularStart.IsZero() {
			x := xAt(quote.RegularStart)
			dc.DrawLine(x, chartY, x, chartY+chartH)
			dc.Stroke()
		}
		if !quote.RegularEnd.IsZero() {
			x := xAt(quote.RegularEnd)
			dc.DrawLine(x, chartY, x, chartY+chartH)
			dc.Stroke()
		}

		dc.SetColor(textSecondary)
		drawCentered(dc, "Pre", (xAt(sessionStart)+xAt(quote.RegularStart))/2, chartY+chartH+22)
		drawCentered(dc, "Market", (xAt(quote.RegularStart)+xAt(quote.RegularEnd))/2, chartY+chartH+22)
		drawCentered(dc, "After hours", (xAt(quote.RegularEnd)+xAt(sessionEnd))/2, chartY+chartH+22)

		if !quote.RegularStart.IsZero() {
			drawCentered(dc, quote.RegularStart.In(loc).Format("3:04 PM"), xAt(quote.RegularStart), chartY+chartH+40)
		}
		if !quote.RegularEnd.IsZero() {
			drawCentered(dc, quote.RegularEnd.In(loc).Format("3:04 PM"), xAt(quote.RegularEnd), chartY+chartH+40)
		}
	} else if quote.MultiDay() {
		drawDateTicks(dc, sessionStart, sessionEnd, loc, xAt, chartY+chartH+22)
	} else {
		span := sessionEnd.Sub(sessionStart)
		ticks := []time.Time{sessionStart}
		if span > 0 {
			ticks = append(ticks, sessionStart.Add(span/3), sessionStart.Add(2*span/3), sessionEnd)
		} else {
			ticks = append(ticks, sessionEnd)
		}
		for _, tick := range ticks {
			drawCentered(dc, tick.In(loc).Format("3:04 PM MST"), xAt(tick), chartY+chartH+22)
		}
	}

	// Price labels on the right
	dc.SetColor(textSecondary)
	dc.DrawStringAnchored(formatNumber(maxPrice, quote.PriceHint), chartX+chartW+10, chartY+4, 0, 0.5)
	dc.DrawStringAnchored(formatNumber(minPrice, quote.PriceHint), chartX+chartW+10, chartY+chartH-2, 0, 0.5)
	if quote.PreviousClose > 0 {
		dc.SetColor(prevCloseColor)
		label := "prev"
		if quote.MultiDay() {
			label = "start"
		}
		dc.DrawStringAnchored(label, chartX+chartW+10, yAt(quote.PreviousClose), 0, 0.5)
	}
	return nil
}

func drawSegment(dc *gg.Context, seg chartSeg, xAt func(time.Time) float64, yAt func(float64) float64, baselineY float64) {
	if len(seg.points) < 2 {
		return
	}
	accent := redColor
	if seg.above {
		accent = greenColor
	}

	dc.NewSubPath()
	dc.MoveTo(xAt(seg.points[0].Time), baselineY)
	for _, p := range seg.points {
		dc.LineTo(xAt(p.Time), yAt(p.Price))
	}
	last := seg.points[len(seg.points)-1]
	dc.LineTo(xAt(last.Time), baselineY)
	dc.ClosePath()
	dc.SetRGBA255(int(accent.R), int(accent.G), int(accent.B), 70)
	dc.Fill()

	dc.SetColor(accent)
	dc.SetLineWidth(2.4)
	dc.SetLineCap(gg.LineCapRound)
	dc.SetLineJoin(gg.LineJoinRound)
	dc.MoveTo(xAt(seg.points[0].Time), yAt(seg.points[0].Price))
	for _, p := range seg.points[1:] {
		dc.LineTo(xAt(p.Time), yAt(p.Price))
	}
	dc.Stroke()
}

func splitByPreviousClose(points []yahoo.Point, prev float64) []chartSeg {
	if len(points) == 0 {
		return nil
	}
	above := func(price float64) bool { return price >= prev }

	var segs []chartSeg
	cur := chartSeg{above: above(points[0].Price), points: []yahoo.Point{points[0]}}
	for i := 1; i < len(points); i++ {
		prevPt := points[i-1]
		pt := points[i]
		isAbove := above(pt.Price)
		if isAbove == cur.above {
			cur.points = append(cur.points, pt)
			continue
		}
		cross := interpolateCross(prevPt, pt, prev)
		cur.points = append(cur.points, cross)
		segs = append(segs, cur)
		cur = chartSeg{above: isAbove, points: []yahoo.Point{cross, pt}}
	}
	if len(cur.points) > 0 {
		segs = append(segs, cur)
	}
	return segs
}

func interpolateCross(a, b yahoo.Point, prev float64) yahoo.Point {
	dy := b.Price - a.Price
	if dy == 0 {
		return yahoo.Point{Time: a.Time, Price: prev}
	}
	t := (prev - a.Price) / dy
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return yahoo.Point{
		Time:  a.Time.Add(time.Duration(float64(b.Time.Sub(a.Time)) * t)),
		Price: prev,
	}
}

func drawDateTicks(dc *gg.Context, start, end time.Time, loc *time.Location, xAt func(time.Time) float64, y float64) {
	span := end.Sub(start)
	ticks := []time.Time{start}
	if span > 0 {
		ticks = append(ticks, start.Add(span/3), start.Add(2*span/3), end)
	} else {
		ticks = append(ticks, end)
	}
	layout := "Jan 2"
	if span >= 300*24*time.Hour {
		layout = "Jan 2006"
	}
	for _, tick := range ticks {
		drawCentered(dc, tick.In(loc).Format(layout), xAt(tick), y)
	}
}

func drawCentered(dc *gg.Context, text string, x, y float64) {
	dc.DrawStringAnchored(text, x, y, 0.5, 0.5)
}

func priceRange(quote *yahoo.Quote) (float64, float64) {
	minP := math.MaxFloat64
	maxP := -math.MaxFloat64
	for _, p := range quote.Points {
		if p.Price < minP {
			minP = p.Price
		}
		if p.Price > maxP {
			maxP = p.Price
		}
	}
	if quote.PreviousClose > 0 {
		if quote.PreviousClose < minP {
			minP = quote.PreviousClose
		}
		if quote.PreviousClose > maxP {
			maxP = quote.PreviousClose
		}
	}
	if minP == math.MaxFloat64 {
		minP = quote.Price
		maxP = quote.Price
	}
	pad := (maxP - minP) * 0.12
	if pad == 0 {
		pad = math.Max(math.Abs(minP)*0.01, 0.01)
	}
	return minP - pad, maxP + pad
}

func sessionCaption(quote *yahoo.Quote) string {
	loc := location(quote.ExchangeTZ)
	when := quote.LastTradeTime
	if when.IsZero() {
		when = time.Now().In(loc)
	} else {
		when = when.In(loc)
	}
	whenText := when.Format("Jan 2, 2006  3:04 PM MST")
	if quote.MultiDay() {
		whenText = when.Format("Jan 2, 2006")
	}
	return fmt.Sprintf("%s  ·  %s  ·  %s",
		whenText,
		quote.SessionLabel(),
		sourceLabel(quote.Currency),
	)
}

func sourceLabel(currency string) string {
	if currency == "" {
		return "Yahoo Finance"
	}
	return currency + " · Yahoo Finance"
}

func location(name string) *time.Location {
	if name == "" {
		name = "America/New_York"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("ET", -5*3600)
	}
	return loc
}

func formatMoney(amount float64, currency string, hint int) string {
	number := formatNumber(amount, hint)
	switch strings.ToUpper(currency) {
	case "", "USD":
		if amount < 0 {
			return "-$" + formatNumber(-amount, hint)
		}
		return "$" + number
	default:
		return number + " " + currency
	}
}

func formatNumber(amount float64, hint int) string {
	if hint <= 0 {
		hint = 2
	}
	return fmt.Sprintf("%.*f", hint, amount)
}

// DecodeSize reports PNG dimensions for tests.
func DecodeSize(png []byte) (image.Config, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(png))
	return cfg, err
}
