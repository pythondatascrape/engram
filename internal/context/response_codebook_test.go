package context_test

import (
	"testing"

	engramctx "github.com/pythondatascrape/engram/internal/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicCodebook_Serialize(t *testing.T) {
	cb := engramctx.AnthropicResponseCodebook()
	resp := map[string]string{
		"role":        "assistant",
		"stop_reason": "end_turn",
		"model":       "claude-sonnet-4-6",
	}
	compressed, err := cb.SerializeTurn(resp)
	require.NoError(t, err)
	assert.Contains(t, compressed, "role=assistant")
	assert.Contains(t, compressed, "stop_reason=end_turn")
}

func TestOpenAICodebook_Serialize(t *testing.T) {
	cb := engramctx.OpenAIResponseCodebook()
	resp := map[string]string{
		"role":          "assistant",
		"finish_reason": "stop",
		"model":         "gpt-4o",
	}
	compressed, err := cb.SerializeTurn(resp)
	require.NoError(t, err)
	assert.Contains(t, compressed, "role=assistant")
	assert.Contains(t, compressed, "finish_reason=stop")
}

func TestResponseCodebook_UnknownField(t *testing.T) {
	cb := engramctx.AnthropicResponseCodebook()
	_, err := cb.SerializeTurn(map[string]string{"not_a_field": "val"})
	assert.Error(t, err)
}
