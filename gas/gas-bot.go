package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/yattoni/discord-bots/discord"
	"github.com/yattoni/discord-bots/gas/aaa"
	"github.com/yattoni/discord-bots/gas/buddy"
)

func runOnce() {
	ctx := context.Background()
	scraper, err := buddy.NewCloudflareFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	national := aaa.GetNationalAverages()
	la, err := buddy.GetFromGasBuddy(ctx, scraper, "https://www.gasbuddy.com/station/10870", "Los Angeles")
	if err != nil {
		log.Fatal(err)
	}
	chicago, err := buddy.GetFromGasBuddy(ctx, scraper, "https://www.gasbuddy.com/station/5355", "Chicago")
	if err != nil {
		log.Fatal(err)
	}
	// casesys := getGasBuddy("https://www.gasbuddy.com/station/145394", "At Casey's in Jacksonville")
	stl, err := buddy.GetFromGasBuddy(ctx, scraper, "https://www.gasbuddy.com/station/14993", "At Jones's QT")
	if err != nil {
		log.Fatal(err)
	}
	message := fmt.Sprintf("%s\n%s\n%s\n%s", national, la, chicago, stl)
	webhookURL := strings.TrimSpace(os.Getenv("WEBHOOK_URL"))
	if webhookURL == "" {
		fmt.Println(message)
		return
	}
	webhook := discord.NewWebhook(webhookURL)
	webhook.SendMessage(message)
}

func main() {
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" {
		runOnce()
		return
	}
	lambda.Start(runOnce)
}
