package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakeOpenAI returns a minimal non-streaming OpenAI response and records the
// request body it received for later inspection.
func fakeOpenAI(t *testing.T) (*httptest.Server, *[]byte) {
	t.Helper()
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = body
		w.Header().Set("Content-Type", "application/json")
		promptTokens := len(body) / 4
		if promptTokens < 1 {
			promptTokens = 1
		}
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
			"usage":   map[string]any{"prompt_tokens": promptTokens, "completion_tokens": 5, "total_tokens": promptTokens + 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return srv, &received
}

func makeOpenAIMessages(n int) []AnthropicMessage {
	msgs := make([]AnthropicMessage, n)
	for i := range msgs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = AnthropicMessage{Role: role, Content: "openai message number " + strings.Repeat("x", 20)}
	}
	return msgs
}

func postOpenAI(t *testing.T, handler http.Handler, msgs []AnthropicMessage, extraHeaders map[string]string) *http.Response {
	t.Helper()
	body := map[string]any{
		"model":    "gpt-4o",
		"messages": msgs,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w.Result()
}

// TestServeOpenAI_NonCompletionsPath ensures non-completions requests are
// forwarded verbatim to the OpenAI upstream.
func TestServeOpenAI_NonCompletionsPath(t *testing.T) {
	upstream, _ := fakeOpenAI(t)
	defer upstream.Close()

	h := NewHandler(10, t.TempDir(), "http://unused", upstream.URL)
	done := make(chan struct{})
	h.afterStats = func() { close(done) }

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

// TestServeOpenAI_InvalidJSON falls back to verbatim forward on unparseable body.
func TestServeOpenAI_InvalidJSON(t *testing.T) {
	upstream, _ := fakeOpenAI(t)
	defer upstream.Close()

	h := NewHandler(10, t.TempDir(), "http://unused", upstream.URL)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

// TestServeOpenAI_CompressesWindow verifies that a large conversation is
// trimmed before being forwarded to the upstream.
func TestServeOpenAI_CompressesWindow(t *testing.T) {
	upstream, received := fakeOpenAI(t)
	defer upstream.Close()

	sessionsDir := t.TempDir()
	done := make(chan struct{})
	h := NewHandler(3, sessionsDir, "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	msgs := makeOpenAIMessages(20)
	resp := postOpenAI(t, http.HandlerFunc(h.ServeOpenAI), msgs, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	<-done

	var forwarded openaiRequest
	if err := json.Unmarshal(*received, &forwarded); err != nil {
		t.Fatalf("upstream received invalid JSON: %v", err)
	}
	if len(forwarded.Messages) >= len(msgs) {
		t.Errorf("expected compression: forwarded %d messages, original %d", len(forwarded.Messages), len(msgs))
	}
}

// TestServeOpenAI_WritesStats verifies that a stats file is written after a
// successful /v1/chat/completions call.
func TestServeOpenAI_WritesStats(t *testing.T) {
	upstream, _ := fakeOpenAI(t)
	defer upstream.Close()

	sessionsDir := t.TempDir()
	done := make(chan struct{})
	sessionID := "test-openai-session"
	h := NewHandler(50, sessionsDir, "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	msgs := makeOpenAIMessages(4)
	postOpenAI(t, http.HandlerFunc(h.ServeOpenAI), msgs, map[string]string{engramSessionHeader: sessionID})

	<-done

	statsFile := sessionsDir + "/" + sessionID + ".ctx.json"
	raw, err := os.ReadFile(statsFile)
	if err != nil {
		t.Fatalf("stats file not written: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("stats file is empty")
	}
}

// TestParseOpenAIUsage_NonStreaming verifies extraction from a plain response.
func TestParseOpenAIUsage_NonStreaming(t *testing.T) {
	body := `{"usage":{"prompt_tokens":42,"completion_tokens":5,"total_tokens":47}}`
	got := parseOpenAIUsage([]byte(body))
	if got != 42 {
		t.Errorf("want 42, got %d", got)
	}
}

// TestParseOpenAIUsage_Streaming verifies extraction from SSE chunks.
func TestParseOpenAIUsage_Streaming(t *testing.T) {
	body := "data: {\"choices\":[]}\ndata: {\"usage\":{\"prompt_tokens\":99,\"completion_tokens\":3}}\ndata: [DONE]\n"
	got := parseOpenAIUsage([]byte(body))
	if got != 99 {
		t.Errorf("want 99, got %d", got)
	}
}

// TestParseOpenAIUsage_Empty returns -1 for empty body.
func TestParseOpenAIUsage_Empty(t *testing.T) {
	if got := parseOpenAIUsage(nil); got != -1 {
		t.Errorf("want -1, got %d", got)
	}
}

func TestEstimateOpenAITokens_CountsInstructions(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4o",
		"instructions":"Follow the repo conventions carefully.",
		"input":[{"role":"user","content":"Review the patch"}]
	}`)
	if got := estimateOpenAITokens(body); got <= len("Review the patch")/4 {
		t.Fatalf("expected instructions to contribute to token estimate, got %d", got)
	}
}

// TestServeOpenAI_PreservesUnknownFields verifies that extra fields (tools,
// temperature, response_format, metadata) survive the messages-only patch.
func TestServeOpenAI_PreservesUnknownFields(t *testing.T) {
	var received []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 1},
		})
	}))
	defer upstream.Close()

	done := make(chan struct{})
	h := NewHandler(50, t.TempDir(), "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	payload := map[string]any{
		"model":           "gpt-4o",
		"temperature":     0.7,
		"response_format": map[string]string{"type": "json_object"},
		"metadata":        map[string]string{"user": "u-123"},
		"tools":           []map[string]any{{"type": "function", "function": map[string]any{"name": "do_thing"}}},
		"tool_choice":     "auto",
		"messages":        []map[string]string{{"role": "user", "content": "hello"}},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(received, &got); err != nil {
		t.Fatalf("upstream received invalid JSON: %v", err)
	}
	for _, field := range []string{"temperature", "response_format", "metadata", "tools", "tool_choice"} {
		if _, ok := got[field]; !ok {
			t.Errorf("field %q missing from forwarded body", field)
		}
	}
	<-done
}

// TestServeOpenAI_StreamingPassthrough verifies that streaming SSE chunks are
// forwarded intact to the client.
func TestServeOpenAI_StreamingPassthrough(t *testing.T) {
	chunks := []string{
		"data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n",
		"data: {\"id\":\"c2\",\"choices\":[{\"delta\":{\"content\":\" world\"}}],\"usage\":{\"prompt_tokens\":5}}\n\n",
		"data: [DONE]\n\n",
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		for _, chunk := range chunks {
			_, _ = w.Write([]byte(chunk))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer upstream.Close()

	h := NewHandler(50, t.TempDir(), "http://unused", upstream.URL)
	done := make(chan struct{})
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	body := map[string]any{
		"model":    "gpt-4o",
		"stream":   true,
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)

	got := w.Body.String()
	for _, chunk := range chunks {
		// strip trailing newlines for comparison
		needle := strings.TrimRight(chunk, "\n")
		if !strings.Contains(got, needle) {
			t.Errorf("response missing chunk %q\nfull body: %s", needle, got)
		}
	}
	<-done
}

// fakeOpenAIResponses returns a fake /v1/responses upstream using input_tokens.
func fakeOpenAIResponses(t *testing.T) (*httptest.Server, *[]byte) {
	t.Helper()
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = body
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "resp-test",
			"object": "response",
			"output": []map[string]any{{"type": "message", "content": []map[string]string{{"type": "output_text", "text": "ok"}}}},
			"usage":  map[string]any{"input_tokens": 10, "output_tokens": 3, "total_tokens": 13},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return srv, &received
}

// TestServeOpenAIResponses_NonStreaming verifies /v1/responses compression and forwarding.
func TestServeOpenAIResponses_NonStreaming(t *testing.T) {
	upstream, received := fakeOpenAIResponses(t)
	defer upstream.Close()

	sessionsDir := t.TempDir()
	done := make(chan struct{})
	sessionID := "test-responses-session"
	h := NewHandler(3, sessionsDir, "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	msgs := makeOpenAIMessages(20)
	b, _ := json.Marshal(map[string]any{"model": "gpt-4o", "input": msgs})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(engramSessionHeader, sessionID)
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	<-done

	var forwarded openaiResponsesRequest
	if err := json.Unmarshal(*received, &forwarded); err != nil {
		t.Fatalf("upstream received invalid JSON: %v", err)
	}
	forwardedMsgs, ok := decodeOpenAIInputMessages(forwarded.Input)
	if !ok {
		t.Fatal("expected forwarded input to remain decodable as messages")
	}
	if len(forwardedMsgs) >= len(msgs) {
		t.Errorf("expected compression: forwarded %d items, original %d", len(forwardedMsgs), len(msgs))
	}

	statsFile := sessionsDir + "/" + sessionID + ".ctx.json"
	raw, err := os.ReadFile(statsFile)
	if err != nil {
		t.Fatalf("stats file not written: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("stats file is empty")
	}
}

func TestServeOpenAIResponses_TypedMessageItemsCompress(t *testing.T) {
	upstream, received := fakeOpenAIResponses(t)
	defer upstream.Close()

	sessionsDir := t.TempDir()
	done := make(chan struct{})
	h := NewHandler(3, sessionsDir, "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	input := make([]map[string]any, 0, 20)
	for i, msg := range makeOpenAIMessages(20) {
		input = append(input, map[string]any{
			"type": "message",
			"role": msg.Role,
			"content": []map[string]any{
				{"type": "input_text", "text": msg.Content},
			},
			"id": "item-" + string(rune('a'+i%26)),
		})
	}
	b, _ := json.Marshal(map[string]any{"model": "gpt-4o", "input": input})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	<-done

	var forwarded struct {
		Input []map[string]any `json:"input"`
	}
	if err := json.Unmarshal(*received, &forwarded); err != nil {
		t.Fatalf("upstream received invalid JSON: %v", err)
	}
	if len(forwarded.Input) >= len(input) {
		t.Fatalf("expected compression: forwarded %d items, original %d", len(forwarded.Input), len(input))
	}
	firstType, _ := forwarded.Input[0]["type"].(string)
	if firstType != "message" {
		t.Fatalf("expected summary item to be a message, got %q", firstType)
	}
}

// TestServeOpenAIResponses_PreservesFields ensures unknown top-level fields survive.
func TestServeOpenAIResponses_PreservesFields(t *testing.T) {
	var received []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "resp-test",
			"usage": map[string]any{"input_tokens": 5, "output_tokens": 1},
		})
	}))
	defer upstream.Close()

	done := make(chan struct{})
	h := NewHandler(50, t.TempDir(), "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	payload := map[string]any{
		"model":       "gpt-4o",
		"temperature": 0.5,
		"tools":       []map[string]any{{"type": "function", "function": map[string]any{"name": "search"}}},
		"input":       []map[string]string{{"role": "user", "content": "hello"}},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(received, &got); err != nil {
		t.Fatalf("upstream received invalid JSON: %v", err)
	}
	for _, field := range []string{"temperature", "tools"} {
		if _, ok := got[field]; !ok {
			t.Errorf("field %q missing from forwarded body", field)
		}
	}
	<-done
}

func TestServeOpenAI_FallbackSessionIDVariesByRequest(t *testing.T) {
	upstream, _ := fakeOpenAI(t)
	defer upstream.Close()

	sessionsDir := t.TempDir()
	h := NewHandler(50, sessionsDir, "http://unused", upstream.URL)
	done := make(chan struct{}, 2)
	h.afterStats = func() { done <- struct{}{} }

	post := func(content string) {
		b, _ := json.Marshal(map[string]any{
			"model": "gpt-4o",
			"messages": []map[string]string{
				{"role": "developer", "content": "Use concise engineering language."},
				{"role": "user", "content": content},
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeOpenAI(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", w.Code)
		}
	}

	post("Review service A")
	post("Review service B")
	<-done
	<-done

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		t.Fatalf("read sessions dir: %v", err)
	}
	var count int
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".ctx.json") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 distinct ctx files, got %d", count)
	}
}

func TestServeOpenAIResponses_FallbackStatsIncludeInstructions(t *testing.T) {
	var received []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp-test",
			"object": "response",
			"output": []map[string]any{{"type": "message"}},
		})
	}))
	defer upstream.Close()

	sessionsDir := t.TempDir()
	done := make(chan struct{})
	sessionID := "responses-instructions"
	h := NewHandler(50, sessionsDir, "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	payload := map[string]any{
		"model":        "gpt-4o",
		"instructions": "Always favor concise engineering summaries.",
		"input": []map[string]string{
			{"role": "user", "content": "Review the diff"},
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(engramSessionHeader, sessionID)
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	<-done

	var forwarded map[string]json.RawMessage
	if err := json.Unmarshal(received, &forwarded); err != nil {
		t.Fatalf("upstream received invalid JSON: %v", err)
	}
	if _, ok := forwarded["instructions"]; !ok {
		t.Fatal("expected instructions to be preserved in forwarded body")
	}

	raw, err := os.ReadFile(sessionsDir + "/" + sessionID + ".ctx.json")
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}
	var stats ctxStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		t.Fatalf("parse stats: %v", err)
	}

	inputOnly := EstimateTokens([]AnthropicMessage{{Role: "user", Content: "Review the diff"}})
	if stats.CtxOrig <= inputOnly {
		t.Fatalf("expected instructions to increase ctx_orig beyond input-only estimate, got %d", stats.CtxOrig)
	}
	if stats.CtxComp <= inputOnly {
		t.Fatalf("expected instructions to increase ctx_comp beyond input-only estimate, got %d", stats.CtxComp)
	}
}

func TestServeOpenAIResponses_StringInputStats(t *testing.T) {
	upstream, _ := fakeOpenAIResponses(t)
	defer upstream.Close()

	sessionsDir := t.TempDir()
	done := make(chan struct{})
	sessionID := "responses-string"
	h := NewHandler(50, sessionsDir, "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	payload := map[string]any{
		"model":        "gpt-4o",
		"instructions": "Reply tersely.",
		"input":        "Summarize the diff in one paragraph.",
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(engramSessionHeader, sessionID)
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	<-done

	raw, err := os.ReadFile(sessionsDir + "/" + sessionID + ".ctx.json")
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}
	var stats ctxStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		t.Fatalf("parse stats: %v", err)
	}
	if stats.CtxOrig == 0 || stats.CtxComp == 0 {
		t.Fatalf("expected non-zero stats for string input, got %+v", stats)
	}
}

func TestServeOpenAIResponses_ExoticItemsSummarizeCleanly(t *testing.T) {
	upstream, received := fakeOpenAIResponses(t)
	defer upstream.Close()

	done := make(chan struct{})
	h := NewHandler(2, t.TempDir(), "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	input := []map[string]any{
		{
			"type": "message",
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": "Please inspect the screenshot and attached spec."},
				{"type": "input_image", "image_url": "https://example.com/screen.png"},
				{"type": "input_file", "filename": "spec.md"},
			},
		},
		{
			"type":    "reasoning",
			"summary": "Compared the UI states before choosing a patch.",
		},
		{
			"type": "function_call_output",
			"output": []map[string]any{
				{"type": "output_text", "text": "All tests passed except the login snapshot."},
			},
		},
		{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": "I recommend updating the login assertions."},
			},
		},
	}
	b, _ := json.Marshal(map[string]any{"model": "gpt-4o", "input": input})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	<-done

	var forwarded struct {
		Input []map[string]json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(*received, &forwarded); err != nil {
		t.Fatalf("upstream received invalid JSON: %v", err)
	}
	if len(forwarded.Input) != 3 {
		t.Fatalf("expected summary + 2 tail items, got %d", len(forwarded.Input))
	}
	var firstType string
	if err := json.Unmarshal(forwarded.Input[0]["type"], &firstType); err != nil {
		t.Fatalf("parse summary type: %v", err)
	}
	if firstType != "message" {
		t.Fatalf("expected first forwarded item to be summary message, got %q", firstType)
	}
	var summaryContent string
	if err := json.Unmarshal(forwarded.Input[0]["content"], &summaryContent); err != nil {
		t.Fatalf("parse summary content: %v", err)
	}
	if !strings.Contains(summaryContent, "[image]") {
		t.Fatalf("expected summary to mention image content, got %q", summaryContent)
	}
	if !strings.Contains(summaryContent, "[file] spec.md") {
		t.Fatalf("expected summary to mention file content, got %q", summaryContent)
	}
	if !strings.Contains(summaryContent, "Compared the UI states") {
		t.Fatalf("expected summary to include reasoning summary text, got %q", summaryContent)
	}
}

func TestServeOpenAIResponses_LargeToolOutputTailIsCompacted(t *testing.T) {
	upstream, received := fakeOpenAIResponses(t)
	defer upstream.Close()

	done := make(chan struct{})
	h := NewHandler(2, t.TempDir(), "http://unused", upstream.URL)
	h.afterStats = func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}

	hugeOutput := strings.Repeat("tool output line ", 80)
	input := []map[string]any{
		{
			"type": "message",
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": "Please inspect previous work."},
			},
		},
		{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": "I started by looking at the current state."},
			},
		},
		{
			"type":   "function_call_output",
			"output": hugeOutput,
		},
		{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": "Next I would patch the failing area."},
			},
		},
	}
	b, _ := json.Marshal(map[string]any{"model": "gpt-4o", "input": input})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeOpenAI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	<-done

	var forwarded struct {
		Input []map[string]json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(*received, &forwarded); err != nil {
		t.Fatalf("upstream received invalid JSON: %v", err)
	}
	if len(forwarded.Input) != 3 {
		t.Fatalf("expected summary + 2 tail items, got %d", len(forwarded.Input))
	}
	var toolOutput string
	if err := json.Unmarshal(forwarded.Input[1]["output"], &toolOutput); err != nil {
		t.Fatalf("parse compacted tool output: %v", err)
	}
	if len(toolOutput) >= len(hugeOutput) {
		t.Fatalf("expected compacted tool output to shrink, original=%d compacted=%d", len(hugeOutput), len(toolOutput))
	}
	if !strings.Contains(toolOutput, " … ") {
		t.Fatalf("expected compacted tool output to include truncation marker, got %q", toolOutput)
	}
}
