package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pythondatascrape/engram/internal/config"
	"github.com/pythondatascrape/engram/internal/events"
	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/pythondatascrape/engram/internal/identity/serializer"
	"github.com/pythondatascrape/engram/internal/plugin/registry"
	"github.com/pythondatascrape/engram/internal/provider"
	"github.com/pythondatascrape/engram/internal/provider/builtin/anthropic"
	"github.com/pythondatascrape/engram/internal/provider/pool"
	"github.com/pythondatascrape/engram/internal/server"
	"github.com/pythondatascrape/engram/internal/session"
)

func main() {
	configPath := flag.String("config", "engram.yaml", "path to configuration file")
	flag.Parse()

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Set up structured logging.
	var logLevel slog.Level
	switch cfg.Logging.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// Initialize components.
	reg := registry.New()
	bus := events.NewBus()
	ser := serializer.New()

	sm := session.NewManager(session.ManagerConfig{
		IdleTimeout: cfg.Sessions.IdleTimeout,
		MaxTTL:      cfg.Sessions.MaxTTL,
		MaxSessions: cfg.Sessions.MaxSessions,
	})

	// Provider pool: factory creates Anthropic provider per API key.
	pp := pool.New(
		pool.Config{MaxConnections: cfg.Pools.DefaultMaxConnections},
		pool.Factory(func(apiKey string) (provider.Provider, error) {
			return anthropic.New(apiKey), nil
		}),
	)

	// Load default codebook if configured.
	var cb *codebook.Codebook
	if cfg.Codebooks.DefaultCodebook != "" {
		cbPath := filepath.Join(cfg.Codebooks.Directory, cfg.Codebooks.DefaultCodebook)
		data, readErr := os.ReadFile(cbPath)
		if readErr != nil {
			slog.Warn("could not read default codebook", "path", cbPath, "error", readErr)
		} else {
			parsed, parseErr := codebook.ParseWithLimits(data, cfg.Codebooks.MaxDimensions, cfg.Codebooks.MaxEnumValues)
			if parseErr != nil {
				slog.Warn("could not parse default codebook", "path", cbPath, "error", parseErr)
			} else {
				cb = parsed
				slog.Info("loaded default codebook", "name", cb.Name)
			}
		}
	}

	// Start plugins.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := reg.StartAll(ctx); err != nil {
		slog.Error("failed to start plugins", "error", err)
		os.Exit(1)
	}

	// Create handler and shutdown coordinator.
	handler := server.NewHandler(sm, ser, cb, pp)
	_ = handler

	sc := server.NewShutdownCoordinator(sm, bus, server.ShutdownConfig{
		GracePeriod:      cfg.Shutdown.GracePeriod,
		NotifyClients:    cfg.Shutdown.NotifyClients,
		ForceKillTimeout: cfg.Shutdown.ForceKillTimeout,
	})

	slog.Info("Engram server ready")

	// Wait for SIGTERM or SIGINT.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	// Graceful shutdown.
	sc.Shutdown(ctx)

	if err := reg.StopAll(ctx); err != nil {
		slog.Warn("error stopping plugins", "error", err)
	}

	slog.Info("stopped")
}
