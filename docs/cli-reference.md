# CLI Reference

Engram provides a single binary with subcommands for setup, analysis, and operation.

## Global Flags

| Flag | Description |
|------|-------------|
| `--config <path>` | Path to configuration file (default: `engram.yaml`) |
| `--version` | Print version and exit |
| `--help` | Show help text |

---

## engram install

Interactive setup wizard for your project.

```bash
engram install [flags]
```

| Flag | Description |
|------|-------------|
| `--non-interactive` | Skip prompts, use defaults |
| `--tools <list>` | Comma-separated list of tools to configure (e.g., `claude-code,openclaw`) |

**What it does:**
- Scans the current directory for project structure
- Detects installed AI coding tools
- Generates an initial identity codebook
- Registers Engram as a plugin for detected tools
- Creates `engram.yaml` if it doesn't exist

---

## engram analyze

Analyze your project and show compression opportunities.

```bash
engram analyze [flags]
```

| Flag | Description |
|------|-------------|
| `--format <fmt>` | Output format: `text` (default), `json` |
| `--verbose` | Show per-file breakdown |

**What it does:**
- Scans project files and structure
- Calculates identity token counts (compressed vs. uncompressed)
- Identifies context patterns and redundancy
- Reports estimated savings per session

---

## engram serve

Start the Engram compression daemon.

```bash
engram serve [flags]
```

| Flag | Description |
|------|-------------|
| `--foreground` | Run in foreground (don't daemonize) |
| `--port <port>` | Listen port (default: 7600) |
| `--config <path>` | Path to config file |

**What it does:**
- Starts the compression daemon
- Listens for connections from AI tools
- Compresses context in real time for active sessions

---

## engram status

Show daemon status and session information.

```bash
engram status [flags]
```

| Flag | Description |
|------|-------------|
| `--format <fmt>` | Output format: `text` (default), `json` |

**What it does:**
- Shows whether the daemon is running
- Displays active session count
- Reports total tokens saved and current savings rate

---

## engram update

Update codebooks and configuration.

```bash
engram update [flags]
```

| Flag | Description |
|------|-------------|
| `--codebooks` | Update codebooks only |
| `--config` | Regenerate config from current project state |

**What it does:**
- Re-scans the project for changes
- Updates codebooks to reflect current project structure
- Applies any configuration updates
