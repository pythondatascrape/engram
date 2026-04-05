package main

import (
	"os"
	"path/filepath"
)

// DefaultSocketPath returns the default Unix socket path for the Engram daemon.
func DefaultSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/engram.sock"
	}
	return filepath.Join(home, ".engram", "engram.sock")
}

// DefaultConfigPath returns the default config file path for the Engram daemon.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "engram.yaml"
	}
	return filepath.Join(home, ".engram", "engram.yaml")
}

// DefaultSessionsDir returns the default sessions directory.
func DefaultSessionsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/engram-sessions"
	}
	return filepath.Join(home, ".engram", "sessions")
}
