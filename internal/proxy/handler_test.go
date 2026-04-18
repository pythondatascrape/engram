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

// fakeAnthropic returns a minimal non-streaming Anthropic response (with usage)
// and records the request body it received for later inspection. Also handles
// /v1/messages/count_tokens by estimating tokens from the request body size.
func fakeAnthropic(t *testing.T) (*httptest.Server, *[]byte) {
	t.Helper()
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		// Handle count_tokens endpoint.
		if r.URL.Path == "/v1/messages/count_tokens" {
			// Estimate tokens from the body to simulate the real API.
			// Use len/4 as a rough stand-in for the real tokenizer.
			tokens := len(body) / 4
			if tokens < 1 {
				tokens = 1
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]int{"input_tokens": tokens})
			return
		}

		received = body
		w.Header().Set("Content-Type", "application/json")
		// Estimate input tokens from the compressed body size for usage.
		inputTokens := len(body) / 4
		if inputTokens < 1 {
			inputTokens = 1
		}
		resp := map[string]any{
			"id":   "test",
			"type": "message",
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
			"usage": map[string]any{
				"input_tokens":  inputTokens,
				"output_tokens": 5,
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
		"model":    "claude-sonnet-4-20250514",
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

func postRawBody(t *testing.T, handler http.Handler, body string, extraHeaders map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
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

	h := NewHandler(5, t.TempDir(), srv.URL, "")
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
	h := NewHandler(5, t.TempDir(), srv.URL, "")
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

// TestCompressedBodyPreservesAllFields verifies that compression only patches
// the "messages" field and preserves all other original request fields like
// max_tokens, tools, temperature, etc.
func TestCompressedBodyPreservesAllFields(t *testing.T) {
	srv, received := fakeAnthropic(t)
	defer srv.Close()

	done := make(chan struct{}, 1)
	h := NewHandler(5, t.TempDir(), srv.URL, "")
	h.afterStats = func() { done <- struct{}{} }

	// Build a request body with extra fields that anthropicRequest doesn't model.
	msgs := makeMessages(15)
	body := map[string]any{
		"model":       "claude-sonnet-4-20250514",
		"messages":    msgs,
		"system":      "sys",
		"stream":      false,
		"max_tokens":  4096,
		"temperature": 0.7,
		"tools":       []map[string]any{{"name": "my_tool", "description": "test"}},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	<-done

	// Parse what the upstream server received.
	var got map[string]json.RawMessage
	if err := json.Unmarshal(*received, &got); err != nil {
		t.Fatalf("parse received body: %v", err)
	}

	// Verify extra fields are preserved.
	for _, key := range []string{"max_tokens", "temperature", "tools"} {
		if _, ok := got[key]; !ok {
			t.Errorf("field %q was dropped from compressed body", key)
		}
	}

	// Verify messages were still compressed.
	var parsedMsgs []AnthropicMessage
	if err := json.Unmarshal(got["messages"], &parsedMsgs); err != nil {
		t.Fatalf("parse messages: %v", err)
	}
	if len(parsedMsgs) != 6 {
		t.Fatalf("expected 6 messages (1 summary + 5 tail), got %d", len(parsedMsgs))
	}
}

// Test 3: below window size — no compression
func TestBelowWindowSizeNoCompression(t *testing.T) {
	srv, received := fakeAnthropic(t)
	defer srv.Close()

	done := make(chan struct{}, 1)
	h := NewHandler(10, t.TempDir(), srv.URL, "")
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
	h := NewHandler(5, dir, srv.URL, "")
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
	h := NewHandler(5, dir, srv.URL, "")
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
	h := NewHandler(5, dir, srv.URL, "")
	h.afterStats = func() { done <- struct{}{} }
	postMessages(t, h, makeMessages(10), "statstest", map[string]string{
		"X-Engram-Session": "stats-session",
	})
	<-done

	data, err := os.ReadFile(filepath.Join(dir, "stats-session.ctx.json"))
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}
	var stats ctxStats
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("parse stats: %v", err)
	}
	if stats.CtxOrig == 0 {
		t.Error("ctx_orig should be > 0")
	}
	if stats.CtxComp == 0 {
		t.Error("ctx_comp should be > 0")
	}
	if stats.Turns != 1 {
		t.Errorf("turns = %d, want 1", stats.Turns)
	}

	// Send a second request and verify accumulation.
	postMessages(t, h, makeMessages(10), "statstest", map[string]string{
		"X-Engram-Session": "stats-session",
	})
	<-done

	data2, err := os.ReadFile(filepath.Join(dir, "stats-session.ctx.json"))
	if err != nil {
		t.Fatalf("read stats file (2nd): %v", err)
	}
	var stats2 ctxStats
	if err := json.Unmarshal(data2, &stats2); err != nil {
		t.Fatalf("parse stats (2nd): %v", err)
	}
	if stats2.Turns != 2 {
		t.Errorf("turns after 2 calls = %d, want 2", stats2.Turns)
	}
	if stats2.CtxOrig <= stats.CtxOrig {
		t.Errorf("ctx_orig should accumulate: %d <= %d", stats2.CtxOrig, stats.CtxOrig)
	}
}

// Test 7: malformed JSON body forwarded verbatim (fail-open)
func TestMalformedJSONFailOpen(t *testing.T) {
	srv, received := fakeAnthropic(t)
	defer srv.Close()

	h := NewHandler(5, t.TempDir(), srv.URL, "")
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
	h := NewHandler(5, dir, srv.URL, "")
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

// TestRegisterSessionEndpoint verifies that a valid POST stores the session ID
// and a subsequent claimPendingSession returns and clears it.
func TestRegisterSessionEndpoint(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	h := NewHandler(5, t.TempDir(), srv.URL, "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/register-session",
		strings.NewReader(`{"session_id":"abc-123"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if got := h.claimPendingSession(); got != "abc-123" {
		t.Fatalf("claimPendingSession() = %q, want %q", got, "abc-123")
	}
	// Second claim must still return the same session (persistent, not consumed).
	if got := h.claimPendingSession(); got != "abc-123" {
		t.Fatalf("expected persistent session %q, got %q", "abc-123", got)
	}
}

// TestRegisterSessionRejectsPlaceholder verifies that the literal "${session_id}"
// injected by engram install is rejected with 400, not stored.
func TestRegisterSessionRejectsPlaceholder(t *testing.T) {
	h := NewHandler(5, t.TempDir(), "", "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/register-session",
		strings.NewReader(`{"session_id":"${session_id}"}`))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
	if got := h.claimPendingSession(); got != "" {
		t.Fatalf("placeholder must not be stored, got %q", got)
	}
}

// TestRegisterSessionRejectsEmpty verifies that an empty session_id returns 400.
func TestRegisterSessionRejectsEmpty(t *testing.T) {
	h := NewHandler(5, t.TempDir(), "", "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/register-session",
		strings.NewReader(`{"session_id":""}`))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

// TestRegisterSessionGetFallsThrough verifies that a GET to /internal/register-session
// is forwarded verbatim to the proxy (falls through), not rejected with 405.
// This locks in the design decision: only POST is intercepted; other methods proxy through.
func TestRegisterSessionGetFallsThrough(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	h := NewHandler(5, t.TempDir(), srv.URL, "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/internal/register-session", nil)
	h.ServeHTTP(rec, req)

	// The fake Anthropic server returns 200 for any request it receives.
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (proxied through)", rec.Code)
	}
}

func TestStatsIncludeSystemPrompt(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	dir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(5, dir, srv.URL, "")
	h.afterStats = func() { done <- struct{}{} }

	// System prompt of 400 chars → ~100 tokens.
	system := strings.Repeat("a", 400)
	// 3 messages (below window, no compression) with ~40 chars each → ~10 tokens each.
	msgs := []AnthropicMessage{
		{Role: "user", Content: strings.Repeat("b", 40)},
		{Role: "assistant", Content: strings.Repeat("c", 40)},
		{Role: "user", Content: strings.Repeat("d", 40)},
	}

	postMessages(t, h, msgs, system, map[string]string{
		"X-Engram-Session": "sysprompt-test",
	})
	<-done

	data, err := os.ReadFile(filepath.Join(dir, "sysprompt-test.ctx.json"))
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}

	var stats ctxStats
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("parse stats: %v", err)
	}

	// Without system prompt: ~30 tokens (3 msgs * 10 tokens each).
	// With system prompt: ~130 tokens (30 + 100).
	if stats.CtxOrig < 100 {
		t.Errorf("ctxOrig should include system prompt tokens; got %d, want >= 100", stats.CtxOrig)
	}
	if stats.CtxComp < 100 {
		t.Errorf("ctxComp should include system prompt tokens; got %d, want >= 100", stats.CtxComp)
	}
	if stats.Turns != 1 {
		t.Errorf("turns = %d, want 1", stats.Turns)
	}
}

func TestSystemArrayCountsTokensAndFingerprintFallback(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	dir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(10, dir, srv.URL, "")
	h.afterStats = func() { done <- struct{}{} }

	systemTextA := strings.Repeat("a", 200)
	systemTextB := strings.Repeat("b", 200)
	body := `{
		"messages":[{"role":"user","content":"hello"}],
		"system":[
			{"type":"text","text":"` + systemTextA + `"},
			{"type":"text","text":"` + systemTextB + `"}
		],
		"stream":false
	}`

	resp := postRawBody(t, h, body, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	<-done

	expectedID := SessionID(systemTextA + "\n" + systemTextB)
	data, err := os.ReadFile(filepath.Join(dir, expectedID+".ctx.json"))
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}

	var stats ctxStats
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("parse stats: %v", err)
	}

	// 400 chars of system prompt should contribute about 100 tokens, plus the user message.
	if stats.CtxOrig < 100 {
		t.Fatalf("ctxOrig should include array-form system prompt tokens; got %d", stats.CtxOrig)
	}
	if stats.CtxComp < 100 {
		t.Fatalf("ctxComp should include array-form system prompt tokens; got %d", stats.CtxComp)
	}
}
