package provider

import "context"

// Message represents a single turn in a conversation.
type Message struct {
	Role    string
	Content string
}

// Request holds the data needed to query a provider.
type Request struct {
	Model               string
	SystemPrompt        string
	Query               string
	ConversationHistory []Message
}

// Chunk is a single streaming token emitted by a provider.
type Chunk struct {
	Text  string
	Index int
	Done  bool
}

// Capabilities describes what a provider supports.
type Capabilities struct {
	Models           []string
	MaxContextWindow int
	SupportsStreaming bool
}

// Provider is the interface that every LLM backend must satisfy.
type Provider interface {
	// Name returns a human-readable identifier for the provider.
	Name() string

	// Send submits a request and returns a channel that streams response chunks.
	// The caller must drain the channel until Done is true or the context is cancelled.
	Send(ctx context.Context, req *Request) (<-chan Chunk, error)

	// Healthcheck verifies the provider is reachable and functional.
	Healthcheck(ctx context.Context) error

	// Capabilities returns the static capabilities of this provider.
	Capabilities() Capabilities

	// Close releases any resources held by the provider.
	Close() error
}
