package context

// AnthropicResponseCodebook returns the built-in ContextCodebook for Anthropic response fields.
func AnthropicResponseCodebook() *ContextCodebook {
	cb, _ := DeriveCodebook("anthropic_response", map[string]string{
		"role":           "enum:assistant",
		"stop_reason":    "enum:end_turn,max_tokens,stop_sequence,tool_use",
		"model":          "text",
		"input_tokens":   "text",
		"output_tokens":  "text",
	})
	return cb
}

// OpenAIResponseCodebook returns the built-in ContextCodebook for OpenAI response fields.
func OpenAIResponseCodebook() *ContextCodebook {
	cb, _ := DeriveCodebook("openai_response", map[string]string{
		"role":               "enum:assistant",
		"finish_reason":      "enum:stop,length,tool_calls,content_filter",
		"model":              "text",
		"prompt_tokens":      "text",
		"completion_tokens":  "text",
	})
	return cb
}
