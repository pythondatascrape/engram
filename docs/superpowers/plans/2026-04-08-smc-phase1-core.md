# SMC Phase 1: Core Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Engram's windowed compression with per-turn structured matrix decomposition controlled by a configurable category schema and k-parameter.

**Architecture:** The new `internal/smc/` package provides the decomposer, matrix, k-controller, and category schema. The proxy handler (`internal/proxy/handler.go`) swaps `Compress()` for matrix-based compression. Config extends `ProxyConfig` with SMC settings. The existing codebook system adapts to serialize matrix rows.

**Tech Stack:** Go, testify, SQLite (Phase 2 — interfaces only here)

---

## File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/smc/category.go` | Category and CategorySchema types, default schema, YAML loading |
| Create | `internal/smc/category_test.go` | Category schema tests |
| Create | `internal/smc/k.go` | KController, compression ratio, per-category overrides |
| Create | `internal/smc/k_test.go` | k-parameter tests |
| Create | `internal/smc/matrix.go` | MatrixRow, ConversationMatrix, serialization to provider.Message |
| Create | `internal/smc/matrix_test.go` | Matrix append, serialization, token counting |
| Create | `internal/smc/decompose.go` | Decomposer interface, rule-based decomposer for Phase 1 |
| Create | `internal/smc/decompose_test.go` | Decomposer tests |
| Create | `internal/smc/preference.go` | PreferenceSignal type |
| Modify | `internal/config/config.go:157-160` | Add SMCConfig to ProxyConfig |
| Modify | `internal/config/config_test.go` | Test SMC config defaults and loading |
| Modify | `internal/proxy/handler.go:187` | Replace `Compress()` call with matrix decomposition |
| Modify | `internal/proxy/handler.go:24-51` | Add matrix + decomposer fields to Handler |
| Modify | `internal/proxy/handler_test.go` | Update handler tests for SMC path |
| Modify | `cmd/engram/serve.go:170` | Pass SMC config to proxy |

---

### Task 1: Category Schema Types

**Files:**
- Create: `internal/smc/category.go`
- Create: `internal/smc/category_test.go`

- [ ] **Step 1: Write the failing test for Category and CategorySchema**

```go
// internal/smc/category_test.go
package smc_test

import (
	"testing"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSchema(t *testing.T) {
	schema := smc.DefaultSchema()
	require.Len(t, schema.Categories, 4)

	names := make([]string, len(schema.Categories))
	for i, c := range schema.Categories {
		names[i] = c.Name
	}
	assert.Equal(t, []string{"intent", "entities", "mutations", "context"}, names)

	// All default categories use global k (indicated by -1)
	for _, c := range schema.Categories {
		assert.Equal(t, -1.0, c.K, "category %s should use global k", c.Name)
	}

	assert.True(t, schema.CrossRefs)
}

func TestCategorySchema_Validate(t *testing.T) {
	tests := []struct {
		name    string
		schema  smc.CategorySchema
		wantErr string
	}{
		{
			name:    "empty categories",
			schema:  smc.CategorySchema{},
			wantErr: "at least one category",
		},
		{
			name: "duplicate names",
			schema: smc.CategorySchema{
				Categories: []smc.Category{
					{Name: "intent", Description: "a"},
					{Name: "intent", Description: "b"},
				},
			},
			wantErr: "duplicate category",
		},
		{
			name: "blank name",
			schema: smc.CategorySchema{
				Categories: []smc.Category{
					{Name: "", Description: "a"},
				},
			},
			wantErr: "name must not be empty",
		},
		{
			name: "valid custom schema",
			schema: smc.CategorySchema{
				Categories: []smc.Category{
					{Name: "diagnosis", Description: "medical diagnosis", K: 0.3},
					{Name: "treatment", Description: "treatment plan", K: 0.7},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -run TestDefaultSchema -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement Category and CategorySchema**

```go
// internal/smc/category.go
package smc

import "fmt"

// Category defines a single decomposition dimension.
type Category struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	K           float64 `yaml:"k"` // per-category k; -1 means use global
}

// CategorySchema defines the set of categories used for turn decomposition.
type CategorySchema struct {
	Categories []Category `yaml:"categories"`
	CrossRefs  bool       `yaml:"cross_refs"`
}

// Validate checks that the schema has at least one category, no duplicates,
// and no blank names.
func (s CategorySchema) Validate() error {
	if len(s.Categories) == 0 {
		return fmt.Errorf("smc: schema must have at least one category")
	}
	seen := make(map[string]bool, len(s.Categories))
	for _, c := range s.Categories {
		if c.Name == "" {
			return fmt.Errorf("smc: category name must not be empty")
		}
		if seen[c.Name] {
			return fmt.Errorf("smc: duplicate category %q", c.Name)
		}
		seen[c.Name] = true
	}
	return nil
}

// Names returns the ordered list of category names.
func (s CategorySchema) Names() []string {
	names := make([]string, len(s.Categories))
	for i, c := range s.Categories {
		names[i] = c.Name
	}
	return names
}

// DefaultSchema returns the default four-category schema for software engineering.
func DefaultSchema() CategorySchema {
	return CategorySchema{
		CrossRefs: true,
		Categories: []Category{
			{Name: "intent", Description: "What the speaker wants or is doing — semantic purpose, goals, directives", K: -1},
			{Name: "entities", Description: "Files, functions, concepts referenced — concrete nouns, identifiers, domain objects", K: -1},
			{Name: "mutations", Description: "What changed — code diffs, state transitions, decisions, commitments", K: -1},
			{Name: "context", Description: "Reasoning, constraints, rationale — the 'why' behind actions and decisions", K: -1},
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -v`
Expected: PASS (both TestDefaultSchema and TestCategorySchema_Validate)

- [ ] **Step 5: Commit**

```bash
git add internal/smc/category.go internal/smc/category_test.go
git commit -m "feat(smc): add category schema types with defaults and validation"
```

---

### Task 2: k-Parameter Controller

**Files:**
- Create: `internal/smc/k.go`
- Create: `internal/smc/k_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/smc/k_test.go
package smc_test

import (
	"testing"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
)

func TestKController_EffectiveK(t *testing.T) {
	kc := smc.KController{
		Global:      0.5,
		PerCategory: map[string]float64{"entities": 0.3},
	}

	assert.Equal(t, 0.5, kc.EffectiveK("intent"))
	assert.Equal(t, 0.3, kc.EffectiveK("entities"))
	assert.Equal(t, 0.5, kc.EffectiveK("unknown"))
}

func TestKController_EffectiveK_NegativeOverrideUsesGlobal(t *testing.T) {
	kc := smc.KController{
		Global:      0.5,
		PerCategory: map[string]float64{"intent": -1},
	}
	assert.Equal(t, 0.5, kc.EffectiveK("intent"))
}

func TestKController_CompressionRatio(t *testing.T) {
	kc := smc.KController{Global: 0.5}

	// compression_ratio = 1 - (1 - min_ratio) * k
	// With min_ratio=0.1, k=0.5: 1 - (1-0.1)*0.5 = 1 - 0.45 = 0.55
	ratio := kc.CompressionRatio("intent", 0.1)
	assert.InDelta(t, 0.55, ratio, 0.001)
}

func TestKController_CompressionRatio_Extremes(t *testing.T) {
	tests := []struct {
		name     string
		k        float64
		minRatio float64
		want     float64
	}{
		{"k=0 maximum compression", 0.0, 0.1, 1.0},
		{"k=1 minimum compression", 1.0, 0.1, 0.1},
		{"k=0.5 mid", 0.5, 0.0, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := smc.KController{Global: tt.k}
			got := kc.CompressionRatio("any", tt.minRatio)
			assert.InDelta(t, tt.want, got, 0.001)
		})
	}
}

func TestNewKController(t *testing.T) {
	schema := smc.DefaultSchema()
	// Override entities k in schema
	schema.Categories[1].K = 0.3

	kc := smc.NewKController(0.5, schema)
	assert.Equal(t, 0.5, kc.Global)
	assert.Equal(t, 0.3, kc.PerCategory["entities"])
	// Categories with K=-1 should not appear in PerCategory
	_, hasIntent := kc.PerCategory["intent"]
	assert.False(t, hasIntent)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -run TestKController -v`
Expected: FAIL — undefined: smc.KController

- [ ] **Step 3: Implement KController**

```go
// internal/smc/k.go
package smc

// KController manages the k-parameter for compression-fidelity tradeoff.
type KController struct {
	Global      float64            `yaml:"k"`
	PerCategory map[string]float64 `yaml:"-"` // built from schema
	AutoK       bool               `yaml:"auto_k"`
}

// NewKController creates a KController from a global k value and a schema.
// Per-category k values from the schema override the global when >= 0.
func NewKController(global float64, schema CategorySchema) KController {
	perCat := make(map[string]float64)
	for _, c := range schema.Categories {
		if c.K >= 0 {
			perCat[c.Name] = c.K
		}
	}
	return KController{
		Global:      global,
		PerCategory: perCat,
	}
}

// EffectiveK returns the k value for a category.
// Uses the per-category override if set and >= 0, otherwise the global value.
func (kc KController) EffectiveK(category string) float64 {
	if pk, ok := kc.PerCategory[category]; ok && pk >= 0 {
		return pk
	}
	return kc.Global
}

// CompressionRatio returns the target compression ratio for a category.
// Formula: compression_ratio = 1 - (1 - minRatio) * k
// At k=0: returns 1.0 (maximum compression).
// At k=1: returns minRatio (minimum compression, maximum fidelity).
func (kc KController) CompressionRatio(category string, minRatio float64) float64 {
	k := kc.EffectiveK(category)
	return 1 - (1-minRatio)*k
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -run TestKController -v`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/smc/k.go internal/smc/k_test.go
git commit -m "feat(smc): add k-parameter controller with per-category overrides"
```

---

### Task 3: PreferenceSignal Type

**Files:**
- Create: `internal/smc/preference.go`

- [ ] **Step 1: Create the PreferenceSignal type**

```go
// internal/smc/preference.go
package smc

// PreferenceSignal captures when a user corrects or overrides a prior response.
// Stored as metadata on the MatrixRow where the correction occurs.
type PreferenceSignal struct {
	Type     string `json:"type"`     // "correction", "rejection", "alternative_chosen"
	FromTurn int    `json:"from_turn"` // which prior turn this corrects (-1 if new)
	Category string `json:"category"` // which category was corrected (empty if general)
	Detail   string `json:"detail"`   // what changed
}
```

This is a data type with no logic — no test needed yet. Tests will come in Task 5 when the decomposer detects preferences.

- [ ] **Step 2: Commit**

```bash
git add internal/smc/preference.go
git commit -m "feat(smc): add PreferenceSignal type for correction tracking"
```

---

### Task 4: Conversation Matrix

**Files:**
- Create: `internal/smc/matrix.go`
- Create: `internal/smc/matrix_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/smc/matrix_test.go
package smc_test

import (
	"testing"
	"time"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationMatrix_Append(t *testing.T) {
	schema := smc.DefaultSchema()
	m := smc.NewConversationMatrix("test-session", schema, smc.KController{Global: 0.5})

	row := smc.MatrixRow{
		TurnIndex: 0,
		Categories: map[string]string{
			"intent":    "refactor authentication module",
			"entities":  "auth.go, middleware.go, JWT",
			"mutations": "replaced session-based auth with JWT tokens",
			"context":   "security audit required stateless auth",
		},
		RawTokens:  500,
		CompTokens: 80,
		Timestamp:  time.Now(),
	}

	m.Append(row)
	assert.Equal(t, 1, m.Len())
}

func TestConversationMatrix_Messages(t *testing.T) {
	schema := smc.DefaultSchema()
	m := smc.NewConversationMatrix("test-session", schema, smc.KController{Global: 0.5})

	m.Append(smc.MatrixRow{
		TurnIndex: 0,
		Categories: map[string]string{
			"intent":    "add logging",
			"entities":  "server.go",
			"mutations": "added slog calls",
			"context":   "debugging production issue",
		},
		RawTokens:  400,
		CompTokens: 60,
		Timestamp:  time.Now(),
	})

	msgs := m.Messages()
	require.Len(t, msgs, 1, "one compressed turn = one synthetic message")
	assert.Equal(t, "user", msgs[0].Role)
	assert.Contains(t, msgs[0].Content, "intent=add logging")
	assert.Contains(t, msgs[0].Content, "entities=server.go")
	assert.Contains(t, msgs[0].Content, "mutations=added slog calls")
	assert.Contains(t, msgs[0].Content, "context=debugging production issue")
}

func TestConversationMatrix_TokenCount(t *testing.T) {
	schema := smc.DefaultSchema()
	m := smc.NewConversationMatrix("test-session", schema, smc.KController{Global: 0.5})

	m.Append(smc.MatrixRow{
		TurnIndex:  0,
		Categories: map[string]string{"intent": "test", "entities": "file.go"},
		CompTokens: 30,
		Timestamp:  time.Now(),
	})
	m.Append(smc.MatrixRow{
		TurnIndex:  1,
		Categories: map[string]string{"intent": "fix", "entities": "bug.go"},
		CompTokens: 25,
		Timestamp:  time.Now(),
	})

	assert.Equal(t, 55, m.TokenCount())
}

func TestConversationMatrix_EmptyMessages(t *testing.T) {
	schema := smc.DefaultSchema()
	m := smc.NewConversationMatrix("test-session", schema, smc.KController{Global: 0.5})

	msgs := m.Messages()
	assert.Empty(t, msgs)
	assert.Equal(t, 0, m.TokenCount())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -run TestConversationMatrix -v`
Expected: FAIL — undefined: smc.NewConversationMatrix

- [ ] **Step 3: Implement ConversationMatrix**

```go
// internal/smc/matrix.go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -run TestConversationMatrix -v`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/smc/matrix.go internal/smc/matrix_test.go
git commit -m "feat(smc): add ConversationMatrix with row storage and message serialization"
```

---

### Task 5: Decomposer

**Files:**
- Create: `internal/smc/decompose.go`
- Create: `internal/smc/decompose_test.go`

The Phase 1 decomposer is **rule-based** (no LLM call). It extracts categories using heuristics from the message text. This keeps Phase 1 self-contained and testable without an LLM. An LLM-based decomposer can be added later behind the same interface.

- [ ] **Step 1: Write the failing tests**

```go
// internal/smc/decompose_test.go
package smc_test

import (
	"context"
	"testing"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleDecomposer_BasicExtraction(t *testing.T) {
	d := smc.NewRuleDecomposer(smc.KController{Global: 0.5})
	schema := smc.DefaultSchema()

	exchange := smc.Exchange{
		UserMessage:      "Please refactor the authentication module in auth.go to use JWT tokens instead of sessions",
		AssistantMessage: "I've updated auth.go and middleware.go to use JWT-based authentication. The session store has been removed.",
		TurnIndex:        0,
	}

	row, err := d.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)
	require.NotNil(t, row)

	assert.Equal(t, 0, row.TurnIndex)
	assert.NotEmpty(t, row.Categories["intent"])
	assert.NotEmpty(t, row.Categories["entities"])
	assert.NotEmpty(t, row.Categories["mutations"])
	assert.NotEmpty(t, row.Categories["context"])
	assert.Greater(t, row.RawTokens, 0)
	assert.Greater(t, row.CompTokens, 0)
	assert.True(t, row.CompTokens < row.RawTokens, "compressed should be smaller than raw")
}

func TestRuleDecomposer_CustomSchema(t *testing.T) {
	d := smc.NewRuleDecomposer(smc.KController{Global: 0.5})
	schema := smc.CategorySchema{
		Categories: []smc.Category{
			{Name: "action", Description: "what to do", K: -1},
			{Name: "target", Description: "what to act on", K: -1},
		},
	}

	exchange := smc.Exchange{
		UserMessage:      "Deploy the app to staging",
		AssistantMessage: "Deployed successfully to staging environment.",
		TurnIndex:        0,
	}

	row, err := d.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)

	// Custom schema: only "action" and "target" keys should exist
	assert.Len(t, row.Categories, 2)
	assert.Contains(t, row.Categories, "action")
	assert.Contains(t, row.Categories, "target")
	assert.NotContains(t, row.Categories, "intent")
}

func TestRuleDecomposer_EmptyExchange(t *testing.T) {
	d := smc.NewRuleDecomposer(smc.KController{Global: 0.5})
	schema := smc.DefaultSchema()

	exchange := smc.Exchange{
		UserMessage:      "",
		AssistantMessage: "",
		TurnIndex:        0,
	}

	row, err := d.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)
	// Should still produce a row, just with minimal content
	assert.Equal(t, 0, row.TurnIndex)
}

func TestRuleDecomposer_KAffectsCompression(t *testing.T) {
	schema := smc.DefaultSchema()

	exchange := smc.Exchange{
		UserMessage:      "Refactor the database connection pooling in db/pool.go. The current implementation leaks connections under high concurrency because the mutex is held during the entire query execution instead of just during pool checkout.",
		AssistantMessage: "I've refactored db/pool.go to use a channel-based pool instead of mutex-guarded slice. Connections are now checked out and returned via buffered channel, eliminating the lock contention.",
		TurnIndex:        0,
	}

	lowK := smc.NewRuleDecomposer(smc.KController{Global: 0.1})
	highK := smc.NewRuleDecomposer(smc.KController{Global: 0.9})

	rowLow, err := lowK.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)

	rowHigh, err := highK.Decompose(context.Background(), exchange, schema)
	require.NoError(t, err)

	// Lower k = more aggressive compression = fewer tokens
	assert.Less(t, rowLow.CompTokens, rowHigh.CompTokens,
		"low k (%d tokens) should produce fewer tokens than high k (%d tokens)",
		rowLow.CompTokens, rowHigh.CompTokens)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -run TestRuleDecomposer -v`
Expected: FAIL — undefined: smc.NewRuleDecomposer

- [ ] **Step 3: Implement the Decomposer interface and RuleDecomposer**

```go
// internal/smc/decompose.go
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
// For the default schema: intent comes from the user message summary,
// entities are nouns/identifiers, mutations from the assistant message,
// context from reasoning phrases. Custom schemas get the full text split
// proportionally across categories.
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
// For default categories, uses targeted extraction. For custom categories,
// uses the full text (the LLM decomposer will do better here).
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
		// Custom category: return combined text (LLM decomposer handles this properly)
		return summarize(exchange.UserMessage+" "+exchange.AssistantMessage, cat.Description)
	}
}

// compress reduces text length based on k value.
// k=0: aggressive (keep ~10% of text). k=1: preserve most text.
func (d *RuleDecomposer) compress(text string, k float64) string {
	if text == "" {
		return ""
	}
	// Target length: between 10% and 100% of original based on k
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

// extractIntent returns the user's apparent goal from their message.
func extractIntent(msg string) string {
	if msg == "" {
		return ""
	}
	// Take the first sentence as a proxy for intent
	for i, r := range msg {
		if r == '.' || r == '!' || r == '?' {
			return msg[:i+1]
		}
	}
	return msg
}

// extractEntities pulls identifiers: filenames, function-like tokens, quoted strings.
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
		// File-like: contains a dot with extension
		if strings.Contains(clean, ".") && !strings.HasPrefix(clean, "e.g") && !strings.HasPrefix(clean, "i.e") {
			entities = append(entities, clean)
			continue
		}
		// CamelCase or snake_case identifiers
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

// extractMutations summarizes what changed from the assistant's response.
func extractMutations(msg string) string {
	if msg == "" {
		return ""
	}
	// Look for action verbs that indicate changes
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

// extractContext pulls reasoning and rationale from the user message.
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

// summarize returns a truncated version of text for custom categories.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -run TestRuleDecomposer -v`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/smc/decompose.go internal/smc/decompose_test.go
git commit -m "feat(smc): add Decomposer interface and rule-based implementation"
```

---

### Task 6: SMC Config Integration

**Files:**
- Modify: `internal/config/config.go:157-160`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestDefaults_SMCConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "engram.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("{}"), 0644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	// SMC defaults
	assert.Equal(t, 0.5, cfg.Proxy.SMC.K, "default global k")
	assert.False(t, cfg.Proxy.SMC.AutoK, "auto_k off by default")
	assert.Len(t, cfg.Proxy.SMC.Categories, 4, "default 4 categories")
	assert.Equal(t, "intent", cfg.Proxy.SMC.Categories[0].Name)
	assert.True(t, cfg.Proxy.SMC.CrossRefs, "cross_refs on by default")
}

func TestLoad_SMCCustomCategories(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "engram.yaml")
	yaml := `
proxy:
  smc:
    k: 0.3
    categories:
      - name: diagnosis
        description: "medical diagnosis"
        k: 0.2
      - name: treatment
        description: "treatment plan"
        k: 0.7
`
	require.NoError(t, os.WriteFile(configPath, []byte(yaml), 0644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, 0.3, cfg.Proxy.SMC.K)
	assert.Len(t, cfg.Proxy.SMC.Categories, 2)
	assert.Equal(t, "diagnosis", cfg.Proxy.SMC.Categories[0].Name)
	assert.Equal(t, 0.2, cfg.Proxy.SMC.Categories[0].K)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/config/ -run TestDefaults_SMC -v`
Expected: FAIL — cfg.Proxy.SMC undefined

- [ ] **Step 3: Add SMCConfig to ProxyConfig**

In `internal/config/config.go`, change the ProxyConfig struct and defaults:

Replace lines 157-160:
```go
type ProxyConfig struct {
	Port       int `yaml:"port"`
	WindowSize int `yaml:"window_size"`
}
```

With:
```go
// SMCCategoryConfig mirrors smc.Category for YAML deserialization.
type SMCCategoryConfig struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	K           float64 `yaml:"k"`
}

// SMCConfig holds Structured Matrix Compression settings.
type SMCConfig struct {
	K          float64             `yaml:"k"`
	AutoK      bool                `yaml:"auto_k"`
	CrossRefs  bool                `yaml:"cross_refs"`
	Categories []SMCCategoryConfig `yaml:"categories"`
}

type ProxyConfig struct {
	Port       int       `yaml:"port"`
	WindowSize int       `yaml:"window_size"`
	SMC        SMCConfig `yaml:"smc"`
}
```

In the `defaults()` function, update the Proxy section (around line 329-332):

Replace:
```go
		Proxy: ProxyConfig{
			Port:       4242,
			WindowSize: 10,
		},
```

With:
```go
		Proxy: ProxyConfig{
			Port:       4242,
			WindowSize: 10,
			SMC: SMCConfig{
				K:         0.5,
				CrossRefs: true,
				Categories: []SMCCategoryConfig{
					{Name: "intent", Description: "What the speaker wants or is doing — semantic purpose, goals, directives", K: -1},
					{Name: "entities", Description: "Files, functions, concepts referenced — concrete nouns, identifiers, domain objects", K: -1},
					{Name: "mutations", Description: "What changed — code diffs, state transitions, decisions, commitments", K: -1},
					{Name: "context", Description: "Reasoning, constraints, rationale — the 'why' behind actions and decisions", K: -1},
				},
			},
		},
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/config/ -v`
Expected: PASS (including the existing TestDefaults test — update its WindowSize assertion if needed)

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add SMC configuration with default category schema"
```

---

### Task 7: Proxy Handler Integration

**Files:**
- Modify: `internal/proxy/handler.go`
- Modify: `internal/proxy/handler_test.go`

This is the core integration: replace `Compress()` with the SMC matrix.

- [ ] **Step 1: Write the failing test**

Add a new test to `internal/proxy/handler_test.go`:

```go
func TestHandler_SMCCompression(t *testing.T) {
	// Set up a mock upstream that echoes back the messages it receives
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		json.Unmarshal(body, &req)

		// Verify that when there are enough messages, SMC matrix format is used
		// (contains "intent=" pattern instead of "[CONTEXT_SUMMARY]")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{"text": "ok", "type": "text"}},
			"role":    "assistant",
		})
	}))
	defer upstream.Close()

	schema := smc.DefaultSchema()
	kc := smc.NewKController(0.5, schema)
	h := NewHandler(10, t.TempDir(), upstream.URL)
	h.EnableSMC(schema, kc)

	// Build a request with enough messages to trigger compression
	messages := make([]AnthropicMessage, 20)
	for i := range messages {
		if i%2 == 0 {
			messages[i] = AnthropicMessage{Role: "user", Content: fmt.Sprintf("Please update file%d.go to add logging", i)}
		} else {
			messages[i] = AnthropicMessage{Role: "assistant", Content: fmt.Sprintf("Updated file%d.go with slog calls", i)}
		}
	}

	body, _ := json.Marshal(map[string]any{
		"messages": messages,
		"system":   "You are a helpful assistant.",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "test-key")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, 200, rr.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/proxy/ -run TestHandler_SMC -v`
Expected: FAIL — h.EnableSMC undefined

- [ ] **Step 3: Add SMC fields and EnableSMC to Handler**

In `internal/proxy/handler.go`, add fields to the Handler struct (after `pendingSession string` on line 40):

```go
	// SMC fields — when smcEnabled is true, use matrix decomposition instead of
	// windowed compression.
	smcEnabled   bool
	smcSchema    smc.CategorySchema
	smcK         smc.KController
	smcDecomposer smc.Decomposer
	smcMatrices  map[string]*smc.ConversationMatrix // sessionID -> matrix
	smcMu        sync.Mutex
```

Add the import for `"github.com/pythondatascrape/engram/internal/smc"`.

Add the EnableSMC method:

```go
// EnableSMC activates structured matrix compression, replacing windowed compression.
func (h *Handler) EnableSMC(schema smc.CategorySchema, k smc.KController) {
	h.smcEnabled = true
	h.smcSchema = schema
	h.smcK = k
	h.smcDecomposer = smc.NewRuleDecomposer(k)
	h.smcMatrices = make(map[string]*smc.ConversationMatrix)
}

// getOrCreateMatrix returns the conversation matrix for a session, creating one if needed.
func (h *Handler) getOrCreateMatrix(sessionID string) *smc.ConversationMatrix {
	h.smcMu.Lock()
	defer h.smcMu.Unlock()
	m, ok := h.smcMatrices[sessionID]
	if !ok {
		m = smc.NewConversationMatrix(sessionID, h.smcSchema, h.smcK)
		h.smcMatrices[sessionID] = m
	}
	return m
}
```

- [ ] **Step 4: Replace the Compress call with SMC path**

In `handler.go` ServeHTTP, replace lines 187-188:

```go
	req.Messages = Compress(req.Messages, h.windowSize)
	ctxComp := EstimateTokens(req.Messages) + systemTokens
```

With:

```go
	if h.smcEnabled {
		req.Messages = h.smcCompress(sessionID, req.Messages)
	} else {
		req.Messages = Compress(req.Messages, h.windowSize)
	}
	ctxComp := EstimateTokens(req.Messages) + systemTokens
```

Add the smcCompress method:

```go
// smcCompress decomposes older messages into the matrix and returns
// the matrix history + recent raw messages.
func (h *Handler) smcCompress(sessionID string, messages []AnthropicMessage) []AnthropicMessage {
	if len(messages) <= 2 {
		return messages
	}

	matrix := h.getOrCreateMatrix(sessionID)

	// Decompose all completed exchanges (pairs) except the last pair.
	// The last user message is the current turn — keep at full fidelity.
	alreadyDecomposed := matrix.Len()
	pairs := len(messages) / 2 // complete user+assistant pairs
	currentTail := messages[pairs*2:] // any trailing unpaired message

	for i := alreadyDecomposed; i < pairs; i++ {
		userIdx := i * 2
		assistIdx := i*2 + 1
		if assistIdx >= len(messages) {
			break
		}
		exchange := smc.Exchange{
			UserMessage:      messageText(messages[userIdx]),
			AssistantMessage: messageText(messages[assistIdx]),
			TurnIndex:        i,
		}
		row, err := h.smcDecomposer.Decompose(context.Background(), exchange, h.smcSchema)
		if err != nil {
			slog.Warn("smc: decomposition failed, keeping raw", "turn", i, "err", err)
			continue
		}
		matrix.Append(*row)
	}

	// Build output: matrix history messages + current raw tail
	matrixMsgs := matrix.Messages()
	result := make([]AnthropicMessage, 0, len(matrixMsgs)+len(currentTail)+2)

	// Convert provider.Message to AnthropicMessage
	for _, m := range matrixMsgs {
		result = append(result, AnthropicMessage{Role: m.Role, Content: m.Content})
	}
	// Add dummy assistant message after matrix to maintain user/assistant alternation
	if len(result) > 0 {
		result = append(result, AnthropicMessage{Role: "assistant", Content: "[compressed history above]"})
	}

	// Append the current raw tail (last pair or unpaired message)
	if pairs > 0 && alreadyDecomposed < pairs {
		// The last complete pair is kept raw for recency
		lastPairStart := (pairs - 1) * 2
		result = append(result, messages[lastPairStart:]...)
	} else {
		result = append(result, currentTail...)
	}

	return result
}
```

Add `"context"` to the import block.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/proxy/ -v`
Expected: PASS (both old and new tests)

- [ ] **Step 6: Commit**

```bash
git add internal/proxy/handler.go internal/proxy/handler_test.go
git commit -m "feat(proxy): integrate SMC matrix compression into handler"
```

---

### Task 8: Wire SMC Config Through Serve Command

**Files:**
- Modify: `cmd/engram/serve.go`

- [ ] **Step 1: Write the integration test**

Add to `cmd/engram/serve_test.go` (or create if needed):

```go
func TestServeCmd_SMCFlagParsing(t *testing.T) {
	cmd := newServeCmd()
	cmd.SetArgs([]string{"--foreground", "--k", "0.3"})
	// Just verify flags parse without error
	err := cmd.ParseFlags([]string{"--k", "0.3"})
	require.NoError(t, err)
	k, _ := cmd.Flags().GetFloat64("k")
	assert.Equal(t, 0.3, k)
}
```

- [ ] **Step 2: Add --k flag to serve command**

In `cmd/engram/serve.go`, in the `newServeCmd()` function where flags are defined, add:

```go
	cmd.Flags().Float64("k", -1, "SMC k-parameter (0-1); overrides config file smc.k")
```

- [ ] **Step 3: Wire SMC config to proxy Handler**

In `serve.go`, after the proxy is created (around line 170), add the SMC enablement:

```go
	// Enable SMC on the proxy handler if configured
	smcCfg := cfg.Proxy.SMC
	kOverride, _ := cmd.Flags().GetFloat64("k")
	if kOverride >= 0 {
		smcCfg.K = kOverride
	}
	smcSchema := configToSMCSchema(smcCfg)
	smcK := smc.NewKController(smcCfg.K, smcSchema)
	// Access the handler from the proxy to enable SMC
	proxySrv.EnableSMC(smcSchema, smcK)
```

Add the helper function:

```go
func configToSMCSchema(cfg config.SMCConfig) smc.CategorySchema {
	cats := make([]smc.Category, len(cfg.Categories))
	for i, c := range cfg.Categories {
		cats[i] = smc.Category{
			Name:        c.Name,
			Description: c.Description,
			K:           c.K,
		}
	}
	return smc.CategorySchema{
		Categories: cats,
		CrossRefs:  cfg.CrossRefs,
	}
}
```

- [ ] **Step 4: Add EnableSMC method to proxy.Server**

In `internal/proxy/proxy.go`, add:

```go
// EnableSMC activates structured matrix compression on the underlying handler.
func (s *Server) EnableSMC(schema smc.CategorySchema, k smc.KController) {
	if h, ok := s.srv.Handler.(*Handler); ok {
		h.EnableSMC(schema, k)
	}
}
```

Add the import for `"github.com/pythondatascrape/engram/internal/smc"`.

- [ ] **Step 5: Run all tests**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./... 2>&1 | tail -20`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/engram/serve.go internal/proxy/proxy.go
git commit -m "feat(serve): wire SMC config and --k flag through to proxy"
```

---

### Task 9: End-to-End Integration Test

**Files:**
- Create: `internal/smc/integration_test.go`

- [ ] **Step 1: Write the end-to-end test**

```go
// internal/smc/integration_test.go
package smc_test

import (
	"context"
	"testing"

	"github.com/pythondatascrape/engram/internal/smc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEnd_MultiTurnConversation(t *testing.T) {
	schema := smc.DefaultSchema()
	kc := smc.NewKController(0.5, schema)
	decomposer := smc.NewRuleDecomposer(kc)
	matrix := smc.NewConversationMatrix("e2e-test", schema, kc)

	exchanges := []smc.Exchange{
		{
			UserMessage:      "Refactor auth.go to use JWT tokens instead of session cookies. The current session store leaks memory under load.",
			AssistantMessage: "I've replaced the session-based authentication in auth.go with JWT tokens. Removed the in-memory session store and added jwt.go with token generation and validation.",
			TurnIndex:        0,
		},
		{
			UserMessage:      "Now update the middleware in middleware.go to validate JWT tokens on each request.",
			AssistantMessage: "Updated middleware.go to extract and validate JWT from the Authorization header. Added token expiry checking and refresh logic.",
			TurnIndex:        1,
		},
		{
			UserMessage:      "Add tests for the JWT validation. Cover expired tokens, invalid signatures, and missing headers.",
			AssistantMessage: "Created jwt_test.go with table-driven tests covering: valid token, expired token, invalid signature, missing Authorization header, and malformed token format.",
			TurnIndex:        2,
		},
	}

	for _, ex := range exchanges {
		row, err := decomposer.Decompose(context.Background(), ex, schema)
		require.NoError(t, err)
		matrix.Append(*row)
	}

	// Verify matrix state
	assert.Equal(t, 3, matrix.Len())

	// Verify messages are well-formed
	msgs := matrix.Messages()
	require.Len(t, msgs, 3)
	for i, msg := range msgs {
		assert.Equal(t, "user", msg.Role, "turn %d should be user role", i)
		assert.Contains(t, msg.Content, "intent=", "turn %d should have intent", i)
		assert.Contains(t, msg.Content, "entities=", "turn %d should have entities", i)
	}

	// Verify compression happened
	totalCompressed := matrix.TokenCount()
	totalRaw := 0
	for _, r := range matrix.Rows() {
		totalRaw += r.RawTokens
	}
	assert.Less(t, totalCompressed, totalRaw,
		"compressed (%d) should be less than raw (%d)", totalCompressed, totalRaw)

	t.Logf("Compression: %d raw tokens -> %d compressed tokens (%.0f%% reduction)",
		totalRaw, totalCompressed, 100*(1-float64(totalCompressed)/float64(totalRaw)))
}

func TestEndToEnd_CustomSchema(t *testing.T) {
	schema := smc.CategorySchema{
		Categories: []smc.Category{
			{Name: "symptom", Description: "patient symptoms", K: 0.3},
			{Name: "diagnosis", Description: "medical diagnosis", K: 0.5},
			{Name: "treatment", Description: "recommended treatment", K: 0.7},
		},
	}
	kc := smc.NewKController(0.5, schema)
	decomposer := smc.NewRuleDecomposer(kc)
	matrix := smc.NewConversationMatrix("custom-test", schema, kc)

	row, err := decomposer.Decompose(context.Background(), smc.Exchange{
		UserMessage:      "Patient presents with fever and cough for 3 days",
		AssistantMessage: "Based on symptoms, likely upper respiratory infection. Recommend rest and fluids.",
		TurnIndex:        0,
	}, schema)
	require.NoError(t, err)
	matrix.Append(*row)

	msgs := matrix.Messages()
	require.Len(t, msgs, 1)
	// Should use custom category names, not defaults
	assert.Contains(t, msgs[0].Content, "symptom=")
	assert.Contains(t, msgs[0].Content, "diagnosis=")
	assert.Contains(t, msgs[0].Content, "treatment=")
	assert.NotContains(t, msgs[0].Content, "intent=")
}
```

- [ ] **Step 2: Run the integration tests**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/smc/ -run TestEndToEnd -v`
Expected: PASS

- [ ] **Step 3: Run the full test suite**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./...`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/smc/integration_test.go
git commit -m "test(smc): add end-to-end integration tests for multi-turn and custom schemas"
```

---

### Task 10: Final Verification and Cleanup

- [ ] **Step 1: Run all tests with race detector**

Run: `cd /Users/emeyer/Desktop/Engram && go test -race ./...`
Expected: All PASS, no races detected

- [ ] **Step 2: Run go vet**

Run: `cd /Users/emeyer/Desktop/Engram && go vet ./...`
Expected: No issues

- [ ] **Step 3: Verify the binary builds**

Run: `cd /Users/emeyer/Desktop/Engram && go build ./cmd/engram/`
Expected: Builds successfully

- [ ] **Step 4: Manual smoke test**

Run: `cd /Users/emeyer/Desktop/Engram && go run ./cmd/engram/ serve --foreground --k 0.5 &`
Then: `curl -s http://localhost:4242/health` (or whatever health endpoint exists)
Then: Kill the background process

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat(smc): complete Phase 1 — structured matrix compression core engine

Replaces windowed compression with per-turn structured matrix
decomposition. Configurable category schema (defaults: intent,
entities, mutations, context) with k-parameter control.

New package: internal/smc/
- category.go: CategorySchema with defaults and validation
- k.go: KController with per-category overrides
- matrix.go: ConversationMatrix with message serialization
- decompose.go: Decomposer interface + rule-based implementation
- preference.go: PreferenceSignal type for correction tracking

Integration: proxy handler uses SMC when enabled, falling back
to windowed compression for backward compatibility."
```
