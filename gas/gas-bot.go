package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/yattoni/discord-bots/discord"
	"github.com/yattoni/discord-bots/gas/aaa"
	"github.com/yattoni/discord-bots/gas/buddy"
)

func runOnce() {
	national := aaa.GetNationalAverages()
	la := buddy.GetFromGasBuddy("https://www.gasbuddy.com/station/10870", "Los Angeles")
	chicago := buddy.GetFromGasBuddy("https://www.gasbuddy.com/station/5355", "Chicago")
	// casesys := getGasBuddy("https://www.gasbuddy.com/station/145394", "At Casey's in Jacksonville")
	stl := buddy.GetFromGasBuddy("https://www.gasbuddy.com/station/14993", "At Jones's QT")
	webhook := discord.NewWebhook(os.Getenv("WEBHOOK_URL"))
	webhook.SendMessage(fmt.Sprintf("%s\n%s\n%s\n%s", national, la, chicago, stl))
}

func main() {
	// runOnce()
	lambda.Start(runOnce)
}
