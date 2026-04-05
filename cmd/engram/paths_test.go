package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSocketPath_UnderHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := DefaultSocketPath()
	want := filepath.Join(home, ".engram", "engram.sock")
	if got != want {
		t.Errorf("DefaultSocketPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPath_UnderHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := DefaultConfigPath()
	want := filepath.Join(home, ".engram", "engram.yaml")
	if got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultSessionsDir_UnderHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := DefaultSessionsDir()
	if !strings.HasPrefix(got, home) {
		t.Errorf("DefaultSessionsDir() = %q, want prefix %q", got, home)
	}
}
