package smc_test

import (
	"context"
	"testing"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEnd_MultiTurnConversation(t *testing.T) {
	schema := smc.DefaultSchema()
	kc := smc.NewKController(0.5, schema)
	decomposer := smc.NewRuleDecomposer(kc)
	matrix := smc.NewConversationMatrix("e2e-test", schema, kc)

	exchanges := []smc.Exchange{
		{
			UserMessage:      "Refactor auth.go to use JWT tokens instead of session cookies. The current session store leaks memory under load.",
			AssistantMessage: "I've replaced the session-based authentication in auth.go with JWT tokens. Removed the in-memory session store and added jwt.go with token generation and validation.",
			TurnIndex:        0,
		},
		{
			UserMessage:      "Now update the middleware in middleware.go to validate JWT tokens on each request.",
			AssistantMessage: "Updated middleware.go to extract and validate JWT from the Authorization header. Added token expiry checking and refresh logic.",
			TurnIndex:        1,
		},
		{
			UserMessage:      "Add tests for the JWT validation. Cover expired tokens, invalid signatures, and missing headers.",
			AssistantMessage: "Created jwt_test.go with table-driven tests covering: valid token, expired token, invalid signature, missing Authorization header, and malformed token format.",
			TurnIndex:        2,
		},
	}

	for _, ex := range exchanges {
		row, err := decomposer.Decompose(context.Background(), ex, schema)
		require.NoError(t, err)
		matrix.Append(*row)
	}

	// Verify matrix state
	assert.Equal(t, 3, matrix.Len())

	// Verify messages are well-formed
	msgs := matrix.Messages()
	require.Len(t, msgs, 3)
	for i, msg := range msgs {
		assert.Equal(t, "user", msg.Role, "turn %d should be user role", i)
		assert.Contains(t, msg.Content, "intent=", "turn %d should have intent", i)
		assert.Contains(t, msg.Content, "entities=", "turn %d should have entities", i)
	}

	// Verify compression happened
	totalCompressed := matrix.TokenCount()
	totalRaw := 0
	for _, r := range matrix.Rows() {
		totalRaw += r.RawTokens
	}
	assert.Less(t, totalCompressed, totalRaw,
		"compressed (%d) should be less than raw (%d)", totalCompressed, totalRaw)

	t.Logf("Compression: %d raw tokens -> %d compressed tokens (%.0f%% reduction)",
		totalRaw, totalCompressed, 100*(1-float64(totalCompressed)/float64(totalRaw)))
}

func TestEndToEnd_CustomSchema(t *testing.T) {
	schema := smc.CategorySchema{
		Categories: []smc.Category{
			{Name: "symptom", Description: "patient symptoms", K: 0.3},
			{Name: "diagnosis", Description: "medical diagnosis", K: 0.5},
			{Name: "treatment", Description: "recommended treatment", K: 0.7},
		},
	}
	kc := smc.NewKController(0.5, schema)
	decomposer := smc.NewRuleDecomposer(kc)
	matrix := smc.NewConversationMatrix("custom-test", schema, kc)

	row, err := decomposer.Decompose(context.Background(), smc.Exchange{
		UserMessage:      "Patient presents with fever and cough for 3 days",
		AssistantMessage: "Based on symptoms, likely upper respiratory infection. Recommend rest and fluids.",
		TurnIndex:        0,
	}, schema)
	require.NoError(t, err)
	matrix.Append(*row)

	msgs := matrix.Messages()
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "symptom=")
	assert.Contains(t, msgs[0].Content, "diagnosis=")
	assert.Contains(t, msgs[0].Content, "treatment=")
	assert.NotContains(t, msgs[0].Content, "intent=")
}
