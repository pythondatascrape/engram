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

// sessionMu provides per-session-ID locking so concurrent writes to different
// sessions do not block each other.
var sessionMu struct {
	sync.Mutex
	m map[string]*sync.Mutex
}

func sessionLock(id string) func() {
	sessionMu.Lock()
	if sessionMu.m == nil {
		sessionMu.m = make(map[string]*sync.Mutex)
	}
	mu, ok := sessionMu.m[id]
	if !ok {
		mu = &sync.Mutex{}
		sessionMu.m[id] = mu
	}
	sessionMu.Unlock()
	mu.Lock()
	return mu.Unlock
}

// SessionID derives a stable proxy session ID from the system prompt content.
// Uses first 8 bytes of SHA-256 so IDs are short but collision-resistant enough
// for single-user local use.
func SessionID(systemPrompt string) string {
	h := sha256.Sum256([]byte(systemPrompt))
	return fmt.Sprintf("proxy-%x", h[:8])
}

// ctxStats is the on-disk structure for proxy-measured context token accounting.
type ctxStats struct {
	CtxOrig  int         `json:"ctx_orig"`
	CtxComp  int         `json:"ctx_comp"`
	Turns    int         `json:"turns"`
	PerTurn  tokenTotals `json:"per_turn,omitempty"`
	Total    tokenTotals `json:"total,omitempty"`
	Identity tokenSeries `json:"identity_tokens,omitempty"`
	Context  tokenSeries `json:"context_tokens,omitempty"`
}

type tokenTotals struct {
	Orig  int `json:"orig"`
	Comp  int `json:"comp"`
	Saved int `json:"saved"`
}

type tokenSeries struct {
	PerTurn tokenTotals `json:"per_turn"`
	Total   tokenTotals `json:"total"`
}

type turnStats struct {
	Total    tokenTotals
	Identity tokenTotals
	Context  tokenTotals
}

func clampSaved(orig, comp int) int {
	if orig <= comp {
		return 0
	}
	return orig - comp
}

// WriteStats accumulates ctx_orig and ctx_comp in sessionsDir/<sessionID>.ctx.json.
// Each call adds the current request's token counts to the running totals and
// increments the turn counter. This file is owned exclusively by the proxy —
// the stop hook writes to <sessionID>.json — so no cross-process coordination
// is needed. Writes are atomic via tmp+rename.
func WriteStats(sessionsDir, sessionID string, ctxOrig, ctxComp int) error {
	return WriteStatsDetailed(sessionsDir, sessionID, turnStats{
		Total: tokenTotals{
			Orig:  ctxOrig,
			Comp:  ctxComp,
			Saved: clampSaved(ctxOrig, ctxComp),
		},
	})
}

// WriteStatsDetailed records both exact total per-turn token usage and
// estimated identity/context breakdowns for richer session analytics.
func WriteStatsDetailed(sessionsDir, sessionID string, turn turnStats) error {
	unlock := sessionLock(sessionID)
	defer unlock()

	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	path := filepath.Join(sessionsDir, sessionID+".ctx.json")

	// Load existing totals (if any) and accumulate.
	var stats ctxStats
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &stats) // ignore errors; start fresh on corrupt file
	}
	stats.CtxOrig += turn.Total.Orig
	stats.CtxComp += turn.Total.Comp
	stats.Turns++
	stats.PerTurn = tokenTotals{
		Orig:  turn.Total.Orig,
		Comp:  turn.Total.Comp,
		Saved: clampSaved(turn.Total.Orig, turn.Total.Comp),
	}
	stats.Total = tokenTotals{
		Orig:  stats.CtxOrig,
		Comp:  stats.CtxComp,
		Saved: clampSaved(stats.CtxOrig, stats.CtxComp),
	}
	stats.Identity.PerTurn = tokenTotals{
		Orig:  turn.Identity.Orig,
		Comp:  turn.Identity.Comp,
		Saved: clampSaved(turn.Identity.Orig, turn.Identity.Comp),
	}
	stats.Identity.Total.Orig += turn.Identity.Orig
	stats.Identity.Total.Comp += turn.Identity.Comp
	stats.Identity.Total.Saved = clampSaved(stats.Identity.Total.Orig, stats.Identity.Total.Comp)
	stats.Context.PerTurn = tokenTotals{
		Orig:  turn.Context.Orig,
		Comp:  turn.Context.Comp,
		Saved: clampSaved(turn.Context.Orig, turn.Context.Comp),
	}
	stats.Context.Total.Orig += turn.Context.Orig
	stats.Context.Total.Comp += turn.Context.Comp
	stats.Context.Total.Saved = clampSaved(stats.Context.Total.Orig, stats.Context.Total.Comp)

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
