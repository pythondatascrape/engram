package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pythondatascrape/engram/internal/provider"
)

const (
	defaultBaseURL        = "https://api.anthropic.com"
	anthropicVersion      = "2023-06-01"
	defaultMaxTokens      = 4096
)

// Option configures the Provider.
type Option func(*Provider)

// WithBaseURL overrides the default Anthropic API base URL.
func WithBaseURL(url string) Option {
	return func(p *Provider) {
		p.baseURL = url
	}
}

// Provider is the built-in Anthropic provider.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// New creates a new Anthropic Provider with the given API key and options.
func New(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return "anthropic"
}

// Capabilities returns static capabilities for the Anthropic provider.
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		Models: []string{
			"claude-opus-4-6",
			"claude-sonnet-4-6",
			"claude-haiku-4-5-20251001",
		},
		MaxContextWindow: 200000,
		SupportsStreaming: true,
	}
}

// Healthcheck verifies the provider is reachable via a GET to the base URL.
func (p *Provider) Healthcheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL, nil)
	if err != nil {
		return fmt.Errorf("anthropic: healthcheck request build: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("anthropic: healthcheck: %w", err)
	}
	resp.Body.Close()
	return nil
}

// Close releases provider resources (no-op for Anthropic).
func (p *Provider) Close() error {
	return nil
}

// anthropicMessage is a single turn in the Anthropic Messages API format.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicRequest is the JSON body sent to /v1/messages.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
}

// sseEvent represents a parsed SSE event.
type sseEvent struct {
	Type  string          `json:"type"`
	Delta *sseDelta       `json:"delta,omitempty"`
	Index int             `json:"index,omitempty"`
}

type sseDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Send submits the request to the Anthropic Messages API and returns a streaming channel.
func (p *Provider) Send(ctx context.Context, req *provider.Request) (<-chan provider.Chunk, error) {
	// Build messages list from history + current query.
	msgs := make([]anthropicMessage, 0, len(req.ConversationHistory)+1)
	for _, m := range req.ConversationHistory {
		msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, anthropicMessage{Role: "user", Content: req.Query})

	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: defaultMaxTokens,
		System:    req.SystemPrompt,
		Messages:  msgs,
		Stream:    true,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: send: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic: unexpected status %d", resp.StatusCode)
	}

	ch := make(chan provider.Chunk)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var eventType string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				switch eventType {
				case "content_block_delta":
					var ev sseEvent
					if err := json.Unmarshal([]byte(data), &ev); err == nil && ev.Delta != nil {
						select {
						case ch <- provider.Chunk{Text: ev.Delta.Text, Index: ev.Index}:
						case <-ctx.Done():
							return
						}
					}
				case "message_stop":
					select {
					case ch <- provider.Chunk{Done: true}:
					case <-ctx.Done():
					}
					return
				}
			}
		}

		// If scanner ends without message_stop, send Done anyway.
		select {
		case ch <- provider.Chunk{Done: true}:
		case <-ctx.Done():
		}
	}()

	return ch, nil
}

// Ensure Provider satisfies the interface at compile time.
var _ provider.Provider = (*Provider)(nil)
