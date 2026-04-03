package install

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// ClaudeCodeConfig represents the relevant portion of ~/.claude/settings.json.
type ClaudeCodeConfig struct {
	Plugins []ClaudeCodePlugin `json:"plugins,omitempty"`
}

// ClaudeCodePlugin represents a single plugin entry in Claude Code's config.
type ClaudeCodePlugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// RegisterClaudeCode installs the engram plugin into Claude Code.
// It copies plugin files from sourceDir to ~/.claude/plugins/cache/engram/engram/<version>/.
func RegisterClaudeCode(sourceDir, version string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	targetDir := filepath.Join(home, ".claude", "plugins", "cache", "engram", "engram", version)

	if _, err := os.Stat(targetDir); err == nil {
		slog.Info("removing previous installation", "path", targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("remove old installation: %w", err)
		}
	}

	slog.Info("installing Claude Code plugin", "source", sourceDir, "target", targetDir)
	if err := copyDir(sourceDir, targetDir); err != nil {
		return fmt.Errorf("copy plugin files: %w", err)
	}

	slog.Info("Claude Code plugin installed", "path", targetDir)
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip node_modules
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	info, err := in.Stat()
	if err != nil {
		return err
	}
	return out.Chmod(info.Mode())
}

func readClaudeCodeConfig(path string) (*ClaudeCodeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ClaudeCodeConfig{}, nil
		}
		return nil, err
	}

	var cfg ClaudeCodeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	return &cfg, nil
}
