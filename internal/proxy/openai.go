package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
)

type openaiRequest struct {
	Messages []AnthropicMessage `json:"messages"`
}

type openaiResponsesRequest struct {
	Input        json.RawMessage `json:"input"`
	Instructions string          `json:"instructions"`
}

type openAIInputEnvelope struct {
	Raw     json.RawMessage
	Message AnthropicMessage
	Type    string
}

type openAIIdentityStats struct {
	SavedTokens int
}

// estimateOpenAITokens counts characters ÷ 4 across all message/input content strings
// in a raw OpenAI request body. Used as a pre-request fallback when EstimateTokens
// returns 0 (e.g. non-text content parts).
func estimateOpenAITokens(rawBody []byte) int {
	var body struct {
		Instructions string          `json:"instructions"`
		User         string          `json:"user"`
		Messages     json.RawMessage `json:"messages"`
		Input        json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return 0
	}
	total := len(body.Instructions) + len(body.User)
	if msgs, ok := decodeOpenAIChatMessages(body.Messages); ok {
		total += EstimateTokens(msgs)
	}
	if input, ok := decodeOpenAIInputEnvelopes(body.Input); ok {
		total += EstimateTokens(envelopeMessages(input))
	} else {
		total += estimateOpenAIInputText(body.Input)
	}
	if total == 0 {
		return 0
	}
	return total / 4
}

func estimateOpenAIInputText(raw json.RawMessage) int {
	return len(extractRawContentText(raw))
}

func rawContentText(raw json.RawMessage) int {
	return len(extractRawContentText(raw))
}

func decodeOpenAIChatMessages(raw json.RawMessage) ([]AnthropicMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var msgs []AnthropicMessage
	if err := json.Unmarshal(raw, &msgs); err == nil {
		return msgs, true
	}
	return nil, false
}

func compressOpenAIIdentityMessages(messages []AnthropicMessage) ([]AnthropicMessage, int) {
	if len(messages) == 0 {
		return messages, 0
	}
	out := make([]AnthropicMessage, len(messages))
	saved := 0
	for i, msg := range messages {
		out[i] = msg
		if msg.Role != "system" && msg.Role != "developer" {
			continue
		}
		content, ok := msg.Content.(string)
		if !ok {
			continue
		}
		if compressed, ok := codebook.CompressIfSafe(content); ok {
			out[i].Content = compressed
			saved += len(content)/4 - len(compressed)/4
		}
	}
	return out, saved
}

func decodeOpenAIInputMessages(raw json.RawMessage) ([]AnthropicMessage, bool) {
	envelopes, ok := decodeOpenAIInputEnvelopes(raw)
	if !ok {
		return nil, false
	}
	return envelopeMessages(envelopes), true
}

func estimateInstructionsTokens(instructions string) int {
	if instructions == "" {
		return 0
	}
	return len(instructions) / 4
}

func compressOpenAIInstructions(instructions string) (string, int) {
	if compressed, ok := codebook.CompressIfSafe(instructions); ok {
		return compressed, len(instructions)/4 - len(compressed)/4
	}
	return instructions, 0
}

func fallbackOpenAISessionID(rawBody []byte) string {
	var body struct {
		Instructions string          `json:"instructions"`
		User         string          `json:"user"`
		Model        string          `json:"model"`
		Messages     json.RawMessage `json:"messages"`
		Input        json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return SessionID(string(rawBody))
	}

	seedParts := make([]string, 0, 6)
	if body.Model != "" {
		seedParts = append(seedParts, "model="+body.Model)
	}
	if body.User != "" {
		seedParts = append(seedParts, "user="+body.User)
	}
	if body.Instructions != "" {
		seedParts = append(seedParts, "instructions="+body.Instructions)
	}

	appendStableTurns := func(items []AnthropicMessage) {
		var firstUser string
		for _, item := range items {
			text := strings.TrimSpace(messageText(item))
			if text == "" {
				continue
			}
			switch item.Role {
			case "system", "developer":
				seedParts = append(seedParts, fmt.Sprintf("%s=%s", item.Role, text))
			case "user":
				if firstUser == "" {
					firstUser = text
				}
			}
		}
		if firstUser != "" {
			seedParts = append(seedParts, "first_user="+firstUser)
		}
	}

	if msgs, ok := decodeOpenAIChatMessages(body.Messages); ok {
		appendStableTurns(msgs)
	}
	if input, ok := decodeOpenAIInputEnvelopes(body.Input); ok {
		appendStableTurns(envelopeMessages(input))
	} else if inputText := strings.TrimSpace(extractRawContentText(body.Input)); inputText != "" {
		appendStableTurns([]AnthropicMessage{{Role: "user", Content: inputText}})
	}

	if len(seedParts) == 0 {
		return SessionID(string(rawBody))
	}
	return SessionID(strings.Join(seedParts, "\n"))
}

func extractRawContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	if text := extractRawArrayText(raw); text != "" {
		return text
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		return extractMapText(obj)
	}
	return ""
}

func extractRawArrayText(raw json.RawMessage) string {
	var parts []map[string]any
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var texts []string
	for _, part := range parts {
		if text := extractMapText(part); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

func extractMapText(obj map[string]any) string {
	switch t, _ := obj["type"].(string); t {
	case "text", "input_text", "output_text":
		if text, _ := obj["text"].(string); text != "" {
			return text
		}
	case "input_image":
		if url, _ := obj["image_url"].(string); url != "" {
			return "[image] " + url
		}
		if imageURL, ok := obj["image_url"].(map[string]any); ok {
			if url, _ := imageURL["url"].(string); url != "" {
				return "[image] " + url
			}
		}
	case "input_file":
		if filename, _ := obj["filename"].(string); filename != "" {
			return "[file] " + filename
		}
		if fileURL, _ := obj["file_url"].(string); fileURL != "" {
			return "[file] " + fileURL
		}
		if fileID, _ := obj["file_id"].(string); fileID != "" {
			return "[file] " + fileID
		}
	case "refusal":
		if refusal, _ := obj["refusal"].(string); refusal != "" {
			return refusal
		}
	case "input_audio":
		if audio, ok := obj["input_audio"].(map[string]any); ok {
			if format, _ := audio["format"].(string); format != "" {
				return "[audio] " + format
			}
		}
		if format, _ := obj["format"].(string); format != "" {
			return "[audio] " + format
		}
		return "[audio]"
	case "function_call", "custom_tool_call":
		if name, _ := obj["name"].(string); name != "" {
			if input, _ := obj["input"].(string); input != "" {
				return name + ": " + input
			}
			if arguments, _ := obj["arguments"].(string); arguments != "" {
				return name + ": " + arguments
			}
			return name
		}
	case "function_call_output", "custom_tool_call_output", "computer_call_output":
		if output, ok := obj["output"]; ok {
			if b, err := json.Marshal(output); err == nil {
				return extractRawContentText(b)
			}
		}
		return t
	case "computer_call":
		if action, ok := obj["action"].(map[string]any); ok {
			if actionType, _ := action["type"].(string); actionType != "" {
				return "computer_call:" + actionType
			}
		}
		return "computer_call"
	case "reasoning":
		if summary, _ := obj["summary"].(string); summary != "" {
			return summary
		}
		return "reasoning"
	}

	if text, _ := obj["text"].(string); text != "" {
		return text
	}
	if output, ok := obj["output"].(string); ok && output != "" {
		return output
	}
	if content, ok := obj["content"]; ok {
		if b, err := json.Marshal(content); err == nil {
			return extractRawContentText(b)
		}
	}
	return ""
}

func decodeOpenAIInputEnvelopes(raw json.RawMessage) ([]openAIInputEnvelope, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var inputString string
	if json.Unmarshal(raw, &inputString) == nil {
		envelope := openAIInputEnvelope{
			Raw: mustMarshalJSON(map[string]any{
				"type":    "message",
				"role":    "user",
				"content": inputString,
			}),
			Message: AnthropicMessage{Role: "user", Content: inputString},
			Type:    "message",
		}
		return []openAIInputEnvelope{envelope}, true
	}

	var rawItems []json.RawMessage
	if json.Unmarshal(raw, &rawItems) != nil {
		return nil, false
	}

	envelopes := make([]openAIInputEnvelope, 0, len(rawItems))
	for _, itemRaw := range rawItems {
		msg, itemType, ok := openAIInputItemToMessage(itemRaw)
		if !ok {
			msg = AnthropicMessage{Role: "user", Content: extractRawContentText(itemRaw)}
			if msg.Content == "" {
				msg.Content = string(itemRaw)
			}
			itemType = "unknown"
		}
		envelopes = append(envelopes, openAIInputEnvelope{Raw: itemRaw, Message: msg, Type: itemType})
	}
	return envelopes, true
}

func openAIInputItemToMessage(raw json.RawMessage) (AnthropicMessage, string, bool) {
	var item struct {
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
		Output  json.RawMessage `json:"output"`
		Input   string          `json:"input"`
		Name    string          `json:"name"`
		Summary string          `json:"summary"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return AnthropicMessage{}, "", false
	}

	if item.Type == "" && item.Role != "" {
		return AnthropicMessage{Role: item.Role, Content: decodeContentForMessage(item.Content)}, "message", true
	}

	switch item.Type {
	case "message":
		role := item.Role
		if role == "" {
			role = "user"
		}
		return AnthropicMessage{Role: role, Content: decodeContentForMessage(item.Content)}, item.Type, true
	case "function_call_output", "custom_tool_call_output", "computer_call_output":
		return AnthropicMessage{Role: "tool", Content: decodeContentForMessage(item.Output)}, item.Type, true
	case "function_call", "custom_tool_call", "computer_call":
		return AnthropicMessage{Role: "assistant", Content: decodeContentForMessage(raw)}, item.Type, true
	case "reasoning":
		if item.Summary != "" {
			return AnthropicMessage{Role: "assistant", Content: item.Summary}, item.Type, true
		}
		return AnthropicMessage{Role: "assistant", Content: "reasoning"}, item.Type, true
	default:
		if item.Role != "" {
			return AnthropicMessage{Role: item.Role, Content: decodeContentForMessage(item.Content)}, item.Type, true
		}
	}

	return AnthropicMessage{}, "", false
}

func decodeContentForMessage(raw json.RawMessage) any {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var arr []any
	if json.Unmarshal(raw, &arr) == nil {
		return arr
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		return obj
	}
	return string(raw)
}

func envelopeMessages(envelopes []openAIInputEnvelope) []AnthropicMessage {
	msgs := make([]AnthropicMessage, 0, len(envelopes))
	for _, env := range envelopes {
		msgs = append(msgs, env.Message)
	}
	return msgs
}

func compressOpenAIInput(raw json.RawMessage, windowSize, budgetTokens int) (json.RawMessage, []AnthropicMessage, []AnthropicMessage, bool) {
	envelopes, ok := decodeOpenAIInputEnvelopes(raw)
	if !ok {
		return raw, nil, nil, false
	}

	envelopes, identityStats := compressOpenAIIdentityEnvelopes(envelopes)
	_ = identityStats
	original := envelopeMessages(envelopes)
	compressed := Compress(original, windowSize)
	if EstimateTokens(compressed) > budgetTokens {
		compressed = CompressBudget(compressed, budgetTokens)
	}
	if len(compressed) == len(original) {
		return raw, original, compressed, true
	}

	tailLen := len(compressed) - 1
	if tailLen < 0 || tailLen > len(envelopes) {
		return raw, original, compressed, true
	}
	tailStart := len(envelopes) - tailLen
	rawItems := make([]json.RawMessage, 0, len(compressed))
	rawItems = append(rawItems, marshalResponsesSummaryMessage(compressed[0]))
	for _, env := range envelopes[tailStart:] {
		rawItems = append(rawItems, compactOpenAITailEnvelope(env))
	}
	rawItems = compactRawTailToBudget(rawItems, budgetTokens)
	out, err := json.Marshal(rawItems)
	if err != nil {
		return raw, original, compressed, true
	}
	return out, original, compressed, true
}

func compressOpenAIIdentityEnvelopes(envelopes []openAIInputEnvelope) ([]openAIInputEnvelope, openAIIdentityStats) {
	if len(envelopes) == 0 {
		return envelopes, openAIIdentityStats{}
	}
	out := make([]openAIInputEnvelope, len(envelopes))
	stats := openAIIdentityStats{}
	for i, env := range envelopes {
		out[i] = env
		if env.Type != "message" {
			continue
		}
		if env.Message.Role != "system" && env.Message.Role != "developer" {
			continue
		}
		rawCompressed, compressedMessage, saved, ok := compressOpenAIIdentityEnvelope(env.Raw)
		if !ok {
			continue
		}
		out[i].Raw = rawCompressed
		out[i].Message = compressedMessage
		stats.SavedTokens += saved
	}
	return out, stats
}

func compressOpenAIIdentityEnvelope(raw json.RawMessage) (json.RawMessage, AnthropicMessage, int, bool) {
	var item map[string]any
	if json.Unmarshal(raw, &item) != nil {
		return nil, AnthropicMessage{}, 0, false
	}
	role, _ := item["role"].(string)
	if role != "system" && role != "developer" {
		return nil, AnthropicMessage{}, 0, false
	}
	originalText := ""
	if content, ok := item["content"]; ok {
		if b, err := json.Marshal(content); err == nil {
			originalText = extractRawContentText(b)
		}
	}
	if originalText == "" {
		return nil, AnthropicMessage{}, 0, false
	}
	compressed, ok := codebook.CompressIfSafe(originalText)
	if !ok {
		return nil, AnthropicMessage{}, 0, false
	}
	item["content"] = compressed
	rawCompressed := mustMarshalJSON(item)
	return rawCompressed, AnthropicMessage{Role: role, Content: compressed}, len(originalText)/4 - len(compressed)/4, true
}

func marshalResponsesSummaryMessage(msg AnthropicMessage) json.RawMessage {
	return mustMarshalJSON(map[string]any{
		"type":    "message",
		"role":    msg.Role,
		"content": msg.Content,
	})
}

func mustMarshalJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func compactOpenAITailEnvelope(env openAIInputEnvelope) json.RawMessage {
	switch env.Type {
	case "function_call_output", "custom_tool_call_output", "computer_call_output":
		return compactOpenAIRawEnvelope(env.Raw, 240)
	case "function_call", "custom_tool_call", "computer_call":
		return compactOpenAIRawEnvelope(env.Raw, 160)
	case "reasoning":
		return compactReasoningEnvelope(env.Raw)
	case "message":
		return compactOpenAIRawEnvelope(env.Raw, 240)
	default:
		return env.Raw
	}
}

func compactRawTailToBudget(items []json.RawMessage, budgetTokens int) []json.RawMessage {
	if budgetTokens <= 0 {
		return items
	}
	total := 0
	for _, item := range items {
		total += estimateOpenAIInputText(item) / 4
	}
	if total <= budgetTokens {
		return items
	}

	out := append([]json.RawMessage(nil), items...)
	for i := 1; i < len(out) && total > budgetTokens; i++ {
		compacted := compactOpenAIRawEnvelope(out[i], 120)
		before := estimateOpenAIInputText(out[i]) / 4
		after := estimateOpenAIInputText(compacted) / 4
		out[i] = compacted
		total -= before - after
	}
	return out
}

func compactReasoningEnvelope(raw json.RawMessage) json.RawMessage {
	var item map[string]any
	if json.Unmarshal(raw, &item) != nil {
		return raw
	}
	out := map[string]any{
		"type": "reasoning",
	}
	if id, ok := item["id"]; ok {
		out["id"] = id
	}
	if summary, ok := item["summary"]; ok {
		out["summary"] = summary
		return mustMarshalJSON(out)
	}
	out["summary"] = "reasoning"
	return mustMarshalJSON(out)
}

func compactOpenAIRawEnvelope(raw json.RawMessage, maxChars int) json.RawMessage {
	var item map[string]any
	if json.Unmarshal(raw, &item) != nil {
		return raw
	}
	compacted := compactOpenAIValue(item, maxChars)
	return mustMarshalJSON(compacted)
}

func compactOpenAIValue(v any, maxChars int) any {
	switch t := v.(type) {
	case string:
		return summarizeLongText(t, maxChars)
	case []any:
		out := make([]any, len(t))
		for i, elem := range t {
			out[i] = compactOpenAIValue(elem, maxChars)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for key, value := range t {
			switch key {
			case "text", "arguments", "input", "output", "summary":
				out[key] = compactOpenAIValue(value, maxChars)
			case "content":
				out[key] = compactOpenAIValue(value, maxChars)
			default:
				out[key] = value
			}
		}
		return out
	default:
		return v
	}
}

func summarizeLongText(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	if maxChars < 32 {
		maxChars = 32
	}
	head := (maxChars * 2) / 3
	tail := maxChars - head - len(" … ")
	if tail < 8 {
		tail = 8
		head = maxChars - tail - len(" … ")
	}
	if head < 8 {
		head = maxChars
	}
	if len(s) <= head+tail {
		return s[:maxChars]
	}
	return s[:head] + " … " + s[len(s)-tail:]
}

// parseOpenAIUsage extracts prompt_tokens from a /v1/chat/completions response body.
// Accuracy tier: response-provided (exact) > character estimate (rough).
// Returns -1 if unavailable; caller should fall back to estimateOpenAITokens.
func parseOpenAIUsage(body []byte) int {
	if len(body) == 0 {
		return -1
	}

	// Non-streaming: {"usage": {"prompt_tokens": N, ...}}
	var resp struct {
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil && resp.Usage.PromptTokens > 0 {
		return resp.Usage.PromptTokens
	}

	// Streaming SSE: scan for last data: chunk with usage
	best := -1
	for _, line := range bytes.Split(body, []byte("\n")) {
		s := strings.TrimPrefix(strings.TrimSpace(string(line)), "data: ")
		if s == "" || s == "[DONE]" {
			continue
		}
		var chunk struct {
			Usage struct {
				PromptTokens int `json:"prompt_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(s), &chunk) == nil && chunk.Usage.PromptTokens > 0 {
			best = chunk.Usage.PromptTokens
		}
	}
	return best
}

// parseOpenAIResponsesUsage extracts input_tokens from a /v1/responses response body.
// Accuracy tier: response-provided (exact) > character estimate (rough).
// Returns -1 if unavailable; caller should fall back to estimateOpenAITokens.
func parseOpenAIResponsesUsage(body []byte) int {
	if len(body) == 0 {
		return -1
	}

	var resp struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil && resp.Usage.InputTokens > 0 {
		return resp.Usage.InputTokens
	}

	best := -1
	for _, line := range bytes.Split(body, []byte("\n")) {
		s := strings.TrimPrefix(strings.TrimSpace(string(line)), "data: ")
		if s == "" || s == "[DONE]" {
			continue
		}
		var chunk struct {
			Usage struct {
				InputTokens int `json:"input_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(s), &chunk) == nil && chunk.Usage.InputTokens > 0 {
			best = chunk.Usage.InputTokens
		}
	}
	return best
}
