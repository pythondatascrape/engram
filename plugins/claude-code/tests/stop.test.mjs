// plugins/claude-code/tests/stop.test.mjs
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

// Isolate HOME so all file writes go to a temp dir.
function makeFakeHome() {
  const dir = join(tmpdir(), `engram-test-${Date.now()}-${Math.random().toString(36).slice(2)}`);
  mkdirSync(dir, { recursive: true });
  return dir;
}

// Minimal CLAUDE.md fixture so the hook can derive orig_per_call.
function setupProject(home) {
  const projectDir = join(home, 'project');
  mkdirSync(projectDir, { recursive: true });
  // 400 chars → 100 tokens
  writeFileSync(join(projectDir, 'CLAUDE.md'), 'x'.repeat(400));
  return projectDir;
}

async function runHook(payload, fakeHome, projectDir) {
  // Isolation relies on homedir() being called inside run() at invocation time,
  // not at module load time. The ?bust param prevents the module from being
  // shared across test files in the same process, but HOME isolation works
  // because homedir() reads process.env.HOME lazily on each call.
  const origHome = process.env.HOME;
  const origDir  = process.env.CLAUDE_PROJECT_DIR;
  process.env.HOME = fakeHome;
  process.env.CLAUDE_PROJECT_DIR = projectDir;
  try {
    const { run } = await import(`../hooks/stop.mjs?bust=${Date.now()}`);
    await run(async () => JSON.stringify(payload));
  } finally {
    process.env.HOME = origHome;
    if (origDir === undefined) delete process.env.CLAUDE_PROJECT_DIR;
    else process.env.CLAUDE_PROJECT_DIR = origDir;
  }
}

// Tests removed: context_window.total_input_tokens is never populated in stop hook payloads.
// The ctx_comp/ctx_orig tracking has been moved to the proxy (.ctx.json file).
