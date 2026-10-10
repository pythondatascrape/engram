package proxy_test

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pythondatascrape/engram/internal/config"
	"github.com/pythondatascrape/engram/internal/proxy"
	"github.com/stretchr/testify/require"
)

const toolHistory = `[{"role":"user","content":"read files"},{"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"original"},{"type":"tool_use","id":"a","name":"read","input":{}},{"type":"tool_use","id":"b","name":"read","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"a","content":"first file"},{"type":"tool_result","tool_use_id":"b","content":"second file"}]}]`

func TestProxyRequestsServerSideToolClearing(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"} {
		t.Run(model, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.JSONEq(t, toolHistory, string(body["messages"]))
				require.JSONEq(t, `{"edits":[{"type":"clear_tool_uses_20250919","trigger":{"type":"input_tokens","value":60000},"keep":{"type":"tool_uses","value":6},"clear_at_least":{"type":"input_tokens","value":10000},"exclude_tools":["memory"]}]}`, string(body["context_management"]))
				require.JSONEq(t, `{"effort":"medium"}`, string(body["output_config"]))
				require.Equal(t, "existing-beta,context-management-2025-06-27", r.Header.Get("Anthropic-Beta"))
				io.WriteString(w, `{"content":[],"usage":{"input_tokens":100}}`)
			}))
			defer upstream.Close()
			h := proxy.NewHandler(2, t.TempDir(), upstream.URL, "", proxy.WithAnthropicContext(config.AnthropicContextConfig{
				Enabled: true, TriggerTokens: 60000, KeepToolUses: 6, ClearAtLeastTokens: 10000, ExcludeTools: []string{"memory"},
			}))
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"`+model+`","messages":`+toolHistory+`,"max_tokens":4096,"output_config":{"effort":"medium"}}`))
			r.Header.Set("Anthropic-Beta", "existing-beta")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, 200, w.Code)
		})
	}
}

func TestProxyReportsExactClearingSavings(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(map[bool]string{false: "JSON", true: "SSE"}[streaming], func(t *testing.T) {
			payload := `{"usage":{"input_tokens":1200,"cache_read_input_tokens":800,"service_tier":"standard","cache_creation":{"ephemeral_5m_input_tokens":0}},"context_management":{"applied_edits":[{"type":"clear_tool_uses_20250919","cleared_input_tokens":7000}]}}`
			if streaming {
				payload = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1200,\"cache_read_input_tokens\":800,\"service_tier\":\"standard\",\"cache_creation\":{\"ephemeral_5m_input_tokens\":0}}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"context_management\":{\"applied_edits\":[{\"type\":\"clear_tool_uses_20250919\",\"cleared_input_tokens\":7000}]}}\n\n"
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if streaming {
					w.Header().Set("Content-Type", "text/event-stream")
				}
				io.WriteString(w, payload)
			}))
			defer upstream.Close()
			h := proxy.NewHandler(2, t.TempDir(), upstream.URL, "", proxy.WithAnthropicContext(config.AnthropicContextConfig{Enabled: true, TriggerTokens: 50000, KeepToolUses: 5}))
			local := httptest.NewServer(h)
			defer local.Close()
			resp, err := http.Post(local.URL+"/v1/messages", "application/json", strings.NewReader(`{"model":"claude-opus-5-5","messages":`+toolHistory+`}`))
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, payload, string(body))
			require.Equal(t, "7000", resp.Trailer.Get("X-Engram-Cleared-Input-Tokens"))
		})
	}
}

// Claude Code accepts "gzip, deflate, br, zstd"; savings must be read from a decoded response
// (phase 05, 2026-10-10: no stats were recorded because usage was parsed from compressed bytes).
func TestProxyRecordsSavingsFromCompressedResponses(t *testing.T) {
	payload := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1200,\"cache_read_input_tokens\":800}}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"context_management\":{\"applied_edits\":[{\"type\":\"clear_tool_uses_20250919\",\"cleared_input_tokens\":7000}]}}\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Like the API: compress whenever the request accepts gzip.
		require.NotContains(t, r.Header.Get("Accept-Encoding"), "zstd")
		w.Header().Set("Content-Type", "text/event-stream")
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			io.WriteString(w, payload)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		io.WriteString(zw, payload)
		zw.Close()
	}))
	defer upstream.Close()
	sessions := t.TempDir()
	h := proxy.NewHandler(2, sessions, upstream.URL, "", proxy.WithAnthropicContext(config.AnthropicContextConfig{Enabled: true, TriggerTokens: 50000, KeepToolUses: 5}))
	local := httptest.NewServer(h)
	defer local.Close()
	req, err := http.NewRequest(http.MethodPost, local.URL+"/v1/messages", strings.NewReader(`{"model":"claude-opus-5-5","messages":`+toolHistory+`}`))
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("X-Engram-Session", "coder-1")
	resp, err := http.DefaultTransport.RoundTrip(req) // no transparent decoding on the client side
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "", resp.Header.Get("Content-Encoding"))
	require.Equal(t, payload, string(body))
	require.Equal(t, "7000", resp.Trailer.Get("X-Engram-Cleared-Input-Tokens"))
	raw, err := os.ReadFile(filepath.Join(sessions, "coder-1.ctx.json"))
	require.NoError(t, err)
	var stats struct {
		Total struct{ Orig, Comp, Saved int } `json:"total"`
	}
	require.NoError(t, json.Unmarshal(raw, &stats))
	require.Equal(t, 9000, stats.Total.Orig)
	require.Equal(t, 2000, stats.Total.Comp)
	require.Equal(t, 7000, stats.Total.Saved)
}

// Claude Code sends its session id as X-Claude-Code-Session-Id (it has no setting for X-Engram-Session);
// stats are filed under it so the statusline finds them. An id unsafe as a file name is not used.
func TestProxyFilesSavingsUnderClaudeCodeSessionID(t *testing.T) {
	for _, tc := range []struct{ name, header, file string }{
		{"claude-code-id", "bcb2f048-5e4f-42e1-8e59-e705fb7028f2", "bcb2f048-5e4f-42e1-8e59-e705fb7028f2.ctx.json"},
		{"unsafe-id", "../escape", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `{"usage":{"input_tokens":100},"context_management":{"applied_edits":[]}}`)
			}))
			defer upstream.Close()
			root := t.TempDir()
			sessions := filepath.Join(root, "sessions")
			h := proxy.NewHandler(2, sessions, upstream.URL, "", proxy.WithAnthropicContext(config.AnthropicContextConfig{Enabled: true, TriggerTokens: 50000, KeepToolUses: 5}))
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-5-5","messages":`+toolHistory+`}`))
			r.Header.Set("X-Claude-Code-Session-Id", tc.header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, 200, w.Code)
			files, err := filepath.Glob(filepath.Join(sessions, "*.ctx.json"))
			require.NoError(t, err)
			require.Len(t, files, 1)
			if tc.file != "" {
				require.Equal(t, tc.file, filepath.Base(files[0]))
			} else {
				require.True(t, strings.HasPrefix(filepath.Base(files[0]), "proxy-"))
				_, err := os.Stat(filepath.Join(root, "escape.ctx.json"))
				require.True(t, os.IsNotExist(err))
			}
		})
	}
}

// Claude Code sends {"edits":[clear_thinking]} on every request; tool clearing joins it, after it
// (the API requires clear_thinking first). Client edits, other policy fields and history stay as sent.
func TestProxyAddsToolClearingAfterClientPolicy(t *testing.T) {
	history := `[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"user","content":"three"},{"role":"assistant","content":"four"},{"role":"user","content":"five"}]`
	for _, tc := range []struct{ name, beta, wantBeta string }{
		{"beta-present", "client-beta,context-management-2025-06-27", "client-beta,context-management-2025-06-27"},
		{"beta-missing", "client-beta", "client-beta,context-management-2025-06-27"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.JSONEq(t, history, string(body["messages"]))
				require.JSONEq(t, `{"future":1,"edits":[{"type":"clear_thinking_20251015","keep":"all"},{"type":"clear_tool_uses_20250919","trigger":{"type":"input_tokens","value":50000},"keep":{"type":"tool_uses","value":5},"clear_at_least":{"type":"input_tokens","value":25000}}]}`, string(body["context_management"]))
				require.Equal(t, tc.wantBeta, r.Header.Get("Anthropic-Beta"))
				io.WriteString(w, `{}`)
			}))
			defer upstream.Close()
			h := proxy.NewHandler(2, t.TempDir(), upstream.URL, "", proxy.WithAnthropicContext(config.AnthropicContextConfig{Enabled: true, TriggerTokens: 50000, KeepToolUses: 5, ClearAtLeastTokens: 25000}))
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-5-5","messages":`+history+`,"context_management":{"future":1,"edits":[{"type":"clear_thinking_20251015","keep":"all"}]}}`))
			r.Header.Set("Anthropic-Beta", tc.beta)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, 200, w.Code)
		})
	}
}

// A client that clears tool results itself, or sends a policy engram cannot read, is forwarded verbatim.
func TestProxyPreservesClientToolClearingAndUnknownPolicies(t *testing.T) {
	for name, policy := range map[string]string{
		"client-clears-tools": `{"edits":[{"type":"clear_thinking_20251015","keep":"all"},{"type":"clear_tool_uses_20250919","keep":{"type":"tool_uses","value":2}}]}`,
		"edits-not-a-list":    `{"edits":"all"}`,
		"policy-not-object":   `[1]`,
		"edit-not-object":     `{"edits":[1]}`,
	} {
		t.Run(name, func(t *testing.T) {
			body := `{"model":"claude-opus-5-5","messages":[{"role":"user","content":"one"}],"context_management":` + policy + `}`
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Equal(t, body, string(got))
				require.Equal(t, "client-beta", r.Header.Get("Anthropic-Beta"))
				io.WriteString(w, `{}`)
			}))
			defer upstream.Close()
			h := proxy.NewHandler(2, t.TempDir(), upstream.URL, "", proxy.WithAnthropicContext(config.AnthropicContextConfig{Enabled: true, TriggerTokens: 50000, KeepToolUses: 5}))
			r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
			r.Header.Set("Anthropic-Beta", "client-beta")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, 200, w.Code)
		})
	}
}

func TestProxyToolClearingPassThroughGuards(t *testing.T) {
	for _, tc := range []struct {
		name, model string
		enabled     bool
	}{
		{"disabled", "claude-opus-5-5", false},
		{"unknown-model", "future-claude", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"` + tc.model + `","messages":` + toolHistory + `}`
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.Equal(t, body, string(got))
				require.Equal(t, "", r.Header.Get("Anthropic-Beta"))
				io.WriteString(w, `{}`)
			}))
			defer upstream.Close()
			h := proxy.NewHandler(2, t.TempDir(), upstream.URL, "", proxy.WithAnthropicContext(config.AnthropicContextConfig{Enabled: tc.enabled, TriggerTokens: 50000, KeepToolUses: 5}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))
			require.Equal(t, 200, w.Code)
		})
	}
}

func TestProxyPreservesClaude55PrefixBeforeThinking(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			history := `[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"user","content":"three"},{"role":"assistant","content":"four"},{"role":"user","content":"five"}]`
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]json.RawMessage
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.JSONEq(t, history, string(body["messages"]))
				_, managed := body["context_management"]
				require.Equal(t, enabled, managed)
				io.WriteString(w, `{}`)
			}))
			defer upstream.Close()
			h := proxy.NewHandler(2, t.TempDir(), upstream.URL, "", proxy.WithAnthropicContext(config.AnthropicContextConfig{Enabled: enabled, TriggerTokens: 50000, KeepToolUses: 5}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-5-5","messages":`+history+`}`)))
			require.Equal(t, 200, w.Code)
		})
	}
}
