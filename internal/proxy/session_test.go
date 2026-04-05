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

func TestWriteStats_CreatesFileWithCtxFields(t *testing.T) {
	dir := t.TempDir()
	err := WriteStats(dir, "test-session", 1000, 300)
	if err != nil {
		t.Fatalf("WriteStats failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "test-session.json"))
	if err != nil {
		t.Fatalf("session file not created: %v", err)
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

func TestWriteStats_PreservesExistingFields(t *testing.T) {
	dir := t.TempDir()
	// Write an existing session file with stop-hook fields.
	existing := `{"session_id":"test-session","turns":5,"total_saved":355}`
	os.WriteFile(filepath.Join(dir, "test-session.json"), []byte(existing), 0600)

	err := WriteStats(dir, "test-session", 2000, 500)
	if err != nil {
		t.Fatalf("WriteStats failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "test-session.json"))
	var got map[string]any
	json.Unmarshal(data, &got)

	// Existing fields preserved
	if got["turns"] != float64(5) {
		t.Errorf("expected turns=5 preserved, got %v", got["turns"])
	}
	// Ctx fields updated
	if got["ctx_orig"] != float64(2000) {
		t.Errorf("expected ctx_orig=2000, got %v", got["ctx_orig"])
	}
}

func TestWriteStats_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	// Run multiple concurrent writes and ensure file is never corrupted.
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
	data, err := os.ReadFile(filepath.Join(dir, "concurrent.json"))
	if err != nil {
		t.Fatal("session file not created after concurrent writes")
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Errorf("session file corrupted after concurrent writes: %v", err)
	}
}
