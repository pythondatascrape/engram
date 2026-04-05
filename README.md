# Engram

**Local-first context compression for AI coding tools.** One binary saves 85-93% of redundant tokens across every LLM call.

---

## What is Engram?

Every time an AI coding tool sends a request to an LLM, it re-sends the same context: who you are, what you're working on, your preferences, your project structure. This redundancy costs real money and eats into context windows.

Engram eliminates it. It runs locally as a lightweight daemon and compresses both your **identity** (CLAUDE.md, system prompts, project instructions) and your **conversation context** (message history, tool results, responses) across every LLM call. The result: dramatically smaller prompts, lower costs, and more room in the context window for what actually matters.

### How It Works

Engram applies three compression stages:

1. **Identity compression** — Verbose CLAUDE.md prose and project instructions are reduced to compact `key=value` codebook entries. Definitions are sent once on the first turn; subsequent turns reference keys only.
2. **Context compression** — Conversation history is serialized using a learned codebook that strips JSON overhead from message objects (`role=user content=...` instead of full JSON).
3. **Response compression** — LLM responses are compressed using provider-specific codebooks tuned to Anthropic and OpenAI output patterns.

### Key Numbers

| Metric | Value |
|--------|-------|
| Identity compression | ~96-98% token reduction |
| Context compression | ~40-60% token reduction |
| Overall savings | 85-93% per session |
| Startup overhead | <50ms |
| Memory footprint | ~30MB resident |

## Quick Start

```bash
# Install via Homebrew (macOS/Linux)
brew install pythondatascrape/tap/engram

# Or download a release binary
curl -fsSL https://github.com/pythondatascrape/engram/releases/latest/download/engram_$(uname -s | tr A-Z a-z)_$(uname -m | sed 's/x86_64/amd64/').tar.gz | tar xz
sudo mv engram /usr/local/bin/

# Or install from source
go install github.com/pythondatascrape/engram/cmd/engram@latest

# Install for Claude Code — one command, no manual steps
engram install --claude-code

# Restart Claude Code and start using Engram immediately
```

### What `engram install --claude-code` does

Starting in v0.3.0, a single install command handles everything:

1. Copies the Engram plugin into `~/.claude/plugins/`
2. Registers the statusline in `~/.claude/settings.json`
3. Creates `~/.engram/engram.yaml` with working defaults (if not already present)
4. Installs the daemon as a background service (launchd on macOS, systemd on Linux)
5. Starts the service immediately
6. Verifies the daemon socket and proxy are reachable
7. **Exits non-zero if any step fails** — no silent partial installs

After install, restart Claude Code. No `engram serve` step required.

## CLI Reference

| Command | Description |
|---------|-------------|
| `engram install --claude-code` | Zero-touch install for Claude Code — daemon, config, and plugin in one step |
| `engram install --openclaw` | Install for OpenClaw |
| `engram install` | Auto-detect and install for all supported clients |
| `engram analyze` | Analyze your project and show compression opportunities |
| `engram advisor` | Show optimization recommendations based on session data |
| `engram serve` | Start the compression daemon manually (advanced) |
| `engram status` | Show daemon status, active sessions, and savings |

Every command supports `--help` for detailed usage.

### Common flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.engram/engram.yaml` | Path to configuration file |
| `--socket` | `~/.engram/engram.sock` | Unix socket path |

## Integrations

### Claude Code

```bash
engram install --claude-code
# Restart Claude Code — compression is active immediately
```

Engram registers as a Claude Code plugin and routes all LLM calls through the local proxy. Context is compressed transparently — no workflow changes needed.

**Manual install** (if you prefer explicit steps):

```bash
engram install --claude-code   # installs plugin + daemon
engram status                  # confirm daemon is running
```

### OpenClaw

```bash
engram install --openclaw
```

### SDKs

For custom integrations, Engram provides thin client SDKs in three languages. All connect to the local daemon over a Unix socket.

**Python:**
```python
from engram import Engram

async with await Engram.connect() as client:
    result = await client.compress({"identity": "...", "history": [], "query": "..."})
```

**Go:**
```go
client, _ := engram.Connect(ctx, "")
defer client.Close()
result, _ := client.Compress(ctx, map[string]any{...})
```

**Node.js:**
```javascript
import { Engram } from "engram";

const client = await Engram.connect();
const result = await client.compress({identity: "...", history: [], query: "..."});
```

See the [Integration Guide](docs/integration-guide.md) for details.

## Configuration

Engram's config file lives at `~/.engram/engram.yaml` and is created automatically during install. The most common settings to adjust:

```yaml
# ~/.engram/engram.yaml
server:
  port: 4433          # daemon JSON-RPC port

proxy:
  port: 4242          # HTTP proxy port (Claude Code routes through this)
  window_size: 10     # context window compression depth
```

Run `engram serve --help` for the full list of options.

## Demo

See the [Travelbound demo project](https://github.com/pythondatascrape/travelbound) for a working example that shows Engram compressing a real project's context from ~4,200 tokens to ~380 tokens.

## Documentation

- [Getting Started](docs/getting-started.md) — Install and configure Engram for your first project
- [CLI Reference](docs/cli-reference.md) — Full command documentation with examples
- [Integration Guide](docs/integration-guide.md) — Configure Claude Code, OpenClaw, and SDK setup
- [Changelog](CHANGELOG.md) — Release history

## License

Apache 2.0 — see [LICENSE](LICENSE).
