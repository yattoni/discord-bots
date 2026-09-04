package main

import (
	"regexp"
	"strings"
)

var mentionPattern = regexp.MustCompile(`<@!?\d+>`)

const helpMessage = `I watch for messages that are **only** a stock ticker and reply with a Yahoo Finance quote card.

**How to use me**
- Send a ticker by itself: ` + "`$NOW`" + `, ` + "`$AAPL`" + `, or ` + "`$BRK.B`" + `
- Crypto spot pairs work too: ` + "`$BTC`" + `, ` + "`$BTC-USD`" + `, ` + "`$ETH-USD`" + `
- Add a range after the ticker: ` + "`$NOW YTD`" + `, ` + "`$AAPL 1M`" + `, or ` + "`$BTC 5D`" + `
- Ranges: ` + "`5D`" + `, ` + "`1M`" + `, ` + "`3M`" + `, ` + "`6M`" + `, ` + "`1Y`" + `, ` + "`YTD`" + ` (case insensitive)
- Without a range I'll show today's price, today's dollar and percent change, and a 1-minute chart (premarket, regular hours, and after hours for stocks; 24h for crypto)
- With a range I'll show the current price, change over that window, and a chart of the range
- Tickers are 1–5 letters, with an optional share class like ` + "`$BRK.B`" + ` or a quote currency like ` + "`$BTC-USD`" + `
- ` + "`$BTC`" + ` is Bitcoin spot (same as ` + "`$BTC-USD`" + `)
- Extra text around the ticker is not allowed — the message must be just the ticker, or the ticker plus a range (whitespace is fine)
- If I can't find a ticker I'll say so. If Yahoo is down or the card fails to send, I'll ask you to try again (or send a text quote)

Mention me with a question and I'll reply. Mention me with ` + "`help`" + ` to see this again.`

// IsHelpMention reports whether the message @mentions the bot and otherwise says "help".
func IsHelpMention(botID, content string, mentionedUserIDs []string) bool {
	if botID == "" || !mentionsBot(botID, content, mentionedUserIDs) {
		return false
	}
	stripped := mentionPattern.ReplaceAllString(content, " ")
	return strings.EqualFold(strings.TrimSpace(stripped), "help")
}

func mentionsBot(botID, content string, mentionedUserIDs []string) bool {
	for _, id := range mentionedUserIDs {
		if id == botID {
			return true
		}
	}
	return strings.Contains(content, "<@"+botID+">") || strings.Contains(content, "<@!"+botID+">")
}
