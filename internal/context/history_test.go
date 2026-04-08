package context_test

import (
	"testing"

	engramctx "github.com/pythondatascrape/engram/internal/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCodebook(t *testing.T) *engramctx.ContextCodebook {
	t.Helper()
	cb, err := engramctx.DeriveCodebook("test", map[string]string{
		"role":    "enum:user,assistant",
		"content": "text",
	})
	require.NoError(t, err)
	return cb
}

func TestHistory_Empty(t *testing.T) {
	h := engramctx.NewHistory()
	assert.Equal(t, 0, h.Len())
	assert.Empty(t, h.Messages())
}

func TestHistory_AppendAndRender(t *testing.T) {
	cb := newTestCodebook(t)
	h := engramctx.NewHistory()

	err := h.Append(cb, map[string]string{"role": "user", "content": "hello"}, "role=assistant content=hi")
	require.NoError(t, err)
	assert.Equal(t, 1, h.Len())

	msgs := h.Messages()
	require.Len(t, msgs, 2) // request + response
	assert.Equal(t, "user", msgs[0].Role)
	assert.Contains(t, msgs[0].Content, "content=hello")
	assert.Equal(t, "assistant", msgs[1].Role)
}

func TestHistory_MultipleTurns(t *testing.T) {
	cb := newTestCodebook(t)
	h := engramctx.NewHistory()

	h.Append(cb, map[string]string{"role": "user", "content": "turn1"}, "role=assistant content=r1")
	h.Append(cb, map[string]string{"role": "user", "content": "turn2"}, "role=assistant content=r2")

	assert.Equal(t, 2, h.Len())
	assert.Len(t, h.Messages(), 4)
}

func TestHistory_TokenCount(t *testing.T) {
	cb := newTestCodebook(t)
	h := engramctx.NewHistory()
	h.Append(cb, map[string]string{"role": "user", "content": "hello"}, "role=assistant content=hi")
	assert.Greater(t, h.TokenCount(), 0)
}
