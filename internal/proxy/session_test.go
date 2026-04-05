// internal/proxy/session_test.go
package proxy

import (
	"encoding/json"
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
	err := WriteStats(dir, "test-session", 1000, 300)
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
	// Exactly two keys.
	if len(got) != 2 {
		t.Errorf("ctx file should contain exactly ctx_orig and ctx_comp, got keys: %v", got)
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
