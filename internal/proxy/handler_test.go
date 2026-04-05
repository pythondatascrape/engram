// internal/proxy/handler_test.go
package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAnthropic returns a minimal non-streaming Anthropic response and
// records the request body it received for later inspection.
func fakeAnthropic(t *testing.T) (*httptest.Server, *[]byte) {
	t.Helper()
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = body
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":   "test",
			"type": "message",
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return srv, &received
}

func makeMessages(n int) []AnthropicMessage {
	msgs := make([]AnthropicMessage, n)
	for i := range msgs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = AnthropicMessage{Role: role, Content: "message content number " + strings.Repeat("x", 20)}
	}
	return msgs
}

func postMessages(t *testing.T, handler http.Handler, msgs []AnthropicMessage, system string, extraHeaders map[string]string) *http.Response {
	t.Helper()
	body := map[string]any{
		"messages": msgs,
		"system":   system,
		"stream":   false,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w.Result()
}

// Test 1: non-messages path forwarded verbatim
func TestNonMessagesPathForwarded(t *testing.T) {
	srv, received := fakeAnthropic(t)
	defer srv.Close()

	h := NewHandler(5, t.TempDir(), srv.URL)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// fake server should have received the request (received will be empty body for GET)
	_ = received
}

// Test 2: messages compressed when above window size
func TestMessagesCompressed(t *testing.T) {
	srv, received := fakeAnthropic(t)
	defer srv.Close()

	done := make(chan struct{}, 1)
	h := NewHandler(5, t.TempDir(), srv.URL)
	h.afterStats = func() { done <- struct{}{} }
	msgs := makeMessages(15)
	resp := postMessages(t, h, msgs, "sys", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	<-done

	var parsed struct {
		Messages []AnthropicMessage `json:"messages"`
	}
	if err := json.Unmarshal(*received, &parsed); err != nil {
		t.Fatalf("parse received body: %v", err)
	}
	if len(parsed.Messages) != 6 {
		t.Fatalf("expected 6 messages (1 summary + 5 tail), got %d", len(parsed.Messages))
	}
	first, ok := parsed.Messages[0].Content.(string)
	if !ok {
		t.Fatal("first message content not a string")
	}
	if !strings.HasPrefix(first, "[CONTEXT_SUMMARY]") {
		t.Fatalf("expected [CONTEXT_SUMMARY] prefix, got: %s", first[:50])
	}
}

// Test 3: below window size — no compression
func TestBelowWindowSizeNoCompression(t *testing.T) {
	srv, received := fakeAnthropic(t)
	defer srv.Close()

	done := make(chan struct{}, 1)
	h := NewHandler(10, t.TempDir(), srv.URL)
	h.afterStats = func() { done <- struct{}{} }
	msgs := makeMessages(3)
	postMessages(t, h, msgs, "sys", nil)
	<-done

	var parsed struct {
		Messages []AnthropicMessage `json:"messages"`
	}
	if err := json.Unmarshal(*received, &parsed); err != nil {
		t.Fatalf("parse received body: %v", err)
	}
	if len(parsed.Messages) != 3 {
		t.Fatalf("expected 3 messages unchanged, got %d", len(parsed.Messages))
	}
}

// Test 4: X-Engram-Session header used as session ID
func TestXEngramSessionHeader(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	dir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(5, dir, srv.URL)
	h.afterStats = func() { done <- struct{}{} }
	postMessages(t, h, makeMessages(3), "sys", map[string]string{
		"X-Engram-Session": "my-session-id",
	})
	<-done

	path := filepath.Join(dir, "my-session-id.ctx.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected ctx file %s, got err: %v", path, err)
	}
}

// Test 5: system prompt fingerprint fallback
func TestSystemPromptFingerprintFallback(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	dir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(5, dir, srv.URL)
	h.afterStats = func() { done <- struct{}{} }
	postMessages(t, h, makeMessages(3), "some text", nil)
	<-done

	expectedID := SessionID("some text")
	path := filepath.Join(dir, expectedID+".ctx.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected ctx file %s, got err: %v", path, err)
	}
}

// Test 6: stats written with ctx_orig and ctx_comp
func TestStatsWritten(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	dir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(5, dir, srv.URL)
	h.afterStats = func() { done <- struct{}{} }
	postMessages(t, h, makeMessages(10), "statstest", map[string]string{
		"X-Engram-Session": "stats-session",
	})
	<-done

	data, err := os.ReadFile(filepath.Join(dir, "stats-session.ctx.json"))
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}
	var stats map[string]any
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("parse stats: %v", err)
	}
	if _, ok := stats["ctx_orig"]; !ok {
		t.Error("missing ctx_orig")
	}
	if _, ok := stats["ctx_comp"]; !ok {
		t.Error("missing ctx_comp")
	}
}

// Test 7: malformed JSON body forwarded verbatim (fail-open)
func TestMalformedJSONFailOpen(t *testing.T) {
	srv, received := fakeAnthropic(t)
	defer srv.Close()

	h := NewHandler(5, t.TempDir(), srv.URL)
	badBody := "this is not json {"
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(badBody))
	req.Header.Set("Authorization", "Bearer sk-test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if string(*received) != badBody {
		t.Fatalf("expected verbatim body forwarded, got: %q", string(*received))
	}
}

// TestPlaceholderSessionIDFallsBackToFingerprint verifies that the literal
// "${session_id}" written by engram install is not used as a real session ID.
func TestPlaceholderSessionIDFallsBackToFingerprint(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	dir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(5, dir, srv.URL)
	h.afterStats = func() { done <- struct{}{} }

	// Send with the literal placeholder that engram install writes.
	postMessages(t, h, makeMessages(3), "my-system-prompt", map[string]string{
		"X-Engram-Session": "${session_id}",
	})
	<-done

	// The stats file must be named after the fingerprint of the system prompt,
	// NOT the literal string "${session_id}".
	expected := SessionID("my-system-prompt")
	if _, err := os.Stat(filepath.Join(dir, expected+".ctx.json")); err != nil {
		t.Fatalf("expected fingerprint file %s.ctx.json, got: %v", expected, err)
	}
	// Also assert the placeholder file was NOT created.
	if _, err := os.Stat(filepath.Join(dir, "${session_id}.ctx.json")); err == nil {
		t.Fatal("placeholder file ${session_id}.ctx.json should not have been created")
	}
}
