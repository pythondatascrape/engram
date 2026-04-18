import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { buildCompressedIdentity, extractSessionId, run } from '../hooks/sessionstart.mjs';

function makeProjectWithClaudeMd() {
  const dir = join(tmpdir(), `engram-sessionstart-${Date.now()}-${Math.random().toString(36).slice(2)}`);
  mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, 'CLAUDE.md'), 'project identity content');
  return dir;
}

describe('sessionstart', () => {
  it('extracts session_id', () => {
    assert.equal(extractSessionId({ session_id: 'uuid-1' }), 'uuid-1');
  });

  it('extracts sessionId', () => {
    assert.equal(extractSessionId({ sessionId: 'uuid-2' }), 'uuid-2');
  });

  it('extracts nested session.id', () => {
    assert.equal(extractSessionId({ session: { id: 'uuid-3' } }), 'uuid-3');
  });

  it('returns empty string when session id is missing', () => {
    assert.equal(extractSessionId({ foo: 'bar' }), '');
  });

  it('builds compressed identity directly through the daemon client', async () => {
    class MockClient {
      constructor() { this.calls = []; }
      async call(method, params) {
        this.calls.push({ method, params });
        if (method === 'engram.deriveCodebook') {
          return { codebook: { lang: 'go', arch: 'monolith' } };
        }
        if (method === 'engram.compressIdentity') {
          return { block: '[identity]\nlang=go arch=monolith\n[/identity]' };
        }
        throw new Error(`unexpected method ${method}`);
      }
      disconnect() {}
    }

    const client = new MockClient();
    const block = await buildCompressedIdentity(() => client, 'identity prose');
    assert.equal(block, '[identity]\nlang=go arch=monolith\n[/identity]');
    assert.equal(client.calls.length, 2);
    assert.equal(client.calls[0].method, 'engram.deriveCodebook');
    assert.equal(client.calls[1].method, 'engram.compressIdentity');
  });

  it('registers the session and emits a derive/compress prompt when CLAUDE.md exists', async () => {
    const projectDir = makeProjectWithClaudeMd();
    const origProjectDir = process.env.CLAUDE_PROJECT_DIR;
    process.env.CLAUDE_PROJECT_DIR = projectDir;

    const registered = [];
    class MockClient {
      async call(method) {
        if (method === 'engram.deriveCodebook') {
          return { codebook: { lang: 'go', arch: 'monolith' } };
        }
        if (method === 'engram.compressIdentity') {
          return { block: '[identity]\nlang=go arch=monolith\n[/identity]' };
        }
        throw new Error(`unexpected method ${method}`);
      }
      disconnect() {}
    }
    const origWrite = process.stdout.write;
    let output = '';
    process.stdout.write = (chunk) => {
      output += String(chunk);
      return true;
    };

    try {
      await run(
        async () => JSON.stringify({ session_id: 'uuid-4' }),
        async (sessionId) => { registered.push(sessionId); },
        undefined,
        () => new MockClient(),
      );
    } finally {
      process.stdout.write = origWrite;
      if (origProjectDir === undefined) delete process.env.CLAUDE_PROJECT_DIR;
      else process.env.CLAUDE_PROJECT_DIR = origProjectDir;
    }

    assert.deepEqual(registered, ['uuid-4']);
    const parsed = JSON.parse(output.trim());
    assert.match(parsed.message, /directly compressed the project identity/);
    assert.match(parsed.message, /\[identity\]/);
  });

  it('registers the session even when no CLAUDE.md files exist', async () => {
    const projectDir = join(tmpdir(), `engram-sessionstart-empty-${Date.now()}-${Math.random().toString(36).slice(2)}`);
    mkdirSync(projectDir, { recursive: true });
    const origProjectDir = process.env.CLAUDE_PROJECT_DIR;
    process.env.CLAUDE_PROJECT_DIR = projectDir;

    const registered = [];
    try {
      await run(
        async () => JSON.stringify({ sessionId: 'uuid-5' }),
        async (sessionId) => { registered.push(sessionId); },
        undefined,
        () => ({ disconnect() {} }),
      );
    } finally {
      if (origProjectDir === undefined) delete process.env.CLAUDE_PROJECT_DIR;
      else process.env.CLAUDE_PROJECT_DIR = origProjectDir;
    }

    assert.deepEqual(registered, ['uuid-5']);
  });

  it('falls back to a non-fatal message when direct compression fails', async () => {
    const projectDir = makeProjectWithClaudeMd();
    const origProjectDir = process.env.CLAUDE_PROJECT_DIR;
    process.env.CLAUDE_PROJECT_DIR = projectDir;

    const origWrite = process.stdout.write;
    let output = '';
    process.stdout.write = (chunk) => {
      output += String(chunk);
      return true;
    };

    try {
      await run(
        async () => JSON.stringify({ session_id: 'uuid-6' }),
        async () => {},
        undefined,
        () => ({
          async call() { throw new Error('daemon not running'); },
          disconnect() {},
        }),
      );
    } finally {
      process.stdout.write = origWrite;
      if (origProjectDir === undefined) delete process.env.CLAUDE_PROJECT_DIR;
      else process.env.CLAUDE_PROJECT_DIR = origProjectDir;
    }

    const parsed = JSON.parse(output.trim());
    assert.match(parsed.message, /could not directly compress identity/);
  });
});
