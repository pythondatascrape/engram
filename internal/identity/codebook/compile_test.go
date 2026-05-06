package codebook_test

import (
	"strings"
	"testing"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile_ExtractsHighFrequencyTokens(t *testing.T) {
	prompt := "authentication authentication authentication configuration configuration configuration"
	subs, err := codebook.Compile(prompt)
	require.NoError(t, err)
	assert.Contains(t, subs, "authentication")
	assert.Contains(t, subs, "configuration")
	// abbreviations must be shorter than original
	for orig, abbr := range subs {
		assert.Less(t, len(abbr), len(orig))
	}
}

func TestCompile_IgnoresShortTokens(t *testing.T) {
	// tokens <=6 chars should never appear in the result
	prompt := "go go go go go run run run run run"
	subs, err := codebook.Compile(prompt)
	require.NoError(t, err)
	for orig := range subs {
		assert.Greater(t, len(orig), 6, "unexpected short token %q in substitution map", orig)
	}
}

func TestCompile_IgnoresRareTokens(t *testing.T) {
	// a long token that only appears once should not be abbreviated
	prompt := "authentication authentication authentication uniquetoken"
	subs, err := codebook.Compile(prompt)
	require.NoError(t, err)
	assert.NotContains(t, subs, "uniquetoken")
}

func TestCompile_AbbreviationsAreUnique(t *testing.T) {
	prompt := strings.Repeat("authentication configuration authorization ", 5)
	subs, err := codebook.Compile(prompt)
	require.NoError(t, err)
	seen := make(map[string]string)
	for orig, abbr := range subs {
		if prev, exists := seen[abbr]; exists {
			t.Errorf("abbreviation %q used for both %q and %q", abbr, prev, orig)
		}
		seen[abbr] = orig
	}
}

func TestCompile_EmptyInput(t *testing.T) {
	subs, err := codebook.Compile("")
	require.NoError(t, err)
	assert.Empty(t, subs)
}

func TestCompile_NormalizesCase(t *testing.T) {
	// tokens are lowercased before counting
	prompt := "Authentication authentication AUTHENTICATION configuration configuration configuration"
	subs, err := codebook.Compile(prompt)
	require.NoError(t, err)
	// all three Authentication variants count toward one token
	assert.Contains(t, subs, "authentication")
}
