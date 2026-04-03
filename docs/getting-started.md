# Getting Started with Engram

This guide walks you through installing Engram and setting it up for your first project.

## Installation

### Homebrew (macOS)

```bash
brew install engram
```

### From Source

```bash
git clone https://github.com/erikmeyer/engram.git
cd engram
go build -o engram ./cmd/engram
sudo mv engram /usr/local/bin/
```

### Verify Installation

```bash
engram --version
```

## First Project Setup

Navigate to your project directory and run the interactive installer:

```bash
cd your-project
engram install
```

Engram will:
1. Scan your project structure
2. Detect installed AI tools (Claude Code, OpenClaw, etc.)
3. Build an initial compression profile
4. Register itself as a plugin for detected tools

## Analyze Your Project

See what Engram found and how much it can compress:

```bash
engram analyze
```

Sample output:
```
Project: my-app
Files scanned: 147
Identity tokens (uncompressed): 4,218
Identity tokens (compressed): 162
Compression ratio: 96.2%

Context patterns detected: 12
Estimated session savings: 87-91%
```

## Start the Daemon

Run Engram as a background daemon to compress context in real time:

```bash
engram serve
```

Or run in the foreground for debugging:

```bash
engram serve --foreground
```

## Verify It's Working

Check the daemon status and active sessions:

```bash
engram status
```

Sample output:
```
Engram daemon: running (PID 48291)
Uptime: 2h 14m
Active sessions: 1
Total tokens saved: 34,892
Current savings rate: 89.3%
```

## Next Steps

- [CLI Reference](cli-reference.md) — Full command documentation
- [Integration Guide](integration-guide.md) — Configure specific tool integrations
