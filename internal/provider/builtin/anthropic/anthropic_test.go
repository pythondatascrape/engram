package anthropic_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pythondatascrape/engram/internal/provider"
	"github.com/pythondatascrape/engram/internal/provider/builtin/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sseBody builds a minimal Anthropic SSE response with the given text chunks.
func sseBody(texts []string) string {
	var sb strings.Builder
	for i, t := range texts {
		fmt.Fprintf(&sb, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":%d,\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", i, t)
	}
	sb.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	return sb.String()
}

func TestAnthropicName(t *testing.T) {
	p := anthropic.New("test-key")
	assert.Equal(t, "anthropic", p.Name())
}

func TestAnthropicCapabilities(t *testing.T) {
	p := anthropic.New("test-key")
	caps := p.Capabilities()

	assert.True(t, caps.SupportsStreaming)
	assert.NotEmpty(t, caps.Models)
	assert.Equal(t, 200000, caps.MaxContextWindow)
}

func TestAnthropicSend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody([]string{"Hello", ", ", "world"}))
	}))
	defer srv.Close()

	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := &provider.Request{
		Model:        "claude-sonnet-4-6",
		SystemPrompt: "You are helpful.",
		Query:        "Say hello",
	}

	ch, err := p.Send(t.Context(), req)
	require.NoError(t, err)

	var collected strings.Builder
	for chunk := range ch {
		if chunk.Done {
			break
		}
		collected.WriteString(chunk.Text)
	}

	assert.Equal(t, "Hello, world", collected.String())
}

func TestAnthropicClose(t *testing.T) {
	p := anthropic.New("test-key")
	assert.NoError(t, p.Close())
}

func TestAnthropicHealthcheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))
	assert.NoError(t, p.Healthcheck(t.Context()))
}
