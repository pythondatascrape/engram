package proxy_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestProxyPreservesClientContextPolicy(t *testing.T) {
	// Even plain text history must not be rewritten under a client's policy.
	body := `{"model":"claude-opus-5-5","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"user","content":"three"},{"role":"assistant","content":"four"},{"role":"user","content":"five"}],"context_management":{"edits":[{"type":"clear_thinking_20251015","keep":"all"}]}}`
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
