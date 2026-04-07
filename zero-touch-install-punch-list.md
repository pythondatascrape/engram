# Zero-Touch Install Punch List

Goal: `engram install --claude-code` should leave the user in a state where they can restart Claude, open a new session, and use Engram without any manual daemon or config steps.

## Phase 1: Shared runtime paths

### 1. Add shared path helpers

- Add helpers for:
  - default config path
  - default socket path
  - default sessions dir
- Use them from both install and serve code.

Files:

- [cmd/engram/serve.go](/Users/emeyer/Desktop/Engram/cmd/engram/serve.go)
- [cmd/engram/install.go](/Users/emeyer/Desktop/Engram/cmd/engram/install.go)
- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)

Tests:

- Add unit tests for path resolution.
- Confirm install and serve use the same socket/config defaults.

## Phase 2: Runnable default config

### 2. Ensure config exists at install time

- Add an install helper that creates `~/.engram/engram.yaml` if missing.
- Write a minimal valid config using current defaults.
- Preserve existing config when present.

Files:

- [internal/config/config.go](/Users/emeyer/Desktop/Engram/internal/config/config.go)
- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)

Tests:

- creates config when missing
- leaves existing config unchanged
- written config loads successfully via `config.Load`

## Phase 3: Correct daemon/service installation

### 3. Fix service install arguments

- Stop passing an empty socket path into `installLaunchd(...)`.
- Use the shared default socket path.
- Ensure generated service definitions always contain valid config and socket paths.

Files:

- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)
- [cmd/engram/install.go](/Users/emeyer/Desktop/Engram/cmd/engram/install.go)

Tests:

- launchd plist contains non-empty socket path
- systemd unit contains non-empty socket path

### 4. Install background service for Claude on all supported OSes

- Keep the existing macOS `launchd` path.
- Add equivalent Linux behavior in the Claude install flow.
- Treat unsupported OSes explicitly.

Files:

- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)
- [cmd/engram/install.go](/Users/emeyer/Desktop/Engram/cmd/engram/install.go)

Tests:

- macOS path triggers launchd install logic
- Linux path triggers systemd install logic

## Phase 4: Start and verify runtime

### 5. Start the service during install

- After service registration, start or reload the service immediately.
- Do not require the user to run `engram serve` separately.

Files:

- [cmd/engram/install.go](/Users/emeyer/Desktop/Engram/cmd/engram/install.go)
- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)

Tests:

- install flow invokes service start path
- failures are surfaced back to the caller

### 6. Add readiness verification

- Add a post-install verification helper that checks:
  - daemon socket is reachable
  - proxy port is listening
- Fail install if readiness does not pass.

Files:

- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)
- [cmd/engram/serve.go](/Users/emeyer/Desktop/Engram/cmd/engram/serve.go)

Tests:

- readiness succeeds when socket and proxy are available
- readiness failure returns a hard install error

## Phase 5: Transactional install behavior

### 7. Make Claude install fail closed

- Treat these as required:
  - plugin copy
  - settings merge
  - config creation
  - service install
  - service start
  - readiness verification
- Remove “success” output when required runtime setup failed.
- Keep warnings only for genuinely optional steps.

Files:

- [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)

Tests:

- install returns error when service setup fails
- install returns error when readiness check fails
- success message appears only on full success

## Phase 6: End-user verification

### 8. Add end-to-end install coverage

- Add a higher-level install test that verifies:
  - Claude plugin installed
  - `settings.json` updated
  - config file created
  - service definition written with valid paths
- If practical, add a smoke test for:
  - installed service start
  - proxy reachable on configured port

Files:

- [cmd/engram/install_test.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_test.go)

## Definition of Done

- User runs `engram install --claude-code`
- No manual `engram serve` step is required
- No manual `engram serve --install-daemon` step is required
- No manual config-file creation is required
- Claude settings point to a live Engram proxy
- Service is installed and started automatically
- Install exits non-zero if runtime setup is incomplete
- User can restart Claude and immediately use a new session
