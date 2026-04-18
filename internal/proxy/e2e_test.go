package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestE2EContextTokenAccumulation sends 10 requests of varying sizes through
// the proxy and verifies that ctx.json accumulates correctly after each turn.
func TestE2EContextTokenAccumulation(t *testing.T) {
	// Track what the upstream receives so we can verify field preservation.
	var mu sync.Mutex
	var lastReceived []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		if r.URL.Path == "/v1/messages/count_tokens" {
			// Return token count proportional to body size.
			tokens := len(body) / 4
			if tokens < 1 {
				tokens = 1
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]int{"input_tokens": tokens})
			return
		}

		mu.Lock()
		lastReceived = make([]byte, len(body))
		copy(lastReceived, body)
		mu.Unlock()

		// Return a response with usage.input_tokens based on the compressed body.
		inputTokens := len(body) / 4
		if inputTokens < 1 {
			inputTokens = 1
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "msg_test",
			"type":    "message",
			"content": []map[string]any{{"type": "text", "text": "ok"}},
			"usage":   map[string]any{"input_tokens": inputTokens, "output_tokens": 5},
		})
	}))
	defer srv.Close()

	sessDir := t.TempDir()
	sessionID := "e2e-test-session"

	done := make(chan struct{}, 1)
	h := NewHandler(10, sessDir, srv.URL, "") // window=10 so compression triggers at >10 messages
	h.afterStats = func() { done <- struct{}{} }

	// Register the session.
	regReq := httptest.NewRequest(http.MethodPost, "/internal/register-session",
		strings.NewReader(fmt.Sprintf(`{"session_id":%q}`, sessionID)))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	h.ServeHTTP(regW, regReq)
	if regW.Code != http.StatusOK {
		t.Fatalf("register-session failed: %d", regW.Code)
	}

	// Define 10 turns with varying sizes.
	type turn struct {
		name     string
		nMsgs   int
		msgSize int // chars per message content
		system  string
		extras  map[string]any // extra fields like max_tokens, tools
	}

	// Realistic system prompt: Claude Code's system prompt is 10k-20k+ tokens.
	bigSystem := "You are Claude Code, an AI assistant made by Anthropic. " +
		strings.Repeat("You have access to tools for reading files, editing code, running commands, and searching. "+
			"Always follow the user's instructions carefully. Use the Read tool for files, Edit for modifications, "+
			"Bash for shell commands. Be concise and direct in your responses. ", 200)
	// ~40k chars => ~10k tokens, realistic for Claude Code

	turns := []turn{
		{name: "first-turn", nMsgs: 3, msgSize: 2000, system: bigSystem},
		{name: "tool-use-turn", nMsgs: 7, msgSize: 3000, system: bigSystem},
		{name: "code-review", nMsgs: 5, msgSize: 8000, system: bigSystem},
		{name: "with-tools-heavy", nMsgs: 10, msgSize: 4000, system: bigSystem,
			extras: map[string]any{
				"max_tokens":  16384,
				"temperature": 1.0,
				"tools": []map[string]any{
					{"name": "Read", "description": "Read a file from the filesystem",
						"input_schema": map[string]any{"type": "object", "properties": map[string]any{
							"file_path": map[string]any{"type": "string", "description": "Absolute path to the file"},
							"offset":    map[string]any{"type": "integer"},
							"limit":     map[string]any{"type": "integer"},
						}}},
					{"name": "Edit", "description": "Edit a file with string replacement",
						"input_schema": map[string]any{"type": "object", "properties": map[string]any{
							"file_path":  map[string]any{"type": "string"},
							"old_string": map[string]any{"type": "string"},
							"new_string": map[string]any{"type": "string"},
						}}},
					{"name": "Bash", "description": "Execute a shell command",
						"input_schema": map[string]any{"type": "object", "properties": map[string]any{
							"command": map[string]any{"type": "string"},
						}}},
					{"name": "Grep", "description": "Search file contents with regex",
						"input_schema": map[string]any{"type": "object", "properties": map[string]any{
							"pattern": map[string]any{"type": "string"},
							"path":    map[string]any{"type": "string"},
						}}},
				},
			}},
		{name: "compress-15msg", nMsgs: 15, msgSize: 3000, system: bigSystem},
		{name: "heavy-20msg", nMsgs: 20, msgSize: 2500, system: bigSystem},
		{name: "huge-single-msg", nMsgs: 1, msgSize: 60000, system: bigSystem},
		{name: "big-context", nMsgs: 12, msgSize: 6000, system: bigSystem},
		{name: "max-context-30msg", nMsgs: 30, msgSize: 4000, system: bigSystem},
		{name: "short-followup", nMsgs: 3, msgSize: 500, system: bigSystem},
	}

	ctxPath := filepath.Join(sessDir, sessionID+".ctx.json")
	var prevOrig, prevComp int

	for i, tt := range turns {
		t.Run(fmt.Sprintf("turn%d_%s", i+1, tt.name), func(t *testing.T) {
			// Build messages.
			msgs := make([]map[string]any, tt.nMsgs)
			for j := range msgs {
				role := "user"
				if j%2 == 1 {
					role = "assistant"
				}
				msgs[j] = map[string]any{
					"role":    role,
					"content": fmt.Sprintf("Turn %d message %d: %s", i+1, j, strings.Repeat("x", tt.msgSize)),
				}
			}

			body := map[string]any{
				"model":    "claude-sonnet-4-20250514",
				"messages": msgs,
				"system":   tt.system,
				"stream":   false,
			}
			for k, v := range tt.extras {
				body[k] = v
			}

			b, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(b)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer sk-test")
			req.Header.Set("Anthropic-Version", "2023-06-01")
			// First request claims the registered session; subsequent ones use the header.
			if i > 0 {
				req.Header.Set("X-Engram-Session", sessionID)
			}

			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("request failed: %d, body: %s", w.Code, w.Body.String())
			}

			// Wait for async stats write.
			<-done

			// Read and validate ctx.json.
			data, err := os.ReadFile(ctxPath)
			if err != nil {
				t.Fatalf("ctx.json not found after turn %d: %v", i+1, err)
			}

			var stats ctxStats
			if err := json.Unmarshal(data, &stats); err != nil {
				t.Fatalf("invalid ctx.json: %v", err)
			}

			t.Logf("turn %d (%s): orig=%d comp=%d turns=%d (delta_orig=%d delta_comp=%d)",
				i+1, tt.name, stats.CtxOrig, stats.CtxComp, stats.Turns,
				stats.CtxOrig-prevOrig, stats.CtxComp-prevComp)

			// Verify turn counter.
			if stats.Turns != i+1 {
				t.Errorf("turns = %d, want %d", stats.Turns, i+1)
			}

			// Verify values are non-trivial (not 0 or 1).
			deltaOrig := stats.CtxOrig - prevOrig
			deltaComp := stats.CtxComp - prevComp
			if deltaOrig < 10 {
				t.Errorf("delta ctx_orig = %d, want >= 10 (total: %d)", deltaOrig, stats.CtxOrig)
			}
			if deltaComp < 10 {
				t.Errorf("delta ctx_comp = %d, want >= 10 (total: %d)", deltaComp, stats.CtxComp)
			}

			// Verify accumulation (monotonically increasing).
			if stats.CtxOrig <= prevOrig && i > 0 {
				t.Errorf("ctx_orig not accumulating: %d <= prev %d", stats.CtxOrig, prevOrig)
			}
			if stats.CtxComp <= prevComp && i > 0 {
				t.Errorf("ctx_comp not accumulating: %d <= prev %d", stats.CtxComp, prevComp)
			}

			// Verify orig >= comp (compression should save or break even).
			if deltaOrig < deltaComp {
				t.Errorf("orig (%d) < comp (%d) for this turn — compression inflated?", deltaOrig, deltaComp)
			}

			// Verify field preservation for the tools turn.
			if tt.extras != nil {
				mu.Lock()
				received := lastReceived
				mu.Unlock()

				var got map[string]json.RawMessage
				if err := json.Unmarshal(received, &got); err != nil {
					t.Fatalf("parse upstream body: %v", err)
				}
				for key := range tt.extras {
					if _, ok := got[key]; !ok {
						t.Errorf("extra field %q dropped from upstream request", key)
					}
				}
			}

			prevOrig = stats.CtxOrig
			prevComp = stats.CtxComp
		})
	}

	// Final summary.
	data, _ := os.ReadFile(ctxPath)
	var finalStats ctxStats
	_ = json.Unmarshal(data, &finalStats)
	t.Logf("FINAL: orig=%d comp=%d turns=%d saved=%d (%.1f%%)",
		finalStats.CtxOrig, finalStats.CtxComp, finalStats.Turns,
		finalStats.CtxOrig-finalStats.CtxComp,
		float64(finalStats.CtxOrig-finalStats.CtxComp)/float64(finalStats.CtxOrig)*100)
}
