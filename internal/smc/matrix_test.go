package smc_test

import (
	"testing"
	"time"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationMatrix_Append(t *testing.T) {
	schema := smc.DefaultSchema()
	m := smc.NewConversationMatrix("test-session", schema, smc.KController{Global: 0.5})

	row := smc.MatrixRow{
		TurnIndex: 0,
		Categories: map[string]string{
			"intent":    "refactor authentication module",
			"entities":  "auth.go, middleware.go, JWT",
			"mutations": "replaced session-based auth with JWT tokens",
			"context":   "security audit required stateless auth",
		},
		RawTokens:  500,
		CompTokens: 80,
		Timestamp:  time.Now(),
	}

	m.Append(row)
	assert.Equal(t, 1, m.Len())
}

func TestConversationMatrix_Messages(t *testing.T) {
	schema := smc.DefaultSchema()
	m := smc.NewConversationMatrix("test-session", schema, smc.KController{Global: 0.5})

	m.Append(smc.MatrixRow{
		TurnIndex: 0,
		Categories: map[string]string{
			"intent":    "add logging",
			"entities":  "server.go",
			"mutations": "added slog calls",
			"context":   "debugging production issue",
		},
		RawTokens:  400,
		CompTokens: 60,
		Timestamp:  time.Now(),
	})

	msgs := m.Messages()
	require.Len(t, msgs, 1, "one compressed turn = one synthetic message")
	assert.Equal(t, "user", msgs[0].Role)
	assert.Contains(t, msgs[0].Content, "intent=add logging")
	assert.Contains(t, msgs[0].Content, "entities=server.go")
	assert.Contains(t, msgs[0].Content, "mutations=added slog calls")
	assert.Contains(t, msgs[0].Content, "context=debugging production issue")
}

func TestConversationMatrix_TokenCount(t *testing.T) {
	schema := smc.DefaultSchema()
	m := smc.NewConversationMatrix("test-session", schema, smc.KController{Global: 0.5})

	m.Append(smc.MatrixRow{
		TurnIndex:  0,
		Categories: map[string]string{"intent": "test", "entities": "file.go"},
		CompTokens: 30,
		Timestamp:  time.Now(),
	})
	m.Append(smc.MatrixRow{
		TurnIndex:  1,
		Categories: map[string]string{"intent": "fix", "entities": "bug.go"},
		CompTokens: 25,
		Timestamp:  time.Now(),
	})

	assert.Equal(t, 55, m.TokenCount())
}

func TestConversationMatrix_EmptyMessages(t *testing.T) {
	schema := smc.DefaultSchema()
	m := smc.NewConversationMatrix("test-session", schema, smc.KController{Global: 0.5})

	msgs := m.Messages()
	assert.Empty(t, msgs)
	assert.Equal(t, 0, m.TokenCount())
}
