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

// ShutdownCoordinator orchestrates graceful announce → drain → eviction.
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

// Shutdown announces, drains, and evicts all sessions.
func (sc *ShutdownCoordinator) Shutdown(ctx context.Context) {
	slog.Info("Shutdown initiated")
	if sc.cfg.NotifyClients {
		sc.bus.Broadcast(events.Event{
			Type:      "server.draining",
			Timestamp: time.Now(),
			Data:      map[string]any{"reason": "server shutdown"},
		})
	}

	if sc.cfg.GracePeriod > 0 {
		drainCtx, cancel := context.WithTimeout(ctx, sc.cfg.GracePeriod)
		defer cancel()
		<-drainCtx.Done()
	}

	evicted := sc.sessions.EvictAll()
	slog.Info("Sessions evicted during shutdown", "count", len(evicted))

	slog.Info("Shutdown sequence complete")
}
