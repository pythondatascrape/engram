# Structured Matrix Compression (SMC) Architecture Design

**Date:** 2026-04-08
**Status:** Design
**Source Documents:**
- McKinsey Digital & AI Practice: Structured Matrix Compression Architecture (April 2026)
- IBM Consulting: Technical Design Review of SMC Architecture (April 2026)

---

## Overview

Evolve Engram's current windowed compression (`compressor.go`) into the Structured Matrix Compression architecture described in the McKinsey and IBM documents. Each completed conversation turn is decomposed into configurable category vectors, controlled by a single `k` parameter, producing a conversation matrix that simultaneously enables compression, statistical learning, fine-tuning data generation, edge deployment, and federated inference.

**Approach:** Evolutionary — replace the existing compressor incrementally across four phases. The proxy stays shippable at every stage.

---

## Phase 1: Core SMC Engine

### 1.1 Configurable Category Schema

Ship with default categories (intent, entities, mutations, context) that users can override, extend, or replace via configuration.

```go
// internal/smc/category.go

type Category struct {
    Name        string  // e.g. "intent", "entities"
    Description string  // LLM prompt guidance for extraction
    K           float64 // per-category compression level (0-1), -1 means use global
}

type CategorySchema struct {
    Categories []Category
    CrossRefs  bool // enable cross-reference fields between categories (IBM rec)
}
```

Default config:

```yaml
smc:
  k: 0.5
  auto_k: false
  cross_refs: true
  categories:
    - name: intent
      description: "What the speaker wants or is doing — semantic purpose, goals, directives"
      k: -1  # use global
    - name: entities
      description: "Files, functions, concepts referenced — concrete nouns, identifiers, domain objects"
      k: 0.3  # tighter — no hallucinated names
    - name: mutations
      description: "What changed — code diffs, state transitions, decisions, commitments"
      k: -1
    - name: context
      description: "Reasoning, constraints, rationale — the 'why' behind actions and decisions"
      k: -1
```

Users override by providing their own `smc.categories` block. The entire schema is replaceable — no hardcoded categories in the engine.

### 1.2 Decomposer

Replaces `compressHead()` in `compressor.go`. Called once per completed exchange (O(1) per turn).

```go
// internal/smc/decompose.go

type Exchange struct {
    UserMessage      string
    AssistantMessage string
    TurnIndex        int
}

type Decomposer interface {
    Decompose(ctx context.Context, exchange Exchange, schema CategorySchema) (*MatrixRow, error)
}

// LLMDecomposer uses the session's LLM provider to extract category vectors.
type LLMDecomposer struct {
    Provider provider.Provider
    K        *KController
}
```

The decomposer constructs a structured prompt asking the LLM to extract each category from the completed exchange. The prompt includes the category descriptions from the schema so the LLM knows what to extract. Output is codebook-normalized.

### 1.3 Matrix Row

```go
// internal/smc/matrix.go

type MatrixRow struct {
    TurnIndex   int
    Categories  map[string]string    // category name -> compressed content
    CrossRefs   map[string][]string  // category -> related category names (optional)
    Preference  *PreferenceSignal    // nil if no correction detected
    RawTokens   int                  // token count of original exchange (for stats)
    CompTokens  int                  // token count of compressed row
    Timestamp   time.Time
}

type PreferenceSignal struct {
    Type       string // "correction", "rejection", "alternative_chosen"
    FromTurn   int    // which prior turn this corrects
    Category   string // which category was corrected
    Detail     string // what changed
}
```

### 1.4 k-Parameter System

```go
// internal/smc/k.go

type KController struct {
    Global      float64             // default k in [0, 1]
    PerCategory map[string]float64  // overrides per category
    AutoK       bool                // Phase 2: auto-adjust based on quality metrics
}

// CompressionRatio returns the target ratio for a category.
// Formula: compression_ratio = 1 - (1 - min_ratio) * k
func (kc *KController) CompressionRatio(category string, minRatio float64) float64 {
    k := kc.Global
    if pk, ok := kc.PerCategory[category]; ok && pk >= 0 {
        k = pk
    }
    return 1 - (1-minRatio)*k
}

// EffectiveK returns the k value for a category (per-category override or global).
func (kc *KController) EffectiveK(category string) float64 {
    if pk, ok := kc.PerCategory[category]; ok && pk >= 0 {
        return pk
    }
    return kc.Global
}
```

### 1.5 Conversation Matrix

Replaces `History` in `internal/context/history.go`.

```go
// internal/smc/matrix.go

type ConversationMatrix struct {
    SessionID   string
    Codebook    *context.ContextCodebook
    Schema      CategorySchema
    K           *KController
    Rows        []MatrixRow
    Stats       *DistributionStats // per-category running stats
}

// Append adds a decomposed turn to the matrix.
func (m *ConversationMatrix) Append(row MatrixRow)

// Messages serializes the matrix for LLM context injection.
// Format:
//   [codebook]
//   [turn1: intent=... | entities=... | mutations=... | context=...]
//   [turn2: ...]
//   ...
func (m *ConversationMatrix) Messages() []provider.Message

// TokenCount returns the total compressed token count.
func (m *ConversationMatrix) TokenCount() int
```

### 1.6 Proxy Integration

In `internal/proxy/handler.go`, replace the `Compress()` call with:

1. After assistant response streams back, call `decomposer.Decompose()` on the completed exchange
2. Append the resulting `MatrixRow` to the session's `ConversationMatrix`
3. On next request, `matrix.Messages()` provides the compressed history

The current turn (user message being sent + assistant response being generated) remains at full fidelity — only completed turns get decomposed.

### 1.7 What Gets Removed/Replaced

| Current | Replacement |
|---------|-------------|
| `proxy.Compress()` | `smc.Decomposer.Decompose()` + `ConversationMatrix` |
| `proxy.compressHead()` | Eliminated — decomposition replaces truncation |
| `context.History` | `smc.ConversationMatrix` |
| `context.CompressedTurn` | `smc.MatrixRow` |
| `--window` flag | `--k` flag (k parameter) + `--window` kept for backward compat |

### 1.8 Validation Criteria (IBM Phase 1 Recommendations)

- Round-trip reconstruction score: can key information be recovered from the four categories?
- Benchmark decomposition quality across at least 100 conversation turns
- Compare token savings vs. current windowed compression
- Map the k-quality curve: identify the "knee" where compression causes quality loss
- Test with configurable schemas (swap default categories for a non-SE domain)

---

## Phase 2: Persistence & Learning

### 2.1 Persistent Matrix Store

SQLite database for cross-session matrix accumulation.

```go
// internal/smc/store.go

type MatrixStore interface {
    Save(sessionID string, row MatrixRow) error
    LoadSession(sessionID string) ([]MatrixRow, error)
    LoadAll() ([][]MatrixRow, error)
    QueryByCategory(category, query string) ([]MatrixRow, error)
    Distribution(category string) (*CategoryDistribution, error)
}
```

Storage: `~/.engram/matrices.db` (configurable).

Tables:
- `sessions` — session metadata (ID, start time, schema version)
- `matrix_rows` — turn decompositions (session_id, turn_index, category, content, k_value, timestamp)
- `distributions` — cached per-category statistics, updated on each append
- `preferences` — correction/preference signals with turn references

### 2.2 Distribution Tracking

```go
// internal/smc/distribution.go

type DistributionStats struct {
    PerCategory map[string]*CategoryDistribution
}

type CategoryDistribution struct {
    Count      int
    TopTerms   map[string]int     // term frequency
    Mean       float64            // average content length (proxy for density)
    StdDev     float64            // variability measure
    Outliers   []int              // turn indices that deviate > 2 sigma
}
```

Convergence milestones (from McKinsey):
- 1-5 conversations: baseline distributions form
- 10-20: user patterns stabilize, outlier detection becomes reliable
- 50+: tight distributions, deep understanding of user's domain/style

Self-correction: new data points shift the distribution center naturally. No stale entry cleanup needed.

### 2.3 Auto-k (IBM Recommendation)

When `auto_k: true` and sufficient distribution data exists, dynamically adjust k based on:
- User correction rate (high corrections → increase k for more fidelity)
- Distribution tightness (tight distribution → safe to decrease k)
- Task completion signals

### 2.4 Fine-Tuning Data Export

```go
// internal/smc/export.go

type Exporter interface {
    ExportJSONL(w io.Writer, opts ExportOptions) error
    ExportTensor(w io.Writer, opts ExportOptions) error
}

type ExportOptions struct {
    K              float64   // compression level for export
    IncludePrefs   bool      // include preference/correction signals
    MinTurns       int       // minimum turns per conversation
    CategoryFilter []string  // subset of categories to include
    CategoryOut    string    // leave-one-category-out validation target
}
```

Category-out validation (IBM's "strongest technical contribution"):
- Train on 3 categories, predict the 4th
- Rotates through all categories
- Tests structural understanding, not memorization
- 100% data utilization (no train/test split waste)

### 2.5 Parallel Multi-Matrix Training Support

Export supports stacking multiple conversation matrices into training batches:
- Fixed schema makes matrices trivially stackable
- Mixed-k export: vary k across matrices in same batch for k-invariant model training
- Curriculum-k: export with increasing k over epochs

IBM caveats to address:
- Measure inter-category mutual information (intent/mutations may be too correlated for independent holdout)
- Ensure batch diversity: matrices from different users/domains per batch
- Monitor gradient norm variance with mixed-k training

---

## Phase 3: Edge, Security & Mobile

### 3.1 Async Decomposition

Return LLM response immediately. Decompose in background goroutine:

```go
// In proxy handler, after response streams back:
go func() {
    row, err := decomposer.Decompose(bgCtx, exchange, schema)
    if err == nil {
        matrix.Append(row)
        store.Save(sessionID, row)
    }
}()
```

This eliminates the per-turn decomposition latency that IBM flagged as the mobile bottleneck (2-5s on a 3B model).

### 3.2 Device-Adaptive k

```go
type AdaptiveK struct {
    BaseK       float64
    DeviceClass string // "phone", "laptop", "desktop", "cloud"
}
```

| Device Class | Default k | Context Budget (20 turns) |
|---|---|---|
| phone (4GB RAM) | 0.2 | ~1K tokens |
| laptop (16GB RAM) | 0.5 | ~4K tokens |
| desktop/cloud | 0.8 | ~8K+ tokens |

Thermal-aware adjustment: if device is throttling, increase compression aggressiveness.

### 3.3 Security Hardening

**Codebook rotation:**
- Versioned codebooks (v1, v2, ...)
- Configurable rotation interval
- Stored matrices tagged with codebook version
- Lazy re-encoding on read (or batch migration)

**Framing (per IBM):** Codebook provides semantic obfuscation, not encryption. TLS remains the transport security boundary. The codebook adds a layer where intercepted payloads are meaningless without the codebook — a structural byproduct of compression, not an added feature.

**VPN-like behavior:**
- All data through the proxy is compressed + codebook-encoded
- On-device storage in encoded form
- Cloud-bound API calls carry encoded tokens, not natural language

### 3.4 Decomposition Quality Benchmarking

IBM caveat 2: benchmark decomposition quality across model sizes to find the minimum viable model for edge deployment. Consider a specialized fine-tuned decomposition model trained on Engram's own accumulated data (virtuous cycle).

---

## Phase 4: Federated Deployment

### 4.1 Federation Architecture (Primary Strategy)

Per IBM recommendation: federated deployment with local inference per node, shared compressed matrices.

```
Site A (Engram Proxy)  <-- Matrix Sync -->  Site B (Engram Proxy)
       |                                           |
   Local LLM                                   Local LLM
```

```go
// internal/smc/federation.go

type FederationNode struct {
    ID        string
    Endpoint  string
    Codebook  *context.ContextCodebook // shared for matrix sync
}

type Federation interface {
    SyncMatrix(ctx context.Context, sessionID string, rows []MatrixRow) error
    PullDistributions(ctx context.Context, nodeID string) (*DistributionStats, error)
    ListNodes(ctx context.Context) ([]FederationNode, error)
}
```

Deployment scenarios:
- Home lab: 2-4 Raspberry Pis, Engram proxy as coordinator (k 0.2-0.4)
- Small office: edge devices per desk, federated proxy (k 0.4-0.6)
- Enterprise: multi-site edge + cloud burst, proxy federation (k 0.6-1.0)

### 4.2 Category-Parallel Inference (Research Track)

Kept as experimental. The fixed-schema enables it architecturally:

```
Node 1: intent + entities   -> partial inference -> |
Node 2: mutations + context -> partial inference -> |--> Aggregator -> Response
Node 3: cross-category attn -> relationship inf  -> |
```

**IBM's warning:** Cross-category attention is essential for coherent output. Category-parallel inference risks incoherence. Design the interfaces but don't invest in production-hardening until empirical results justify it.

Alternative distributed strategies to evaluate first:
- Pipeline parallelism (distribute transformer layers)
- Data parallelism (different conversations on different nodes)
- Hybrid: category decomposition for storage/routing, standard model parallelism for inference

---

## Risk Register

| Risk | Severity | Mitigation | Phase |
|------|----------|------------|-------|
| Decomposition quality varies by LLM | Medium | Round-trip reconstruction score; benchmark across models | 1 |
| k sensitivity is non-linear | Medium | Empirically map k-quality curve; find the "knee" | 1 |
| Default categories insufficient for non-SE domains | Low | Fully configurable schema; ship defaults as starting point | 1 |
| Cross-session storage = privacy liability | High | On-device-first; codebook encoding; user-owned keys | 2-3 |
| Compression introduces information loss | Low | k parameter for user-controlled fidelity; raw retention optional | 1 |
| Mobile decomposition latency (2-5s) | Medium | Async decomposition; thermal-aware k | 3 |
| Distributed inference incoherence | Medium | Federated (not category-parallel) as primary; research track for parallel | 4 |
| Mixed-k training instability | Medium | Narrow k ranges per batch; monitor gradient norm variance | 2 |

---

## Phase Dependencies

```
Phase 1 (Core SMC)
    |
    v
Phase 2 (Persistence + Learning)  <-- Phase 2b (Training Validation, concurrent)
    |
    v
Phase 3 (Edge + Security)
    |
    v
Phase 4 (Federation)
```

Phase 1 is the foundation. Everything downstream depends on the decomposer, matrix, and k-parameter being solid. Phase 2b (parallel training validation) can run concurrently with Phase 2 persistence work.
