# Codebook Optimizations Design Spec

**Date:** 2026-04-08
**Status:** Draft
**Authors:** Erik Meyer, Claude
**Reviewers:** —

---

## 1. Purpose

Engram's codebook compression saves tokens by serializing identity and conversation context into compact `key=value` format. This spec describes three optimizations that increase token savings and fix a measurement gap, without changing the core architecture:

1. **Periodic codebook re-injection** — stop sending codebook definitions on every turn; re-inject every N turns aligned with the proxy's compression window
2. **System prompt token accounting** — fix the proxy's token counter to include the system prompt, making the statusline context chart accurate
3. **Enum default omission** — skip emitting key=value pairs when the value matches the schema default, reducing compressed turn size

**Current state:** Identity compression achieves ~95% savings (75 → 4 tokens per turn). Context compression is instrumented but reporting `ctx_orig: 1, ctx_comp: 1` because the proxy ignores the system prompt in its token estimate. Codebook definitions add ~20 tokens of overhead on every turn unnecessarily. Enum fields like `role=user` repeat across the majority of history turns despite being predictable.

**Target state:** Accurate context token accounting in the statusline chart, ~60 additional tokens saved per turn at turn 9 (scaling with conversation length), and a single `--window` CLI flag controlling both the proxy compression window and codebook re-injection interval.

---

## 2. Background

### 2.1 How the codebook pipeline works today

The Engram system intercepts LLM API calls through a local proxy (`localhost:4242`). On each request:

1. **Identity serialization** — the handler (`internal/server/handler.go`) converts the client's identity map (e.g., `{"role": "admin", "domain": "fire"}`) into a compact `role=admin domain=fire` string using the identity codebook, then injects it as `[IDENTITY]` in the system prompt.

2. **Context codebook** — if the client provides a `ContextSchema` on the first request, the handler derives a `ContextCodebook` that defines how to compress conversation turns. The codebook definition (e.g., `client-1: content(text) role(enum:user,assistant)`) is injected into the system prompt as `[CONTEXT_CODEBOOK]` on every turn.

3. **Response codebook** — a built-in codebook for compressing LLM response metadata (stop_reason, model, token counts). Also injected as `[RESPONSE_CODEBOOK]` on every turn.

4. **History compression** — after each turn, the handler compresses the user query and assistant response using the context/response codebooks and appends them to a `History` object. On subsequent turns, the compressed history is injected as `[HISTORY]`.

5. **Proxy-level compression** — the proxy (`internal/proxy/handler.go`) intercepts the outbound API call, counts tokens in the `messages` array, applies a sliding-window compression (keeping the last N messages verbatim, rolling older ones into a `[CONTEXT_SUMMARY]` block), and writes `ctx_orig`/`ctx_comp` stats to a session JSON file.

6. **Statusline** — `~/.claude/statusline-command.sh` reads the session's `total_saved` (identity savings) and `ctx_orig`/`ctx_comp` (context savings from proxy), then calls `engram statusline --ctx-orig X --ctx-comp Y` to render the side-by-side bar chart.

### 2.2 What's broken or suboptimal

| Issue | Impact | Root cause |
|---|---|---|
| Codebook definitions sent every turn | ~20 wasted tokens/turn | No turn-awareness in handler; defs always populated |
| `ctx_orig` and `ctx_comp` report as `1` | Context chart never appears | `EstimateTokens()` counts `req.Messages` only, ignores `req.System` |
| `role=user` emitted on every user turn | ~3–5 wasted tokens/turn in history | `SerializeTurn` has no concept of enum defaults |
| Proxy window size not configurable | Can't tune compression behavior | Hardcoded at call site |

---

## 3. Tasks

### Task 1: `--window` CLI flag (default 10)

**Owner:** CLI / configuration layer
**Depends on:** Nothing
**Blocks:** Task 2

**Problem:** The proxy's compression window size is passed as an integer to `proxy.NewHandler()` but is not exposed to the user. The handler has no concept of a re-injection interval. Both should be driven from a single configurable value.

**Solution:**

1. Add a `--window N` flag to the `engram` CLI serve command with a default of `10`
2. Validate the input: any value < 2 is treated as "inject every turn" (no optimization, safe fallback)
3. Pass the value to `proxy.NewHandler(windowSize, sessionsDir, upstream)` — this parameter already exists
4. Pass the value to `server.NewHandler(...)` via a new `windowSize int` constructor parameter
5. Store `windowSize` as a field on the `Handler` struct

**Acceptance criteria:**
- `engram serve --window 5` starts the proxy with a window size of 5
- `engram serve` (no flag) defaults to window size 10
- `engram serve --window 0` falls back to "inject every turn"
- The handler struct exposes the window size for use by Task 2

**Files changed:**
| File | Change |
|---|---|
| `cmd/engram/...` (serve command) | Add `--window` flag, wire to proxy and handler constructors |
| `internal/server/handler.go` | Add `windowSize int` field to `Handler` struct; update `NewHandler` and `NewHandlerWithSecurity` signatures |
| `internal/server/handler_test.go` | Update `newTestDeps` helper to pass window size |

---

### Task 2: Periodic codebook definition re-injection

**Owner:** Handler / prompt assembly layer
**Depends on:** Task 1 (needs `windowSize` on handler)
**Blocks:** Nothing

**Problem:** `ContextCodebookDef` and `ResponseCodebookDef` are injected into the system prompt on every turn via `AssemblePrompt`. These definitions (~15–25 tokens combined) teach the LLM how to decode compressed history entries. After the first turn, they are redundant — the LLM retains them in its context window. They only need to be re-sent when the proxy's sliding window rolls up older messages (which could discard the turn that originally carried the definitions).

**Solution:**

In `handler.go` `HandleRequest`, before building `PromptParts`:

```go
// Inject codebook definitions on turn 0 and every windowSize turns thereafter.
// This aligns with the proxy's compression window — when old turns are rolled
// into [CONTEXT_SUMMARY], the definitions are re-injected so the LLM can still
// decode the remaining compressed history entries.
snap := sess.Snapshot()
injectDefs := h.windowSize < 2 || snap.Turns%h.windowSize == 0

var ctxCodebookDef, respCodebookDef string
if injectDefs && rctx.ContextCodebook != nil {
    ctxCodebookDef = rctx.ContextCodebook.Definition()
    respCodebookDef = engramctx.AnthropicResponseCodebook().Definition()
}
```

`AssemblePrompt` already skips empty `[CONTEXT_CODEBOOK]` and `[RESPONSE_CODEBOOK]` blocks when the strings are empty — no assembler changes needed.

**Prompt structure by turn:**

| Turn | Prompt blocks |
|---|---|
| 0 | `[IDENTITY]` + `[CONTEXT_CODEBOOK]` + `[RESPONSE_CODEBOOK]` + `[QUERY]` |
| 1–9 | `[IDENTITY]` + `[HISTORY]` + `[QUERY]` |
| 10 | `[IDENTITY]` + `[CONTEXT_CODEBOOK]` + `[RESPONSE_CODEBOOK]` + `[HISTORY]` + `[QUERY]` |
| 11–19 | `[IDENTITY]` + `[HISTORY]` + `[QUERY]` |

**Savings:** ~20 tokens saved on 9 out of every 10 turns. Over a 20-turn session: ~180 tokens saved.

**Acceptance criteria:**
- Codebook definitions appear in assembled prompt on turn 0
- Codebook definitions are absent on turns 1 through `windowSize - 1`
- Codebook definitions reappear on turn N (where N = windowSize)
- Window size of 0 or 1 injects every turn (no regression from current behavior)

**Files changed:**
| File | Change |
|---|---|
| `internal/server/handler.go` | Add turn-modulo gate before populating codebook def strings |

---

### Task 3: System prompt token accounting

**Owner:** Proxy layer
**Depends on:** Nothing
**Blocks:** Nothing (but fixes the statusline chart immediately)

**Problem:** The proxy's `ctxOrig` is computed as `EstimateTokens(req.Messages)`, which only counts the byte length of the `messages` array divided by 4. The system prompt (`req.System`) — typically the largest component of the payload, containing `[IDENTITY]`, `[KNOWLEDGE]`, codebook definitions, and `[QUERY]` — is excluded entirely. This causes the proxy to write `ctx_orig: 1, ctx_comp: 1` to session files, which means:
- The statusline context chart never renders (the script checks `TOTAL_SAVED > 0`)
- Token accounting is systematically undercounted by thousands of tokens

**Current code (`proxy/handler.go`):**
```go
ctxOrig := EstimateTokens(req.Messages)
req.Messages = Compress(req.Messages, h.windowSize)
ctxComp := EstimateTokens(req.Messages)
```

**Fixed code:**
```go
systemTokens := len(req.System) / 4
ctxOrig := EstimateTokens(req.Messages) + systemTokens
req.Messages = Compress(req.Messages, h.windowSize)
ctxComp := EstimateTokens(req.Messages) + systemTokens
```

The system prompt is not compressed by the proxy (it passes through unchanged), so `systemTokens` appears in both `ctxOrig` and `ctxComp`. This makes the absolute values accurate. When future work adds system-prompt compression, the delta will automatically appear.

**Why this matters for the statusline:**
The `statusline-command.sh` script computes `CTX_ORIG = total_input + TOTAL_SAVED` and `CTX_FLAGS="--ctx-orig $CTX_ORIG --ctx-comp $total_input"`. But it only shows the context chart when `TOTAL_SAVED > 0`. With the proxy writing `1:1`, the identity savings go into `total_saved` (from the session start hook) but context savings stay at 0. After this fix, `ctx_orig` and `ctx_comp` will differ by however many message tokens the proxy's `Compress()` actually removes, and the chart will render.

**Acceptance criteria:**
- `ctxOrig` includes system prompt token estimate
- `ctxComp` includes system prompt token estimate
- Session `.ctx.json` files show realistic values (hundreds to thousands, not 1)
- Statusline context chart renders when proxy compression has saved tokens

**Files changed:**
| File | Change |
|---|---|
| `internal/proxy/handler.go` | Add `systemTokens` variable, include in both `ctxOrig` and `ctxComp` |

---

### Task 4: Enum default omission in SerializeTurn

**Owner:** Context compression layer
**Depends on:** Nothing
**Blocks:** Nothing

**Problem:** `SerializeTurn` emits every field present in the turn as `key=value`, regardless of whether the value is predictable. For enum fields like `role`, the most common value (`user`) appears in the vast majority of turns. Emitting `role=user` repeatedly wastes ~3–5 tokens per turn in the compressed history. Over an 8-turn history, that's ~24–40 tokens of avoidable overhead.

**Solution — three sub-tasks:**

#### 4a. Derive defaults from schema

In `DeriveCodebook`, when parsing a type hint that starts with `"enum:"`:
1. Split the remainder by `,` to get the list of valid values
2. The **first value** in the list becomes the default for that field
3. Store all defaults in a new `defaults map[string]string` field on `ContextCodebook`

For type hints that don't start with `"enum:"` (e.g., `"text"`), no default is stored.

**Example:**
```go
schema := map[string]string{
    "role":    "enum:user,assistant",   // default: "user"
    "content": "text",                   // no default
}
// cb.defaults = {"role": "user"}
```

#### 4b. Omit defaults in SerializeTurn

Before emitting `key=value`, check if the value matches the default:

```go
for _, k := range c.keys {
    v, ok := turn[k]
    if !ok {
        continue
    }
    // Skip if value matches the enum default — the LLM infers it from the codebook.
    if def, hasDef := c.defaults[k]; hasDef && v == def {
        continue
    }
    // ... emit key=value as before
}
```

If all fields in the turn are defaults, the function returns an empty string. The turn is still stored in history — an empty compressed request represents "a turn occurred with all-default field values."

**Before:** `role=user content=What is fire code Section 4.2?`
**After:** `content=What is fire code Section 4.2?`

#### 4c. Mark defaults in Definition()

Change the definition format to signal defaults with `*`:

**Before:** `client-1: content(text) role(enum:user,assistant)`
**After:** `client-1: content(text) role(enum:user*,assistant)`

The `*` after the first enum value tells the LLM: "when `role` is absent from a compressed turn, assume `user`."

Implementation in `Definition()`:
```go
typeHint := c.schema[k]
if def, ok := c.defaults[k]; ok && strings.HasPrefix(typeHint, "enum:") {
    // Mark the default value with * in the enum list
    typeHint = "enum:" + strings.Replace(
        strings.TrimPrefix(typeHint, "enum:"),
        def, def+"*", 1,
    )
}
b.WriteString(typeHint)
```

**Savings:** ~5 tokens per user turn in history. At turn 9 with 8 prior turns (6 user, 2 assistant): ~30 tokens saved. Compounds with conversation length.

**Acceptance criteria:**
- `SerializeTurn` omits `role=user` when `user` is the default enum value
- `SerializeTurn` includes `role=assistant` (non-default)
- Turn with all default values returns empty string
- Text fields are never omitted regardless of value
- `Definition()` output marks first enum value with `*`
- Schema with no enum fields: `defaults` map is empty, behavior unchanged

**Files changed:**
| File | Change |
|---|---|
| `internal/context/codebook.go` | Add `defaults` field to `ContextCodebook`, parse in `DeriveCodebook`, skip in `SerializeTurn`, mark in `Definition()` |

---

## 4. Testing Plan

### 4.1 Unit Tests

#### `internal/context/codebook_test.go`

| Test | Description | Validates |
|---|---|---|
| `TestDeriveCodebook_ParsesEnumDefaults` | Schema with `"enum:user,assistant"` produces `defaults["role"] = "user"` | 4a |
| `TestDeriveCodebook_NoEnumNoDefaults` | Schema with only `"text"` fields produces empty defaults map | 4a |
| `TestDeriveCodebook_MultipleEnums` | Multiple enum fields each get their first value as default | 4a |
| `TestSerializeTurn_OmitsDefaultEnum` | Turn with `role=user` (default) omits it; turn with `role=assistant` includes it | 4b |
| `TestSerializeTurn_AllDefaults` | Turn with only default values returns empty string | 4b |
| `TestSerializeTurn_NonEnumUnchanged` | Text fields emit regardless of value (no false omission) | 4b |
| `TestSerializeTurn_MixedDefaultAndNon` | Turn with one default and one non-default only emits the non-default | 4b |
| `TestDefinition_MarksDefaults` | Output contains `role(enum:user*,assistant)` | 4c |
| `TestDefinition_NoDefaultNoMarker` | Text fields show no `*` marker | 4c |

#### `internal/server/handler_test.go`

| Test | Description | Validates |
|---|---|---|
| `TestHandleRequest_CodebookDefFirstTurn` | Codebook definitions present in system prompt on turn 0 | Task 2 |
| `TestHandleRequest_CodebookDefSkippedMidWindow` | Definitions absent on turns 1 through windowSize-1 | Task 2 |
| `TestHandleRequest_CodebookDefReinjected` | Definitions reappear on turn N (N = windowSize) | Task 2 |
| `TestHandleRequest_WindowSizeZero` | Window size 0 injects every turn (safe fallback) | Task 2 |
| `TestHandleRequest_WindowSizeOne` | Window size 1 injects every turn | Task 2 |

#### `internal/proxy/compressor_test.go` or `handler_test.go`

| Test | Description | Validates |
|---|---|---|
| `TestCtxOrig_IncludesSystemPrompt` | `ctxOrig` includes `len(system)/4` | Task 3 |
| `TestCtxOrig_EmptySystem` | Empty system prompt contributes 0 | Task 3 |
| `TestCtxOrig_LargeSystem` | 4000-char system prompt contributes ~1000 tokens | Task 3 |

### 4.2 Integration Verification

After all tasks are implemented, run the following manual verification:

1. **Full test suite:** `go test ./...` — all existing and new tests pass
2. **Multi-turn session with `--window 5`:**
   - Start engram: `engram serve --window 5`
   - Send 6+ turns via a test client
   - Inspect assembled prompts: codebook defs present on turns 0 and 5, absent on 1–4
   - Inspect compressed history: user turns show `content=...` without `role=user`
3. **Statusline chart verification:**
   - After a multi-turn session, check `~/.engram/sessions/<id>.ctx.json`
   - Verify `ctx_orig` and `ctx_comp` are realistic values (hundreds+, not 1)
   - Verify the statusline renders both identity and context columns
4. **Before/after token comparison:**
   - Run two identical 10-turn sessions: one on current main, one with these changes
   - Compare total `TokensSent` from session JSON files
   - Expected improvement: ~200+ tokens saved over 10 turns

### 4.3 Regression Checks

| Area | Check |
|---|---|
| Identity serialization | Unchanged — identity codebook is separate from context codebook |
| Assembler | Unchanged — already skips empty blocks |
| Proxy forwarding | Unchanged — only token counting is affected |
| Session lifecycle | Unchanged — no new session state, turn counter already exists |
| Security (injection detection) | Unchanged — runs before codebook logic |

---

## 5. Edge Cases

| Scenario | Expected behavior | Rationale |
|---|---|---|
| `--window 0` or `--window 1` | Inject codebook defs every turn | Safe fallback; no optimization but no regression |
| Turn contains only default enum values | `SerializeTurn` returns empty string; turn still stored in history | Empty request represents "turn occurred with all-default fields" |
| Unknown enum value at runtime | Emitted normally as `key=value` | Only known defaults are omitted; unknown values pass through |
| Schema with no enum fields | `defaults` map is empty; no behavior change | Text fields are never candidates for omission |
| Codebook changes mid-session | Not supported | Codebook is derived once on session creation and immutable (existing constraint) |
| Proxy restart mid-session | Handler's turn gate continues working | Turn count is on the session, not the proxy |
| Session evicted and recreated | New session starts at turn 0; defs re-injected | Natural reset; no stale state |
| Very long system prompt (>100K chars) | `len(system)/4` gives ~25K token estimate | Rough but directionally accurate; matches existing estimation approach |
| Concurrent sessions with different window sizes | Each handler instance has its own `windowSize` | No shared state between sessions |

---

## 6. Rollout

These changes are backward-compatible and can ship together in a single release:

- **No wire format changes** — the compressed `key=value` format is the same; we just emit fewer pairs
- **No API changes** — `IncomingRequest`, `Response`, and `ContextSchema` are unchanged
- **No migration needed** — existing session files continue to work; new sessions benefit automatically
- **CLI flag is additive** — `--window` has a sensible default; existing deployments don't need to change their invocation
