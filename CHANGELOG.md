# Changelog

All notable changes to Engram will be documented in this file.

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
