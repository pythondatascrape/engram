package events

import (
	"sync"
	"time"
)

// Event represents a typed event pushed to clients.
type Event struct {
	Type      string
	Timestamp time.Time
	Data      map[string]interface{}
}

// subscriber holds the delivery channel and optional type filter for one client.
type subscriber struct {
	ch      chan Event
	filters map[string]bool // nil means accept all event types
}

// Bus is a thread-safe, in-process event bus.
type Bus struct {
	sync.RWMutex
	subscribers map[string]*subscriber
}

// NewBus creates a ready-to-use Bus.
func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[string]*subscriber),
	}
}

// Subscribe registers clientID and returns a channel on which the client will
// receive events. Pass nil (or empty) types to receive all event types.
func (b *Bus) Subscribe(clientID string, types []string) <-chan Event {
	b.Lock()
	defer b.Unlock()

	var filters map[string]bool
	if len(types) > 0 {
		filters = make(map[string]bool, len(types))
		for _, t := range types {
			filters[t] = true
		}
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
	b.Lock()
	defer b.Unlock()

	if sub, ok := b.subscribers[clientID]; ok {
		close(sub.ch)
		delete(b.subscribers, clientID)
	}
}

// Publish sends evt to the specific client. The event is dropped (non-blocking)
// if the client's buffer is full or the type is filtered out.
func (b *Bus) Publish(evt Event, clientID string) {
	b.RLock()
	sub, ok := b.subscribers[clientID]
	b.RUnlock()

	if !ok {
		return
	}
	deliver(sub, evt)
}

// Broadcast sends evt to every subscriber, respecting per-subscriber filters.
func (b *Bus) Broadcast(evt Event) {
	b.RLock()
	subs := make([]*subscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subs = append(subs, sub)
	}
	b.RUnlock()

	for _, sub := range subs {
		deliver(sub, evt)
	}
}

// deliver sends an event to a subscriber, dropping it if the buffer is full or
// the event type is filtered out.
func deliver(sub *subscriber, evt Event) {
	if sub.filters != nil && !sub.filters[evt.Type] {
		return
	}
	select {
	case sub.ch <- evt:
	default:
		// buffer full — drop event
	}
}
