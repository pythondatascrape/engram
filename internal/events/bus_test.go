package events_test

import (
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
