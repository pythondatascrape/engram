# Changelog

All notable changes to Engram will be documented in this file.

## [0.3.3] - 2026-04-19

### Changed
- Anthropic proxy requests now keep the `system` prompt verbatim instead of attempting identity compression, while still applying conversation window compression.

### Fixed
- Removed the Anthropic `count_tokens` preflight from the proxy path, avoiding extra upstream traffic before each Claude request.
- Anthropic proxy failures are now easier to diagnose with clearer upstream error logging.
- Claude-through-Engram reliability is restored for Anthropic by disabling the system/identity rewrite that triggered upstream throttling.

## [0.3.0] - 2026-04-05

### Added
- **Zero-touch install** — `engram install --claude-code` now handles the full setup in one command: plugin copy, settings merge, config creation, daemon service registration, service start, and readiness verification
- `config.EnsureDefault` — creates `~/.engram/engram.yaml` with working defaults if absent; leaves existing config unchanged
- `verifyReadiness` — post-install check that polls the daemon Unix socket and proxy TCP port; install exits non-zero if either is unreachable within 15 seconds
- Linux systemd support in the Claude install flow (`engram install --claude-code` now works on Linux as well as macOS)
- Shared path helpers (`DefaultSocketPath`, `DefaultConfigPath`, `DefaultSessionsDir`) — single source of truth used by both `serve` and `install` code paths

### Changed
- `engram install --claude-code` is now **fail-closed**: the install exits with a non-zero status if any required step fails (plugin copy, settings merge, config creation, service install, service start, or readiness check). Success output only appears when all steps complete.
- `RegisterProxyHeaders` failure is now a hard error instead of a warning — a partial install where Claude isn't routed through Engram is no longer possible
- `--config` flag defaults to `~/.engram/engram.yaml` on all commands (`serve`, root persistent flag). Previously defaulted to `engram.yaml` relative to the working directory, which caused silent failures when no config existed in CWD
- `engram serve` no longer requires a config file in the working directory — it finds `~/.engram/engram.yaml` automatically
- Readiness check uses the proxy port from the actual loaded config, not a hardcoded default — custom proxy ports work correctly end-to-end

### Fixed
- `installLaunchd` and `installSystemd` were previously called with an empty socket path, producing invalid service definitions. Both now receive the real socket path from `DefaultSocketPath()`
- Config path detection in `installService` used a fragile string sentinel (`"engram.yaml"`); replaced with cobra's `flag.Changed` to correctly distinguish user-provided vs default values

## [0.2.0] - 2026-04-02

### Added
- Connection pooling across all SDKs (Go, Python, Node.js) for lower latency
- Recursive identity file scanning (finds CLAUDE.md, AGENTS.md, .cursorrules in subdirectories)
- Schema-once optimization — codebook definitions injected only on first turn
- Token savings bar chart visualization in `engram analyze`
- `engram advisor` command for optimization recommendations
- Go SDK with channel-based connection pool
- Python async SDK with `asyncio.Queue` pool
- Node.js SDK with persistent connection pool
- GitHub Actions CI (Go core + all 3 SDK test suites)
- GoReleaser configuration for cross-platform binary releases

### Changed
- Context compression pipeline: codebook, history, and response compression stages
- Identity compression achieves ~96-98% token reduction on real projects
- SDKs use persistent connections (previously fresh connection per call)

### Fixed
- Scanner now finds all identity files in subdirectories (previously root-only)
- Python FakeDaemon updated for persistent connection framing
- Node SDK uses `rm` instead of deprecated `rmdir`

## [0.1.0] - 2026-03-27

### Added
- Initial release
- Local daemon with JSON-RPC 2.0 over Unix socket
- Identity compression via codebook derivation
- Context codebook for message serialization
- MCP plugin for Claude Code integration
- `engram install`, `engram analyze`, `engram serve`, `engram status` commands
