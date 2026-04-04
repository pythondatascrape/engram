package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the Engram MCP server for Claude Code integration",
		RunE:  runMCP,
	}
}

func runMCP(cmd *cobra.Command, args []string) error {
	serverPath, err := resolveMCPServerPath()
	if err != nil {
		return err
	}

	node := exec.CommandContext(cmd.Context(), "node", serverPath)
	node.Stdout = os.Stdout
	node.Stderr = os.Stderr
	node.Stdin = os.Stdin
	return node.Run()
}

func resolveMCPServerPath() (string, error) {
	// Allow override via environment variable.
	if root := os.Getenv("ENGRAM_PLUGIN_ROOT"); root != "" {
		p := filepath.Join(root, "server.mjs")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("server.mjs not found at ENGRAM_PLUGIN_ROOT: %s", p)
	}

	// Default: resolve relative to the running binary.
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	p := filepath.Join(filepath.Dir(exe), "..", "..", "plugins", "claude-code", "server.mjs")
	p = filepath.Clean(p)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("server.mjs not found at %s (set ENGRAM_PLUGIN_ROOT to override)", p)
	}
	return p, nil
}
