package security

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is a per-client token-bucket rate limiter with optional TTL-based eviction.
type RateLimiter struct {
	mu         sync.Mutex
	limiters   map[string]*entry
	ipLimiters map[string]*entry
	rpm        int
	burst      int
	ttl        time.Duration
}

// NewRateLimiter creates a RateLimiter with the given requests-per-minute and burst size.
func NewRateLimiter(rpm, burst int) *RateLimiter {
	return NewRateLimiterWithTTL(rpm, burst, 10*time.Minute)
}

// NewRateLimiterWithTTL creates a RateLimiter whose idle entries are evicted after ttl.
func NewRateLimiterWithTTL(rpm, burst int, ttl time.Duration) *RateLimiter {
	return &RateLimiter{
		limiters:   make(map[string]*entry),
		ipLimiters: make(map[string]*entry),
		rpm:        rpm,
		burst:      burst,
		ttl:        ttl,
	}
}

func (rl *RateLimiter) newLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Every(time.Minute/time.Duration(rl.rpm)), rl.burst)
}

// Allow reports whether the given clientID is permitted to proceed.
// Returns true unconditionally when rpm <= 0 (disabled).
func (rl *RateLimiter) Allow(clientID string) bool {
	if rl.rpm <= 0 {
		return true
	}
	rl.mu.Lock()
	e, ok := rl.limiters[clientID]
	if !ok {
		e = &entry{lim: rl.newLimiter()}
		rl.limiters[clientID] = e
	}
	e.lastSeen = time.Now()
	rl.mu.Unlock()
	return e.lim.Allow()
}

// AllowIP reports whether the given IP address is permitted to proceed.
// Returns true unconditionally when rpm <= 0 (disabled).
func (rl *RateLimiter) AllowIP(ip string) bool {
	if rl.rpm <= 0 {
		return true
	}
	rl.mu.Lock()
	e, ok := rl.ipLimiters[ip]
	if !ok {
		e = &entry{lim: rl.newLimiter()}
		rl.ipLimiters[ip] = e
	}
	e.lastSeen = time.Now()
	rl.mu.Unlock()
	return e.lim.Allow()
}

// EvictStale removes entries that have been idle longer than the configured TTL.
func (rl *RateLimiter) EvictStale() {
	threshold := time.Now().Add(-rl.ttl)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for k, e := range rl.limiters {
		if e.lastSeen.Before(threshold) {
			delete(rl.limiters, k)
		}
	}
	for k, e := range rl.ipLimiters {
		if e.lastSeen.Before(threshold) {
			delete(rl.ipLimiters, k)
		}
	}
}

// Evict is an alias for EvictStale.
func (rl *RateLimiter) Evict() { rl.EvictStale() }

// LimiterCount returns the number of tracked client entries (for testing).
func (rl *RateLimiter) LimiterCount() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.limiters)
}

// Len is an alias for LimiterCount.
func (rl *RateLimiter) Len() int { return rl.LimiterCount() }

// IPLimiterCount returns the number of tracked IP entries (for testing).
func (rl *RateLimiter) IPLimiterCount() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.ipLimiters)
}
