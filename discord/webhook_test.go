package discord

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendMessage(t *testing.T) {
	url := os.Getenv("WEBHOOK_URL")
	webhook := NewWebhook(url)

	webhook.SendMessage("Hello, World!")

	assert.Equal(t, 1, 2)
}
