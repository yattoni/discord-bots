package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsHelpMention(t *testing.T) {
	const botID = "1234567890"

	cases := []struct {
		name       string
		botID      string
		content    string
		mentionIDs []string
		want       bool
	}{
		{
			name:       "mention then help",
			botID:      botID,
			content:    "<@" + botID + "> help",
			mentionIDs: []string{botID},
			want:       true,
		},
		{
			name:       "nickname mention then help",
			botID:      botID,
			content:    "<@!" + botID + "> help",
			mentionIDs: []string{botID},
			want:       true,
		},
		{
			name:       "help then mention",
			botID:      botID,
			content:    "help <@" + botID + ">",
			mentionIDs: []string{botID},
			want:       true,
		},
		{
			name:       "uppercase help",
			botID:      botID,
			content:    "<@" + botID + "> HELP",
			mentionIDs: []string{botID},
			want:       true,
		},
		{
			name:       "whitespace around help",
			botID:      botID,
			content:    "  <@" + botID + ">   help  \n",
			mentionIDs: []string{botID},
			want:       true,
		},
		{
			name:    "mention in content without mentions slice",
			botID:   botID,
			content: "<@" + botID + "> help",
			want:    true,
		},
		{
			name:       "help without mentioning the bot",
			botID:      botID,
			content:    "help",
			mentionIDs: []string{"999"},
			want:       false,
		},
		{
			name:       "mention without help",
			botID:      botID,
			content:    "<@" + botID + ">",
			mentionIDs: []string{botID},
			want:       false,
		},
		{
			name:       "mention with extra text",
			botID:      botID,
			content:    "<@" + botID + "> help please",
			mentionIDs: []string{botID},
			want:       false,
		},
		{
			name:       "mention plus ticker",
			botID:      botID,
			content:    "<@" + botID + "> $NOW",
			mentionIDs: []string{botID},
			want:       false,
		},
		{
			name:       "empty bot id",
			content:    "<@1234567890> help",
			mentionIDs: []string{"1234567890"},
			want:       false,
		},
		{
			name:       "mentions another user plus help",
			botID:      botID,
			content:    "<@999> help",
			mentionIDs: []string{"999"},
			want:       false,
		},
		{
			name:       "multiple mentions including bot and help",
			botID:      botID,
			content:    "<@999> <@" + botID + "> help",
			mentionIDs: []string{"999", botID},
			want:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsHelpMention(tc.botID, tc.content, tc.mentionIDs))
		})
	}
}

func TestHelpMessageDocumentsBitcoinSpot(t *testing.T) {
	assert.Contains(t, helpMessage, "$BTC")
	assert.Contains(t, helpMessage, "$BTC-USD")
	assert.Contains(t, helpMessage, "Bitcoin spot")
	assert.Contains(t, helpMessage, "$ETH-USD")
	assert.Contains(t, helpMessage, "Mention me with a question")
	assert.NotContains(t, helpMessage, "Grayscale")
}
