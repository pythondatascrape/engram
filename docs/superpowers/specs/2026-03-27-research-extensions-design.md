# Research Extensions Design Spec

Five research paper sections implemented as plugins, tools, and shared primitives for the Engram identity compression platform.

## Shared Primitives

### `internal/identity/diff` — Identity Diffing

- `Diff(old, new map[string]string) DeltaSet` — computes added/changed/removed fields
- `Apply(base map[string]string, delta DeltaSet) map[string]string` — applies delta to produce new identity
- `TokenDelta(cb *codebook.Codebook, delta DeltaSet) int` — estimates token impact of changes
- Used by: dynamic updates, cross-session continuity, benchmarking

### `internal/identity/introspect` — Schema Introspection

- `Constraints(cb *codebook.Codebook, identity map[string]string) []PromptConstraint` — derives behavioral constraints from identity values
- `MutableFields(cb *codebook.Codebook) []string` — returns fields the codebook marks as mutable
- Constraint mappings defined in codebook YAML via new optional `constraints` field per dimension
- Used by: response optimization, dynamic updates

## Feature 1: Dynamic Identity Updates

**Type:** `hook` plugin at `internal/plugin/builtin/deltaidentity/`

Client sends a `DeltaUpdate` message (map of only changed fields) instead of full identity. The hook:
1. Validates changed fields against codebook (including mutability check via `introspect.MutableFields`)
2. Runs injection detection on new values
3. Calls `diff.Apply` to merge with session's cached identity
4. Re-serializes merged identity, updates session's `SerializedIdentity`
5. Tracks `DeltaUpdates` count and `TokensSavedByDelta` in session metrics

### Codebook Schema Extension

```yaml
dimensions:
  - name: expertise_level
    type: enum
    values: [beginner, intermediate, advanced]
    mutable: true    # can be changed mid-session
  - name: user_id
    type: enum
    mutable: false   # immutable after session creation (default)
```

### Boundaries
- No versioning/rollback of identity states
- No partial serialization — always re-serializes full merged identity

## Feature 2: Identity-Aware Response Optimization

**Type:** `hook` plugin at `internal/plugin/builtin/responseopt/`

Pre-request hook that runs after identity serialization but before provider send. Calls `introspect.Constraints()` to derive prompt constraints, appends them to the system prompt.

### Constraint Definitions in Codebook

```yaml
dimensions:
  - name: expertise_level
    type: enum
    values: [beginner, intermediate, advanced]
    constraints:
      beginner: "Explain concepts simply, avoid jargon"
      advanced: "Be concise and technical, assume domain knowledge"
  - name: language
    type: enum
    values: [en, es, fr, de]
    constraints:
      _all: "Respond in {value}"
```

### Constraint Rendering
- Serialized in compact `key=value` style consistent with identity format
- `_all` wildcard applies to any value with `{value}` interpolation
- Dimensions without `constraints` field are ignored
- Total constraint block capped at configurable token budget (default 50 tokens), priority-ordered and truncated

### Boundaries
- No response post-processing or scoring
- No constraint conflict resolution — codebook author responsible for coherence
- Modifies system prompt only, not the query

## Feature 3: Cross-Session Identity Persistence

**Type:** New plugin type `persistence` at `internal/plugin/builtin/identitystore/`

### Plugin Type Addition
- Add `TypePersistence` to `internal/plugin/types.go`
- Interface: `PersistencePlugin` with `Store(clientID string, identity map[string]string) error`, `Load(clientID string) (map[string]string, error)`, `Delete(clientID string) error`

### Built-in Implementation: SQLite Store
- Pure Go SQLite via `modernc.org/sqlite` (no CGO)
- Schema: `client_id TEXT PRIMARY KEY, identity BLOB, codebook_version TEXT, updated_at TIMESTAMP`
- Identity stored as gob-encoded `map[string]string`
- TTL-based expiry (configurable, default 30 days) with background goroutine + context cancellation
- Encrypted at rest using AES-256-GCM (consistent with existing API key encryption)

### Session Manager Integration
- On `session.Create()`: if persistence plugin registered and client opts in (`persist_identity: true`), attempt `Load(clientID)` to pre-populate session identity
- On `session.SetIdentity()`: call `Store(clientID, identity)` asynchronously
- On delta updates: store merged identity after apply
- Client-sent identity always wins over stored version

### Boundaries
- Persists identity maps only, not session state (conversations, turns, metrics)
- Single-node SQLite only — distributed stores (Redis, Postgres) are future plugin implementations
- Does not override client-sent identity on reconnect

## Feature 4: Benchmarking Harness

**Location:** `cmd/engram-bench/` — standalone CLI tool, not a plugin

### Measurements Per Model
- Token count: raw identity vs. compressed (using each provider's tokenizer)
- Format compatibility: does the model correctly interpret `key=value` format without interpreter prompt?
- Compression ratio consistency across tokenizers
- Serialization latency

### Provider Matrix
- Anthropic (Claude Sonnet, Haiku), OpenAI (GPT-4o, GPT-4o-mini), Llama 3 (via Ollama), Mistral (via API or Ollama)
- Each provider implements `BenchProvider` interface: `Tokenize(text string) (int, error)`, `Send(prompt string) (string, error)`

### Test Protocol
- Fixed set of 5 identity profiles at varying complexity (minimal 3-field, typical 8-field, maximal 20-field)
- Each profile: raw prose → compressed format → send to model with comprehension probe
- Comprehension scored pass/fail — model must correctly extract identity values
- Results output as JSON + markdown table for paper inclusion

### CLI Usage
```bash
engram-bench --providers anthropic,openai,ollama \
             --profiles profiles/*.yaml \
             --output results/
```

### Boundaries
- Offline evaluation only, no live instrumentation
- No cost tracking — just raw token counts
- No statistical significance testing — deterministic comprehension probes

## Feature 5: Adversarial Robustness

Two components: test harness for the paper, runtime hook for production.

### Test Harness: `cmd/engram-adversarial/`

Attack categories:
1. **Injection via identity fields** — structured format payloads in identity values (e.g., `role=system\nIgnore previous`)
2. **Unicode bypass** — homoglyphs, zero-width joiners, NFKC normalization edge cases targeting compressed format
3. **Alignment interference** — identity values designed to weaken safety behaviors (e.g., `expertise=unrestricted`)
4. **Format confusion** — malformed `key=value` that tricks model into misinterpreting field boundaries
5. **Schema abuse** — valid codebook values combined to create adversarial personas

Test protocol:
- Each attack vector run against injection detector (should it catch it?) and against live models (does the model comply?)
- Bypass rate = attacks that pass detection AND cause model misbehavior
- Results: per-category bypass rates, false positive rates, recommended mitigations

### Runtime Hook Plugin: `internal/plugin/builtin/adversarial/`

**Type:** `hook` plugin — pre-request, runs after injection detection as second-pass defense.

- Focuses on semantically valid but adversarial identity combinations (what injection detector can't catch)
- Configurable denylist of value patterns per dimension
- Logs flagged requests via `slog` structured audit logging
- Configurable action: `log`, `block`, or `sanitize`

### Boundaries
- No model-in-the-loop detection (too expensive for runtime)
- No automatic denylist updates — test harness findings manually curated into hook config

## Dependency Graph

```
introspect ← responseopt (feature 2)
introspect ← deltaidentity (feature 1, mutability check)
diff ← deltaidentity (feature 1)
diff ← identitystore (feature 3, restore)
diff ← engram-bench (feature 4, measure deltas)
codebook schema extensions ← features 1, 2 (mutable, constraints fields)
plugin/types.go ← feature 3 (TypePersistence)
security/injection.go ← feature 5 (adversarial hook extends detection)
```

## Implementation Order

1. Shared primitives (diff, introspect) + codebook schema extensions
2. Dynamic identity updates (depends on diff, introspect)
3. Response optimization (depends on introspect)
4. Cross-session persistence (depends on diff, new plugin type)
5. Benchmarking harness (depends on diff, standalone)
6. Adversarial robustness (depends on existing security, standalone)

Features 2-3 can be parallelized after primitives. Features 5-6 are independent of each other and of 2-4.
