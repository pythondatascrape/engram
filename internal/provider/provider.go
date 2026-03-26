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
	Name() string
	Send(ctx context.Context, req *Request) (<-chan Chunk, error)
	Healthcheck(ctx context.Context) error
	Capabilities() Capabilities
	Close() error
}
