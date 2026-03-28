package context_test

import (
	"testing"

	engramctx "github.com/pythondatascrape/engram/internal/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseCodebook_Serialize(t *testing.T) {
	tests := []struct {
		name       string
		codebook   func() *engramctx.ContextCodebook
		input      map[string]string
		wantFields []string
	}{
		{
			name:     "anthropic",
			codebook: engramctx.AnthropicResponseCodebook,
			input: map[string]string{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"model":       "claude-sonnet-4-6",
			},
			wantFields: []string{"role=assistant", "stop_reason=end_turn"},
		},
		{
			name:     "openai",
			codebook: engramctx.OpenAIResponseCodebook,
			input: map[string]string{
				"role":          "assistant",
				"finish_reason": "stop",
				"model":         "gpt-4o",
			},
			wantFields: []string{"role=assistant", "finish_reason=stop"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cb := tc.codebook()
			compressed, err := cb.SerializeTurn(tc.input)
			require.NoError(t, err)
			for _, want := range tc.wantFields {
				assert.Contains(t, compressed, want)
			}
		})
	}
}

func TestResponseCodebook_UnknownField(t *testing.T) {
	cb := engramctx.AnthropicResponseCodebook()
	_, err := cb.SerializeTurn(map[string]string{"not_a_field": "val"})
	assert.Error(t, err)
}
