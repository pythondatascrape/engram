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

**Claude Code Skills**

| Skill | Invocation | Description |
|-------|-----------|-------------|
| Codebook Manager | `/engram:codebook` | Show, diff, init, or validate your active codebook |
| Codebook Wizard | `/engram:engram-codebook` | Interactive wizard — builds identity + prompt codebook from scratch, scoped to session, project, or global |
| Token Report | `/engram:report` | Generate a session token savings report |
| Settings | `/engram:config` | Edit plugin configuration interactively |

The **Codebook Wizard** (`/engram:engram-codebook`) is the fastest way to wire up compression for a new project. It reads your `CLAUDE.md`, derives identity dimensions, builds a prompt vocabulary from your project's domain language, and writes a `.engram-codebook.yaml` + `engram.yaml` in two minutes. Behavioral dimensions that `derive_codebook` misses from prose-style CLAUDE.md files are pre-populated and reviewable before writing.

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

## Best Use

Engram's identity compression is only as good as the codebook it works from. A well-structured codebook can reduce identity context from thousands of tokens to a handful of key=value pairs. This section shows how to maximize that savings — and how to wire it into your LLM calls.

### Why This Matters

Every LLM call re-sends your identity: who you are, what you prefer, how you work. Written as prose, that identity is verbose by design — natural language is redundant. Engram replaces prose with a derived codebook so the same information travels in a fraction of the tokens.

| Format | Example | ~Tokens |
|--------|---------|---------|
| Plain prose | `"I prefer concise responses without summaries. I'm a senior Go engineer who has been writing Go for ten years. I'm working on a CLI tool targeting macOS and Linux."` | ~45 |
| Codebook (key=value) | `role=engineer lang=go exp=10y platform=macos,linux style=concise no_summaries=true` | ~14 |
| **Savings** | | **~69%** |

At scale — a full CLAUDE.md, system prompt, and project instructions — this compression reaches **96–98%**. That headroom goes directly toward more productive context: longer conversation history, bigger diffs, more tool results.

### How Codebooks Work

Engram's `derive_codebook` MCP tool scans your CLAUDE.md and project instructions, extracts structured dimensions, and produces a compact codebook. On the first turn, the codebook definition is sent once so the model understands the format. Every subsequent turn references keys only.

```
# First turn — definition sent once
codebook: role(enum:engineer,researcher) lang(text) style(enum:concise,verbose) no_summaries(bool)

# All subsequent turns — keys only
role=engineer lang=go style=concise no_summaries=true
```

You can also define codebooks manually in YAML for domain-specific applications:

```yaml
# ~/.engram-codebook.yaml
name: my-project
version: 1
dimensions:
  - name: role
    type: enum
    values: [engineer, researcher, designer]
    required: true

  - name: lang
    type: text
    required: true

  - name: style
    type: enum
    values: [concise, verbose]
    required: false

  - name: no_summaries
    type: boolean
    required: false
```

Pass the override path to `derive_codebook` and it merges your explicit dimensions with anything auto-detected:

```python
result = await client.compress({
    "identity": open("CLAUDE.md").read(),
    "yamlOverridePath": ".engram-codebook.yaml",
    ...
})
```

### Maximizing Your Codebook

**1. Make dimensions specific and bounded.** Enum and boolean fields compress best — they map to a single token. Free-text fields still compress, but less aggressively.

```yaml
# Good — bounded enum
- name: experience
  type: enum
  values: [junior, mid, senior, staff]

# Less efficient — open text
- name: experience
  type: text
```

**2. Prefer `required: true` for your most-repeated attributes.** Required dimensions are always included in the serialized output and always stripped from subsequent prose. Optional ones are only emitted when set.

**3. Use `check_redundancy` before sending responses.** The MCP tool flags prose that re-states what the codebook already encodes, so you can strip it before it enters context.

```python
# In a PostToolUse hook or response filter
check = await mcp.check_redundancy(content=llm_response)
if check["redundant"]:
    llm_response = check["stripped"]  # codebook entries removed
```

**4. Run `engram analyze` after your first session.** It reads session data and suggests dimensions you haven't captured yet — common repeated phrases that are good codebook candidates.

```bash
engram analyze
# → Suggested dimensions: deadline_pressure (bool), test_style (enum: tdd,bdd,none)
```

### Updating LLM Calls to Use Engram

For transparent proxy use (Claude Code, OpenClaw), no code changes are needed — all calls route through Engram automatically after `engram install`.

For SDK use, replace the direct Anthropic/OpenAI base URL with the local proxy:

**Before:**
```python
import anthropic
client = anthropic.Anthropic(api_key="...")

response = client.messages.create(
    model="claude-sonnet-4-6",
    system="You are a concise senior Go engineer. I prefer no trailing summaries. "
           "I'm working on a macOS/Linux CLI. My style preference is direct and brief...",
    messages=[{"role": "user", "content": "..."}]
)
```

**After (proxy):**
```python
import anthropic
import httpx

# Route through Engram's local proxy — compression is transparent
client = anthropic.Anthropic(
    api_key="...",
    http_client=httpx.Client(proxies="http://localhost:4242")
)

# Same call — Engram compresses identity and context before forwarding
response = client.messages.create(
    model="claude-sonnet-4-6",
    system="You are a concise senior Go engineer...",  # compressed automatically
    messages=[{"role": "user", "content": "..."}]
)
```

**After (MCP tools, explicit):**
```python
from engram import Engram

async with await Engram.connect() as client:
    # Derive codebook from your identity text once per session
    codebook = await client.derive_codebook(content=open("CLAUDE.md").read())

    # Compress identity — use the returned block as your system prompt prefix
    compressed = await client.compress_identity()
    # compressed["block"] → "role=engineer lang=go style=concise no_summaries=true"

    response = anthropic_client.messages.create(
        model="claude-sonnet-4-6",
        system=compressed["block"],  # ~14 tokens instead of ~450
        messages=[{"role": "user", "content": "..."}]
    )
```

### Plain-Text vs. Structured Context: A Comparison

| Attribute | Plain-Text CLAUDE.md | Engram Codebook |
|-----------|---------------------|-----------------|
| Format | Natural language prose | `key=value` pairs |
| Tokens (typical) | 300–800 | 10–30 |
| Sent every turn | Yes | Keys only (definition sent once) |
| LLM interpretability | High (verbose) | High (structured, unambiguous) |
| Redundancy detection | None | `check_redundancy` strips re-stated values |
| Manual tuning | Edit CLAUDE.md | Edit `.engram-codebook.yaml` |
| Auto-derived | — | `derive_codebook` extracts from CLAUDE.md |
| Identity compression | — | **~96–98%** |

The key insight: plain-text identity is high-fidelity but pays a large per-turn token tax. A codebook pays a one-time definition cost and then transmits identity in a handful of tokens for every turn that follows. On a 50-turn session, a 400-token identity compressed to 15 tokens saves ~19,000 tokens — roughly equivalent to adding 15 pages of usable context back to your window.

## License

Apache 2.0 — see [LICENSE](LICENSE).
