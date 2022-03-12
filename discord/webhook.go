package discord

import (
	"log"
	"net/url"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type Webhook struct {
	url     *url.URL
	session *discordgo.Session
}

func NewWebhook(urlString string) *Webhook {
	u, err := url.Parse(urlString)
	if err != nil {
		log.Fatal("Error parsing url: ", urlString, err)
	}

	session, err := discordgo.New()
	if err != nil {
		log.Fatal("Error creating Discord session,", err)
	}
	return &Webhook{u, session}
}

func (webhook *Webhook) SendMessage(message string) {
	log.Println(webhook.url.Path)
	pathSegments := strings.Split(webhook.url.Path, "/")

	if len(pathSegments) != 5 {
		log.Fatal("Not 5 path segments in url: ", pathSegments)
	}

	webhookId := pathSegments[3]
	webhookToken := pathSegments[4]

	log.Println("Id: ", webhookId)
	log.Println("Token: ", webhookToken)
	log.Println("Message:\n", message)

	_, err := webhook.session.WebhookExecute(webhookId, webhookToken, true, &discordgo.WebhookParams{
		Content: message,
	})

	if err != nil {
		log.Fatal("Error sending message", err)
	}
}
