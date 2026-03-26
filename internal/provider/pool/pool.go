// Package pool manages a keyed set of provider connections per API key.
package pool

import (
	"context"
	"fmt"
	"sync"

	"github.com/pythondatascrape/engram/internal/provider"
)

// Config controls pool-wide limits.
type Config struct {
	MaxConnections int
}

// Factory creates a new provider connection for the given API key.
type Factory func(apiKey string) (provider.Provider, error)

// Conn wraps a provider with the pool key for return routing.
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

type subPool struct {
	available []*Conn
	active    int
	maxConns  int
	waiters   []chan struct{}
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

// getOrCreateSubPool returns the sub-pool for key, creating if needed. Caller must hold p.mu.
func (p *Pool) getOrCreateSubPool(key string) *subPool {
	sp, ok := p.pools[key]
	if !ok {
		sp = &subPool{maxConns: p.cfg.MaxConnections}
		p.pools[key] = sp
	}
	return sp
}

// Get returns a connection for apiKey, reusing idle connections, creating new
// ones under the limit, or blocking until one is returned or ctx is cancelled.
func (p *Pool) Get(ctx context.Context, apiKey string) (*Conn, error) {
	for {
		p.mu.Lock()
		sp := p.getOrCreateSubPool(apiKey)

		if len(sp.available) > 0 {
			conn := sp.available[len(sp.available)-1]
			sp.available = sp.available[:len(sp.available)-1]
			sp.active++
			p.mu.Unlock()
			return conn, nil
		}

		if sp.active < sp.maxConns {
			sp.active++
			p.mu.Unlock()

			prov, err := p.factory(apiKey)
			if err != nil {
				p.mu.Lock()
				sp.active--
				p.mu.Unlock()
				return nil, fmt.Errorf("pool: factory error for key %q: %w", apiKey, err)
			}
			return &Conn{Provider: prov, key: apiKey}, nil
		}

		wait := make(chan struct{}, 1)
		sp.waiters = append(sp.waiters, wait)
		p.mu.Unlock()

		select {
		case <-ctx.Done():
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
		return
	}

	sp.active--
	sp.available = append(sp.available, conn)

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
