package main

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/yattoni/discord-bots/stock/openrouter"
)

const (
	discordMessageLimit = 2000
	mentionSystemPrompt = `You are the stock quote Discord bot in this server. Users @mention you to ask a question.

Reply helpfully and concisely in Discord markdown (bold, italics, inline code). Stay under 1800 characters. Do not use tables or headings.

You can explain tickers, markets, and how this bot works. Do not invent live prices, charts, or news — tell people to send a ticker by itself (like $AAPL or $BTC) for a quote card. If they only mentioned you, greet them briefly and say they can ask a question or send a ticker.`
)

// MentionsBot reports whether the message @mentions the bot.
func MentionsBot(botID, content string, mentionedUserIDs []string) bool {
	return mentionsBot(botID, content, mentionedUserIDs)
}

// MentionPrompt returns the message text with Discord mention tokens removed.
func MentionPrompt(content string) string {
	stripped := mentionPattern.ReplaceAllString(content, " ")
	return strings.TrimSpace(strings.Join(strings.Fields(stripped), " "))
}

func clipDiscordMessage(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) <= discordMessageLimit {
		return s
	}
	runes := []rune(s)
	const ellipsis = "…"
	keep := discordMessageLimit - utf8.RuneCountInString(ellipsis)
	if keep < 1 {
		return ellipsis
	}
	return string(runes[:keep]) + ellipsis
}

func mentionErrorReply(err error) string {
	switch {
	case err == nil, errors.Is(err, openrouter.ErrEmpty):
		return "I didn't have anything to say. Try asking again."
	case errors.Is(err, openrouter.ErrUnauthorized):
		return "Chat replies aren't configured right now. Send a ticker like `$AAPL` or mention me with `help`."
	case errors.Is(err, openrouter.ErrRateLimited):
		return "I'm getting too many chat requests. Try again in a bit, or send a ticker like `$AAPL`."
	default:
		return "I couldn't get a chat reply just now. Try again in a bit, or send a ticker like `$AAPL`."
	}
}
