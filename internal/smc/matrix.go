package smc

import (
	"strings"
	"time"

	"github.com/pythondatascrape/engram/internal/provider"
)

// MatrixRow holds the decomposed categories for one completed conversation turn.
type MatrixRow struct {
	TurnIndex  int                 `json:"turn_index"`
	Categories map[string]string   `json:"categories"`
	CrossRefs  map[string][]string `json:"cross_refs,omitempty"`
	Preference *PreferenceSignal   `json:"preference,omitempty"`
	RawTokens  int                 `json:"raw_tokens"`
	CompTokens int                 `json:"comp_tokens"`
	Timestamp  time.Time           `json:"timestamp"`
}

// ConversationMatrix holds the structured compressed history for a session.
type ConversationMatrix struct {
	SessionID string
	Schema    CategorySchema
	K         KController
	rows      []MatrixRow
}

// NewConversationMatrix creates an empty matrix for a session.
func NewConversationMatrix(sessionID string, schema CategorySchema, k KController) *ConversationMatrix {
	return &ConversationMatrix{
		SessionID: sessionID,
		Schema:    schema,
		K:         k,
	}
}

// Append adds a decomposed turn to the matrix.
func (m *ConversationMatrix) Append(row MatrixRow) {
	m.rows = append(m.rows, row)
}

// Len returns the number of turns stored.
func (m *ConversationMatrix) Len() int {
	return len(m.rows)
}

// Messages serializes the matrix as provider.Message slices for LLM context injection.
// Each row becomes a single user message in "category=value | category=value" format.
func (m *ConversationMatrix) Messages() []provider.Message {
	if len(m.rows) == 0 {
		return nil
	}
	msgs := make([]provider.Message, 0, len(m.rows))
	names := m.Schema.Names()
	for _, row := range m.rows {
		var b strings.Builder
		first := true
		for _, name := range names {
			v, ok := row.Categories[name]
			if !ok || v == "" {
				continue
			}
			if !first {
				b.WriteString(" | ")
			}
			b.WriteString(name)
			b.WriteByte('=')
			b.WriteString(v)
			first = false
		}
		msgs = append(msgs, provider.Message{
			Role:    "user",
			Content: b.String(),
		})
	}
	return msgs
}

// TokenCount returns the sum of compressed token counts across all rows.
func (m *ConversationMatrix) TokenCount() int {
	total := 0
	for _, r := range m.rows {
		total += r.CompTokens
	}
	return total
}

// Rows returns a copy of the matrix rows.
func (m *ConversationMatrix) Rows() []MatrixRow {
	out := make([]MatrixRow, len(m.rows))
	copy(out, m.rows)
	return out
}
