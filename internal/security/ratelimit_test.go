package security_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pythondatascrape/engram/internal/security"
)

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rl := security.NewRateLimiter(10, 2)
	require.True(t, rl.Allow("client-1"))
	require.True(t, rl.Allow("client-1"))
}

func TestRateLimiter_BlocksOverBurst(t *testing.T) {
	rl := security.NewRateLimiter(10, 2)
	require.True(t, rl.Allow("client-1"))
	require.True(t, rl.Allow("client-1"))
	require.False(t, rl.Allow("client-1"))
}

func TestRateLimiter_IsolatesClients(t *testing.T) {
	rl := security.NewRateLimiter(10, 1)
	require.True(t, rl.Allow("client-1"))
	require.False(t, rl.Allow("client-1"))
	require.True(t, rl.Allow("client-2"))
}

func TestRateLimiter_Disabled(t *testing.T) {
	rl := security.NewRateLimiter(0, 0)
	for i := 0; i < 100; i++ {
		require.True(t, rl.Allow("client-1"))
	}
}
