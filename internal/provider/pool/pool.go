// Package pool manages a keyed set of provider connections.
// Each API key gets its own sub-pool so that connections are never
// shared across tenants.
package pool

import (
	"context"
	"fmt"
	"sync"

	"github.com/pythondatascrape/engram/internal/provider"
)

// Config controls pool-wide limits.
type Config struct {
	// MaxConnections is the maximum number of live provider connections
	// that may exist for a single API key at one time.
	MaxConnections int
}

// Factory is the function used to create a new provider connection.
type Factory func(apiKey string) (provider.Provider, error)

// Conn wraps a provider together with the pool key so that Return knows
// which sub-pool to put it back into.
type Conn struct {
	Provider provider.Provider
	key      string
}

// Stats is a snapshot of a single sub-pool.
type Stats struct {
	Key       string
	Active    int
	Available int
	MaxConns  int
}

// subPool tracks the connections for one API key.
type subPool struct {
	available []*Conn
	active    int
	maxConns  int
	// waiters is notified each time a connection is returned.
	waiters []chan struct{}
}

// Pool is a concurrency-safe connection pool keyed by API key.
type Pool struct {
	mu      sync.Mutex
	pools   map[string]*subPool
	cfg     Config
	factory Factory
}

// New creates a new Pool.
func New(cfg Config, factory Factory) *Pool {
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = 1
	}
	return &Pool{
		pools:   make(map[string]*subPool),
		cfg:     cfg,
		factory: factory,
	}
}

// getOrCreateSubPool returns the sub-pool for key, creating it if needed.
// Caller must hold p.mu.
func (p *Pool) getOrCreateSubPool(key string) *subPool {
	sp, ok := p.pools[key]
	if !ok {
		sp = &subPool{maxConns: p.cfg.MaxConnections}
		p.pools[key] = sp
	}
	return sp
}

// Get returns a connection for apiKey.  If a connection is available it is
// returned immediately.  If the pool is under its limit a new connection is
// created via the factory.  Otherwise Get blocks until a connection is
// returned or ctx is cancelled.
func (p *Pool) Get(ctx context.Context, apiKey string) (*Conn, error) {
	for {
		p.mu.Lock()
		sp := p.getOrCreateSubPool(apiKey)

		// Return an idle connection if one exists.
		if len(sp.available) > 0 {
			conn := sp.available[len(sp.available)-1]
			sp.available = sp.available[:len(sp.available)-1]
			sp.active++
			p.mu.Unlock()
			return conn, nil
		}

		// Create a new connection if under the limit.
		if sp.active < sp.maxConns {
			sp.active++
			p.mu.Unlock()

			prov, err := p.factory(apiKey)
			if err != nil {
				// Roll back the active count.
				p.mu.Lock()
				sp.active--
				p.mu.Unlock()
				return nil, fmt.Errorf("pool: factory error for key %q: %w", apiKey, err)
			}
			return &Conn{Provider: prov, key: apiKey}, nil
		}

		// Pool is exhausted — register a waiter channel and block.
		wait := make(chan struct{}, 1)
		sp.waiters = append(sp.waiters, wait)
		p.mu.Unlock()

		select {
		case <-ctx.Done():
			// Remove our waiter so it is not notified after we leave.
			p.mu.Lock()
			sp2 := p.pools[apiKey]
			if sp2 != nil {
				for i, w := range sp2.waiters {
					if w == wait {
						sp2.waiters = append(sp2.waiters[:i], sp2.waiters[i+1:]...)
						break
					}
				}
			}
			p.mu.Unlock()
			return nil, fmt.Errorf("pool: context cancelled while waiting for connection: %w", ctx.Err())

		case <-wait:
			// A connection was returned; loop back and try again.
		}
	}
}

// Return puts conn back into the pool and notifies any waiting goroutines.
func (p *Pool) Return(conn *Conn) {
	if conn == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	sp, ok := p.pools[conn.key]
	if !ok {
		// Sub-pool was somehow removed; just discard.
		return
	}

	sp.active--
	sp.available = append(sp.available, conn)

	// Wake the oldest waiter, if any.
	if len(sp.waiters) > 0 {
		w := sp.waiters[0]
		sp.waiters = sp.waiters[1:]
		w <- struct{}{}
	}
}

// AllStats returns a snapshot of all sub-pools.
func (p *Pool) AllStats() []Stats {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make([]Stats, 0, len(p.pools))
	for key, sp := range p.pools {
		out = append(out, Stats{
			Key:       key,
			Active:    sp.active,
			Available: len(sp.available),
			MaxConns:  sp.maxConns,
		})
	}
	return out
}
