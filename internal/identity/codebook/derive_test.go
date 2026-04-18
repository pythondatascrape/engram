package codebook_test

import (
	"testing"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/stretchr/testify/assert"
)

func TestDerive_KeyEqValue(t *testing.T) {
	dims, serialized := codebook.Derive("lang=go arch=modular_monolith db=postgresql")
	assert.Equal(t, "go", dims["lang"])
	assert.Equal(t, "modular_monolith", dims["arch"])
	assert.Equal(t, "postgresql", dims["db"])
	assert.Contains(t, serialized, "lang=go")
	assert.Contains(t, serialized, "arch=modular_monolith")
}

func TestDerive_KeyColonValue(t *testing.T) {
	dims, serialized := codebook.Derive("lang: go\ndb: postgresql")
	assert.Equal(t, "go", dims["lang"])
	assert.Equal(t, "postgresql", dims["db"])
	assert.Contains(t, serialized, "lang=go")
}

func TestDerive_Empty(t *testing.T) {
	dims, serialized := codebook.Derive("")
	assert.Empty(t, dims)
	assert.Equal(t, "", serialized)
}

func TestDerive_SerializedIsDeterministic(t *testing.T) {
	_, s1 := codebook.Derive("b=2 a=1 c=3")
	_, s2 := codebook.Derive("c=3 b=2 a=1")
	assert.Equal(t, s1, s2)
	assert.Equal(t, "a=1 b=2 c=3", s1)
}

func TestDerive_Prose_ResponseStyle(t *testing.T) {
	dims, _ := codebook.Derive("Please prefer concise responses.")
	assert.Equal(t, "concise", dims["response_style"])
}

func TestDerive_Prose_SummaryPolicy(t *testing.T) {
	dims, _ := codebook.Derive("Do not include a trailing summary after each reply.")
	assert.Equal(t, "no_trailing_summary", dims["summary_policy"])
}

func TestDerive_Prose_Role(t *testing.T) {
	dims, _ := codebook.Derive("I am a senior software engineer on the platform team.")
	assert.Equal(t, "engineer", dims["role"])
}

func TestDerive_Prose_KeyValueOverridesProse(t *testing.T) {
	// Explicit key=value must beat prose inference for the same key.
	dims, _ := codebook.Derive("response_style=verbose prefer concise responses")
	assert.Equal(t, "verbose", dims["response_style"])
}

func TestDerive_Prose_NoFalsePositive(t *testing.T) {
	dims, _ := codebook.Derive("The dog ran to the park.")
	assert.Empty(t, dims)
}

func TestCompressIfSafe_AllowsStructuredIdentity(t *testing.T) {
	compressed, ok := codebook.CompressIfSafe("role: engineer\nresponse_style: concise\nplatform: macos")
	assert.True(t, ok)
	assert.Contains(t, compressed, "role=engineer")
}

func TestCompressIfSafe_RejectsThinProse(t *testing.T) {
	compressed, ok := codebook.CompressIfSafe("Please prefer concise responses and do not include a trailing summary.")
	assert.False(t, ok)
	assert.Equal(t, "", compressed)
}

func TestCompressIfSafe_AllowsDenseProse(t *testing.T) {
	content := "I am a senior software engineer. Please prefer concise responses. Do not include a trailing summary. " +
		"Target macOS and linux. This repository is written in Go."
	compressed, ok := codebook.CompressIfSafe(content)
	assert.True(t, ok)
	assert.Contains(t, compressed, "response_style=concise")
	assert.Contains(t, compressed, "platform=macos,linux")
}
