// internal/proxy/session_test.go
package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSessionID_DeterministicFromSystemPrompt(t *testing.T) {
	id1 := SessionID("my system prompt")
	id2 := SessionID("my system prompt")
	if id1 != id2 {
		t.Errorf("expected same ID for same prompt, got %s and %s", id1, id2)
	}
	if id1 == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestSessionID_DifferentPromptsDifferentIDs(t *testing.T) {
	id1 := SessionID("prompt A")
	id2 := SessionID("prompt B")
	if id1 == id2 {
		t.Errorf("expected different IDs for different prompts, got same: %s", id1)
	}
}

func TestSessionID_EmptyPromptStable(t *testing.T) {
	id := SessionID("")
	if id == "" {
		t.Error("expected non-empty ID even for empty prompt")
	}
}

func TestWriteStats_CreatesCtxFile(t *testing.T) {
	dir := t.TempDir()
	err := WriteStatsDetailed(dir, "test-session", turnStats{
		Total:    tokenTotals{Orig: 1000, Comp: 300, Saved: 700},
		Identity: tokenTotals{Orig: 200, Comp: 50, Saved: 150},
		Context:  tokenTotals{Orig: 800, Comp: 250, Saved: 550},
	})
	if err != nil {
		t.Fatalf("WriteStats failed: %v", err)
	}
	// Must write to .ctx.json, not .json
	ctxPath := filepath.Join(dir, "test-session.ctx.json")
	data, err := os.ReadFile(ctxPath)
	if err != nil {
		t.Fatalf("ctx file not created at %s: %v", ctxPath, err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["ctx_orig"] != float64(1000) {
		t.Errorf("expected ctx_orig=1000, got %v", got["ctx_orig"])
	}
	if got["ctx_comp"] != float64(300) {
		t.Errorf("expected ctx_comp=300, got %v", got["ctx_comp"])
	}
	identity, ok := got["identity_tokens"].(map[string]any)
	if !ok {
		t.Fatalf("expected identity_tokens object, got %T", got["identity_tokens"])
	}
	perTurn, ok := identity["per_turn"].(map[string]any)
	if !ok {
		t.Fatalf("expected identity_tokens.per_turn object, got %T", identity["per_turn"])
	}
	if perTurn["orig"] != float64(200) {
		t.Errorf("expected identity per_turn orig=200, got %v", perTurn["orig"])
	}
	context, ok := got["context_tokens"].(map[string]any)
	if !ok {
		t.Fatalf("expected context_tokens object, got %T", got["context_tokens"])
	}
	total, ok := context["total"].(map[string]any)
	if !ok {
		t.Fatalf("expected context_tokens.total object, got %T", context["total"])
	}
	if total["comp"] != float64(250) {
		t.Errorf("expected context total comp=250, got %v", total["comp"])
	}
}

func TestWriteStats_DoesNotTouchMainSessionFile(t *testing.T) {
	dir := t.TempDir()
	// Simulate a stop-hook-written session file.
	mainFile := filepath.Join(dir, "test-session.json")
	os.WriteFile(mainFile, []byte(`{"session_id":"test-session","turns":5}`), 0o600)

	if err := WriteStats(dir, "test-session", 500, 100); err != nil {
		t.Fatalf("WriteStats failed: %v", err)
	}

	// Main session file must be untouched.
	data, _ := os.ReadFile(mainFile)
	var got map[string]any
	json.Unmarshal(data, &got)
	if got["ctx_orig"] != nil {
		t.Errorf("WriteStats must not write ctx_orig to the main session file; got %v", got["ctx_orig"])
	}
	if got["turns"] != float64(5) {
		t.Errorf("main session file should be unchanged; turns=%v", got["turns"])
	}
}

func TestWriteStats_OnlyWritesCtxFields(t *testing.T) {
	dir := t.TempDir()
	if err := WriteStats(dir, "only-ctx", 999, 111); err != nil {
		t.Fatalf("WriteStats failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "only-ctx.ctx.json"))
	var got map[string]any
	json.Unmarshal(data, &got)
	for _, key := range []string{"ctx_orig", "ctx_comp", "turns", "per_turn", "total", "identity_tokens", "context_tokens"} {
		if _, ok := got[key]; !ok {
			t.Errorf("ctx file missing key %q; got keys: %v", key, got)
		}
	}
}

func TestWriteStatsDetailed_AccumulatesPerCategoryTotals(t *testing.T) {
	dir := t.TempDir()
	if err := WriteStatsDetailed(dir, "detailed", turnStats{
		Total:    tokenTotals{Orig: 100, Comp: 60},
		Identity: tokenTotals{Orig: 20, Comp: 8},
		Context:  tokenTotals{Orig: 80, Comp: 52},
	}); err != nil {
		t.Fatalf("first WriteStatsDetailed failed: %v", err)
	}
	if err := WriteStatsDetailed(dir, "detailed", turnStats{
		Total:    tokenTotals{Orig: 120, Comp: 70},
		Identity: tokenTotals{Orig: 20, Comp: 8},
		Context:  tokenTotals{Orig: 100, Comp: 62},
	}); err != nil {
		t.Fatalf("second WriteStatsDetailed failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "detailed.ctx.json"))
	if err != nil {
		t.Fatalf("read ctx file: %v", err)
	}
	var got ctxStats
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal ctx file: %v", err)
	}
	if got.Turns != 2 {
		t.Fatalf("turns = %d, want 2", got.Turns)
	}
	if got.Identity.Total.Orig != 40 || got.Identity.Total.Comp != 16 {
		t.Fatalf("identity totals = %+v, want orig=40 comp=16", got.Identity.Total)
	}
	if got.Context.Total.Orig != 180 || got.Context.Total.Comp != 114 {
		t.Fatalf("context totals = %+v, want orig=180 comp=114", got.Context.Total)
	}
	if got.PerTurn.Orig != 120 || got.PerTurn.Comp != 70 {
		t.Fatalf("per_turn total = %+v, want orig=120 comp=70", got.PerTurn)
	}
}

// TestWriteStats_PerSessionLocking verifies that writes to different sessions
// do not block each other — distinct sessions must hold independent locks.
func TestWriteStats_PerSessionLocking(t *testing.T) {
	dir := t.TempDir()
	const sessions = 20
	done := make(chan error, sessions)
	for i := 0; i < sessions; i++ {
		go func(n int) {
			done <- WriteStats(dir, fmt.Sprintf("session-%d", n), n*100, n*30)
		}(i)
	}
	for i := 0; i < sessions; i++ {
		if err := <-done; err != nil {
			t.Errorf("parallel WriteStats failed: %v", err)
		}
	}
	// Every session file must exist and be valid JSON.
	for i := 0; i < sessions; i++ {
		data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("session-%d.ctx.json", i)))
		if err != nil {
			t.Errorf("session-%d ctx file missing: %v", i, err)
			continue
		}
		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Errorf("session-%d ctx file corrupted: %v", i, err)
		}
	}
}

func TestWriteStats_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			done <- WriteStats(dir, "concurrent", n*100, n*30)
		}(i)
	}
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent WriteStats failed: %v", err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "concurrent.ctx.json"))
	if err != nil {
		t.Fatal("ctx file not created after concurrent writes")
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Errorf("ctx file corrupted after concurrent writes: %v", err)
	}
}
