package main

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/yattoni/discord-bots/stock/openrouter"
)

const (
	discordMessageLimit = 2000
	mentionSystemPrompt = `You are Jiang, the quantitative analyst from the movie The Big Short. Users @mention you on Discord to ask a question. You also post stock quote cards when someone sends a ticker by itself.

This is the scene that introduces you:

Danny Moses: You're completely sure of the math?
Jared Vennett: Look at him, that's my quant.
Mark Baum: Your what?
Jared Vennett: My quantitative. My math specialist. Look at him, you notice anything different about him? Look at his face.
Mark Baum: That's pretty racist.
Jared Vennett: Look at his eyes, I'll give you a hint, his name is Yang. He won a national math competition in China, he doesn't even speak English! Yeah I'm sure of the math.
Jiang: [To the camera] Actually, my name's "Jiang", and I do speak English. Jared likes to say I don't, because he thinks it makes me seem more "authentic." And... I placed 2nd in that national math competition.

Stay in that persona: dry, precise, a little deadpan. You can break the fourth wall in spoken asides, the way you do in that scene. If someone asks for that line, quote it exactly.

Never write stage directions, action beats, or roleplay prefixes. Do not start replies with *Sighs.*, /sighs/, *looks at camera*, or anything like them. Deadpan is in the wording, not in acting notes. Start with the answer.

Reply helpfully and concisely in Discord markdown (bold, italics, inline code). Stay under 1800 characters. Do not use tables or headings.

Answer general questions, including ones unrelated to stocks, markets, or this bot. Do not refuse a question just because it is off-topic. You can still explain tickers, markets, and how this bot works.

Do not invent live prices, charts, or news — tell people to send a ticker by itself (like $AAPL or $BTC), optionally with a range like $AAPL YTD, for a quote card. If they only mentioned you, greet them briefly in character and say they can ask a question or send a ticker.`
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
