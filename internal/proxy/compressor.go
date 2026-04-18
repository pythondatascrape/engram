// internal/proxy/compressor.go
package proxy

import (
	"encoding/json"
	"regexp"
	"strings"
)

// AnthropicMessage is the wire format for a single message in the Anthropic API.
// Content is string for simple text, or []interface{} for structured content blocks
// (tool use, images). json.Unmarshal decodes JSON arrays into []interface{}, not []map[string]any.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// Compress splits messages into a tail (last windowSize messages kept verbatim)
// and a head (older messages compressed into a [CONTEXT_SUMMARY] block).
// If len(messages) <= windowSize, messages are returned unchanged.
func Compress(messages []AnthropicMessage, windowSize int) []AnthropicMessage {
	if windowSize < 2 || len(messages) <= windowSize {
		return messages
	}
	head := messages[:len(messages)-windowSize]
	tail := messages[len(messages)-windowSize:]
	summary := compressHead(head)
	synthetic := AnthropicMessage{
		Role:    "user",
		Content: "[CONTEXT_SUMMARY]\n" + summary + "\n[/CONTEXT_SUMMARY]",
	}
	result := make([]AnthropicMessage, 0, 1+len(tail))
	result = append(result, synthetic)
	result = append(result, tail...)
	return result
}

// EstimateTokens returns a rough token count for a messages slice (len/4 per message text).
func EstimateTokens(messages []AnthropicMessage) int {
	total := 0
	for _, m := range messages {
		total += len(messageText(m)) / 4
	}
	return total
}

// compressHead reduces head messages to a concise role: text summary.
func compressHead(msgs []AnthropicMessage) string {
	parts := make([]string, len(msgs))
	for i, m := range msgs {
		text := messageText(m)
		// Only convert to runes when the byte length could exceed the limit,
		// avoiding allocation for the common short-message case.
		if len(text) > 120 {
			if runes := []rune(text); len(runes) > 120 {
				text = string(runes[:120])
			}
		}
		parts[i] = m.Role + ": " + text
	}
	return strings.Join(parts, "\n")
}

// messageText extracts text from a message content field regardless of type.
func messageText(m AnthropicMessage) string {
	switch v := m.Content.(type) {
	case string:
		return v
	default:
		if summary := summariseMessage(m); summary != "" {
			return summary
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "<unmarshalable content>"
		}
		return string(b)
	}
}

// CompressBudget returns a copy of messages that fits within budgetTokens (by
// estimate). Tail messages are kept verbatim; head messages are summarised:
//   - tool_use / tool_result turns → "name: <outcome>" (one line)
//   - text turns → first sentence only
//
// If the full set already fits, it is returned as-is.
func CompressBudget(messages []AnthropicMessage, budgetTokens int) []AnthropicMessage {
	if EstimateTokens(messages) <= budgetTokens {
		return messages
	}
	// Walk from the tail, accumulating verbatim messages until budget is met.
	// The remaining head is summarised into a single user message.
	used := 0
	splitIdx := len(messages) // index where the verbatim tail starts
	for i := len(messages) - 1; i >= 0; i-- {
		cost := EstimateTokens([]AnthropicMessage{messages[i]})
		if used+cost > budgetTokens && i < len(messages)-1 {
			splitIdx = i + 1
			break
		}
		used += cost
		splitIdx = i
	}
	if splitIdx == 0 {
		// Budget too tight to fit even the tail — return as-is (fail-open).
		return messages
	}

	head := messages[:splitIdx]
	summaryLines := make([]string, 0, len(head))
	for _, m := range head {
		line := summariseMessage(m)
		if line != "" {
			summaryLines = append(summaryLines, m.Role+": "+line)
		}
	}
	summary := "[CONTEXT_SUMMARY]\n" + strings.Join(summaryLines, "\n") + "\n[/CONTEXT_SUMMARY]"
	out := make([]AnthropicMessage, 0, 1+len(messages)-splitIdx)
	out = append(out, AnthropicMessage{Role: "user", Content: summary})
	out = append(out, messages[splitIdx:]...)
	return out
}

var sentenceEnd = regexp.MustCompile(`[.!?]\s`)

// summariseMessage returns a one-line summary of a message's content.
func summariseMessage(m AnthropicMessage) string {
	switch v := m.Content.(type) {
	case string:
		return firstSentence(v)
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, block := range v {
			if line := summariseContentBlock(block); line != "" {
				parts = append(parts, line)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]interface{}:
		return summariseContentBlock(v)
	default:
		return ""
	}
}

func summariseContentBlock(block any) string {
	bm, ok := block.(map[string]interface{})
	if !ok {
		return ""
	}
	switch bm["type"] {
	case "tool_use":
		name, _ := bm["name"].(string)
		return "tool_use:" + name
	case "tool_result":
		name, _ := bm["tool_use_id"].(string)
		return "tool_result:" + name + "=" + firstSentence(extractBlockText(bm["content"]))
	case "function_call":
		name, _ := bm["name"].(string)
		args, _ := bm["arguments"].(string)
		if args == "" {
			args, _ = bm["input"].(string)
		}
		if args == "" {
			return "function_call:" + name
		}
		return "function_call:" + name + "=" + firstSentence(args)
	case "custom_tool_call":
		name, _ := bm["name"].(string)
		input, _ := bm["input"].(string)
		if input == "" {
			return "custom_tool:" + name
		}
		return "custom_tool:" + name + "=" + firstSentence(input)
	case "computer_call":
		action, _ := bm["action"].(map[string]interface{})
		if actionType, _ := action["type"].(string); actionType != "" {
			return "computer_call:" + actionType
		}
		return "computer_call"
	case "text", "input_text", "output_text":
		if t, ok := bm["text"].(string); ok {
			return firstSentence(t)
		}
	case "refusal":
		if t, ok := bm["refusal"].(string); ok {
			return "refusal:" + firstSentence(t)
		}
	case "input_image":
		if imageURL, ok := bm["image_url"].(string); ok && imageURL != "" {
			return "[image] " + imageURL
		}
		if imageURL, ok := bm["image_url"].(map[string]interface{}); ok {
			if url, _ := imageURL["url"].(string); url != "" {
				return "[image] " + url
			}
		}
		if fileID, _ := bm["file_id"].(string); fileID != "" {
			return "[image] " + fileID
		}
		return "[image]"
	case "input_file":
		if filename, _ := bm["filename"].(string); filename != "" {
			return "[file] " + filename
		}
		if fileURL, _ := bm["file_url"].(string); fileURL != "" {
			return "[file] " + fileURL
		}
		if fileID, _ := bm["file_id"].(string); fileID != "" {
			return "[file] " + fileID
		}
		return "[file]"
	case "input_audio":
		if format, _ := bm["format"].(string); format != "" {
			return "[audio] " + format
		}
		if audio, ok := bm["input_audio"].(map[string]interface{}); ok {
			if format, _ := audio["format"].(string); format != "" {
				return "[audio] " + format
			}
		}
		return "[audio]"
	case "computer_call_output":
		return "computer_output:" + firstSentence(extractBlockText(bm["output"]))
	case "reasoning":
		return "reasoning"
	}
	return firstSentence(extractBlockText(block))
}

func extractBlockText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := summariseContentBlock(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]interface{}:
		if text, ok := t["text"].(string); ok {
			return text
		}
		if output, ok := t["output"].(string); ok {
			return output
		}
		if data, err := json.Marshal(t); err == nil {
			return string(data)
		}
	}
	return ""
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if loc := sentenceEnd.FindStringIndex(s); loc != nil {
		return s[:loc[0]+1]
	}
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
