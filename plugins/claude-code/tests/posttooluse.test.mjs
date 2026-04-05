import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { run } from '../hooks/posttooluse.mjs';

const LONG = 'x'.repeat(800);

function makeStdin(payload) {
  return async () => JSON.stringify(payload);
}

class MockClient {
  constructor() { this.calls = []; this.disconnected = false; }
  async call(method, params) { this.calls.push({ method, params }); }
  disconnect() { this.disconnected = true; }
}

describe('posttooluse', () => {
  it('does nothing for short output', async () => {
    const client = new MockClient();
    await run(() => client, makeStdin({ tool_name: 'bash', tool_output: 'short' }));
    assert.equal(client.calls.length, 0);
  });

  it('does nothing for own engram tools', async () => {
    const client = new MockClient();
    await run(() => client, makeStdin({ tool_name: 'mcp__engram-ccode__derive_codebook', tool_output: LONG }));
    assert.equal(client.calls.length, 0);
  });

  it('calls engram.checkRedundancy silently for large output', async () => {
    const client = new MockClient();
    await run(() => client, makeStdin({ tool_name: 'bash', tool_output: LONG }));
    assert.equal(client.calls.length, 1);
    assert.equal(client.calls[0].method, 'engram.checkRedundancy');
    assert.equal(client.calls[0].params.content, LONG);
    assert.equal(client.disconnected, true);
  });

  it('silently ignores daemon connection errors', async () => {
    class FailClient {
      async call() { throw new Error('daemon not running'); }
      disconnect() {}
    }
    // Must not throw or reject
    await run(() => new FailClient(), makeStdin({ tool_name: 'bash', tool_output: LONG }));
  });
});
