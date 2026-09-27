package buddy

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var priceToken = regexp.MustCompile(`\$\d+\.\d{2,3}|\d+\.\d{2,3}`)

type stationPrices struct {
	Regular string
	Mid     string
	Premium string
}

// GetFromGasBuddy loads Regular, Mid, and Premium from GasBuddy GraphQL.
func GetFromGasBuddy(ctx context.Context, client *Client, pageURL, city string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("%w: missing client", ErrUnavailable)
	}
	prices, err := client.stationPrices(ctx, pageURL)
	if err != nil {
		return "", err
	}
	return formatStation(city, prices), nil
}

func formatStation(city string, prices stationPrices) string {
	return fmt.Sprintf("%s:\n\tRegular: %s\n\tMid: %s\n\tPremium: %s", city, prices.Regular, prices.Mid, prices.Premium)
}

func assignGrade(prices *stationPrices, product, label, value string) {
	key := normalizeGrade(product)
	if key == "" {
		key = normalizeGrade(label)
	}
	switch {
	case strings.Contains(key, "regular") || strings.Contains(normalizeGrade(label), "regular"):
		prices.Regular = value
	case strings.Contains(key, "mid") || strings.Contains(normalizeGrade(label), "mid"):
		prices.Mid = value
	case strings.Contains(key, "premium") || strings.Contains(normalizeGrade(label), "premium"):
		prices.Premium = value
	}
}

func normalizeGrade(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.ReplaceAll(label, " ", "")
	return label
}

func normalizePrice(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if match := priceToken.FindString(text); match != "" {
		if strings.HasPrefix(match, "$") {
			return match
		}
		return "$" + match
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "loader") || strings.Contains(lower, "- - -") {
		return ""
	}
	return ""
}
