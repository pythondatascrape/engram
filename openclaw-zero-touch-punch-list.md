# OpenClaw Zero-Touch Punch List

Goal: `engram install --openclaw` should leave the user in a state where they can start OpenClaw and use Engram without manually running `engram serve` or creating config files.

## Current gap

The current OpenClaw install path only copies the plugin:

- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)
- [internal/install/claude_code.go](/Users/emeyer/Desktop/Engram/internal/install/claude_code.go)

Unlike Claude, it does not currently:

- ensure default Engram config exists
- install/start the background daemon service
- verify readiness
- validate that OpenClaw can actually reach the daemon socket

## Phase 1: Match Claude runtime setup

### 1. Reuse the same runtime bootstrap steps for OpenClaw

- During `--openclaw` install:
  - ensure `~/.engram/engram.yaml` exists
  - resolve the default socket path
  - install the background service
  - verify readiness

Files:

- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)
- [cmd/engram/install.go](/Users/emeyer/Desktop/Engram/cmd/engram/install.go)
- [cmd/engram/paths.go](/Users/emeyer/Desktop/Engram/cmd/engram/paths.go)
- [cmd/engram/readiness.go](/Users/emeyer/Desktop/Engram/cmd/engram/readiness.go)

### 2. Keep install fail-closed for OpenClaw

- If plugin copy succeeds but daemon/service/readiness fails, return a non-zero install error.
- Do not print a successful install message unless the runtime is actually ready.

File:

- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)

## Phase 2: Validate OpenClaw runtime assumptions

### 3. Confirm the adapter socket path is aligned with install/runtime defaults

OpenClaw currently connects to:

- `~/.engram/engram.sock` in [plugins/openclaw/adapter.go](/Users/emeyer/Desktop/Engram/plugins/openclaw/adapter.go)

Direct change needed:

- Either keep this path and explicitly treat it as the shared daemon socket contract, or
- move the adapter to use the same shared path helper source as the CLI/runtime code

Recommended:

- Avoid duplicating the socket path contract across packages without a shared constant/helper.

### 4. Decide whether OpenClaw needs any settings/environment merge step

Claude needed settings mutation because it routes through an HTTP proxy. OpenClaw appears to speak directly to the Engram Unix socket through its adapter.

Direct work needed:

- Confirm no extra OpenClaw settings are required beyond plugin installation.
- If OpenClaw needs a registration/config hook, add it during install.
- If not, document that plugin presence plus daemon readiness is the full runtime contract.

Files:

- [plugins/openclaw/adapter.go](/Users/emeyer/Desktop/Engram/plugins/openclaw/adapter.go)
- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)

## Phase 3: Tests

### 5. Add OpenClaw install coverage equivalent to Claude’s zero-touch path

- Verify `engram install --openclaw`:
  - installs the plugin files
  - ensures config exists
  - installs/starts the daemon service
  - verifies readiness
  - exits non-zero on readiness failure

File:

- [cmd/engram/install_test.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_test.go)

### 6. Add adapter/runtime integration coverage

- Add a test that verifies the OpenClaw adapter connects to the same socket path the installer/runtime set up.
- If possible, add a smoke test that:
  - starts the daemon
  - connects the adapter
  - performs one `engram.compressIdentity` request

Files:

- [plugins/openclaw/adapter_test.go](/Users/emeyer/Desktop/Engram/plugins/openclaw/adapter_test.go)
- [cmd/engram/install_test.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_test.go)

## Definition of Done

- User runs `engram install --openclaw`
- No manual `engram serve` step is required
- No manual config creation is required
- Background daemon is installed and started automatically
- OpenClaw adapter connects to the expected socket path
- Install exits non-zero if runtime setup is incomplete
- User can start OpenClaw and immediately use Engram
