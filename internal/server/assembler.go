package server

import "strings"

// PromptParts holds the components of a structured prompt.
type PromptParts struct {
	Identity  string
	Knowledge string
	Query     string
}

// AssemblePrompt creates the structured system prompt with [IDENTITY],
// optional [KNOWLEDGE], and [QUERY] delimiters.
func AssemblePrompt(parts PromptParts) string {
	// Pre-calculate exact size to avoid reallocation.
	// Fixed delimiters: "[IDENTITY]\n" (11) + "\n" (1) + "\n[QUERY]\n" (9) = 21
	size := 21 + len(parts.Identity) + len(parts.Query)
	if parts.Knowledge != "" {
		// "\n[KNOWLEDGE]\n" (14) + "\n" (1) = 15
		size += 15 + len(parts.Knowledge)
	}

	var b strings.Builder
	b.Grow(size)

	b.WriteString("[IDENTITY]\n")
	b.WriteString(parts.Identity)
	b.WriteString("\n")

	if parts.Knowledge != "" {
		b.WriteString("\n[KNOWLEDGE]\n")
		b.WriteString(parts.Knowledge)
		b.WriteString("\n")
	}

	b.WriteString("\n[QUERY]\n")
	b.WriteString(parts.Query)

	return b.String()
}
