// internal/proxy/session.go
package proxy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// mu serializes all session file writes. A single global mutex is intentional:
// this is single-user local software and per-file mutexes add complexity without
// meaningful throughput benefit.
var mu sync.Mutex

// SessionID derives a stable proxy session ID from the system prompt content.
// Uses first 8 bytes of SHA-256 so IDs are short but collision-resistant enough
// for single-user local use.
func SessionID(systemPrompt string) string {
	h := sha256.Sum256([]byte(systemPrompt))
	return fmt.Sprintf("proxy-%x", h[:8])
}

// WriteStats merges ctx_orig and ctx_comp into the session file at
// sessionsDir/<sessionID>.json. Preserves all existing fields (e.g. stop-hook stats).
// Writes are atomic via tmp+rename.
func WriteStats(sessionsDir, sessionID string, ctxOrig, ctxComp int) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	path := filepath.Join(sessionsDir, sessionID+".json")

	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing) // ignore: treat corrupt/partial file as empty
	}

	existing["ctx_orig"] = ctxOrig
	existing["ctx_comp"] = ctxComp

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session stats: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp session file: %w", err)
	}
	return os.Rename(tmp, path)
}
