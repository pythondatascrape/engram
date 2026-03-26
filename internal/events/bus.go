package events

import (
	"sync"
	"time"
)

// Event represents a typed event pushed to clients.
type Event struct {
	Type      string
	Timestamp time.Time
	Data      map[string]any
}

// subscriber holds the delivery channel and optional type filter for one client.
type subscriber struct {
	ch      chan Event
	filters map[string]bool // nil means accept all event types
}

// Bus is a thread-safe, in-process event bus.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string]*subscriber
}

// NewBus creates a ready-to-use Bus.
func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[string]*subscriber),
	}
}

// Subscribe registers clientID and returns an event channel. Pass nil types to receive all.
func (b *Bus) Subscribe(clientID string, types []string) <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	var filters map[string]bool
	if len(types) > 0 {
		filters = make(map[string]bool, len(types))
		for _, t := range types {
			filters[t] = true
		}
	}

	if old, ok := b.subscribers[clientID]; ok {
		close(old.ch)
	}

	sub := &subscriber{
		ch:      make(chan Event, 64),
		filters: filters,
	}
	b.subscribers[clientID] = sub
	return sub.ch
}

// Unsubscribe closes the client's channel and removes them from the bus.
func (b *Bus) Unsubscribe(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, ok := b.subscribers[clientID]; ok {
		close(sub.ch)
		delete(b.subscribers, clientID)
	}
}

// Publish sends evt to a specific client (non-blocking, drops if buffer full).
func (b *Bus) Publish(evt Event, clientID string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	sub, ok := b.subscribers[clientID]
	if !ok {
		return
	}
	deliver(sub, evt)
}

// Broadcast sends evt to every subscriber, respecting per-subscriber filters.
// Delivery happens under RLock so that Unsubscribe (which requires the write
// lock) cannot close a channel while we are sending to it.
func (b *Bus) Broadcast(evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		deliver(sub, evt)
	}
}

// deliver sends an event to a subscriber, dropping if filtered or buffer full.
// A recover guards against any residual send-on-closed-channel panic.
func deliver(sub *subscriber, evt Event) {
	if sub.filters != nil && !sub.filters[evt.Type] {
		return
	}
	defer func() { recover() }()
	select {
	case sub.ch <- evt:
	default:
	}
}
