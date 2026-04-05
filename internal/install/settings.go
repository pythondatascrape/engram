package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MergeClaudeSettings reads the Claude Code settings file at settingsPath,
// sets settings["statusLine"] = {"type":"command","command":cmd},
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

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	return os.WriteFile(settingsPath, append(out, '\n'), 0o644)
}
