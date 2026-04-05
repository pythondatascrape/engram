package install

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// MergeClaudeSettings reads the Claude Code settings file at settingsPath,
// sets settings["statusLine"] = {"type":"command","command":cmd},
// registers the Stop hook for per-session stats tracking,
// and writes it back. All other keys are preserved.
// If settingsPath does not exist, it is created along with parent directories.
func MergeClaudeSettings(settingsPath, cmd string) error {
	settings := make(map[string]any)

	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse %s: %w", settingsPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", settingsPath, err)
	}

	settings["statusLine"] = map[string]string{
		"type":    "command",
		"command": cmd,
	}

	// Register the Stop hook so per-session stats update after every response.
	// We resolve the stop hook path relative to the plugin install directory.
	pluginInstallDir := resolvePluginDir(settingsPath)
	stopHookCmd := filepath.Join(pluginInstallDir, "hooks", "stop.mjs")

	settings["hooks"] = mergeStopHook(settings["hooks"], stopHookCmd)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	slog.Info("merged claude settings", "path", settingsPath)
	return os.WriteFile(settingsPath, append(out, '\n'), 0o644)
}

// resolvePluginDir returns the path where the engram plugin is installed,
// derived from the settings file location.
func resolvePluginDir(settingsPath string) string {
	// settingsPath is typically ~/.claude/settings.json
	claudeDir := filepath.Dir(settingsPath)
	// Plugin cache path: ~/.claude/plugins/cache/engram/engram/<version>/
	// We use a glob-friendly path for the hook command so it works across versions.
	// Fallback to a stable symlink path if the versioned path isn't resolvable at install time.
	return filepath.Join(claudeDir, "plugins", "cache", "engram", "engram", "latest")
}

// mergeStopHook adds the engram Stop hook entry to the existing hooks config,
// preserving any hooks that are already registered. It is idempotent.
func mergeStopHook(existing any, stopHookCmd string) map[string]any {
	hooks := make(map[string]any)

	if m, ok := existing.(map[string]any); ok {
		for k, v := range m {
			hooks[k] = v
		}
	}

	engramStopHook := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": "node " + stopHookCmd,
			},
		},
	}

	// Append to existing Stop hooks without clobbering them.
	existing_stop, ok := hooks["Stop"]
	if !ok {
		hooks["Stop"] = []any{engramStopHook}
		return hooks
	}

	stopList, ok := existing_stop.([]any)
	if !ok {
		hooks["Stop"] = []any{engramStopHook}
		return hooks
	}

	// Check if the engram stop hook is already registered (idempotent).
	for _, entry := range stopList {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		innerHooks, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range innerHooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if c, ok := hm["command"].(string); ok && c == "node "+stopHookCmd {
				return hooks // already registered
			}
		}
	}

	hooks["Stop"] = append(stopList, engramStopHook)
	return hooks
}
