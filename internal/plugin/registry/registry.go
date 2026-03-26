// Package registry provides a thread-safe store for Engram plugins.
package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/pythondatascrape/engram/internal/plugin"
)

// Registry is a thread-safe collection of plugins indexed by name.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]plugin.Plugin
	order   []string // insertion order for deterministic iteration
}

// New returns an empty, ready-to-use Registry.
func New() *Registry {
	return &Registry{
		plugins: make(map[string]plugin.Plugin),
	}
}

// Register adds a plugin. Returns an error if the name is already taken.
func (r *Registry) Register(p plugin.Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %q is already registered", name)
	}
	r.plugins[name] = p
	r.order = append(r.order, name)
	return nil
}

// Get returns the plugin with the given name, or an error if not found.
func (r *Registry) Get(name string) (plugin.Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	return p, nil
}

// ListByType returns all registered plugins of the given type.
func (r *Registry) ListByType(t plugin.Type) []plugin.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []plugin.Plugin
	for _, name := range r.order {
		if p := r.plugins[name]; p.Type() == t {
			result = append(result, p)
		}
	}
	return result
}

// All returns every registered plugin in insertion order.
func (r *Registry) All() []plugin.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]plugin.Plugin, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.plugins[name])
	}
	return result
}

// StartAll starts every plugin in insertion order, returning the first error.
func (r *Registry) StartAll(ctx context.Context) error {
	for _, p := range r.All() {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops every plugin; returns the first error seen.
func (r *Registry) StopAll(ctx context.Context) error {
	var firstErr error
	for _, p := range r.All() {
		if err := p.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Deregister removes the named plugin, or returns an error if not found.
func (r *Registry) Deregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.plugins[name]; !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	delete(r.plugins, name)

	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}
