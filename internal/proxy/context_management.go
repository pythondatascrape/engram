package proxy

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/pythondatascrape/engram/internal/config"
)

const contextManagementBeta = "context-management-2025-06-27"

// Option configures a proxy handler before it starts serving requests.
type Option func(*Handler)

// WithAnthropicContext enables server-side tool-result clearing. It never
// changes message history or thinking blocks, or client-supplied policies.
func WithAnthropicContext(c config.AnthropicContextConfig) Option {
	return func(h *Handler) {
		c.ExcludeTools = append([]string(nil), c.ExcludeTools...)
		h.anthropicContext = c
	}
}

func (h *Handler) withContextManagement(r *http.Request, body []byte, model string) ([]byte, bool) {
	c := h.anthropicContext
	if !c.Enabled || c.TriggerTokens <= 0 || c.KeepToolUses < 0 || c.ClearAtLeastTokens < 0 {
		return body, false
	}
	// Only advertise models whose context-management contract was verified.
	switch model {
	case "claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5":
	default:
		return body, false
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil {
		return body, false
	}
	if _, exists := raw["context_management"]; exists {
		return body, true
	}
	edit := map[string]any{
		"type":    "clear_tool_uses_20250919",
		"trigger": map[string]any{"type": "input_tokens", "value": c.TriggerTokens},
		"keep":    map[string]any{"type": "tool_uses", "value": c.KeepToolUses},
	}
	if c.ClearAtLeastTokens > 0 {
		edit["clear_at_least"] = map[string]any{"type": "input_tokens", "value": c.ClearAtLeastTokens}
	}
	if len(c.ExcludeTools) > 0 {
		edit["exclude_tools"] = c.ExcludeTools
	}
	policy, err := json.Marshal(map[string]any{"edits": []any{edit}})
	if err != nil {
		return body, false
	}
	raw["context_management"] = policy
	encoded, err := json.Marshal(raw)
	if err != nil {
		return body, false
	}
	// Clone the request headers before extending the beta list.
	r.Header = r.Header.Clone()
	betas := strings.Join(r.Header.Values("Anthropic-Beta"), ",")
	found := false
	for _, beta := range strings.Split(betas, ",") {
		if strings.TrimSpace(beta) == contextManagementBeta {
			found = true
		}
	}
	if !found {
		if betas != "" {
			betas += ","
		}
		betas += contextManagementBeta
	}
	r.Header.Set("Anthropic-Beta", betas)
	return encoded, true
}

// forwardContextManaged reports only server-confirmed savings. Responses are
// streamed unchanged; the trailer is available after the response is consumed.
func (h *Handler) forwardContextManaged(w http.ResponseWriter, r *http.Request, body []byte, system string) {
	resp, err := h.doUpstreamRequest(r, body, h.upstream)
	if err != nil {
		http.Error(w, "failed to forward request", http.StatusBadGateway)
		return
	}
	if resp.StatusCode != http.StatusOK {
		h.writeUpstreamResponse(w, resp)
		return
	}
	const trailer = "X-Engram-Cleared-Input-Tokens"
	w.Header().Add("Trailer", trailer)
	// A trailer requires a streamed transfer rather than a fixed content length.
	resp.Header.Del("Content-Length")
	captured := h.writeUpstreamResponse(w, resp)
	input, cleared := contextManagementUsage(captured)
	w.Header().Set(trailer, strconv.Itoa(cleared))
	if input < 0 {
		return // No estimates presented as measured savings.
	}
	id := r.Header.Get(engramSessionHeader)
	if id == "" || isPlaceholder(id) {
		id = h.claimPendingSession()
		if id == "" {
			id = SessionID(system)
		}
	}
	_ = WriteStatsDetailed(h.sessionsDir, id, turnStats{
		Total: tokenTotals{Orig: input + cleared, Comp: input, Saved: cleared},
	})
	if h.afterStats != nil {
		h.afterStats()
	}
}

// contextManagementUsage reads JSON or SSE usage without modifying the stream.
// Cache hits/writes are input tokens too, and must be included in totals.
func contextManagementUsage(body []byte) (input, cleared int) {
	input = -1
	type inputUsage struct {
		InputTokens   *int `json:"input_tokens"`
		CacheRead     int  `json:"cache_read_input_tokens"`
		CacheCreation int  `json:"cache_creation_input_tokens"`
	}
	consume := func(raw []byte) {
		var event struct {
			Type    string     `json:"type"`
			Usage   inputUsage `json:"usage"`
			Message struct {
				Usage inputUsage `json:"usage"`
			} `json:"message"`
			ContextManagement struct {
				AppliedEdits []struct {
					ClearedInputTokens int `json:"cleared_input_tokens"`
				} `json:"applied_edits"`
			} `json:"context_management"`
		}
		if json.Unmarshal(raw, &event) != nil {
			return
		}
		usage := event.Usage
		if event.Type == "message_start" {
			usage = event.Message.Usage
		}
		if n := usage.InputTokens; n != nil && *n >= 0 {
			input = *n + usage.CacheRead + usage.CacheCreation
		}
		if event.ContextManagement.AppliedEdits != nil {
			cleared = 0
			for _, edit := range event.ContextManagement.AppliedEdits {
				if edit.ClearedInputTokens > 0 {
					cleared += edit.ClearedInputTokens
				}
			}
		}
	}
	if json.Valid(body) {
		consume(body)
	} else {
		for _, line := range strings.Split(string(body), "\n") {
			if data, ok := strings.CutPrefix(strings.TrimSpace(line), "data:"); ok {
				consume([]byte(strings.TrimSpace(data)))
			}
		}
	}
	return input, cleared
}
