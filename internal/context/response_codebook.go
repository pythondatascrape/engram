package context

// mustDeriveCodebook derives a ContextCodebook from a hardcoded schema.
// Panics if derivation fails — callers use known-good static schemas only.
func mustDeriveCodebook(name string, schema map[string]string) *ContextCodebook {
	cb, err := DeriveCodebook(name, schema)
	if err != nil {
		panic("context: mustDeriveCodebook: " + err.Error())
	}
	return cb
}

// Package-level singletons for built-in response codebooks.
var (
	anthropicCB = mustDeriveCodebook("anthropic_response", map[string]string{
		"role":          "enum:assistant",
		"stop_reason":   "enum:end_turn,max_tokens,stop_sequence,tool_use",
		"model":         "text",
		"input_tokens":  "text",
		"output_tokens": "text",
	})
	openaiCB = mustDeriveCodebook("openai_response", map[string]string{
		"role":              "enum:assistant",
		"finish_reason":     "enum:stop,length,tool_calls,content_filter",
		"model":             "text",
		"prompt_tokens":     "text",
		"completion_tokens": "text",
	})
)

// AnthropicResponseCodebook returns the built-in ContextCodebook for Anthropic response fields.
func AnthropicResponseCodebook() *ContextCodebook { return anthropicCB }

// OpenAIResponseCodebook returns the built-in ContextCodebook for OpenAI response fields.
func OpenAIResponseCodebook() *ContextCodebook { return openaiCB }
