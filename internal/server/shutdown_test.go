package server

import (
	"context"
	"testing"
	"time"

	"github.com/pythondatascrape/engram/internal/events"
	"github.com/pythondatascrape/engram/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownCoordinator_BroadcastAndEvict(t *testing.T) {
	// Build a session manager with one session.
	mgr := session.NewManager(session.ManagerConfig{MaxSessions: 10})
	_, err := mgr.Create(context.Background(), "client-1", session.Opts{
		Provider: "anthropic",
		Model:    "claude-3-5-haiku-20241022",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, mgr.Count(), "expected 1 session before shutdown")

	// Set up the event bus and subscribe before shutdown.
	bus := events.NewBus()
	ch := bus.Subscribe("test-subscriber", []string{"server.draining"})

	cfg := ShutdownConfig{
		GracePeriod:      100 * time.Millisecond,
		NotifyClients:    true,
		ForceKillTimeout: 200 * time.Millisecond,
	}

	coord := NewShutdownCoordinator(mgr, bus, cfg)

	// Run shutdown in a goroutine; capture when it finishes.
	done := make(chan struct{})
	go func() {
		defer close(done)
		coord.Shutdown(context.Background())
	}()

	// Verify "server.draining" event is received within a reasonable timeout.
	select {
	case evt := <-ch:
		assert.Equal(t, "server.draining", evt.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server.draining event")
	}

	// Wait for shutdown to finish.
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for shutdown to complete")
	}

	// All sessions must have been evicted.
	assert.Equal(t, 0, mgr.Count(), "expected 0 sessions after shutdown")
}

func TestShutdownCoordinator_NoNotify(t *testing.T) {
	mgr := session.NewManager(session.ManagerConfig{MaxSessions: 10})
	_, err := mgr.Create(context.Background(), "client-2", session.Opts{})
	require.NoError(t, err)

	bus := events.NewBus()
	ch := bus.Subscribe("test-subscriber-2", nil) // subscribe to all events

	cfg := ShutdownConfig{
		GracePeriod:   100 * time.Millisecond,
		NotifyClients: false,
	}

	coord := NewShutdownCoordinator(mgr, bus, cfg)

	done := make(chan struct{})
	go func() {
		defer close(done)
		coord.Shutdown(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for shutdown to complete")
	}

	// No event should have been published.
	select {
	case evt := <-ch:
		t.Fatalf("unexpected event received: %v", evt)
	default:
		// correct — no event
	}

	assert.Equal(t, 0, mgr.Count())
}
