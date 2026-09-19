package buddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCloudflareAPI = "https://api.cloudflare.com/client/v4"
	// cacheTTL=0 disables Browser Run's default 5s cache so daily prices stay fresh.
	scrapePath = "/accounts/%s/browser-rendering/scrape?cacheTTL=0"

	labelSelector = `[class*="GasPriceCollection-module__fuelTypeDisplay"]`
	priceSelector = `[class*="FuelTypePriceDisplay-module__price"]`
	// Price nodes exist immediately with a spinner inside; wait until at least one
	// spinner is gone so we scrape rendered dollar amounts instead of loaders.
	priceReadySelector = `[class*="FuelTypePriceDisplay-module__price"]:not(:has([class*="loader"]))`
)

var (
	// ErrUnauthorized means the Cloudflare API token or account id was rejected.
	ErrUnauthorized = errors.New("cloudflare scrape unauthorized")
	// ErrRateLimited means Browser Run is throttling requests.
	ErrRateLimited = errors.New("cloudflare scrape rate limited")
	// ErrUnavailable means the scrape API could not be reached or failed.
	ErrUnavailable = errors.New("cloudflare scrape unavailable")
	// ErrEmpty means the page rendered but no fuel prices were present.
	ErrEmpty = errors.New("cloudflare scrape empty prices")
)

// Cloudflare calls Cloudflare Browser Run's /scrape Quick Action.
type Cloudflare struct {
	httpClient *http.Client
	accountID  string
	apiToken   string
	baseURL    string
}

// NewCloudflare builds a Browser Run client. accountID and apiToken come from
// a Cloudflare account with the Browser Rendering Edit permission.
func NewCloudflare(accountID, apiToken string) *Cloudflare {
	return &Cloudflare{
		httpClient: &http.Client{Timeout: 90 * time.Second},
		accountID:  strings.TrimSpace(accountID),
		apiToken:   strings.TrimSpace(apiToken),
		baseURL:    defaultCloudflareAPI,
	}
}

// NewCloudflareFromEnv reads CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN.
func NewCloudflareFromEnv() (*Cloudflare, error) {
	accountID := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	apiToken := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if accountID == "" || apiToken == "" {
		return nil, fmt.Errorf("%w: CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN are required", ErrUnauthorized)
	}
	return NewCloudflare(accountID, apiToken), nil
}

type scrapeRequest struct {
	URL                 string             `json:"url"`
	Elements            []scrapeSelector   `json:"elements"`
	GotoOptions         scrapeGotoOptions  `json:"gotoOptions"`
	WaitForSelector     scrapeWaitSelector `json:"waitForSelector"`
	BestAttempt         bool               `json:"bestAttempt"`
	ActionTimeout       int                `json:"actionTimeout"`
	RejectResourceTypes []string           `json:"rejectResourceTypes"`
}

type scrapeSelector struct {
	Selector string `json:"selector"`
}

type scrapeGotoOptions struct {
	WaitUntil string `json:"waitUntil"`
	Timeout   int    `json:"timeout"`
}

type scrapeWaitSelector struct {
	Selector string `json:"selector"`
	Visible  bool   `json:"visible"`
	Timeout  int    `json:"timeout"`
}

type scrapeAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type scrapeEnvelope struct {
	Success bool             `json:"success"`
	Errors  []scrapeAPIError `json:"errors"`
	Result  []selectorResult `json:"result"`
}

// selectorResult is one requested CSS selector plus every matching element.
type selectorResult struct {
	Selector string      `json:"selector"`
	Results  elementHits `json:"results"`
}

type elementHit struct {
	Text string `json:"text"`
	HTML string `json:"html"`
}

// elementHits accepts both the documented array shape and the OpenAPI example
// that serializes a single match as an object.
type elementHits []elementHit

func (h *elementHits) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*h = nil
		return nil
	}
	if data[0] == '[' {
		var hits []elementHit
		if err := json.Unmarshal(data, &hits); err != nil {
			return err
		}
		*h = hits
		return nil
	}
	var one elementHit
	if err := json.Unmarshal(data, &one); err != nil {
		return err
	}
	*h = elementHits{one}
	return nil
}

func (c *Cloudflare) scrape(ctx context.Context, pageURL string) ([]selectorResult, error) {
	if c == nil || c.accountID == "" || c.apiToken == "" {
		return nil, fmt.Errorf("%w: missing API token", ErrUnauthorized)
	}

	body, err := json.Marshal(scrapeRequest{
		URL: pageURL,
		Elements: []scrapeSelector{
			{Selector: labelSelector},
			{Selector: priceSelector},
		},
		GotoOptions: scrapeGotoOptions{
			WaitUntil: "networkidle2",
			Timeout:   45000,
		},
		WaitForSelector: scrapeWaitSelector{
			Selector: priceReadySelector,
			Visible:  true,
			Timeout:  30000,
		},
		BestAttempt:   true,
		ActionTimeout: 60000,
		// Skip heavy assets so networkidle2 can fire after the price XHR, not ads.
		RejectResourceTypes: []string{"image", "media", "font", "stylesheet"},
	})
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(c.baseURL, "/") + fmt.Sprintf(scrapePath, c.accountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrUnavailable, err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, rateLimitError(resp, raw)
	}

	var parsed scrapeEnvelope
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, classifyScrapeStatus(resp.StatusCode, fmt.Sprintf("decode response: %v", err))
	}
	if !parsed.Success || len(parsed.Errors) > 0 {
		msg := strings.TrimSpace(joinScrapeErrors(parsed.Errors))
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		status := resp.StatusCode
		if status < 400 && len(parsed.Errors) > 0 && parsed.Errors[0].Code != 0 {
			status = parsed.Errors[0].Code
		}
		return nil, classifyScrapeStatus(status, msg)
	}
	if resp.StatusCode >= 400 {
		return nil, classifyScrapeStatus(resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return parsed.Result, nil
}

func rateLimitError(resp *http.Response, raw []byte) error {
	retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After"))
	msg := strings.TrimSpace(string(raw))
	if retryAfter != "" {
		if msg == "" {
			msg = "retry after " + retryAfter + "s"
		} else {
			msg += "; retry after " + retryAfter + "s"
		}
		if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
			return fmt.Errorf("%w: %s", ErrRateLimited, msg)
		}
	}
	if msg == "" {
		return ErrRateLimited
	}
	return fmt.Errorf("%w: %s", ErrRateLimited, msg)
}

func joinScrapeErrors(errs []scrapeAPIError) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if msg := strings.TrimSpace(err.Message); msg != "" {
			parts = append(parts, msg)
		}
	}
	return strings.Join(parts, "; ")
}

func classifyScrapeStatus(status int, detail string) error {
	detail = strings.TrimSpace(detail)
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		if detail == "" {
			return ErrUnauthorized
		}
		return fmt.Errorf("%w: %s", ErrUnauthorized, detail)
	case http.StatusTooManyRequests:
		if detail == "" {
			return ErrRateLimited
		}
		return fmt.Errorf("%w: %s", ErrRateLimited, detail)
	default:
		if status >= 500 || status == 0 {
			if detail == "" {
				return ErrUnavailable
			}
			return fmt.Errorf("%w: %s", ErrUnavailable, detail)
		}
		if detail == "" {
			return fmt.Errorf("%w: unexpected status %d", ErrUnavailable, status)
		}
		return fmt.Errorf("%w: %s", ErrUnavailable, detail)
	}
}
