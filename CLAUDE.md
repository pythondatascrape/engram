# CLAUDE.md

## What This Is

Engram is a Go server that compresses LLM context through identity-aware serialization. Clients send structured identity once per session; Engram serializes it into a compact self-describing format and injects it into every LLM call, saving 85-93% of redundant tokens.

**Architecture:** Modular monolith. Single Go binary. Plugin system for extensibility. QUIC transport for clients, WebSocket/TCP for LLM providers.

## Build & Run

```bash
go build -o engram ./cmd/engram
./engram serve --config engram.yaml
```

### Tests

```bash
go test ./...                    # all tests
go test ./internal/session/...   # specific package
go test -race ./...              # race detector
go test -count=1 ./...           # no cache
```

### Proto Generation

```bash
buf generate                     # generate Go code from proto/
buf lint                         # lint proto files
buf breaking --against .git#branch=main  # check breaking changes
```

## Project Structure

```
cmd/engram/              → main entry point
internal/
  config/                → YAML + env var config loading
  server/                → HTTP/QUIC server lifecycle
  session/               → session state, eviction, TTL
  auth/                  → JWT signing/validation, client registration
  identity/
    codebook/            → codebook schema, validation, registry
    serializer/          → identity → self-describing format
  provider/
    pool/                → dynamic connection pooling per (provider, api_key)
    builtin/anthropic/   → built-in Anthropic provider plugin
    builtin/openai/      → built-in OpenAI provider plugin
  knowledge/             → knowledge ref resolution
  security/              → injection detection, response filtering
  plugin/
    registry/            → plugin lifecycle, discovery
    grpc/                → gRPC plugin protocol
  transport/
    quic/                → QUIC listener, stream handling
    websocket/           → WebSocket fallback for clients
    webtransport/        → WebTransport for browsers
  events/                → push event system
  admin/                 → admin API endpoints
proto/engram/v1/         → protobuf definitions (source of truth)
codebooks/               → YAML codebook files
```

## Code Conventions

### Go Best Practices

- **Error handling:** Return errors, don't panic. Wrap with `fmt.Errorf("context: %w", err)`.
- **Context:** Every public function accepts `context.Context` as first param. Use it for cancellation and timeouts.
- **Interfaces:** Define where consumed, not where implemented. Keep them small (1-3 methods).
- **Naming:** Follow Go conventions. No stuttering (`session.Session` not `session.SessionManager`). Acronyms all-caps (`HTTPServer`, `JWTClaims`).
- **Concurrency:** Goroutines must be owned. Every goroutine has a clear shutdown path via context cancellation. Use `errgroup` for coordinated goroutine lifecycle.
- **Testing:** Table-driven tests. Use `testify` for assertions. Test behavior, not implementation.
- **Packages:** No circular imports. Internal packages enforce boundaries.
- **Logging:** Structured logging with `slog`. No `fmt.Println` in production code.

### Plugin System

Everything is a plugin. 5 types: `provider`, `serializer`, `codebook`, `hook`, `observability`. Built-in plugins are in-process Go. External plugins communicate via gRPC.

### Proto/API Changes

- Protobuf definitions in `proto/engram/v1/` are the source of truth for all client-server communication
- Never reuse field numbers. Reserve removed fields.
- Breaking changes require new package version (`engram.v2`)

## Issue Tracking

All issues, features, and bugs are tracked using GitHub Issues via the `/gitissue` skill. Use `/gitissue` to create structured issues with impact scoring and branch linkage.

## Python Tooling

Python scripts (codebook tools, test utilities) use `uv` as the project wrapper:

```bash
uv run python scripts/some_tool.py
uv add some-package
```

## Key Design Decisions

1. **Self-describing format over opaque vectors:** Identity serialized as human-readable key-value pairs (~100 tokens) instead of numerical vectors + interpreter prompt (~350 tokens).
2. **QUIC for clients, WebSocket for LLM providers:** QUIC gives 0-RTT reconnection for clients. LLM providers only expose HTTP/WebSocket, so server bridges the gap.
3. **Client selects provider/model:** Client pays for tokens, so client chooses. Server manages the connection pool.
4. **Reference tokens (JWT):** Short-lived Ed25519 JWTs. API keys stored server-side, encrypted at rest (AES-256-GCM). Clients never handle raw provider keys.
5. **Session state is ephemeral:** Sessions live in memory. No persistence. Eviction via idle timeout, TTL, or memory pressure. This is by design.
6. **Plugin-first architecture:** Codebook distribution, serialization, provider adapters, middleware hooks, and observability are all plugins. Built-in features are just plugins that ship with the binary.

## What Not To Do

- Don't add interpreter/decoder prompts. The self-describing format replaced that architecture.
- Don't persist sessions to disk. Sessions are intentionally ephemeral.
- Don't bypass the plugin registry. Even built-in features register as plugins.
- Don't use raw goroutines without context cancellation. Every goroutine must have a shutdown path.
- Don't commit API keys or secrets. Use `engram.yaml` config with env var overrides.
- Don't skip proto generation. If you change `.proto` files, run `buf generate`.


<claude-mem-context>
# Recent Activity

<!-- This section is auto-generated by claude-mem. Edit content outside the tags. -->

*No recent activity*
</claude-mem-context>