package security

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter is a per-client token-bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rpm      int
	burst    int
}

// NewRateLimiter creates a RateLimiter with the given requests-per-minute and burst size.
func NewRateLimiter(rpm, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rpm:      rpm,
		burst:    burst,
	}
}

// Allow reports whether the given clientID is permitted to proceed.
// Returns true unconditionally when rpm <= 0 (disabled).
func (rl *RateLimiter) Allow(clientID string) bool {
	if rl.rpm <= 0 {
		return true
	}
	rl.mu.Lock()
	lim, ok := rl.limiters[clientID]
	if !ok {
		lim = rate.NewLimiter(rate.Every(time.Minute/time.Duration(rl.rpm)), rl.burst)
		rl.limiters[clientID] = lim
	}
	rl.mu.Unlock()
	return lim.Allow()
}
