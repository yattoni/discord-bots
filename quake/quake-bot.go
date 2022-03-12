package main

import (
	"log"
	"os"
	"time"

	"github.com/yattoni/discord-bots/discord"
	"github.com/yattoni/discord-bots/maps"
	"github.com/yattoni/discord-bots/quake/usgs"
)

func runOnce() {
	now := time.Now().UTC()
	startOfLastHour := now.Add(-1 * time.Hour).Truncate(time.Hour)
	endOfLastHour := startOfLastHour.Add(1 * time.Hour)
	queryResults := usgs.NewClient().FetchQuakes(startOfLastHour, endOfLastHour)

	webhook := discord.NewWebhook(os.Getenv("WEBHOOK_URL"))
	googleMaps := maps.NewGoogleMaps(os.Getenv("GOOGLE_MAPS_API_KEY"))

	log.Println("Found ", len(queryResults.Features), " quakes")
	for _, feature := range queryResults.Features {
		mapUrl := googleMaps.FormatStaticMapUrl(feature.Geometry.Coordinates[1], feature.Geometry.Coordinates[0])
		if feature.Properties.Type != "earthquake" {
			log.Printf("Sending message for %s %s with magnitude %f\n", feature.Properties.Type, feature.ID, feature.Properties.Mag)
			webhook.SendMessage(feature.String())
			webhook.SendMessage(mapUrl)
		} else if feature.Properties.Mag >= 5.0 {
			log.Printf("Sending message for earthquake %s with magnitude %f\n", feature.ID, feature.Properties.Mag)
			webhook.SendMessage(feature.String())
			webhook.SendMessage(mapUrl)
		} else {
			log.Printf("Not sending %s %s with magnitude %f\n", feature.Properties.Type, feature.ID, feature.Properties.Mag)
		}
	}
}

func main() {
	runOnce()
	// lambda.Start(runOnce)
}
