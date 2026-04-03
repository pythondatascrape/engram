// Package install provides client detection and plugin registration.
package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Client represents a detected client installation.
type Client struct {
	Name string // "claude-code" or "openclaw"
	Dir  string // installation directory
}

// DetectClaudeCode checks if Claude Code is installed by looking for ~/.claude/.
func DetectClaudeCode() (*Client, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, false
	}

	claudeDir := filepath.Join(home, ".claude")
	info, err := os.Stat(claudeDir)
	if err != nil || !info.IsDir() {
		return nil, false
	}

	return &Client{Name: "claude-code", Dir: claudeDir}, true
}

// DetectOpenClaw checks if OpenClaw is installed via ~/.openclaw/ or PATH.
func DetectOpenClaw() (*Client, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, false
	}

	openclawDir := filepath.Join(home, ".openclaw")
	info, err := os.Stat(openclawDir)
	if err == nil && info.IsDir() {
		return &Client{Name: "openclaw", Dir: openclawDir}, true
	}

	path, err := exec.LookPath("openclaw")
	if err == nil {
		return &Client{Name: "openclaw", Dir: filepath.Dir(path)}, true
	}

	return nil, false
}

// DetectAll returns all detected clients.
func DetectAll() []Client {
	var clients []Client
	if c, ok := DetectClaudeCode(); ok {
		clients = append(clients, *c)
	}
	if c, ok := DetectOpenClaw(); ok {
		clients = append(clients, *c)
	}
	return clients
}

// PluginSourceDir returns the relative path to plugin files for a client.
func PluginSourceDir(clientName string) (string, error) {
	switch clientName {
	case "claude-code":
		return "plugins/claude-code", nil
	case "openclaw":
		return "plugins/openclaw", nil
	default:
		return "", fmt.Errorf("unknown client: %s", clientName)
	}
}
