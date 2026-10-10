package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicProtocolPassThrough(t *testing.T) {
	for _, model := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"} {
		for _, block := range []string{
			`{"type":"tool_use","id":"a","name":"read","input":{}},{"type":"tool_use","id":"b","name":"read","input":{}}`,
			`{"type":"thinking","thinking":"","signature":"signed-original"}`,
			`{"type":"redacted_thinking","data":"opaque"}`,
			`{"type":"server_tool_use","id":"s","name":"web_search","input":{}}`,
			`{"type":"future_protocol_block","opaque":"preserve"}`,
		} {
			t.Run(model+"/"+block, func(t *testing.T) {
				t.Parallel()
				messages := makeMessages(9)
				// Both compression passes would otherwise discard the assistant
				// calls while retaining the two parallel results.
				messages[0].Content = strings.Repeat("history ", 14000)
				var blocks []any
				if err := json.Unmarshal([]byte("["+block+"]"), &blocks); err != nil {
					t.Fatal(err)
				}
				messages[7] = AnthropicMessage{Role: "assistant", Content: blocks}
				messages[8] = AnthropicMessage{Role: "user", Content: []any{
					map[string]any{"type": "tool_result", "tool_use_id": "a", "content": "one"},
					map[string]any{"type": "tool_result", "tool_use_id": "b", "content": "two"},
				}}
				encoded, err := json.Marshal(messages)
				if err != nil {
					t.Fatal(err)
				}
				body := fmt.Sprintf(`{ "model":%q,"messages":%s,"tools":[{"name":"read","input_schema":{"type":"object"}}],"thinking":{"type":"adaptive"},"output_config":{"effort":"medium"},"max_tokens":4096,"stream":true }`, model, encoded)
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					got, _ := io.ReadAll(r.Body)
					if string(got) != body {
						t.Error("protocol request body changed")
					}
					if r.Header.Get("Anthropic-Beta") != "test-beta" {
						t.Error("beta header lost")
					}
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
				}))
				defer upstream.Close()
				h := NewHandler(2, t.TempDir(), upstream.URL, "")
				resp := postRawBody(t, h, body, map[string]string{"Anthropic-Beta": "test-beta"})
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("status %d", resp.StatusCode)
				}
				got, _ := io.ReadAll(resp.Body)
				if string(got) != "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n" {
					t.Error("stream changed")
				}
			})
		}
	}
}

func TestAnthropicProtocolDetection(t *testing.T) {
	for _, content := range []any{
		[]any{map[string]any{"type": "thinking", "signature": "signed"}},
		[]any{map[string]any{"type": "tool_result", "tool_use_id": "a"}},
		nil,
	} {
		if !hasAnthropicProtocolContent([]AnthropicMessage{{Role: "assistant", Content: content}}) {
			t.Fatalf("expected pass-through for %v", content)
		}
	}
	if hasAnthropicProtocolContent([]AnthropicMessage{{Role: "user", Content: []any{map[string]any{"type": "text", "text": "hello"}}}}) {
		t.Fatal("plain text should remain compressible")
	}
}
