package main

// Help text for CLI subcommands.
// These are wired into cobra.Command.Long fields during CLI init.

const helpRoot = `Engram — local-first context compression for AI coding tools.

Engram runs as a lightweight daemon that compresses redundant context
out of every LLM call, saving 85-93% of tokens per session.

Usage:
  engram <command> [flags]

Commands:
  install    Set up Engram for your project
  analyze    Analyze your project and show compression opportunities
  serve      Start the compression daemon
  status     Show daemon status and session information
  update     Update codebooks and configuration

Use "engram <command> --help" for more information about a command.`

const helpInstall = `Set up Engram for your project.

Scans your project directory, detects installed AI tools (Claude Code,
OpenClaw), builds an initial compression profile, and registers Engram
as a plugin.

Usage:
  engram install [flags]

Flags:
  --non-interactive   Skip prompts, use defaults
  --tools <list>      Comma-separated tools to configure (claude-code,openclaw)`

const helpAnalyze = `Analyze your project and show compression opportunities.

Scans project files, calculates identity token counts (compressed vs.
uncompressed), identifies context patterns, and reports estimated
savings per session.

Usage:
  engram analyze [flags]

Flags:
  --format <fmt>   Output format: text (default), json
  --verbose        Show per-file breakdown`

const helpServe = `Start the Engram compression daemon.

Listens for connections from AI tools and compresses context in real
time for active sessions.

Usage:
  engram serve [flags]

Flags:
  --foreground     Run in foreground (don't daemonize)
  --port <port>    Listen port (default: 7600)
  --config <path>  Path to config file`

const helpStatus = `Show daemon status and session information.

Displays whether the daemon is running, active session count, total
tokens saved, and current savings rate.

Usage:
  engram status [flags]

Flags:
  --format <fmt>   Output format: text (default), json`

const helpUpdate = `Update codebooks and configuration.

Re-scans the project for changes and updates codebooks to reflect
the current project structure.

Usage:
  engram update [flags]

Flags:
  --codebooks   Update codebooks only
  --config      Regenerate config from current project state`
