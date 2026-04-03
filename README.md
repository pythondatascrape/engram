# Engram

**Local-first context compression for AI coding tools.** One binary saves 85-93% of redundant tokens across every LLM call.

---

## What is Engram?

Every time an AI coding tool sends a request to an LLM, it re-sends the same context: who you are, what you're working on, your preferences, your project structure. This redundancy costs real money and eats into context windows.

Engram eliminates it. It runs locally as a lightweight daemon, learns your development identity and project context, and compresses redundant information out of every LLM call. The result: dramatically smaller prompts, lower costs, and more room in the context window for what actually matters.

### Key Numbers

| Metric | Value |
|--------|-------|
| Identity compression | ~96% token reduction |
| Overall context savings | 85-93% per session |
| Startup overhead | <50ms |
| Memory footprint | ~30MB resident |

## Quick Start

```bash
# Install
brew install engram

# Set up Engram for your project
cd your-project
engram install

# See what Engram found
engram analyze

# Start the compression daemon
engram serve
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `engram install` | Interactive setup — detects your tools, configures integration |
| `engram analyze` | Analyze your project and show compression opportunities |
| `engram serve` | Start the compression daemon |
| `engram status` | Show daemon status, active sessions, and savings |
| `engram update` | Update codebooks and configuration |

Every command supports `--help` for detailed usage.

## Integrations

Engram works as a plugin for AI coding tools:

### Claude Code

```bash
engram install
# Engram auto-detects Claude Code and registers as an MCP plugin
```

Once installed, Engram compresses context automatically — no workflow changes needed.

### OpenClaw

```bash
engram install
# Engram auto-detects OpenClaw and configures the integration
```

### SDK

For custom integrations, use the Python SDK:

```bash
pip install engram-sdk
```

```python
from engram import Engram

client = Engram()
compressed = client.compress(context)
```

See the [Integration Guide](docs/integration-guide.md) for details.

## Demo

See the [Vanilla Ice demo project](https://github.com/erikmeyer/vanilla-ice) for a working example that shows Engram compressing a real project's context from ~4,200 tokens to ~380 tokens.

## Documentation

- [Getting Started](docs/getting-started.md) — Install and configure Engram for your first project
- [CLI Reference](docs/cli-reference.md) — Full command documentation with examples
- [Integration Guide](docs/integration-guide.md) — Configure Claude Code, OpenClaw, and SDK setup

## License

Apache 2.0 — see [LICENSE](LICENSE).
