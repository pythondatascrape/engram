package smc

import (
	"context"
	"strings"
	"time"
)

// Exchange holds a completed user+assistant turn for decomposition.
type Exchange struct {
	UserMessage      string
	AssistantMessage string
	TurnIndex        int
}

// Decomposer extracts category vectors from a completed exchange.
type Decomposer interface {
	Decompose(ctx context.Context, exchange Exchange, schema CategorySchema) (*MatrixRow, error)
}

// RuleDecomposer is a heuristic-based decomposer for Phase 1.
// It splits the exchange text by category using simple rules.
// An LLM-based decomposer can replace this behind the same interface.
type RuleDecomposer struct {
	k KController
}

// NewRuleDecomposer creates a rule-based decomposer.
func NewRuleDecomposer(k KController) *RuleDecomposer {
	return &RuleDecomposer{k: k}
}

// Decompose extracts categories from the exchange using heuristic rules.
func (d *RuleDecomposer) Decompose(_ context.Context, exchange Exchange, schema CategorySchema) (*MatrixRow, error) {
	combined := exchange.UserMessage + " " + exchange.AssistantMessage
	rawTokens := len(combined) / 4
	if rawTokens == 0 {
		rawTokens = 1
	}

	categories := make(map[string]string, len(schema.Categories))

	for _, cat := range schema.Categories {
		content := d.extractCategory(cat, exchange)
		k := d.k.EffectiveK(cat.Name)
		content = d.compress(content, k)
		categories[cat.Name] = content
	}

	compTokens := 0
	for _, v := range categories {
		compTokens += len(v) / 4
	}
	if compTokens == 0 {
		compTokens = 1
	}

	return &MatrixRow{
		TurnIndex:  exchange.TurnIndex,
		Categories: categories,
		RawTokens:  rawTokens,
		CompTokens: compTokens,
		Timestamp:  time.Now(),
	}, nil
}

// extractCategory pulls relevant content for a category from the exchange.
func (d *RuleDecomposer) extractCategory(cat Category, exchange Exchange) string {
	switch cat.Name {
	case "intent":
		return extractIntent(exchange.UserMessage)
	case "entities":
		return extractEntities(exchange.UserMessage + " " + exchange.AssistantMessage)
	case "mutations":
		return extractMutations(exchange.AssistantMessage)
	case "context":
		return extractContext(exchange.UserMessage)
	default:
		return summarize(exchange.UserMessage+" "+exchange.AssistantMessage, cat.Description)
	}
}

// compress reduces text length based on k value.
// k=0: aggressive (keep ~10% of text). k=1: preserve most text.
func (d *RuleDecomposer) compress(text string, k float64) string {
	if text == "" {
		return ""
	}
	targetRatio := 0.1 + 0.9*k
	targetLen := int(float64(len([]rune(text))) * targetRatio)
	if targetLen < 1 {
		targetLen = 1
	}

	runes := []rune(text)
	if len(runes) <= targetLen {
		return text
	}
	return string(runes[:targetLen])
}

func extractIntent(msg string) string {
	if msg == "" {
		return ""
	}
	for i, r := range msg {
		if r == '.' || r == '!' || r == '?' {
			return msg[:i+1]
		}
	}
	return msg
}

func extractEntities(text string) string {
	if text == "" {
		return ""
	}
	var entities []string
	words := strings.Fields(text)
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'`()[]{}")
		if clean == "" {
			continue
		}
		if strings.Contains(clean, ".") && !strings.HasPrefix(clean, "e.g") && !strings.HasPrefix(clean, "i.e") {
			entities = append(entities, clean)
			continue
		}
		if strings.Contains(clean, "_") || (len(clean) > 1 && clean[0] >= 'a' && clean[0] <= 'z' && strings.ContainsAny(clean, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")) {
			entities = append(entities, clean)
			continue
		}
	}
	if len(entities) == 0 {
		return strings.Join(words[:min(len(words), 5)], " ")
	}
	return strings.Join(entities, ", ")
}

func extractMutations(msg string) string {
	if msg == "" {
		return ""
	}
	indicators := []string{"updated", "added", "removed", "replaced", "refactored", "created", "deleted", "modified", "changed", "fixed", "implemented"}
	var mutations []string
	sentences := strings.Split(msg, ".")
	for _, s := range sentences {
		lower := strings.ToLower(s)
		for _, ind := range indicators {
			if strings.Contains(lower, ind) {
				mutations = append(mutations, strings.TrimSpace(s))
				break
			}
		}
	}
	if len(mutations) == 0 {
		return msg
	}
	return strings.Join(mutations, ". ")
}

func extractContext(msg string) string {
	if msg == "" {
		return ""
	}
	indicators := []string{"because", "since", "so that", "in order to", "the reason", "due to", "need to", "should", "must", "want to"}
	var reasons []string
	sentences := strings.Split(msg, ".")
	for _, s := range sentences {
		lower := strings.ToLower(s)
		for _, ind := range indicators {
			if strings.Contains(lower, ind) {
				reasons = append(reasons, strings.TrimSpace(s))
				break
			}
		}
	}
	if len(reasons) == 0 {
		return msg
	}
	return strings.Join(reasons, ". ")
}

func summarize(text, _ string) string {
	runes := []rune(text)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return text
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
