package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/pythondatascrape/engram/internal/events"
	"github.com/pythondatascrape/engram/internal/session"
)

// ShutdownConfig controls the behaviour of the ShutdownCoordinator.
type ShutdownConfig struct {
	GracePeriod      time.Duration
	NotifyClients    bool
	ForceKillTimeout time.Duration
}

// ShutdownCoordinator orchestrates the announce → drain → session-eviction phases
// of a graceful server shutdown.
type ShutdownCoordinator struct {
	sessions *session.Manager
	bus      *events.Bus
	cfg      ShutdownConfig
}

// NewShutdownCoordinator constructs a ShutdownCoordinator.
func NewShutdownCoordinator(sessions *session.Manager, bus *events.Bus, cfg ShutdownConfig) *ShutdownCoordinator {
	return &ShutdownCoordinator{
		sessions: sessions,
		bus:      bus,
		cfg:      cfg,
	}
}

// Shutdown runs the coordinated shutdown sequence:
//  1. ANNOUNCE — log and optionally broadcast "server.draining"
//  2. DRAIN    — wait for GracePeriod to let in-flight requests finish
//  3. CLOSE    — evict all remaining sessions
func (sc *ShutdownCoordinator) Shutdown(ctx context.Context) {
	// Phase 1: ANNOUNCE
	slog.Info("Shutdown initiated")
	if sc.cfg.NotifyClients {
		sc.bus.Broadcast(events.Event{
			Type:      "server.draining",
			Timestamp: time.Now(),
			Data:      map[string]any{"reason": "server shutdown"},
		})
	}

	// Phase 3: DRAIN — wait for GracePeriod
	if sc.cfg.GracePeriod > 0 {
		drainCtx, cancel := context.WithTimeout(ctx, sc.cfg.GracePeriod)
		defer cancel()
		<-drainCtx.Done()
	}

	// Phase 4: CLOSE SESSIONS
	evicted := sc.sessions.EvictAll()
	slog.Info("Sessions evicted during shutdown", "count", len(evicted))

	slog.Info("Shutdown sequence complete")
}
