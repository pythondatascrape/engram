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
		wantOmit   []string
	}{
		{
			name:     "anthropic",
			codebook: engramctx.AnthropicResponseCodebook,
			input: map[string]string{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"model":       "claude-sonnet-4-6",
			},
			wantFields: []string{"model=claude-sonnet-4-6"},
			wantOmit:   []string{"role=assistant", "stop_reason=end_turn"},
		},
		{
			name:     "anthropic_non_default_stop",
			codebook: engramctx.AnthropicResponseCodebook,
			input: map[string]string{
				"role":        "assistant",
				"stop_reason": "max_tokens",
				"model":       "claude-sonnet-4-6",
			},
			wantFields: []string{"model=claude-sonnet-4-6", "stop_reason=max_tokens"},
			wantOmit:   []string{"role=assistant"},
		},
		{
			name:     "openai",
			codebook: engramctx.OpenAIResponseCodebook,
			input: map[string]string{
				"role":          "assistant",
				"finish_reason": "stop",
				"model":         "gpt-4o",
			},
			wantFields: []string{"model=gpt-4o"},
			wantOmit:   []string{"role=assistant", "finish_reason=stop"},
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
			for _, omit := range tc.wantOmit {
				assert.NotContains(t, compressed, omit)
			}
		})
	}
}

func TestResponseCodebook_UnknownField(t *testing.T) {
	cb := engramctx.AnthropicResponseCodebook()
	_, err := cb.SerializeTurn(map[string]string{"not_a_field": "val"})
	assert.Error(t, err)
}
