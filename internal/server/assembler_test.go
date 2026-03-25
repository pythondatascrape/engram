package server_test

import (
	"testing"

	"github.com/pythondatascrape/engram/internal/server"
)

func TestAssemblePrompt(t *testing.T) {
	result := server.AssemblePrompt(server.PromptParts{
		Identity:  "domain=fire rank=captain experience=20",
		Knowledge: "Fire code Section 4.2: All commercial buildings require...",
		Query:     "What are the egress requirements?",
	})

	// Check all sections are present
	if !containsSubstring(result, "[IDENTITY]") {
		t.Error("result should contain [IDENTITY] delimiter")
	}
	if !containsSubstring(result, "domain=fire") {
		t.Error("result should contain identity content")
	}
	if !containsSubstring(result, "[KNOWLEDGE]") {
		t.Error("result should contain [KNOWLEDGE] delimiter")
	}
	if !containsSubstring(result, "Fire code") {
		t.Error("result should contain knowledge content")
	}
	if !containsSubstring(result, "[QUERY]") {
		t.Error("result should contain [QUERY] delimiter")
	}
	if !containsSubstring(result, "egress requirements") {
		t.Error("result should contain query content")
	}
}

func TestAssemblePromptNoKnowledge(t *testing.T) {
	result := server.AssemblePrompt(server.PromptParts{
		Identity: "domain=fire rank=captain",
		Query:    "Hello?",
	})

	// Check that identity and query are present
	if !containsSubstring(result, "[IDENTITY]") {
		t.Error("result should contain [IDENTITY] delimiter")
	}
	if !containsSubstring(result, "domain=fire") {
		t.Error("result should contain identity content")
	}

	// Check that knowledge is NOT present
	if containsSubstring(result, "[KNOWLEDGE]") {
		t.Error("result should NOT contain [KNOWLEDGE] delimiter when knowledge is empty")
	}

	// Check that query is present
	if !containsSubstring(result, "[QUERY]") {
		t.Error("result should contain [QUERY] delimiter")
	}
	if !containsSubstring(result, "Hello?") {
		t.Error("result should contain query content")
	}
}

// Helper function to check substring presence
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
