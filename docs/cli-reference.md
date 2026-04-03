# CLI Reference

Engram provides a single binary with subcommands for setup, analysis, and operation.

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Path to configuration file (default: `engram.yaml`) |
| `--socket <path>` | Unix socket path (default: `~/.engram/engram.sock`) |
| `-v, --version` | Print version and exit |
| `-h, --help` | Show help text |

---

## engram install

Auto-detects installed AI clients and installs the Engram compression plugin.

```bash
engram install [flags]
```

| Flag | Description |
|------|-------------|
| `--claude-code` | Install Claude Code plugin only |
| `--openclaw` | Install OpenClaw plugin only |
| `--source <dir>` | Override plugin source directory |

**What it does:**
- Detects installed AI coding tools (Claude Code, OpenClaw)
- Registers Engram as a plugin for detected tools
- Copies plugin files to the appropriate configuration directory

---

## engram analyze

Analyze your project and show compression opportunities with estimated savings.

```bash
engram analyze [directory] [flags]
```

| Flag | Description |
|------|-------------|
| `--sessions <n>` | Override sessions per day (default: 50) |
| `--cost <n>` | Override cost per million input tokens (default: $3.00) |
| `-y, --yes` | Auto-accept all optimizations |

**What it does:**
- Scans project files for identity files (CLAUDE.md, AGENTS.md, etc.)
- Estimates token savings from Engram compression
- Presents a prioritized report with monthly dollar savings

---

## engram advisor

Show optimization recommendations based on actual session data.

```bash
engram advisor [directory]
```

**What it does:**
- Reads accumulated session data for a project
- Analyzes compression patterns and efficiency
- Shows actionable recommendations for improving savings

---

## engram serve

Start the Engram compression daemon.

```bash
engram serve [flags]
```

| Flag | Description |
|------|-------------|
| `--socket <path>` | Unix socket path (default: `~/.engram/engram.sock`) |
| `--install-daemon` | Install as a system daemon (launchd on macOS, systemd on Linux) |

**What it does:**
- Starts the compression daemon
- Listens on a Unix socket for JSON-RPC requests
- Compresses context in real time for active sessions

---

## engram status

Show daemon status and session information.

```bash
engram status [flags]
```

| Flag | Description |
|------|-------------|
| `--socket <path>` | Unix socket path (default: `~/.engram/engram.sock`) |

**What it does:**
- Shows whether the daemon is running
- Displays active session count and uptime
- Reports total tokens saved and current savings rate
