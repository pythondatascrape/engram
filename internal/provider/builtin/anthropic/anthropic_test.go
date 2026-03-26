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

func TestAnthropicHealthcheckFailure(t *testing.T) {
	// Point to a server that immediately closes.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer srv.Close()

	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))
	err := p.Healthcheck(t.Context())
	assert.Error(t, err)
}

func TestAnthropicSendNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := &provider.Request{
		Model: "claude-sonnet-4-6",
		Query: "hello",
	}

	_, err := p.Send(t.Context(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 429")
}

func TestAnthropicSendWithConversationHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseBody([]string{"response"}))
	}))
	defer srv.Close()

	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := &provider.Request{
		Model:        "claude-sonnet-4-6",
		SystemPrompt: "Be helpful.",
		Query:        "Follow up question",
		ConversationHistory: []provider.Message{
			{Role: "user", Content: "First question"},
			{Role: "assistant", Content: "First answer"},
		},
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
	assert.Equal(t, "response", collected.String())
}

func TestAnthropicSendScannerEndsWithoutMessageStop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Send content but NO message_stop event.
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\n")
	}))
	defer srv.Close()

	p := anthropic.New("test-key", anthropic.WithBaseURL(srv.URL))

	req := &provider.Request{
		Model: "claude-sonnet-4-6",
		Query: "hello",
	}

	ch, err := p.Send(t.Context(), req)
	require.NoError(t, err)

	var gotDone bool
	for chunk := range ch {
		if chunk.Done {
			gotDone = true
			break
		}
	}
	assert.True(t, gotDone, "should receive Done chunk even without message_stop")
}
