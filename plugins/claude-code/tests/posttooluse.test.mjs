import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { extractToolFields, run } from '../hooks/posttooluse.mjs';

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
  it('extracts alternate tool payload fields', () => {
    assert.deepEqual(
      extractToolFields({ toolName: 'bash', output: LONG }),
      { toolName: 'bash', toolOutput: LONG },
    );
    assert.deepEqual(
      extractToolFields({ tool: { name: 'bash', output: LONG } }),
      { toolName: 'bash', toolOutput: LONG },
    );
  });

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

  it('calls engram.checkRedundancy for alternate payload field names', async () => {
    const client = new MockClient();
    await run(() => client, makeStdin({ toolName: 'bash', output: LONG }));
    assert.equal(client.calls.length, 1);
    assert.equal(client.calls[0].method, 'engram.checkRedundancy');
    assert.equal(client.calls[0].params.content, LONG);
  });

  it('emits a guidance message when redundancy is detected', async () => {
    class RedundantClient extends MockClient {
      async call(method, params) {
        this.calls.push({ method, params });
        return { isRedundant: true, kind: 'normalized' };
      }
    }

    const client = new RedundantClient();
    const origWrite = process.stdout.write;
    let output = '';
    process.stdout.write = (chunk) => {
      output += String(chunk);
      return true;
    };

    try {
      await run(() => client, makeStdin({ tool_name: 'bash', tool_output: LONG }));
    } finally {
      process.stdout.write = origWrite;
    }

    const parsed = JSON.parse(output.trim());
    assert.match(parsed.message, /redundancy check detected normalized tool output/);
  });

  it('silently ignores daemon connection errors', async () => {
    class FailClient {
      async call() { throw new Error('daemon not running'); }
      disconnect() {}
    }
    // Must not throw or reject
    await run(() => new FailClient(), makeStdin({ tool_name: 'bash', tool_output: LONG }));
  });

  it('does nothing when tool_output is missing', async () => {
    const client = new MockClient();
    await run(() => client, makeStdin({ tool_name: 'bash' }));
    assert.equal(client.calls.length, 0);
  });

  it('does nothing when stdin is empty', async () => {
    const client = new MockClient();
    await run(() => client, async () => '');
    assert.equal(client.calls.length, 0);
  });

  it('does nothing when stdin is malformed JSON', async () => {
    const client = new MockClient();
    await run(() => client, async () => 'not json');
    assert.equal(client.calls.length, 0);
  });
});
