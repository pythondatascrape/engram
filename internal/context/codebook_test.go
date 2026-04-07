// internal/context/codebook_test.go
package context_test

import (
	"testing"

	engramctx "github.com/pythondatascrape/engram/internal/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveCodebook(t *testing.T) {
	tests := []struct {
		name       string
		cbName     string
		schema     map[string]string
		wantErr    bool
		wantName   string
		wantKeyLen int
	}{
		{
			name:       "BasicFields",
			cbName:     "travel_agent",
			schema:     map[string]string{"role": "enum:user,assistant,tool", "content": "text"},
			wantErr:    false,
			wantName:   "travel_agent",
			wantKeyLen: 2,
		},
		{
			name:       "EmptySchema",
			cbName:     "app",
			schema:     map[string]string{},
			wantErr:    true,
			wantName:   "",
			wantKeyLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb, err := engramctx.DeriveCodebook(tt.cbName, tt.schema)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, cb.Name)
			assert.Len(t, cb.Keys(), tt.wantKeyLen)
		})
	}
}

func TestSerializeTurn(t *testing.T) {
	tests := []struct {
		name         string
		schema       map[string]string
		turn         map[string]string
		wantErr      bool
		wantContains []string
	}{
		{
			name:         "RoundTrip",
			schema:       map[string]string{"role": "enum:user,assistant", "content": "text"},
			turn:         map[string]string{"role": "user", "content": "What flights are available?"},
			wantErr:      false,
			wantContains: []string{"role=user", "content=What flights are available?"},
		},
		{
			name:    "UnknownField",
			schema:  map[string]string{"role": "text"},
			turn:    map[string]string{"role": "user", "unknown": "val"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb, err := engramctx.DeriveCodebook("app", tt.schema)
			require.NoError(t, err)

			compressed, err := cb.SerializeTurn(tt.turn)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			for _, want := range tt.wantContains {
				assert.Contains(t, compressed, want)
			}
		})
	}
}

func TestDefinition_ContainsAllKeys(t *testing.T) {
	schema := map[string]string{
		"role":    "enum:user,assistant",
		"content": "text",
		"city":    "text",
	}
	cb, err := engramctx.DeriveCodebook("travel_agent", schema)
	require.NoError(t, err)
	def := cb.Definition()
	assert.Contains(t, def, "role")
	assert.Contains(t, def, "content")
	assert.Contains(t, def, "city")
}
