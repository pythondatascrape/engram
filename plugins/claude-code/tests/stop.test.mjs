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

describe('stop hook — context tracking', () => {
  it('writes ctx_orig and ctx_comp to session file on first turn', async () => {
    const home = makeFakeHome();
    const projectDir = setupProject(home);

    await runHook({
      session_id: 'sess-001',
      context_window: { total_input_tokens: 3000 },
    }, home, projectDir);

    const data = JSON.parse(readFileSync(join(home, '.engram', 'sessions', 'sess-001.json'), 'utf8'));
    // ctx_comp = actual input tokens
    assert.equal(data.ctx_comp, 3000);
    // ctx_orig = actual + identity saved this turn (3000 + saved_per_call * 1)
    assert.ok(data.ctx_orig > data.ctx_comp, 'ctx_orig should exceed ctx_comp');
  });

  it('accumulates ctx_orig and ctx_comp across multiple turns', async () => {
    const home = makeFakeHome();
    const projectDir = setupProject(home);

    // Turn 1
    await runHook({ session_id: 'sess-002', context_window: { total_input_tokens: 1000 } }, home, projectDir);
    // Turn 2 — context window grew
    await runHook({ session_id: 'sess-002', context_window: { total_input_tokens: 2000 } }, home, projectDir);

    const data = JSON.parse(readFileSync(join(home, '.engram', 'sessions', 'sess-002.json'), 'utf8'));
    assert.equal(data.ctx_comp, 2000, 'ctx_comp should reflect latest total_input_tokens');
    assert.ok(data.ctx_orig > 2000, 'ctx_orig should be latest input + cumulative identity savings');
  });

  it('ctx_orig and ctx_comp absent when no context_window in payload', async () => {
    const home = makeFakeHome();
    const projectDir = setupProject(home);

    await runHook({ session_id: 'sess-003' }, home, projectDir);

    const data = JSON.parse(readFileSync(join(home, '.engram', 'sessions', 'sess-003.json'), 'utf8'));
    // When total_input_tokens is absent, ctx fields should be absent or zero — no crash
    assert.ok(!data.ctx_orig || data.ctx_orig === 0, 'no ctx data without context_window');
  });
});
