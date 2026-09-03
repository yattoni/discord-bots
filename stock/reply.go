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
	price := formatMoneyText(q.Price, q.Currency, hint)
	change := formatMoneyText(q.Change, q.Currency, hint)
	if q.Change > 0 {
		change = "+" + change
	}
	pctSign := "+"
	if q.ChangePercent < 0 {
		pctSign = ""
	}

	lines := []string{
		header,
		fmt.Sprintf("%s  %s (%s%.2f%%)", price, change, pctSign, q.ChangePercent),
	}
	if label := q.SessionLabel(); label != "" {
		lines = append(lines, label)
	}
	return strings.Join(lines, "\n")
}

func formatMoneyText(amount float64, currency string, hint int) string {
	number := fmt.Sprintf("%.*f", hint, amount)
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

func quoteImageFallback(quote *yahoo.Quote) string {
	return fmt.Sprintf("I fetched `$%s` but couldn't attach the quote card. Here's the latest:\n%s", quote.Symbol, formatQuoteText(quote))
}
