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

## SDKs

All SDKs are thin JSON-RPC clients that connect to the local Engram daemon over a Unix socket. The daemon must be running (`engram serve`).

### Python

**Requirements:** Python 3.11+, Engram daemon running

```bash
pip install engram
```

```python
from engram import Engram

async with await Engram.connect() as client:
    # Compress context before sending to an LLM
    result = await client.compress({
        "identity": "...",
        "history": [],
        "query": "hello",
    })

    # Get session statistics
    stats = await client.get_stats()
    print(f"Tokens saved: {stats['total_tokens_saved']}")
```

### Go

**Requirements:** Go 1.22+, Engram daemon running

```go
import engram "github.com/pythondatascrape/engram/sdk/go"

client, err := engram.Connect(ctx, "") // empty string = default socket
if err != nil { log.Fatal(err) }
defer client.Close()

result, err := client.Compress(ctx, map[string]any{
    "identity": "...",
    "history":  []any{},
    "query":    "hello",
})

stats, err := client.GetStats(ctx)
```

### Node.js

**Requirements:** Node.js 18+, Engram daemon running

```bash
npm install engram
```

```javascript
import { Engram } from "engram";

const client = await Engram.connect();

const result = await client.compress({
    identity: "...",
    history: [],
    query: "hello",
});

const stats = await client.getStats();
console.log(`Tokens saved: ${stats.total_tokens_saved}`);

await client.close();
```

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
