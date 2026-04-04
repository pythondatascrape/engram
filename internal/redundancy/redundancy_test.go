package redundancy_test

import (
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
