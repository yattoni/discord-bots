package maps

import "fmt"

type GoogleMaps struct {
	apiKey string
}

func NewGoogleMaps(apiKey string) *GoogleMaps {
	return &GoogleMaps{apiKey}
}

func (maps *GoogleMaps) FormatStaticMapUrl(xCoord, yCoord float64) string {
	return fmt.Sprintf("https://maps.googleapis.com/maps/api/staticmap?markers=%.4f,%.4f&zoom=2&size=400x250&key=%s", xCoord, yCoord, maps.apiKey)
}
