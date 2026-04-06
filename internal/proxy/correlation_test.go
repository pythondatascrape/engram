// internal/proxy/correlation_test.go
package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSessionCorrelation verifies that a session registered via
// /internal/register-session causes the proxy to write <uuid>.ctx.json
// (not proxy-*.ctx.json) when the request carries a placeholder session header.
//
// This is the end-to-end regression test for the statusline correlation fix.
func TestSessionCorrelation(t *testing.T) {
	upstream, _ := fakeAnthropic(t)
	defer upstream.Close()

	sessionsDir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(512, sessionsDir, upstream.URL)
	h.afterStats = func() { done <- struct{}{} }

	// 1. Register the real Claude session UUID (as sessionstart.mjs will do).
	rec := httptest.NewRecorder()
	reg := httptest.NewRequest(http.MethodPost, "/internal/register-session",
		strings.NewReader(`{"session_id":"corr-test-uuid"}`))
	h.ServeHTTP(rec, reg)
	if rec.Code != http.StatusOK {
		t.Fatalf("register-session: got %d, want 200", rec.Code)
	}

	// 2. Send /v1/messages with the placeholder header that engram install writes.
	postMessages(t, h,
		makeMessages(3),
		"test system prompt",
		map[string]string{"X-Engram-Session": "${session_id}"},
	)
	<-done

	// 3. The ctx file must use the registered UUID, not the fingerprint.
	ctxFile := filepath.Join(sessionsDir, "corr-test-uuid.ctx.json")
	if _, err := os.Stat(ctxFile); err != nil {
		t.Fatalf("expected %s to exist: %v", ctxFile, err)
	}

	// 4. No fingerprint-based ctx file must exist.
	matches, _ := filepath.Glob(filepath.Join(sessionsDir, "proxy-*.ctx.json"))
	if len(matches) > 0 {
		t.Fatalf("expected no proxy-*.ctx.json files, found: %v", matches)
	}
}

// TestFingerprintFallbackWhenNoSessionRegistered verifies that the proxy
// still falls back to the system-prompt fingerprint when no session was registered.
// This ensures the degraded path (proxy running without sessionstart hook) still works.
func TestFingerprintFallbackWhenNoSessionRegistered(t *testing.T) {
	upstream, _ := fakeAnthropic(t)
	defer upstream.Close()

	sessionsDir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(512, sessionsDir, upstream.URL)
	h.afterStats = func() { done <- struct{}{} }

	// Send /v1/messages with placeholder header but NO prior registration.
	postMessages(t, h,
		makeMessages(3),
		"fallback-system-prompt",
		map[string]string{"X-Engram-Session": "${session_id}"},
	)
	<-done

	expectedID := SessionID("fallback-system-prompt")
	ctxFile := filepath.Join(sessionsDir, expectedID+".ctx.json")
	if _, err := os.Stat(ctxFile); err != nil {
		t.Fatalf("expected fingerprint ctx file %s: %v", ctxFile, err)
	}
}
