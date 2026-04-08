package smc_test

import (
	"context"
	"testing"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleDecomposer_BasicExtraction(t *testing.T) {
	d := smc.NewRuleDecomposer(smc.KController{Global: 0.5})
	schema := smc.DefaultSchema()

	exchange := smc.Exchange{
		UserMessage:      "Please refactor the authentication module in auth.go to use JWT tokens instead of sessions",
		AssistantMessage: "I've updated auth.go and middleware.go to use JWT-based authentication. The session store has been removed.",
		TurnIndex:        0,
	}

	row, err := d.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)
	require.NotNil(t, row)

	assert.Equal(t, 0, row.TurnIndex)
	assert.NotEmpty(t, row.Categories["intent"])
	assert.NotEmpty(t, row.Categories["entities"])
	assert.NotEmpty(t, row.Categories["mutations"])
	assert.NotEmpty(t, row.Categories["context"])
	assert.Greater(t, row.RawTokens, 0)
	assert.Greater(t, row.CompTokens, 0)
	assert.True(t, row.CompTokens < row.RawTokens, "compressed should be smaller than raw")
}

func TestRuleDecomposer_CustomSchema(t *testing.T) {
	d := smc.NewRuleDecomposer(smc.KController{Global: 0.5})
	schema := smc.CategorySchema{
		Categories: []smc.Category{
			{Name: "action", Description: "what to do", K: -1},
			{Name: "target", Description: "what to act on", K: -1},
		},
	}

	exchange := smc.Exchange{
		UserMessage:      "Deploy the app to staging",
		AssistantMessage: "Deployed successfully to staging environment.",
		TurnIndex:        0,
	}

	row, err := d.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)

	// Custom schema: only "action" and "target" keys should exist
	assert.Len(t, row.Categories, 2)
	assert.Contains(t, row.Categories, "action")
	assert.Contains(t, row.Categories, "target")
	assert.NotContains(t, row.Categories, "intent")
}

func TestRuleDecomposer_EmptyExchange(t *testing.T) {
	d := smc.NewRuleDecomposer(smc.KController{Global: 0.5})
	schema := smc.DefaultSchema()

	exchange := smc.Exchange{
		UserMessage:      "",
		AssistantMessage: "",
		TurnIndex:        0,
	}

	row, err := d.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)
	// Should still produce a row, just with minimal content
	assert.Equal(t, 0, row.TurnIndex)
}

func TestRuleDecomposer_KAffectsCompression(t *testing.T) {
	schema := smc.DefaultSchema()

	exchange := smc.Exchange{
		UserMessage:      "Refactor the database connection pooling in db/pool.go. The current implementation leaks connections under high concurrency because the mutex is held during the entire query execution instead of just during pool checkout.",
		AssistantMessage: "I've refactored db/pool.go to use a channel-based pool instead of mutex-guarded slice. Connections are now checked out and returned via buffered channel, eliminating the lock contention.",
		TurnIndex:        0,
	}

	lowK := smc.NewRuleDecomposer(smc.KController{Global: 0.1})
	highK := smc.NewRuleDecomposer(smc.KController{Global: 0.9})

	rowLow, err := lowK.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)

	rowHigh, err := highK.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)

	// Lower k = more aggressive compression = fewer tokens
	assert.Less(t, rowLow.CompTokens, rowHigh.CompTokens,
		"low k (%d tokens) should produce fewer tokens than high k (%d tokens)",
		rowLow.CompTokens, rowHigh.CompTokens)
}
