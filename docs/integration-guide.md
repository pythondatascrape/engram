# Integration Guide

Engram integrates with AI coding tools as a plugin. This guide covers setup for each supported tool.

## Claude Code

### Automatic Setup

```bash
engram install
```

Engram auto-detects Claude Code and registers itself as an MCP plugin. No manual configuration needed.

### Manual Setup

If auto-detection doesn't work, add Engram to your Claude Code MCP settings manually:

1. Open your Claude Code settings (`~/.claude/settings.json`)
2. Add Engram to the `mcpServers` section:

```json
{
  "mcpServers": {
    "engram": {
      "command": "engram",
      "args": ["mcp"]
    }
  }
}
```

3. Restart Claude Code

### How It Works

Once installed, Engram runs transparently. It compresses identity and context before each LLM call and decompresses responses. You don't need to change your workflow — just use Claude Code as normal and benefit from reduced token usage.

### Verifying

```bash
engram status
```

You should see an active session when Claude Code is running.

---

## OpenClaw

### Automatic Setup

```bash
engram install
```

Engram detects OpenClaw and configures the integration automatically.

### Manual Setup

Add Engram to your OpenClaw configuration:

```yaml
# ~/.openclaw/config.yaml
plugins:
  - name: engram
    command: engram
    args: ["plugin", "openclaw"]
```

### Verifying

```bash
engram status
```

---

## Python SDK

For custom integrations or programmatic access, use the Python SDK.

### Installation

```bash
pip install engram-sdk
```

### Basic Usage

```python
from engram import Engram

# Connect to the local Engram daemon
client = Engram()

# Compress context before sending to an LLM
compressed = client.compress(context)

# Get session statistics
stats = client.stats()
print(f"Tokens saved: {stats.tokens_saved}")
print(f"Savings rate: {stats.savings_rate}%")
```

### Requirements

- Python 3.10+
- Engram daemon running locally (`engram serve`)

---

## Troubleshooting

### Engram daemon not running

```bash
engram serve
```

The daemon must be running for integrations to work.

### Plugin not detected

Run `engram install` again to re-detect tools, or use the manual setup instructions above.

### High memory usage

Engram targets ~30MB resident memory. If usage is higher, check for stale sessions:

```bash
engram status --format json
```

Sessions are ephemeral and auto-evict after idle timeout.
