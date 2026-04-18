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

// mu serializes all ctx file writes. A single global mutex is intentional:
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

// ctxStats is the on-disk structure for proxy-measured context token accounting.
type ctxStats struct {
	CtxOrig int `json:"ctx_orig"`
	CtxComp int `json:"ctx_comp"`
	Turns   int `json:"turns"`
}

// WriteStats accumulates ctx_orig and ctx_comp in sessionsDir/<sessionID>.ctx.json.
// Each call adds the current request's token counts to the running totals and
// increments the turn counter. This file is owned exclusively by the proxy —
// the stop hook writes to <sessionID>.json — so no cross-process coordination
// is needed. Writes are atomic via tmp+rename.
func WriteStats(sessionsDir, sessionID string, ctxOrig, ctxComp int) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	path := filepath.Join(sessionsDir, sessionID+".ctx.json")

	// Load existing totals (if any) and accumulate.
	var stats ctxStats
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &stats) // ignore errors; start fresh on corrupt file
	}
	stats.CtxOrig += ctxOrig
	stats.CtxComp += ctxComp
	stats.Turns++

	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ctx stats: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp ctx file: %w", err)
	}
	return os.Rename(tmp, path)
}
