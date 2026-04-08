# Codebook Optimizations Design

## Purpose

Engram's codebook compression saves tokens by serializing identity and conversation context into compact `key=value` format. Three optimizations increase savings and improve accuracy without changing the core architecture:

1. **Periodic codebook re-injection** — stop sending codebook definitions on every turn; re-inject every N turns aligned with the proxy's compression window
2. **System prompt token accounting** — fix the proxy's token counter to include the system prompt, making the statusline chart accurate
3. **Enum default omission** — skip emitting key=value pairs when the value matches the schema default, reducing compressed turn size

Combined estimated savings: ~60 tokens per turn at turn 9 (growing with conversation length).

---

## Tasks

### Task 1: `--window` CLI flag (default 10)

**Problem:** The proxy's compression window size is hardcoded. The handler has no concept of a re-injection interval. Both should be configurable from a single knob.

**Solution:**
- Add `--window N` flag to the `engram` CLI (root or serve command), default `10`
- Pass the value to `proxy.NewHandler(windowSize, ...)` (already accepts it)
- Pass the value to `server.NewHandler(...)` via a new `windowSize int` constructor parameter
- Store `windowSize` as a field on `Handler`
- Minimum effective value is `2`; values `0` or `1` mean "inject every turn" (no optimization)

**Files changed:**
- `cmd/engram/...` — add flag, wire to both proxy and handler constructors
- `internal/server/handler.go` — add `windowSize int` field and constructor param

---

### Task 2: Periodic codebook definition re-injection

**Problem:** `ContextCodebookDef` and `ResponseCodebookDef` (~15–25 tokens) are injected into every system prompt via `AssemblePrompt`, even though the LLM only needs them to decode the compressed history format. After the first turn, they are redundant until the proxy rolls up old turns into `[CONTEXT_SUMMARY]`.

**Solution:**
- In `handler.go` `HandleRequest`, before building `PromptParts`:
  - Read `sess.Snapshot().Turns`
  - If `turns % h.windowSize == 0` (including turn 0): populate `ctxCodebookDef` and `respCodebookDef`
  - Otherwise: leave them as empty strings
- `AssemblePrompt` already skips empty `[CONTEXT_CODEBOOK]` and `[RESPONSE_CODEBOOK]` blocks — no assembler changes needed

**Data flow:**
```
Turn 0:  [IDENTITY] + [CONTEXT_CODEBOOK] + [RESPONSE_CODEBOOK] + [QUERY]
Turn 1-9: [IDENTITY] + [HISTORY] + [QUERY]
Turn 10: [IDENTITY] + [CONTEXT_CODEBOOK] + [RESPONSE_CODEBOOK] + [HISTORY] + [QUERY]
```

**Savings:** ~20 tokens per turn on turns 1–9, 11–19, etc.

**Files changed:**
- `internal/server/handler.go` — add turn-modulo gate before populating codebook def strings

---

### Task 3: System prompt token accounting

**Problem:** The proxy's `ctxOrig` is computed as `EstimateTokens(req.Messages)`, which only counts the `messages` array. The system prompt (`req.System`) — often the largest part of the payload — is excluded. This makes the statusline context chart show artificially low values.

**Solution:**
- In `proxy/handler.go` `ServeHTTP`, change:
  ```go
  ctxOrig := EstimateTokens(req.Messages)
  ```
  to:
  ```go
  ctxOrig := EstimateTokens(req.Messages) + len(req.System)/4
  ```
- Apply the same adjustment to `ctxComp` (after compression):
  ```go
  ctxComp := EstimateTokens(req.Messages) + len(req.System)/4
  ```
  The system prompt is not compressed by the proxy, so it appears in both orig and comp — but now the absolute values are correct, and future system-prompt compression would show up as a delta.

**Files changed:**
- `internal/proxy/handler.go` — two lines changed at the `EstimateTokens` call sites

---

### Task 4: Enum default omission in SerializeTurn

**Problem:** `SerializeTurn` emits every field present in the turn as `key=value`. For enum fields like `role`, the most common value (`user`) appears in the majority of turns. Emitting `role=user` on every user turn wastes ~3 tokens per turn in the compressed history.

**Solution:**

**4a. Derive defaults from schema:**
- In `DeriveCodebook`, when parsing a type hint that starts with `"enum:"`, extract the comma-separated values
- The first value becomes the default for that field
- Store defaults in a new `defaults map[string]string` field on `ContextCodebook`

**4b. Omit defaults in SerializeTurn:**
- Before emitting `key=value`, check if `value == c.defaults[key]`
- If equal, skip it
- If the turn contains only default values, return an empty string (the turn still gets stored in history — an empty compressed request represents "a turn occurred with all-default fields")

**4c. Mark defaults in Definition():**
- Change the definition format to mark defaults with `*`
- Example: `role(enum:user*,assistant)` — the `*` signals "assume user when role is absent"
- This teaches the LLM how to decode compressed turns that omit the default

**Savings:** ~5 tokens per history turn where `role=user` (majority of turns). At turn 9 with 8 prior turns: ~40 additional tokens saved.

**Files changed:**
- `internal/context/codebook.go` — add `defaults` field, parse defaults in `DeriveCodebook`, skip in `SerializeTurn`, mark in `Definition()`

---

## Testing and Verification

### Unit Tests

**`internal/context/codebook_test.go`:**
- `TestSerializeTurn_OmitsDefaultEnum` — turn with `role=user` (the default) omits it from output; turn with `role=assistant` includes it
- `TestSerializeTurn_AllDefaults` — turn with all default values returns empty string
- `TestSerializeTurn_NonEnumUnchanged` — text fields are never omitted regardless of value
- `TestDefinition_MarksDefaults` — definition output contains `*` on first enum value (e.g. `role(enum:user*,assistant)`)
- `TestDeriveCodebook_NoEnumNoDefaults` — schema with only text fields produces empty defaults map

**`internal/server/handler_test.go`:**
- `TestHandleRequest_CodebookDefFirstTurn` — codebook definitions appear in system prompt on turn 0
- `TestHandleRequest_CodebookDefSkippedMidWindow` — codebook definitions absent on turns 1 through windowSize-1
- `TestHandleRequest_CodebookDefReinjected` — codebook definitions reappear on turn N (where N = windowSize)
- `TestHandleRequest_WindowSizeZero` — window size 0 injects every turn (safe fallback)

**`internal/proxy/handler_test.go` (or compressor_test.go):**
- `TestEstimateTokens_IncludesSystemPrompt` — verify ctxOrig counts system prompt bytes
- `TestEstimateTokens_EmptySystem` — system prompt of "" adds 0

### Integration Verification

After implementation:
1. Run full test suite: `go test ./...`
2. Start engram with `--window 5`, run a multi-turn session, verify:
   - Statusline chart shows realistic context values (not ~1)
   - Codebook defs appear in assembled prompt on turns 0 and 5, absent on 1–4
   - Compressed history turns with `role=user` omit the role field
3. Compare token counts before/after by examining session JSON files

---

## Edge Cases

| Scenario | Behavior |
|---|---|
| `--window 0` or `--window 1` | Inject every turn (no optimization, safe) |
| Turn contains only default enum values | Empty string stored; turn still recorded in history |
| Unknown enum value at runtime | Emitted normally as `key=value`; only defaults are omitted |
| Schema with no enum fields | No defaults derived; `SerializeTurn` unchanged |
| Codebook changes mid-session | Not supported (codebook is immutable per session) |
| Proxy restart mid-session | Handler's turn gate is self-contained; no proxy state needed |
| Session resumed after eviction | New session starts at turn 0; codebook defs re-injected |
