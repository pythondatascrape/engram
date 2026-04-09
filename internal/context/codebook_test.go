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

func TestDeriveCodebook_ParsesEnumDefaults(t *testing.T) {
	schema := map[string]string{
		"role":    "enum:user,assistant",
		"content": "text",
	}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)

	defaults := cb.Defaults()
	assert.Equal(t, "user", defaults["role"])
	assert.Empty(t, defaults["content"], "text fields should have no default")
}

func TestDeriveCodebook_NoEnumNoDefaults(t *testing.T) {
	schema := map[string]string{"content": "text", "city": "text"}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)
	assert.Empty(t, cb.Defaults())
}

func TestDeriveCodebook_MultipleEnums(t *testing.T) {
	schema := map[string]string{
		"role":   "enum:user,assistant,tool",
		"status": "enum:active,inactive",
	}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)

	defaults := cb.Defaults()
	assert.Equal(t, "user", defaults["role"])
	assert.Equal(t, "active", defaults["status"])
}

func TestSerializeTurn(t *testing.T) {
	tests := []struct {
		name         string
		schema       map[string]string
		turn         map[string]string
		wantErr      bool
		wantContains []string
		wantOmit     []string
		wantExact    string
		checkExact   bool
	}{
		{
			name:         "OmitsDefaultEnum",
			schema:       map[string]string{"role": "enum:user,assistant", "content": "text"},
			turn:         map[string]string{"role": "user", "content": "What flights are available?"},
			wantErr:      false,
			wantContains: []string{"content=What flights are available?"},
			wantOmit:     []string{"role=user"},
		},
		{
			name:    "UnknownField",
			schema:  map[string]string{"role": "text"},
			turn:    map[string]string{"role": "user", "unknown": "val"},
			wantErr: true,
		},
		{
			name:         "IncludesNonDefaultEnum",
			schema:       map[string]string{"role": "enum:user,assistant", "content": "text"},
			turn:         map[string]string{"role": "assistant", "content": "Here are the flights"},
			wantErr:      false,
			wantContains: []string{"role=assistant", "content=Here are the flights"},
		},
		{
			name:       "AllDefaultsEmptyOutput",
			schema:     map[string]string{"role": "enum:user,assistant"},
			turn:       map[string]string{"role": "user"},
			wantErr:    false,
			wantExact:  "",
			checkExact: true,
		},
		{
			name:         "MixedDefaultAndNonDefault",
			schema:       map[string]string{"role": "enum:user,assistant", "status": "enum:active,inactive", "content": "text"},
			turn:         map[string]string{"role": "user", "status": "inactive", "content": "hello"},
			wantErr:      false,
			wantContains: []string{"status=inactive", "content=hello"},
			wantOmit:     []string{"role=user"},
		},
		{
			name:         "TextFieldNeverOmitted",
			schema:       map[string]string{"content": "text"},
			turn:         map[string]string{"content": "hello"},
			wantErr:      false,
			wantContains: []string{"content=hello"},
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
			if tt.checkExact {
				assert.Equal(t, tt.wantExact, compressed)
			}
			for _, want := range tt.wantContains {
				assert.Contains(t, compressed, want)
			}
			for _, omit := range tt.wantOmit {
				assert.NotContains(t, compressed, omit)
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

func TestDefinition_MarksDefaults(t *testing.T) {
	schema := map[string]string{
		"role":    "enum:user,assistant",
		"content": "text",
	}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)
	def := cb.Definition()

	assert.Contains(t, def, "role(enum:user*,assistant)")
	assert.Contains(t, def, "content(text)")
	assert.NotContains(t, def, "content(text*")
}

func TestDefinition_NoDefaultNoMarker(t *testing.T) {
	schema := map[string]string{"content": "text", "city": "text"}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)
	def := cb.Definition()

	assert.NotContains(t, def, "*")
}
