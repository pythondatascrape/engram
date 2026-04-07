# Zero-Touch Install Guidance

Goal: after the end user runs `engram install --claude-code`, Claude should work without the user manually starting `engram serve`, installing a daemon separately, or creating config files by hand.

## Direct code changes needed

### 1. Fix the daemon install call in the Claude install flow

Current code in [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go) installs `launchd` like this:

```go
if err := installLaunchd(binary, configPath, ""); err != nil {
```

That passes an empty socket path into the service template. But the service template in [cmd/engram/install.go](/Users/emeyer/Desktop/Engram/cmd/engram/install.go) always emits:

```text
serve --config <path> --socket <path>
```

So the installed service is configured with `--socket ""`, which is not a valid zero-touch runtime setup.

Direct change needed:

- Use the same default socket path that `serve` uses in [cmd/engram/serve.go](/Users/emeyer/Desktop/Engram/cmd/engram/serve.go).
- Prefer a shared helper for “default runtime paths” so install and serve cannot drift.

### 2. Ensure install creates or materializes a runnable config

The install flow points the service at `~/.engram/engram.yaml`, but `serve` requires the config file to exist:

- service install uses `~/.engram/engram.yaml` in [cmd/engram/install_plugin.go](/Users/emeyer/Desktop/Engram/cmd/engram/install_plugin.go)
- `runServe()` calls `config.Load(configPath)` in [cmd/engram/serve.go](/Users/emeyer/Desktop/Engram/cmd/engram/serve.go)
- `config.Load()` fails if the file is missing in [internal/config/config.go](/Users/emeyer/Desktop/Engram/internal/config/config.go)

I do not see install creating that file.

Direct change needed:

- During install, create `~/.engram/engram.yaml` if it does not exist.
- Write a minimal default config that is valid for proxy use.
- Keep this idempotent: do not overwrite a user-edited config unless explicitly requested.

Alternative acceptable change:

- Teach `serve` to start from in-code defaults when the config file is missing, then use that same behavior for service startup.

### 3. Make daemon/service setup part of normal Claude install on all supported OSes

Right now the Claude install flow auto-installs `launchd` only on macOS. Linux users do not get equivalent zero-touch behavior from `engram install --claude-code`.

Direct change needed:

- During Claude install, install the background service on Linux as well, not just macOS.
- Keep `engram serve --install-daemon` for explicit/manual use, but do not make it the only path to zero-touch behavior.

### 4. Start and verify the service as part of install

Even after writing plugin files and Claude settings, the current UX still leaves room for `ConnectionRefused` if the service is not actually running.

Direct change needed:

- After installing the service, start it immediately.
- Add a post-install readiness check that confirms:
  - the daemon socket exists and accepts connections
  - the proxy port from config is listening
- If readiness fails, surface a hard install error with the exact recovery step instead of printing a soft warning and leaving Claude pointed at a dead proxy.

### 5. Tighten install semantics: plugin wiring should not succeed if runtime wiring failed

Right now the install path mixes required steps and best-effort warnings. For end-user UX, that is the wrong contract. If Claude is configured to use the proxy, runtime availability is not optional.

Direct change needed:

- Treat these as a single install transaction for Claude:
  - plugin copied
  - settings merged
  - service installed
  - service started
  - readiness verified
- If any required step fails, return a non-zero install error and do not present install as complete.

### 6. Add an explicit end-user integration test

The current tests cover pieces of install behavior, but not the actual “user installs once and Claude is ready” contract.

Direct change needed:

- Add a higher-level install test that verifies:
  - `engram install --claude-code` writes plugin files
  - Claude settings include status line and proxy wiring
  - a runnable config exists at the service config path
  - the generated service definition contains a real socket path, not `""`
- If feasible in CI, add a smoke test that starts the installed service and verifies the proxy endpoint is reachable.

## Recommended implementation shape

Use a single internal installer path for “Claude zero-touch setup”, for example:

1. Resolve runtime paths:
   - config path
   - socket path
   - sessions dir
2. Ensure config exists.
3. Install plugin files.
4. Merge Claude settings.
5. Install OS service.
6. Start/reload service.
7. Verify readiness.
8. Print one success message only after verification passes.

That keeps `engram install --claude-code` aligned with what the user expects: install once, open Claude, no manual daemon steps.

## Product bar

For the customer, the expected workflow should be:

1. Run `engram install --claude-code`
2. Restart Claude
3. Start a new Claude session

There should be no separate requirement to run `engram serve`, `engram serve --install-daemon`, or hand-create `engram.yaml`.
