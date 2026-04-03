package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pythondatascrape/engram/internal/config"
	"github.com/pythondatascrape/engram/internal/daemon"
	"github.com/pythondatascrape/engram/internal/events"
	"github.com/pythondatascrape/engram/internal/identity/serializer"
	"github.com/pythondatascrape/engram/internal/plugin/registry"
	"github.com/pythondatascrape/engram/internal/provider"
	"github.com/pythondatascrape/engram/internal/provider/pool"
	"github.com/pythondatascrape/engram/internal/server"
	"github.com/pythondatascrape/engram/internal/session"
	"github.com/spf13/cobra"
)

func defaultSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/engram.sock"
	}
	return filepath.Join(home, ".engram", "engram.sock")
}

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Engram daemon",
		Long:  "Start the Engram daemon process, listening on a Unix socket\nfor JSON-RPC requests from CLI clients.",
		RunE:  runServe,
	}
	cmd.Flags().String("config", "engram.yaml", "Path to configuration file")
	cmd.Flags().String("socket", defaultSocketPath(), "Unix socket path for daemon")
	cmd.Flags().Bool("install-daemon", false, "Install as a system daemon (launchd/systemd)")
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	socketPath, _ := cmd.Flags().GetString("socket")

	// Load configuration.
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Set up structured logging.
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("starting engram daemon", "socket", socketPath, "config", configPath)

	// Initialize components.
	_ = registry.New()
	_ = events.NewBus()
	ser := serializer.New()

	mgr := session.NewManager(session.ManagerConfig{
		IdleTimeout: cfg.Sessions.IdleTimeout,
		MaxTTL:      cfg.Sessions.MaxTTL,
		MaxSessions: cfg.Sessions.MaxSessions,
	})

	p := pool.New(pool.Config{
		MaxConnections: cfg.Pools.DefaultMaxConnections,
	}, func(apiKey string) (provider.Provider, error) {
		return nil, fmt.Errorf("no provider factory configured")
	})

	handler := server.NewHandler(mgr, ser, nil, p)

	// Create daemon listener and server.
	listener, err := daemon.NewListener(socketPath)
	if err != nil {
		return fmt.Errorf("create listener: %w", err)
	}

	srv := daemon.NewServerWithSessions(listener, handler, mgr)

	// Start serving in background.
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve()
	}()

	slog.Info("engram daemon ready", "socket", socketPath)

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigCh:
		slog.Info("received signal, shutting down", "signal", sig)
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve error: %w", err)
		}
	case <-ctx.Done():
	}

	srv.Stop()
	slog.Info("engram daemon stopped")
	return nil
}
