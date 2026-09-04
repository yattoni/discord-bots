package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/yattoni/discord-bots/stock/openrouter"
)

func TestMentionsBot(t *testing.T) {
	const botID = "1234567890"

	cases := []struct {
		name       string
		botID      string
		content    string
		mentionIDs []string
		want       bool
	}{
		{
			name:       "mentioned in slice",
			botID:      botID,
			content:    "what is a pe ratio",
			mentionIDs: []string{botID},
			want:       true,
		},
		{
			name:    "mention token in content",
			botID:   botID,
			content: "<@" + botID + "> hello",
			want:    true,
		},
		{
			name:    "nickname mention token",
			botID:   botID,
			content: "<@!" + botID + ">",
			want:    true,
		},
		{
			name:       "other user only",
			botID:      botID,
			content:    "<@999> hello",
			mentionIDs: []string{"999"},
			want:       false,
		},
		{
			name:       "empty bot id",
			content:    "<@1234567890> hello",
			mentionIDs: []string{"1234567890"},
			want:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MentionsBot(tc.botID, tc.content, tc.mentionIDs))
		})
	}
}

func TestMentionPrompt(t *testing.T) {
	const botID = "1234567890"

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "mention then question",
			content: "<@" + botID + "> what is a P/E ratio?",
			want:    "what is a P/E ratio?",
		},
		{
			name:    "nickname mention and extra spaces",
			content: "  <@!" + botID + ">   hello   there  ",
			want:    "hello there",
		},
		{
			name:    "mention only",
			content: "<@" + botID + ">",
			want:    "",
		},
		{
			name:    "multiple mentions",
			content: "<@999> <@" + botID + "> compare $AAPL and $MSFT",
			want:    "compare $AAPL and $MSFT",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MentionPrompt(tc.content))
		})
	}
}

func TestClipDiscordMessage(t *testing.T) {
	assert.Equal(t, "short", clipDiscordMessage("  short  "))
	assert.Equal(t, "", clipDiscordMessage("   "))

	long := strings.Repeat("a", discordMessageLimit+25)
	got := clipDiscordMessage(long)
	assert.Equal(t, discordMessageLimit, utf8.RuneCountInString(got))
	assert.True(t, strings.HasSuffix(got, "…"))
	assert.Equal(t, strings.Repeat("a", discordMessageLimit-1)+"…", got)
}

func TestMentionSystemPromptAllowsGeneralQuestions(t *testing.T) {
	p := strings.ToLower(mentionSystemPrompt)
	assert.Contains(t, p, "general questions")
	assert.Contains(t, p, "do not refuse")
	assert.Contains(t, p, "unrelated to stocks")
	assert.Contains(t, mentionSystemPrompt, "Do not invent live prices")
	assert.Contains(t, mentionSystemPrompt, "$AAPL")
	assert.Contains(t, mentionSystemPrompt, "$BTC")
}

func TestMentionSystemPromptIsJiangFromTheBigShort(t *testing.T) {
	assert.Contains(t, mentionSystemPrompt, `You are Jiang`)
	assert.Contains(t, mentionSystemPrompt, "The Big Short")
	assert.Contains(t, mentionSystemPrompt, `his name is Yang`)
	assert.Contains(t, mentionSystemPrompt, `Actually, my name's "Jiang", and I do speak English.`)
	assert.Contains(t, mentionSystemPrompt, `I placed 2nd in that national math competition.`)
	assert.Contains(t, mentionSystemPrompt, "quote it exactly")
}

func TestMentionErrorReply(t *testing.T) {
	assert.Contains(t, mentionErrorReply(nil), "didn't have anything")
	assert.Contains(t, mentionErrorReply(openrouter.ErrEmpty), "didn't have anything")
	assert.Contains(t, mentionErrorReply(openrouter.ErrUnauthorized), "aren't configured")
	assert.Contains(t, mentionErrorReply(fmt.Errorf("%w: invalid key", openrouter.ErrUnauthorized)), "aren't configured")
	assert.Contains(t, mentionErrorReply(openrouter.ErrRateLimited), "too many chat requests")
	assert.Contains(t, mentionErrorReply(openrouter.ErrUnavailable), "couldn't get a chat reply")
	assert.NotContains(t, mentionErrorReply(fmt.Errorf("%w: boom", openrouter.ErrUnavailable)), "boom")
}
