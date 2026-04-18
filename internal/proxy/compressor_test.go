// internal/proxy/compressor_test.go
package proxy

import (
	"strings"
	"testing"
)

func TestCompress_FewerThanWindowNoop(t *testing.T) {
	msgs := []AnthropicMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}
	got := Compress(msgs, 10)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages unchanged, got %d", len(got))
	}
}

func TestCompress_ExactlyWindowNoop(t *testing.T) {
	msgs := make([]AnthropicMessage, 10)
	for i := range msgs {
		msgs[i] = AnthropicMessage{Role: "user", Content: "msg"}
	}
	got := Compress(msgs, 10)
	if len(got) != 10 {
		t.Fatalf("expected 10 messages unchanged, got %d", len(got))
	}
}

func TestCompress_HeadCompressedToSummaryBlock(t *testing.T) {
	// 12 messages, window=10 → head=2, tail=10
	msgs := make([]AnthropicMessage, 12)
	for i := range msgs {
		msgs[i] = AnthropicMessage{Role: "user", Content: "message content here"}
	}
	got := Compress(msgs, 10)

	// Result: 1 synthetic summary + 10 tail = 11
	if len(got) != 11 {
		t.Fatalf("expected 11 messages (1 summary + 10 tail), got %d", len(got))
	}
	// First message is the synthetic summary
	summary, ok := got[0].Content.(string)
	if !ok {
		t.Fatal("expected summary content to be a string")
	}
	if !strings.Contains(summary, "[CONTEXT_SUMMARY]") {
		t.Errorf("expected [CONTEXT_SUMMARY] block, got: %s", summary)
	}
	if !strings.Contains(summary, "[/CONTEXT_SUMMARY]") {
		t.Errorf("expected [/CONTEXT_SUMMARY] closing tag, got: %s", summary)
	}
	// Tail messages are verbatim
	for i, m := range got[1:] {
		if m.Content != "message content here" {
			t.Errorf("tail message %d content changed: %v", i, m.Content)
		}
	}
}

func TestCompress_LongContentTruncatedInSummary(t *testing.T) {
	long := strings.Repeat("x", 300)
	msgs := []AnthropicMessage{
		{Role: "user", Content: long},      // head
		{Role: "assistant", Content: long}, // head
		{Role: "user", Content: "recent"},  // tail (window=1)
	}
	got := Compress(msgs, 1)
	summary, _ := got[0].Content.(string)
	// Each head message is truncated to 120 chars + role prefix
	if len(summary) > 500 {
		t.Errorf("summary unexpectedly long (%d chars); expected truncation", len(summary))
	}
}

func TestEstimateTokens_Basic(t *testing.T) {
	msgs := []AnthropicMessage{
		{Role: "user", Content: strings.Repeat("a", 400)}, // 400 chars → 100 tokens
	}
	got := EstimateTokens(msgs)
	if got != 100 {
		t.Errorf("expected 100 tokens, got %d", got)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	got := EstimateTokens(nil)
	if got != 0 {
		t.Errorf("expected 0 tokens for nil, got %d", got)
	}
}

func TestCompressBudget_UnderBudgetNoop(t *testing.T) {
	msgs := []AnthropicMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}
	got := CompressBudget(msgs, 10000)
	if len(got) != len(msgs) {
		t.Fatalf("expected no change under budget, got %d messages", len(got))
	}
}

func TestCompressBudget_OverBudgetCollapses(t *testing.T) {
	// Build messages that exceed a tiny budget.
	// Each 400-char message ≈ 100 tokens.
	chunk := strings.Repeat("w", 400)
	var msgs []AnthropicMessage
	for i := 0; i < 6; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs = append(msgs, AnthropicMessage{Role: role, Content: chunk})
	}
	// budget = 200 tokens → only ~2 messages fit; rest should be summarised
	got := CompressBudget(msgs, 200)
	if len(got) >= len(msgs) {
		t.Fatalf("expected fewer messages after budget compression, got %d", len(got))
	}
	summary, ok := got[0].Content.(string)
	if !ok || !strings.Contains(summary, "[CONTEXT_SUMMARY]") {
		t.Errorf("expected [CONTEXT_SUMMARY] block in first message, got: %v", got[0].Content)
	}
}

func TestCompressBudget_SingleMessageNoop(t *testing.T) {
	msgs := []AnthropicMessage{{Role: "user", Content: "hi"}}
	got := CompressBudget(msgs, 1)
	if len(got) != 1 {
		t.Fatalf("expected single message preserved, got %d", len(got))
	}
}
