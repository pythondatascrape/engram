# Context Compression Punch List

Goal: make the real Engram runtime path reliable so context compression and redundancy checks work in actual Claude sessions, even before we polish the statusline.

## 1. Harden hook payload parsing

Current risk:

- `sessionstart.mjs` only looks for `payload.session_id`
- `posttooluse.mjs` only looks for `payload.tool_name` and `payload.tool_output`

If Claude emits a different but equivalent payload shape, the hook silently does nothing.

Direct changes:

- Add tolerant session ID extraction in `sessionstart.mjs`
- Add tolerant tool-name/output extraction in `posttooluse.mjs`
- Keep fail-open behavior, but do not depend on a single JSON key spelling

Files:

- [plugins/claude-code/hooks/sessionstart.mjs](/Users/emeyer/Desktop/Engram/plugins/claude-code/hooks/sessionstart.mjs)
- [plugins/claude-code/hooks/posttooluse.mjs](/Users/emeyer/Desktop/Engram/plugins/claude-code/hooks/posttooluse.mjs)

## 2. Add regression tests for real hook inputs

Direct changes:

- Add SessionStart tests for:
  - `session_id`
  - `sessionId`
  - nested `session.id`
  - message emission when `CLAUDE.md` exists
- Add PostToolUse tests for alternate tool name/output fields

Files:

- `plugins/claude-code/tests/sessionstart.test.mjs`
- [plugins/claude-code/tests/posttooluse.test.mjs](/Users/emeyer/Desktop/Engram/plugins/claude-code/tests/posttooluse.test.mjs)

## 3. Keep proxy correlation fail-open but observable

Direct changes:

- Make SessionStart registration more robust, but keep it non-fatal if the proxy is unavailable
- Keep proxy context compression independent from statusline rendering

Files:

- [plugins/claude-code/hooks/sessionstart.mjs](/Users/emeyer/Desktop/Engram/plugins/claude-code/hooks/sessionstart.mjs)

## 4. Verify end-to-end behavior

After code changes:

- restart Claude
- open a new session in a project with `CLAUDE.md`
- confirm SessionStart emits the Engram identity-compression prompt
- confirm large tool outputs trigger `engram.checkRedundancy`
- confirm proxy continues intercepting `/v1/messages`

## Definition of Done

- SessionStart registers and emits its prompt for real Claude payloads
- PostToolUse triggers redundancy checks for real Claude payloads
- Hooks remain fail-open when Engram is unavailable
- We have regression tests covering the payload shapes we rely on
