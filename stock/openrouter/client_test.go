package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteSuccess(t *testing.T) {
	var gotModel, gotAuth, gotTitle string
	var gotBody chatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotTitle = r.Header.Get("X-Title")
		assert.Equal(t, "/chat/completions", r.URL.Path)
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		gotModel = gotBody.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"  Hello from Gemma  "}}]}`))
	}))
	defer srv.Close()

	client := NewClient("test-key")
	client.baseURL = srv.URL
	client.httpClient = srv.Client()

	got, err := client.Complete(context.Background(), "be brief", "hi")
	require.NoError(t, err)
	assert.Equal(t, "Hello from Gemma", got)
	assert.Equal(t, "Bearer test-key", gotAuth)
	assert.Equal(t, "discord-stock-bot", gotTitle)
	assert.Equal(t, DefaultModel, gotModel)
	require.Len(t, gotBody.Messages, 2)
	assert.Equal(t, "system", gotBody.Messages[0].Role)
	assert.Equal(t, "be brief", gotBody.Messages[0].Content)
	assert.Equal(t, "user", gotBody.Messages[1].Role)
	assert.Equal(t, "hi", gotBody.Messages[1].Content)
}

func TestCompleteEmptyContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"   ","reasoning":"thinking"}}]}`))
	}))
	defer srv.Close()

	client := NewClient("test-key")
	client.baseURL = srv.URL
	client.httpClient = srv.Client()

	_, err := client.Complete(context.Background(), "", "hi")
	assert.ErrorIs(t, err, ErrEmpty)
}

func TestCompleteAPIErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{
			name:   "unauthorized json",
			status: http.StatusUnauthorized,
			body:   `{"error":{"message":"invalid key","code":401}}`,
			want:   ErrUnauthorized,
		},
		{
			name:   "rate limited",
			status: http.StatusTooManyRequests,
			body:   `{"error":{"message":"slow down","code":429}}`,
			want:   ErrRateLimited,
		},
		{
			name:   "server error",
			status: http.StatusBadGateway,
			body:   `{"error":{"message":"upstream down","code":502}}`,
			want:   ErrUnavailable,
		},
		{
			name:   "empty choices",
			status: http.StatusOK,
			body:   `{"choices":[]}`,
			want:   ErrEmpty,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			client := NewClient("test-key")
			client.baseURL = srv.URL
			client.httpClient = srv.Client()

			_, err := client.Complete(context.Background(), "", "hi")
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestCompleteMissingAPIKey(t *testing.T) {
	_, err := NewClient("").Complete(context.Background(), "", "hi")
	assert.ErrorIs(t, err, ErrUnauthorized)
}
