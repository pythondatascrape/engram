package context_test

import (
	"testing"

	engramctx "github.com/pythondatascrape/engram/internal/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveCodebook_BasicFields(t *testing.T) {
	schema := map[string]string{
		"role":    "enum:user,assistant,tool",
		"content": "text",
	}
	cb, err := engramctx.DeriveCodebook("travel_agent", schema)
	require.NoError(t, err)
	assert.Equal(t, "travel_agent", cb.Name)
	assert.Len(t, cb.Keys(), 2)
}

func TestDeriveCodebook_EmptySchema(t *testing.T) {
	_, err := engramctx.DeriveCodebook("app", map[string]string{})
	assert.Error(t, err)
}

func TestSerializeTurn_RoundTrip(t *testing.T) {
	schema := map[string]string{
		"role":    "enum:user,assistant",
		"content": "text",
	}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)

	turn := map[string]string{
		"role":    "user",
		"content": "What flights are available?",
	}
	compressed, err := cb.SerializeTurn(turn)
	require.NoError(t, err)
	assert.Contains(t, compressed, "role=user")
	assert.Contains(t, compressed, "content=What flights are available?")
}

func TestSerializeTurn_UnknownField(t *testing.T) {
	schema := map[string]string{"role": "text"}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)

	_, err = cb.SerializeTurn(map[string]string{"role": "user", "unknown": "val"})
	assert.Error(t, err)
}

func TestDefinition_ContainsAllKeys(t *testing.T) {
	schema := map[string]string{
		"role":    "enum:user,assistant",
		"content": "text",
		"city":    "text",
	}
	cb, _ := engramctx.DeriveCodebook("travel_agent", schema)
	def := cb.Definition()
	assert.Contains(t, def, "role")
	assert.Contains(t, def, "content")
	assert.Contains(t, def, "city")
}
