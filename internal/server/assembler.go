package server

import (
	"strings"

	"github.com/pythondatascrape/engram/internal/provider"
)

// PromptParts holds the components of a structured prompt.
type PromptParts struct {
	Identity            string
	Knowledge           string
	ContextCodebookDef  string             // optional — injected as [CONTEXT_CODEBOOK] block
	ResponseCodebookDef string             // optional — injected as [RESPONSE_CODEBOOK] block
	History             []provider.Message // optional — injected as [HISTORY] block
	Query               string
}

// AssemblePrompt creates the structured system prompt with [IDENTITY],
// optional [KNOWLEDGE], optional [CONTEXT_CODEBOOK], optional [RESPONSE_CODEBOOK],
// optional [HISTORY], and [QUERY] delimiters.
func AssemblePrompt(parts PromptParts) string {
	var b strings.Builder

	b.WriteString("[IDENTITY]\n")
	b.WriteString(parts.Identity)
	b.WriteByte('\n')

	if parts.Knowledge != "" {
		b.WriteString("\n[KNOWLEDGE]\n")
		b.WriteString(parts.Knowledge)
		b.WriteByte('\n')
	}

	if parts.ContextCodebookDef != "" {
		b.WriteString("\n[CONTEXT_CODEBOOK]\n")
		b.WriteString(parts.ContextCodebookDef)
		b.WriteByte('\n')
	}

	if parts.ResponseCodebookDef != "" {
		b.WriteString("\n[RESPONSE_CODEBOOK]\n")
		b.WriteString(parts.ResponseCodebookDef)
		b.WriteByte('\n')
	}

	if len(parts.History) > 0 {
		b.WriteString("\n[HISTORY]\n")
		for _, msg := range parts.History {
			b.WriteString(msg.Role)
			b.WriteString(": ")
			b.WriteString(msg.Content)
			b.WriteByte('\n')
		}
	}

	b.WriteString("\n[QUERY]\n")
	b.WriteString(parts.Query)

	return b.String()
}
