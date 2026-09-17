package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/yattoni/discord-bots/stock/yahoo"
)

func quoteErrorReply(ticker string, err error) string {
	switch {
	case errors.Is(err, yahoo.ErrNotFound):
		return fmt.Sprintf("I couldn't find `$%s`. Double-check the ticker — it might be mistyped or delisted.", ticker)
	case errors.Is(err, yahoo.ErrNoData):
		return fmt.Sprintf("I found `$%s`, but there's no recent price data to chart.", ticker)
	case errors.Is(err, yahoo.ErrUnavailable):
		return fmt.Sprintf("Yahoo Finance didn't respond for `$%s`. Try again in a bit.", ticker)
	default:
		return fmt.Sprintf("Something went wrong fetching `$%s`. Try again in a bit.", ticker)
	}
}

func formatQuoteText(q *yahoo.Quote) string {
	name := strings.TrimSpace(q.ShortName)
	if name == "" {
		name = strings.TrimSpace(q.LongName)
	}
	header := "**" + q.Symbol + "**"
	if name != "" {
		header += " · " + name
	}

	hint := q.PriceHint
	if hint <= 0 {
		hint = 2
	}
	priceLine := formatPriceChangeLine(q.Price, q.Change, q.ChangePercent, q.Currency, hint, q.IsIndex())
	lines := []string{header, priceLine}
	if badge := q.HeadlineSessionBadge(); badge != "" {
		lines[1] += "  ·  " + badge
	}
	if q.ShowRegularClose() {
		closeLine := "Close " + formatPriceChangeLine(q.RegularPrice, q.RegularChange, q.RegularChangePercent, q.Currency, hint, q.IsIndex())
		lines = append(lines, closeLine)
	}
	if label := q.SessionLabel(); label != "" && q.HeadlineSessionBadge() == "" {
		lines = append(lines, label)
	}
	return strings.Join(lines, "\n")
}

func formatPriceChangeLine(price, change, pct float64, currency string, hint int, index bool) string {
	priceText := formatMoneyText(price, currency, hint, index)
	changeText := formatMoneyText(change, currency, hint, index)
	if change > 0 {
		changeText = "+" + changeText
	}
	pctSign := "+"
	if pct < 0 {
		pctSign = ""
	}
	return fmt.Sprintf("%s  %s (%s%.2f%%)", priceText, changeText, pctSign, pct)
}

func formatMoneyText(amount float64, currency string, hint int, index bool) string {
	number := fmt.Sprintf("%.*f", hint, amount)
	if index {
		return number
	}
	switch strings.ToUpper(currency) {
	case "", "USD":
		if amount < 0 {
			return "-$" + fmt.Sprintf("%.*f", hint, -amount)
		}
		return "$" + number
	default:
		return number + " " + currency
	}
}

func unknownRangeReply(ticker, extra string) string {
	shown := strings.ReplaceAll(extra, "`", "'")
	if shown == "" {
		shown = extra
	}
	return fmt.Sprintf("I don't recognize `%s` as a time range. Valid ranges are %s — for example `$%s YTD`. Leave the range off for today's session.", shown, quoteRangeOptions, ticker)
}

func quoteImageFallback(quote *yahoo.Quote) string {
	return fmt.Sprintf("I fetched `$%s` but couldn't attach the quote card. Here's the latest:\n%s", quote.Symbol, formatQuoteText(quote))
}
