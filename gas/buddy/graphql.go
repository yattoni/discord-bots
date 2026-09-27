package buddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultGasBuddyOrigin = "https://www.gasbuddy.com"
	defaultGraphQLPath    = "/graphql"
	userAgent             = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	stationQuery          = `query GetStation($id: ID!) {
  station(id: $id) {
    id
    name
    prices {
      fuelProduct
      longName
      credit { formattedPrice price }
      cash { formattedPrice price }
    }
  }
}`
)

var (
	// ErrUnauthorized means GasBuddy rejected the request (missing CSRF or cookies).
	ErrUnauthorized = errors.New("gasbuddy unauthorized")
	// ErrRateLimited means GasBuddy is throttling requests.
	ErrRateLimited = errors.New("gasbuddy rate limited")
	// ErrUnavailable means the page or GraphQL API could not be reached or failed.
	ErrUnavailable = errors.New("gasbuddy unavailable")
	// ErrEmpty means the station rendered but no fuel prices were present.
	ErrEmpty = errors.New("gasbuddy empty prices")

	csrfPattern    = regexp.MustCompile(`window\.gbcsrf\s*=\s*"([^"]+)"`)
	stationIDPath  = regexp.MustCompile(`(?i)/station/(\d+)`)
	csrfErrorToken = regexp.MustCompile(`(?i)csrf|token`)
)

// Client fetches station prices from GasBuddy's GraphQL API after
// bootstrapping the gbcsrf cookie from a station page GET.
type Client struct {
	http     *http.Client
	origin   string
	graphql  string
	csrf     string
	mu       sync.Mutex
	maxBytes int64
}

// NewClient builds a GasBuddy client with a cookie jar for CSRF.
func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		http:     &http.Client{Timeout: 30 * time.Second, Jar: jar},
		origin:   defaultGasBuddyOrigin,
		maxBytes: 4 << 20,
	}, nil
}

type gqlRequest struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
	Query         string         `json:"query"`
}

type gqlEnvelope struct {
	Data struct {
		Station *gqlStation `json:"station"`
	} `json:"data"`
	Errors []gqlError `json:"errors"`
}

type gqlError struct {
	Message string `json:"message"`
}

type gqlStation struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Prices []gqlPrice `json:"prices"`
}

type gqlPrice struct {
	FuelProduct string    `json:"fuelProduct"`
	LongName    string    `json:"longName"`
	Credit      *gqlOffer `json:"credit"`
	Cash        *gqlOffer `json:"cash"`
}

type gqlOffer struct {
	FormattedPrice string  `json:"formattedPrice"`
	Price          float64 `json:"price"`
}

func (c *Client) graphqlURL() string {
	if c != nil && strings.TrimSpace(c.graphql) != "" {
		return strings.TrimSpace(c.graphql)
	}
	return strings.TrimRight(c.siteOrigin(), "/") + defaultGraphQLPath
}

func (c *Client) siteOrigin() string {
	if c != nil && strings.TrimSpace(c.origin) != "" {
		return strings.TrimRight(strings.TrimSpace(c.origin), "/")
	}
	return defaultGasBuddyOrigin
}

func (c *Client) limit() int64 {
	if c != nil && c.maxBytes > 0 {
		return c.maxBytes
	}
	return 4 << 20
}

func stationIDFromURL(pageURL string) (string, error) {
	if match := stationIDPath.FindStringSubmatch(pageURL); len(match) == 2 {
		return match[1], nil
	}
	return "", fmt.Errorf("%w: no station id in %s", ErrUnavailable, pageURL)
}

func (c *Client) ensureCSRF(ctx context.Context, pageURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(c.csrf) != "" {
		return nil
	}
	csrf, err := c.fetchCSRF(ctx, pageURL)
	if err != nil {
		return err
	}
	c.csrf = csrf
	return nil
}

func (c *Client) clearCSRF() {
	c.mu.Lock()
	c.csrf = ""
	c.mu.Unlock()
}

func (c *Client) currentCSRF() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.csrf
}

func (c *Client) fetchCSRF(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.limit()))
	if err != nil {
		return "", fmt.Errorf("%w: read station page: %v", ErrUnavailable, err)
	}
	if looksLikeChallengeHTML(raw) || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("%w: got a Cloudflare challenge page instead of a station page", ErrUnavailable)
	}
	if resp.StatusCode >= 400 {
		return "", classifyStatus(resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	match := csrfPattern.FindSubmatch(raw)
	if len(match) != 2 {
		return "", fmt.Errorf("%w: station page missing gbcsrf", ErrUnavailable)
	}
	return string(match[1]), nil
}

func (c *Client) stationPrices(ctx context.Context, pageURL string) (stationPrices, error) {
	id, err := stationIDFromURL(pageURL)
	if err != nil {
		return stationPrices{}, err
	}
	if err := c.ensureCSRF(ctx, pageURL); err != nil {
		return stationPrices{}, fmt.Errorf("gasbuddy %s: %w", pageURL, err)
	}

	prices, err := c.queryStation(ctx, pageURL, id)
	if err != nil && shouldRefreshCSRF(err) {
		c.clearCSRF()
		if refreshErr := c.ensureCSRF(ctx, pageURL); refreshErr != nil {
			return stationPrices{}, fmt.Errorf("gasbuddy %s: %w", pageURL, refreshErr)
		}
		prices, err = c.queryStation(ctx, pageURL, id)
	}
	if err != nil {
		return stationPrices{}, fmt.Errorf("gasbuddy %s: %w", pageURL, err)
	}
	return prices, nil
}

func shouldRefreshCSRF(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrUnauthorized) {
		return true
	}
	return csrfErrorToken.MatchString(err.Error())
}

func (c *Client) queryStation(ctx context.Context, pageURL, id string) (stationPrices, error) {
	body, err := json.Marshal(gqlRequest{
		OperationName: "GetStation",
		Variables:     map[string]any{"id": id},
		Query:         stationQuery,
	})
	if err != nil {
		return stationPrices{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL(), bytes.NewReader(body))
	if err != nil {
		return stationPrices{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", originFromPageURL(pageURL))
	req.Header.Set("Referer", pageURL)
	req.Header.Set("apollo-require-preflight", "true")
	if csrf := c.currentCSRF(); csrf != "" {
		req.Header.Set("gbcsrf", csrf)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return stationPrices{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.limit()))
	if err != nil {
		return stationPrices{}, fmt.Errorf("%w: read graphql: %v", ErrUnavailable, err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return stationPrices{}, fmt.Errorf("%w: %s", ErrRateLimited, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return stationPrices{}, classifyStatus(resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed gqlEnvelope
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return stationPrices{}, classifyStatus(resp.StatusCode, fmt.Sprintf("decode graphql: %v", err))
	}
	if msg := joinGQLErrors(parsed.Errors); msg != "" {
		status := resp.StatusCode
		if status < 400 {
			status = http.StatusBadRequest
		}
		return stationPrices{}, classifyStatus(status, msg)
	}
	if resp.StatusCode >= 400 {
		return stationPrices{}, classifyStatus(resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if parsed.Data.Station == nil {
		return stationPrices{}, ErrEmpty
	}
	return pricesFromStation(parsed.Data.Station)
}

func pricesFromStation(station *gqlStation) (stationPrices, error) {
	parsed := stationPrices{}
	for _, price := range station.Prices {
		value := offerPrice(price.Credit)
		if value == "" {
			value = offerPrice(price.Cash)
		}
		if value == "" {
			continue
		}
		assignGrade(&parsed, price.FuelProduct, price.LongName, value)
	}
	if parsed.Regular == "" && parsed.Mid == "" && parsed.Premium == "" {
		return stationPrices{}, ErrEmpty
	}
	return parsed, nil
}

func offerPrice(offer *gqlOffer) string {
	if offer == nil {
		return ""
	}
	if p := normalizePrice(offer.FormattedPrice); p != "" {
		return p
	}
	if offer.Price > 0 {
		return normalizePrice(fmt.Sprintf("%.3f", offer.Price))
	}
	return ""
}

func joinGQLErrors(errs []gqlError) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if msg := strings.TrimSpace(err.Message); msg != "" {
			parts = append(parts, msg)
		}
	}
	return strings.Join(parts, "; ")
}

func classifyStatus(status int, detail string) error {
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
		if csrfErrorToken.MatchString(detail) {
			if detail == "" {
				return ErrUnauthorized
			}
			return fmt.Errorf("%w: %s", ErrUnauthorized, detail)
		}
		if detail == "" {
			return fmt.Errorf("%w: unexpected status %d", ErrUnavailable, status)
		}
		return fmt.Errorf("%w: %s", ErrUnavailable, detail)
	}
}

func looksLikeChallengeHTML(raw []byte) bool {
	lower := strings.ToLower(string(raw))
	return strings.Contains(lower, "just a moment") ||
		strings.Contains(lower, "attention required") ||
		strings.Contains(lower, "sorry, you have been blocked")
}

func originFromPageURL(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return defaultGasBuddyOrigin
	}
	return u.Scheme + "://" + u.Host
}
