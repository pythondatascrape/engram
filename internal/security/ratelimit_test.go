package security_test

import (
	"testing"
	"time"

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

func TestRateLimiter_EvictsIdleEntries(t *testing.T) {
	ttl := 50 * time.Millisecond
	rl := security.NewRateLimiterWithTTL(10, 1, ttl)
	require.True(t, rl.Allow("client-1"))
	require.False(t, rl.Allow("client-1"))

	time.Sleep(ttl * 3)
	rl.Evict()

	require.True(t, rl.Allow("client-1"))
}

func TestRateLimiter_LenAfterEviction(t *testing.T) {
	ttl := 50 * time.Millisecond
	rl := security.NewRateLimiterWithTTL(10, 1, ttl)
	rl.Allow("a")
	rl.Allow("b")
	rl.Allow("c")
	require.Equal(t, 3, rl.Len())

	time.Sleep(ttl * 3)
	rl.Evict()
	require.Equal(t, 0, rl.Len())
}

func TestRateLimiter_ActiveEntryNotEvicted(t *testing.T) {
	ttl := 100 * time.Millisecond
	rl := security.NewRateLimiterWithTTL(60, 5, ttl)
	rl.Allow("active")
	rl.Allow("idle")

	time.Sleep(ttl / 2)
	rl.Allow("active")

	time.Sleep(ttl / 2)
	rl.Evict()

	require.Equal(t, 1, rl.Len())
}

func TestRateLimiter_AllowIP_BlocksOverBurst(t *testing.T) {
	rl := security.NewRateLimiter(10, 1)
	require.True(t, rl.AllowIP("192.0.2.1"))
	require.False(t, rl.AllowIP("192.0.2.1"))
}

func TestRateLimiter_AllowIP_IsolatesAddresses(t *testing.T) {
	rl := security.NewRateLimiter(10, 1)
	require.True(t, rl.AllowIP("192.0.2.1"))
	require.False(t, rl.AllowIP("192.0.2.1"))
	require.True(t, rl.AllowIP("192.0.2.2"))
}

func TestRateLimiter_AllowIP_Disabled(t *testing.T) {
	rl := security.NewRateLimiter(0, 0)
	for i := 0; i < 50; i++ {
		require.True(t, rl.AllowIP("192.0.2.1"))
	}
}

func TestRateLimiter_IPAndClientIndependent(t *testing.T) {
	rl := security.NewRateLimiter(10, 1)
	require.True(t, rl.AllowIP("192.0.2.1"))
	require.False(t, rl.AllowIP("192.0.2.1"))
	require.True(t, rl.Allow("192.0.2.1"))
}

func TestRateLimiter_EvictsStaleIPEntries(t *testing.T) {
	rl := security.NewRateLimiterWithTTL(10, 1, 50*time.Millisecond)
	rl.AllowIP("192.0.2.1")
	time.Sleep(100 * time.Millisecond)
	rl.EvictStale()
	require.Equal(t, 0, rl.IPLimiterCount())
}
