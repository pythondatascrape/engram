package install

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// engramPluginPathFragment identifies hook commands that point into the engram
// plugin install directory (see resolvePluginDir).
var engramPluginPathFragment = filepath.Join("plugins", "cache", "engram", "engram") + string(filepath.Separator)

// UnregisterClaudeCode removes the engram plugin from ~/.claude/plugins/cache/engram/.
// It is a no-op when the plugin is not installed.
func UnregisterClaudeCode() error {
	return unregisterPlugin(".claude", "plugins", "cache", "engram")
}

// UnregisterOpenClaw removes the engram plugin from ~/.openclaw/plugins/engram/.
// It is a no-op when the plugin is not installed.
func UnregisterOpenClaw() error {
	return unregisterPlugin(".openclaw", "plugins", "engram")
}

func unregisterPlugin(pathElems ...string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	target := filepath.Join(append([]string{home}, pathElems...)...)
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove %s: %w", target, err)
	}
	slog.Info("plugin removed", "path", target)
	return nil
}

// UnregisterClaudeSettings reverses MergeClaudeSettings and MergeProxySettings
// on the Claude Code settings file at settingsPath (default ~/.claude/settings.json).
// Only entries engram wrote are removed:
//   - statusLine, when its command is "engram statusline"
//   - Stop hooks pointing into the engram plugin directory
//   - env.ANTHROPIC_BASE_URL / env.OPENAI_BASE_URL, when they point at one of proxyPorts on localhost
//   - requestHeaders.X-Engram-Session
//
// Maps left empty by the removal are dropped. All other keys are preserved.
// It reports whether the file was changed; a missing file is not an error.
func UnregisterClaudeSettings(settingsPath string, proxyPorts ...int) (bool, error) {
	sp, err := resolveSettingsPath(settingsPath)
	if err != nil {
		return false, err
	}

	data, err := os.ReadFile(sp)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("read %s: %w", sp, err)
	}

	settings := make(map[string]any)
	if err := json.Unmarshal(data, &settings); err != nil {
		return false, fmt.Errorf("parse %s: %w", sp, err)
	}

	changed := false

	if sl, ok := settings["statusLine"].(map[string]any); ok {
		if cmd, _ := sl["command"].(string); cmd == "engram statusline" {
			delete(settings, "statusLine")
			changed = true
		}
	}

	if hooks, ok := settings["hooks"].(map[string]any); ok {
		if stop, ok := hooks["Stop"].([]any); ok {
			kept, removed := removeEngramHooks(stop)
			if removed {
				changed = true
				if len(kept) == 0 {
					delete(hooks, "Stop")
				} else {
					hooks["Stop"] = kept
				}
			}
		}
		if len(hooks) == 0 {
			delete(settings, "hooks")
		}
	}

	if env, ok := settings["env"].(map[string]any); ok {
		for _, key := range []string{"ANTHROPIC_BASE_URL", "OPENAI_BASE_URL"} {
			if v, _ := env[key].(string); isEngramProxyURL(v, proxyPorts) {
				delete(env, key)
				changed = true
			}
		}
		if len(env) == 0 {
			delete(settings, "env")
		}
	}

	if rh, ok := settings["requestHeaders"].(map[string]any); ok {
		if _, ok := rh["X-Engram-Session"]; ok {
			delete(rh, "X-Engram-Session")
			changed = true
		}
		if len(rh) == 0 {
			delete(settings, "requestHeaders")
		}
	}

	if !changed {
		return false, nil
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal settings: %w", err)
	}
	tmp := sp + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
		return false, fmt.Errorf("write tmp settings: %w", err)
	}
	if err := os.Rename(tmp, sp); err != nil {
		return false, fmt.Errorf("replace settings: %w", err)
	}
	slog.Info("removed engram entries from claude settings", "path", sp)
	return true, nil
}

// removeEngramHooks drops hook commands that point into the engram plugin
// directory, and any hook groups left empty as a result.
func removeEngramHooks(groups []any) (kept []any, removed bool) {
	for _, g := range groups {
		m, ok := g.(map[string]any)
		if !ok {
			kept = append(kept, g)
			continue
		}
		inner, ok := m["hooks"].([]any)
		if !ok {
			kept = append(kept, g)
			continue
		}
		var keptInner []any
		for _, h := range inner {
			if hm, ok := h.(map[string]any); ok {
				if c, _ := hm["command"].(string); strings.Contains(c, engramPluginPathFragment) {
					removed = true
					continue
				}
			}
			keptInner = append(keptInner, h)
		}
		if len(keptInner) == 0 {
			continue
		}
		m["hooks"] = keptInner
		kept = append(kept, m)
	}
	return kept, removed
}

func isEngramProxyURL(v string, ports []int) bool {
	for _, p := range ports {
		if v == fmt.Sprintf("http://localhost:%d", p) {
			return true
		}
	}
	return false
}
