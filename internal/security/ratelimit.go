package security

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is a per-client token-bucket rate limiter with optional TTL-based eviction.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	rpm     int
	burst   int
	ttl     time.Duration
}

// NewRateLimiter creates a RateLimiter with the given requests-per-minute and burst size.
// Entries are never evicted unless Evict is called explicitly or a background ticker calls it.
func NewRateLimiter(rpm, burst int) *RateLimiter {
	return NewRateLimiterWithTTL(rpm, burst, 0)
}

// NewRateLimiterWithTTL creates a RateLimiter that considers entries older than ttl as
// evictable. Set ttl = 0 to disable eviction.
func NewRateLimiterWithTTL(rpm, burst int, ttl time.Duration) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*entry),
		rpm:     rpm,
		burst:   burst,
		ttl:     ttl,
	}
}

// Allow reports whether the given clientID is permitted to proceed.
// Returns true unconditionally when rpm <= 0 (disabled).
func (rl *RateLimiter) Allow(clientID string) bool {
	if rl.rpm <= 0 {
		return true
	}
	rl.mu.Lock()
	e, ok := rl.entries[clientID]
	if !ok {
		e = &entry{
			limiter: rate.NewLimiter(rate.Every(time.Minute/time.Duration(rl.rpm)), rl.burst),
		}
		rl.entries[clientID] = e
	}
	e.lastSeen = time.Now()
	rl.mu.Unlock()
	return e.limiter.Allow()
}

// Evict removes entries that have not been seen for longer than the configured TTL.
// No-op when TTL is zero.
func (rl *RateLimiter) Evict() {
	if rl.ttl <= 0 {
		return
	}
	cutoff := time.Now().Add(-rl.ttl)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for id, e := range rl.entries {
		if e.lastSeen.Before(cutoff) {
			delete(rl.entries, id)
		}
	}
}

// Len returns the current number of tracked client entries.
func (rl *RateLimiter) Len() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.entries)
}
