# Statusline Context Correlation Review

Goal: make the statusline show the context-token chart for end users, not just the identity chart.

## What is already true

The context chart is implemented correctly in principle.

- [internal/proxy/session.go](/Users/emeyer/Desktop/Engram/internal/proxy/session.go) writes context metrics:
  - `ctx_orig`
  - `ctx_comp`
- [cmd/engram/statusline.go](/Users/emeyer/Desktop/Engram/cmd/engram/statusline.go) reads those metrics from `<session_id>.ctx.json`
- [internal/optimizer/format.go](/Users/emeyer/Desktop/Engram/internal/optimizer/format.go) renders a side-by-side chart:
  - left: identity `orig / comp / saved`
  - right: context `orig / comp / saved`

So this is not just identity repeated twice. The right-hand chart is intended to represent context-window savings.

## What is broken

On the current machine, the session IDs for the two data sources do not match.

Observed files under `~/.engram/sessions`:

- Claude Stop hook session files use Claude UUIDs such as:
  - `0490d190-0574-4301-9682-68d51f141513.json`
- Proxy context file uses a fallback proxy fingerprint:
  - `proxy-e3b0c44298fc1c14.ctx.json`

The statusline logic in [cmd/engram/statusline.go](/Users/emeyer/Desktop/Engram/cmd/engram/statusline.go) only renders the context chart when it can load both:

- `<session_id>.json`
- `<session_id>.ctx.json`

for the same `session_id`.

Because the filenames do not match, the command falls back to the identity-only chart even though context data exists on disk.

## Root cause

The Stop hook and the proxy are not correlating on the same session ID.

- The Stop hook writes the real Claude `session_id`
- The proxy writes `*.ctx.json` using its own fallback session ID
- In the observed case, the proxy session ID is `proxy-e3b0c44298fc1c14`, which is the SHA-derived fallback for an empty system prompt

That means the proxy is not getting a usable Claude session identifier for this request path.

## Direct changes needed

### 1. Make proxy context stats use the same session ID as the Stop hook

Preferred outcome:

- the proxy writes `~/.engram/sessions/<claude-session-id>.ctx.json`

not:

- `~/.engram/sessions/proxy-<fingerprint>.ctx.json`

This is the cleanest way to make the current statusline code work as intended.

### 2. Treat fallback proxy IDs as a degraded path, not the normal chart path

If the proxy cannot obtain a real Claude session ID, it should still work for compression, but that path should be considered insufficient for end-user statusline correlation.

At minimum:

- log when the proxy had to fall back
- add a regression test proving that installed Claude traffic produces matching `.json` and `.ctx.json` filenames for one session

### 3. Add an end-to-end test for statusline correlation

Add a test that verifies one logical Claude session produces:

- `<session>.json`
- `<session>.ctx.json`

and that `engram statusline` renders the side-by-side chart when invoked for that session.

## Definition of Done

- User opens a fresh Claude session
- Stop hook writes `~/.engram/sessions/<session>.json`
- Proxy writes `~/.engram/sessions/<session>.ctx.json`
- `engram statusline` renders both identity and context charts for that session
- End users can actually see context savings in the terminal, not just identity savings
