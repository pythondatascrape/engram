package redundancy_test

import (
	"strings"
	"testing"

	"github.com/pythondatascrape/engram/internal/redundancy"
	"github.com/stretchr/testify/require"
)

func TestExactDuplicate(t *testing.T) {
	c := redundancy.NewChecker(0.9)
	c.Record("lang=go arch=monolith")
	result := c.Check("lang=go arch=monolith")
	require.True(t, result.IsRedundant)
	require.Equal(t, "exact", result.Kind)
	require.InDelta(t, 1.0, result.Similarity, 0.01)
}

func TestNormalizedDuplicate(t *testing.T) {
	c := redundancy.NewChecker(0.9)
	c.Record("lang=go arch=monolith")
	result := c.Check("arch=monolith  lang=go") // different order, extra space
	require.True(t, result.IsRedundant)
	require.Equal(t, "normalized", result.Kind)
}

func TestSimilarAboveThreshold(t *testing.T) {
	c := redundancy.NewChecker(0.5)
	c.Record("lang=go arch=monolith env=prod")
	result := c.Check("lang=go arch=monolith env=staging")
	require.True(t, result.IsRedundant)
	require.Equal(t, "similar", result.Kind)
	require.GreaterOrEqual(t, result.Similarity, 0.5)
}

func TestBelowThreshold(t *testing.T) {
	c := redundancy.NewChecker(0.9)
	c.Record("lang=go arch=monolith")
	result := c.Check("lang=rust framework=axum")
	require.False(t, result.IsRedundant)
}

func TestFirstCallNotRedundant(t *testing.T) {
	c := redundancy.NewChecker(0.9)
	result := c.Check("lang=go")
	require.False(t, result.IsRedundant)
}

// TestExactLookupUsesIndex verifies that exact duplicates are found via O(1) map lookup.
// The Checker must expose an ExactIndex() map for inspection.
func TestExactLookupUsesIndex(t *testing.T) {
	c := redundancy.NewChecker(0.9)
	c.Record("lang=go arch=monolith")
	c.Record("lang=rust framework=axum")
	idx := c.ExactIndex()
	require.True(t, idx["lang=go arch=monolith"], "exact index must contain recorded entry")
	require.True(t, idx["lang=rust framework=axum"], "exact index must contain second entry")
	require.False(t, idx["lang=go"], "non-recorded string must not appear in exact index")
}

// TestNormalizedLookupUsesIndex verifies that normalized duplicates are found via O(1) map lookup.
func TestNormalizedLookupUsesIndex(t *testing.T) {
	c := redundancy.NewChecker(0.9)
	c.Record("lang=go arch=monolith")
	idx := c.NormalizedIndex()
	// normalized form of "lang=go arch=monolith" is tokens sorted: "arch=monolith lang=go"
	require.True(t, idx["arch=monolith lang=go"], "normalized index must contain sorted-token form")
}

// TestLargeExactIndex ensures exact match stays fast with many entries (index must not degrade).
func TestLargeExactIndex(t *testing.T) {
	c := redundancy.NewChecker(0.9)
	const n = 10_000
	for i := 0; i < n; i++ {
		c.Record(strings.Repeat("x", i+1))
	}
	target := strings.Repeat("x", n/2)
	result := c.Check(target)
	require.True(t, result.IsRedundant)
	require.Equal(t, "exact", result.Kind)
}

// TestLargeNormalizedIndex ensures normalized match stays fast with many entries.
func TestLargeNormalizedIndex(t *testing.T) {
	c := redundancy.NewChecker(0.9)
	const n = 10_000
	for i := 0; i < n; i++ {
		c.Record(strings.Repeat("y", i+1))
	}
	// Check reverse-order of a recorded entry — same tokens, different order (single token so same).
	target := strings.Repeat("y", n/2)
	result := c.Check(target)
	require.True(t, result.IsRedundant)
}
