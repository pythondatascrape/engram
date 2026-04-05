package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/pythondatascrape/engram/internal/config"
	"github.com/pythondatascrape/engram/internal/daemon"
	"github.com/pythondatascrape/engram/internal/events"
	"github.com/pythondatascrape/engram/internal/identity/serializer"
	"github.com/pythondatascrape/engram/internal/plugin/registry"
	"github.com/pythondatascrape/engram/internal/provider"
	"github.com/pythondatascrape/engram/internal/provider/pool"
	"github.com/pythondatascrape/engram/internal/proxy"
	"github.com/pythondatascrape/engram/internal/server"
	"github.com/pythondatascrape/engram/internal/session"
	"github.com/pythondatascrape/engram/internal/updater"
	"github.com/spf13/cobra"
)

// daemonize re-executes the current binary as a background child process,
// detached from the terminal, then returns so the parent can exit.
func daemonize(configPath, socketPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".engram", "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	logFile, err := os.OpenFile(filepath.Join(logDir, "engram.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	cmd := exec.Command(exe, "serve", "--foreground", "--config", configPath, "--socket", socketPath)
	cmd.Env = append(os.Environ(), daemonChildEnv+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	fmt.Fprintf(os.Stderr, "engram daemon started (pid %d), logging to %s\n", cmd.Process.Pid, logFile.Name())
	return nil
}

// daemonChildEnv is set on the child process to prevent re-daemonizing.
const daemonChildEnv = "ENGRAM_DAEMON_CHILD"

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
		Long:  "Start the Engram daemon process, listening on a Unix socket\nfor JSON-RPC requests from CLI clients.\n\nBy default, engram daemonizes itself and returns immediately.\nUse --foreground to keep it attached to the terminal.",
		RunE:  runServe,
	}
	cmd.Flags().String("config", "engram.yaml", "Path to configuration file")
	cmd.Flags().String("socket", defaultSocketPath(), "Unix socket path for daemon")
	cmd.Flags().Bool("install-daemon", false, "Install as a system daemon (launchd/systemd)")
	cmd.Flags().Bool("foreground", false, "Run in foreground instead of daemonizing")
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	installDaemon, _ := cmd.Flags().GetBool("install-daemon")
	if installDaemon {
		return installService(cmd)
	}

	foreground, _ := cmd.Flags().GetBool("foreground")
	configPath, _ := cmd.Flags().GetString("config")
	socketPath, _ := cmd.Flags().GetString("socket")

	// Daemonize unless --foreground or already a child process.
	if !foreground && os.Getenv(daemonChildEnv) == "" {
		return daemonize(configPath, socketPath)
	}

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

	// Check for updates in the background; writes ~/.engram/.update-available if found.
	go updater.CheckAndNotify(Version)

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

	home, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(home, ".engram", "sessions")
	proxySrv := proxy.New(cfg.Proxy.Port, cfg.Proxy.WindowSize, sessionsDir, "https://api.anthropic.com")
	if err := proxySrv.Start(); err != nil {
		return fmt.Errorf("start proxy on :%d: %w", cfg.Proxy.Port, err)
	}
	slog.Info("proxy listening", "port", cfg.Proxy.Port)

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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = proxySrv.Stop(shutdownCtx)

	srv.Stop()
	slog.Info("engram daemon stopped")
	return nil
}
