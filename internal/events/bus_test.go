package events_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pythondatascrape/engram/internal/events"
)

func makeEvent(t string) events.Event {
	return events.Event{
		Type:      t,
		Timestamp: time.Now(),
		Data:      map[string]any{"key": "value"},
	}
}

func TestPublishAndSubscribe(t *testing.T) {
	bus := events.NewBus()

	ch := bus.Subscribe("client1", nil)
	evt := makeEvent("server.draining")
	bus.Publish(evt, "client1")

	select {
	case received := <-ch:
		if received.Type != "server.draining" {
			t.Fatalf("expected server.draining, got %s", received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestSubscribeWithFilter(t *testing.T) {
	bus := events.NewBus()

	ch := bus.Subscribe("client2", []string{"session.expiring"})

	// Publish a non-matching event — should be filtered out
	bus.Publish(makeEvent("provider.degraded"), "client2")
	// Publish a matching event
	bus.Publish(makeEvent("session.expiring"), "client2")

	select {
	case received := <-ch:
		if received.Type != "session.expiring" {
			t.Fatalf("expected session.expiring, got %s", received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for filtered event")
	}

	// Channel should now be empty (the non-matching event was dropped)
	select {
	case extra := <-ch:
		t.Fatalf("unexpected extra event: %s", extra.Type)
	default:
		// good — channel is empty
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := events.NewBus()

	ch := bus.Subscribe("client3", nil)
	bus.Unsubscribe("client3")

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestPublishToNonExistentClient(t *testing.T) {
	bus := events.NewBus()
	// Should not panic — event is silently dropped.
	bus.Publish(makeEvent("test.event"), "nobody")
}

func TestResubscribe(t *testing.T) {
	bus := events.NewBus()

	// First subscription.
	ch1 := bus.Subscribe("client-re", []string{"a"})

	// Re-subscribing should close the old channel.
	ch2 := bus.Subscribe("client-re", []string{"b"})

	// Old channel should be closed.
	select {
	case _, ok := <-ch1:
		if ok {
			t.Fatal("expected old channel to be closed after re-subscribe")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for old channel close")
	}

	// New channel should work.
	bus.Publish(makeEvent("b"), "client-re")
	select {
	case received := <-ch2:
		if received.Type != "b" {
			t.Fatalf("expected type b, got %s", received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event on new channel")
	}
}

func TestBroadcastDuringUnsubscribe_NoPanic(t *testing.T) {
	bus := events.NewBus()
	for i := 0; i < 100; i++ {
		bus.Subscribe(fmt.Sprintf("client-%d", i), nil)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			bus.Broadcast(makeEvent("test.event"))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			bus.Unsubscribe(fmt.Sprintf("client-%d", i))
		}
	}()
	wg.Wait()
}

func TestBroadcast(t *testing.T) {
	bus := events.NewBus()

	ch1 := bus.Subscribe("clientA", nil)
	ch2 := bus.Subscribe("clientB", nil)

	evt := makeEvent("server.draining")
	bus.Broadcast(evt)

	for _, ch := range []<-chan events.Event{ch1, ch2} {
		select {
		case received := <-ch:
			if received.Type != "server.draining" {
				t.Fatalf("expected server.draining, got %s", received.Type)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for broadcast event")
		}
	}
}
