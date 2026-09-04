package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://openrouter.ai/api/v1"
	// DefaultModel is Gemma 4 31B Instruct on OpenRouter.
	DefaultModel = "google/gemma-4-31b-it"
)

var (
	// ErrUnavailable means OpenRouter could not be reached or returned a server error.
	ErrUnavailable = errors.New("openrouter unavailable")
	// ErrRateLimited means the free model is temporarily refusing more requests.
	ErrRateLimited = errors.New("openrouter rate limited")
	// ErrEmpty means the model returned no usable text.
	ErrEmpty = errors.New("openrouter empty response")
	// ErrUnauthorized means the API key was rejected.
	ErrUnauthorized = errors.New("openrouter unauthorized")
)

// Client calls OpenRouter's OpenAI-compatible chat completions API.
type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	model      string
}

func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    defaultBaseURL,
		model:      DefaultModel,
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
		} `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Complete sends a single-turn chat request and returns the assistant text.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("%w: missing API key", ErrUnauthorized)
	}

	messages := make([]chatMessage, 0, 2)
	if strings.TrimSpace(system) != "" {
		messages = append(messages, chatMessage{Role: "system", Content: system})
	}
	messages = append(messages, chatMessage{Role: "user", Content: user})

	body, err := json.Marshal(chatRequest{Model: c.model, Messages: messages})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/yattoni/discord-bots")
	req.Header.Set("X-Title", "discord-stock-bot")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("%w: read response: %v", ErrUnavailable, err)
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", classifyStatus(resp.StatusCode, fmt.Sprintf("decode response: %v", err))
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		code := parsed.Error.Code
		if code == 0 {
			code = resp.StatusCode
		}
		return "", classifyStatus(code, parsed.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return "", classifyStatus(resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if len(parsed.Choices) == 0 {
		return "", ErrEmpty
	}

	text := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if text == "" {
		return "", ErrEmpty
	}
	return text, nil
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
		if detail == "" {
			return fmt.Errorf("%w: unexpected status %d", ErrUnavailable, status)
		}
		return fmt.Errorf("%w: %s", ErrUnavailable, detail)
	}
}
