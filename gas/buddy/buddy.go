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

// GetFromGasBuddy renders a GasBuddy station page with Cloudflare Browser Run
// /scrape and formats Regular, Mid, and Premium prices.
func GetFromGasBuddy(ctx context.Context, client *Cloudflare, pageURL, city string) (string, error) {
	results, err := client.scrape(ctx, pageURL)
	if err != nil {
		return "", fmt.Errorf("gasbuddy %s: %w", pageURL, err)
	}
	prices, err := parseStationPrices(results)
	if err != nil {
		return "", fmt.Errorf("gasbuddy %s: %w", pageURL, err)
	}
	return formatStation(city, prices), nil
}

func formatStation(city string, prices stationPrices) string {
	return fmt.Sprintf("%s:\n\tRegular: %s\n\tMid: %s\n\tPremium: %s", city, prices.Regular, prices.Mid, prices.Premium)
}

func parseStationPrices(results []selectorResult) (stationPrices, error) {
	var labels, prices []string
	for _, result := range results {
		switch classifyResult(result) {
		case "labels":
			labels = textsFor(result)
		case "prices":
			prices = textsFor(result)
		}
	}
	if len(prices) == 0 && len(results) == 1 {
		prices = textsFor(results[0])
	}

	if looksLikeChallenge(labels, prices) {
		return stationPrices{}, fmt.Errorf("%w: got a Cloudflare challenge page instead of station prices", ErrUnavailable)
	}

	parsed := stationPrices{}
	if len(labels) > 0 {
		limit := len(prices)
		if len(labels) < limit {
			limit = len(labels)
		}
		for i := 0; i < limit; i++ {
			assignGrade(&parsed, labels[i], prices[i])
		}
	} else if len(prices) >= 3 {
		parsed.Regular = normalizePrice(prices[0])
		parsed.Mid = normalizePrice(prices[1])
		parsed.Premium = normalizePrice(prices[2])
	} else if len(prices) > 0 {
		parsed.Regular = normalizePrice(prices[0])
		if len(prices) > 1 {
			parsed.Mid = normalizePrice(prices[1])
		}
		if len(prices) > 2 {
			parsed.Premium = normalizePrice(prices[2])
		}
	}

	if parsed.Regular == "" && parsed.Mid == "" && parsed.Premium == "" {
		return stationPrices{}, ErrEmpty
	}
	return parsed, nil
}

func assignGrade(prices *stationPrices, label, value string) {
	switch normalizeGrade(label) {
	case "regular":
		prices.Regular = normalizePrice(value)
	case "mid", "midgrade":
		prices.Mid = normalizePrice(value)
	case "premium":
		prices.Premium = normalizePrice(value)
	}
}

func textsFor(result selectorResult) []string {
	out := make([]string, 0, len(result.Results))
	for _, hit := range result.Results {
		text := strings.TrimSpace(hit.Text)
		if text == "" {
			text = strings.TrimSpace(hit.HTML)
		}
		out = append(out, text)
	}
	return out
}

func classifyResult(result selectorResult) string {
	sel := result.Selector
	if selectorMatches(sel, labelSelector) || strings.Contains(sel, "fuelTypeDisplay") {
		return "labels"
	}
	if selectorMatches(sel, priceSelector) || strings.Contains(sel, "FuelTypePriceDisplay") {
		return "prices"
	}
	texts := textsFor(result)
	if len(texts) == 0 {
		return ""
	}
	if normalizeGrade(texts[0]) == "regular" {
		return "labels"
	}
	if priceToken.MatchString(texts[0]) {
		return "prices"
	}
	return ""
}

func selectorMatches(got, want string) bool {
	return strings.TrimSpace(got) == strings.TrimSpace(want)
}

func normalizeGrade(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.ReplaceAll(label, " ", "")
	return label
}

func looksLikeChallenge(groups ...[]string) bool {
	for _, group := range groups {
		for _, text := range group {
			lower := strings.ToLower(text)
			if strings.Contains(lower, "just a moment") || strings.Contains(lower, "attention required") {
				return true
			}
		}
	}
	return false
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
	if strings.Contains(lower, "loader") {
		return ""
	}
	return text
}
