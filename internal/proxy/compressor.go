// internal/proxy/compressor.go
package proxy

import (
	"encoding/json"
	"strings"
)

// AnthropicMessage is the wire format for a single message in the Anthropic API.
// Content is string for simple text, or []interface{} for structured content blocks
// (tool use, images). json.Unmarshal decodes JSON arrays into []interface{}, not []map[string]any.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// Compress splits messages into a tail (last windowSize messages kept verbatim)
// and a head (older messages compressed into a [CONTEXT_SUMMARY] block).
// If len(messages) <= windowSize, messages are returned unchanged.
func Compress(messages []AnthropicMessage, windowSize int) []AnthropicMessage {
	if len(messages) <= windowSize {
		return messages
	}
	head := messages[:len(messages)-windowSize]
	tail := messages[len(messages)-windowSize:]
	summary := compressHead(head)
	synthetic := AnthropicMessage{
		Role:    "user",
		Content: "[CONTEXT_SUMMARY]\n" + summary + "\n[/CONTEXT_SUMMARY]",
	}
	result := make([]AnthropicMessage, 0, 1+len(tail))
	result = append(result, synthetic)
	result = append(result, tail...)
	return result
}

// EstimateTokens returns a rough token count for a messages slice (len/4 per message text).
func EstimateTokens(messages []AnthropicMessage) int {
	total := 0
	for _, m := range messages {
		total += len(messageText(m)) / 4
	}
	return total
}

// compressHead reduces head messages to a concise role: text summary.
func compressHead(msgs []AnthropicMessage) string {
	parts := make([]string, len(msgs))
	for i, m := range msgs {
		text := messageText(m)
		// Only convert to runes when the byte length could exceed the limit,
		// avoiding allocation for the common short-message case.
		if len(text) > 120 {
			if runes := []rune(text); len(runes) > 120 {
				text = string(runes[:120])
			}
		}
		parts[i] = m.Role + ": " + text
	}
	return strings.Join(parts, "\n")
}

// messageText extracts text from a message content field regardless of type.
func messageText(m AnthropicMessage) string {
	switch v := m.Content.(type) {
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "<unmarshalable content>"
		}
		return string(b)
	}
}
