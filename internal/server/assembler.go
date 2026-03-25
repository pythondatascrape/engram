package server

import "strings"

// PromptParts holds the components of a structured prompt.
type PromptParts struct {
	Identity  string
	Knowledge string
	Query     string
}

// AssemblePrompt creates the structured system prompt with delimiters.
// The assembled prompt uses clear structural delimiters to help the LLM
// distinguish between identity context, knowledge, and user input.
//
// If Knowledge is empty, the [KNOWLEDGE] section is omitted entirely.
//
// Example output:
//
//	[IDENTITY]
//	domain=fire rank=captain experience=20
//
//	[KNOWLEDGE]
//	Fire code Section 4.2: All commercial buildings require...
//
//	[QUERY]
//	What are the egress requirements?
func AssemblePrompt(parts PromptParts) string {
	var b strings.Builder

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
