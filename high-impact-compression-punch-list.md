# High-Impact Compression Punch List

## Goal

Increase Engram's real token savings in the paths that matter most:

1. **Identity savings** must work on normal prose `CLAUDE.md` files, not only hand-written `key=value` files.
2. **Context savings** must materially improve on long Claude Code sessions, especially tool-heavy sessions.
3. **Overall session savings** must move closer to the README claims by reducing the large unchanged system/identity footprint in addition to message history.

## Priority 1 — Make Identity Compression Work On Prose

### 1. Upgrade `engram.deriveCodebook`

**Why it matters**

Current behavior in [internal/identity/codebook/derive.go](/Users/emeyer/Desktop/Engram/internal/identity/codebook/derive.go) only recognizes:

- `key=value`
- `key: value`

That means normal prose `CLAUDE.md` produces zero dimensions and effectively no useful identity compression.

**Changes**

- Expand derive logic to extract dimensions from common prose patterns:
  - "I prefer concise responses"
  - "Do not add trailing summaries"
  - "I am a senior Go engineer"
  - "Target macOS and Linux"
- Add a normalization layer that maps prose phrases to bounded dimensions:
  - `response_style=concise`
  - `summary_policy=no_trailing_summary`
  - `role=engineer`
  - `seniority=senior`
  - `platform=macos,linux`
- Keep extraction deterministic so the same prose yields the same codebook.
- Avoid unbounded free-text dimensions unless no bounded mapping exists.

**Tests**

- prose-style `CLAUDE.md` derives non-zero dimensions
- existing `key=value` input still works
- existing `key: value` input still works
- repeated prose phrasing normalizes to the same dimension values

**Success criteria**

- a normal prose `CLAUDE.md` produces non-empty dimensions
- identity compression on prose is meaningfully smaller than the original text

## Priority 2 — Compress System/Identity Payload In The Proxy Path

### 2. Reduce the unchanged system prompt footprint

**Why it matters**

In real Claude Code traffic, the system payload is large. The current proxy compresses `messages`, but it does not reduce the raw `system` payload before forwarding. That caps overall savings even when message history shrinks.

**Changes**

- teach the proxy to detect and compress codebook-friendly identity/system content before upstream
- if SessionStart already produced a compressed identity block, prefer forwarding that compact representation instead of the original prose where possible
- add a safe fallback:
  - if identity compression fails, forward the original system content unchanged
- preserve Anthropic wire compatibility for both string and block-array `system` formats

**Tests**

- string `system` payload is compressed when eligible
- block-array `system` payload is compressed when eligible
- malformed or unsupported system payload still forwards fail-open
- all non-system request fields remain unchanged

**Success criteria**

- end-to-end requests show savings even when the system prompt dominates token count

## Priority 3 — Replace Naive Head Summaries With Token-Budget Summaries

### 3. Improve `internal/proxy/compressor.go`

**Why it matters**

Current context compression is simple:

- keep the last `window_size` messages
- summarize older messages as `role: text`
- truncate each old message to about 120 chars

That works, but it leaves a lot of redundant structure and tool noise in the request.

**Changes**

- replace line-per-message summarization with a token-budget summarizer
- summarize by semantic category instead of literal replay:
  - user goals
  - assistant decisions
  - files changed
  - open issues
  - unresolved constraints
- give tool-heavy turns a much tighter representation than plain text turns
- let the summary budget scale with total thread length instead of fixed per-message truncation
- preserve the last `window_size` turns verbatim

**Tests**

- long thread compresses to a smaller payload than current implementation
- summary still preserves recent goals and unresolved tasks
- tool-heavy threads shrink materially more than plain conversational threads
- no-op behavior remains for threads below the window

**Success criteria**

- long sessions show better savings than the current ~13% range from the existing e2e test

## Priority 4 — Reduce Tool Output Before It Pollutes Future Context

### 4. Make PostToolUse do real reduction, not only guidance

**Why it matters**

Right now [plugins/claude-code/hooks/posttooluse.mjs](/Users/emeyer/Desktop/Engram/plugins/claude-code/hooks/posttooluse.mjs) detects redundancy and emits guidance. That helps behaviorally, but large tool output can still bloat later turns.

**Changes**

- add a transform step for oversized tool output:
  - strip repeated boilerplate
  - collapse unchanged lines
  - prefer diff/delta summaries over full output replay
- emit structured summaries for common tool classes:
  - file reads
  - test output
  - lint output
  - grep/search output
  - command output
- keep the raw output available when needed, but stop feeding the full body forward by default

**Tests**

- repeated test output is summarized to failures and changed lines
- redundant search output is collapsed
- Engram’s own tool outputs remain ignored
- short outputs still pass through untouched

**Success criteria**

- tool-heavy sessions stop inflating later context as quickly

## Priority 5 — Make The Savings Visible With Honest Metrics

### 5. Split identity vs context vs tool-output savings

**Why it matters**

If savings are not broken out cleanly, it is hard to know which change helped and which claim is actually supported.

**Changes**

- record separate counters for:
  - identity/system savings
  - message-history savings
  - tool-output savings
- expose those counters in session stats and status output
- add a benchmark fixture set:
  - prose `CLAUDE.md`
  - structured `CLAUDE.md`
  - long coding thread
  - tool-heavy thread

**Tests**

- session stats reflect each category independently
- fixture benchmarks are reproducible

**Success criteria**

- README claims can be tied to named benchmark scenarios instead of mixed totals

## Recommended Execution Order

1. Improve prose identity derivation.
2. Compress the system/identity payload in the proxy.
3. Replace naive context summaries with token-budget summaries.
4. Turn PostToolUse into real tool-output reduction.
5. Add breakout metrics and benchmark fixtures.

## Expected Impact

- **Highest immediate impact:** prose-aware identity derivation
- **Highest overall session impact:** system/identity compression inside the proxy path
- **Highest long-thread impact:** better context summary logic
- **Highest tool-heavy session impact:** PostToolUse output reduction

## Definition Of Done

- prose `CLAUDE.md` files derive meaningful codebook dimensions
- identity compression works without forcing users into manual `key=value` authoring
- long Claude Code sessions show materially higher context savings than the current baseline
- tool-heavy workflows stop carrying large repeated outputs forward
- Engram can report identity and context savings separately with believable benchmark numbers
