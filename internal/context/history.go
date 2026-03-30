package context

import (
	"github.com/pythondatascrape/engram/internal/provider"
)

// CompressedTurn holds one compressed request+response pair.
type CompressedTurn struct {
	Request  string // compressed with ContextCodebook
	Response string // compressed with response codebook
}

// History accumulates compressed turn pairs for a session.
type History struct {
	turns []CompressedTurn
}

// NewHistory returns an empty History.
func NewHistory() *History {
	return &History{}
}

// Append compresses a request turn using cb, stores it alongside the pre-compressed response.
// response is expected to already be serialized (returned from the LLM call post-compression).
func (h *History) Append(cb *ContextCodebook, requestTurn map[string]string, compressedResponse string) error {
	compressed, err := cb.SerializeTurn(requestTurn)
	if err != nil {
		return err
	}
	h.turns = append(h.turns, CompressedTurn{
		Request:  compressed,
		Response: compressedResponse,
	})
	return nil
}

// Len returns the number of turn pairs stored.
func (h *History) Len() int {
	return len(h.turns)
}

// Messages returns the history as a flat slice of provider.Message for inclusion
// in the LLM request. Each turn pair produces two messages: user + assistant.
func (h *History) Messages() []provider.Message {
	msgs := make([]provider.Message, 0, len(h.turns)*2)
	for _, t := range h.turns {
		msgs = append(msgs,
			provider.Message{Role: "user", Content: t.Request},
			provider.Message{Role: "assistant", Content: t.Response},
		)
	}
	return msgs
}

// TokenCount returns a rough byte-based token estimate for the full history.
func (h *History) TokenCount() int {
	total := 0
	for _, t := range h.turns {
		total += len(t.Request) + len(t.Response)
	}
	return total
}
