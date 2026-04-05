package install

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// RegisterClaudeCode installs the engram plugin into Claude Code.
// It copies plugin files from sourceDir to ~/.claude/plugins/cache/engram/engram/<version>/.
func RegisterClaudeCode(sourceDir, version string) error {
	return registerPlugin(sourceDir, version, ".claude", "plugins", "cache", "engram", "engram")
}

// RegisterOpenClaw installs the engram plugin into OpenClaw.
// It copies plugin files from sourceDir to ~/.openclaw/plugins/engram/engram/<version>/.
// OpenClaw uses plugins/ directly without a cache/ layer.
func RegisterOpenClaw(sourceDir, version string) error {
	return registerPlugin(sourceDir, version, ".openclaw", "plugins", "engram", "engram")
}

// RegisterClaudeCodeWithStatusline installs the engram plugin into Claude Code
// and registers the statusLine entry in ~/.claude/settings.json.
// settingsPath defaults to ~/.claude/settings.json when empty.
func RegisterClaudeCodeWithStatusline(sourceDir, version, settingsPath string) error {
	if err := RegisterClaudeCode(sourceDir, version); err != nil {
		return err
	}
	if settingsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		settingsPath = filepath.Join(home, ".claude", "settings.json")
	}
	return MergeClaudeSettings(settingsPath, "engram statusline")
}

// registerPlugin copies sourceDir into <home>/<pathElems...>/<version>/, removing any
// previous installation first. os.RemoveAll is a no-op when the target does not exist.
func registerPlugin(sourceDir, version string, pathElems ...string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	parts := append([]string{home}, pathElems...)
	parts = append(parts, version)
	targetDir := filepath.Join(parts...)

	if _, err := os.Stat(targetDir); err == nil {
		slog.Info("removing previous installation", "path", targetDir)
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("remove old installation: %w", err)
	}

	slog.Info("installing plugin", "source", sourceDir, "target", targetDir)
	if err := copyDir(sourceDir, targetDir); err != nil {
		return fmt.Errorf("copy plugin files: %w", err)
	}

	slog.Info("plugin installed", "path", targetDir)
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

