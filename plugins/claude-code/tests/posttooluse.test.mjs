import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { extractToolFields, summarizeToolOutput, run } from '../hooks/posttooluse.mjs';

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
    assert.match(parsed.content, /redundancy check detected normalized tool output/);
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

describe('summarizeToolOutput', () => {
  it('truncates long bash output with head+tail', () => {
    const lines = Array.from({ length: 100 }, (_, i) => `line ${i}`);
    const input = lines.join('\n');
    const result = summarizeToolOutput('bash', input);
    assert.ok(result.length < input.length);
    assert.match(result, /\[20 lines omitted\]/);
    assert.match(result, /line 0/);
    assert.match(result, /line 99/);
  });

  it('summarizes a large JSON object', () => {
    const obj = Object.fromEntries(Array.from({ length: 30 }, (_, i) => [`key${i}`, i]));
    const input = JSON.stringify(obj);
    const result = summarizeToolOutput('bash', input);
    assert.ok(result.length < input.length);
    assert.match(result, /key0:number/);
    assert.match(result, /\+10 keys/);
  });

  it('summarizes a large JSON array', () => {
    const arr = Array.from({ length: 50 }, (_, i) => ({ id: i, name: `n${i}` }));
    const input = JSON.stringify(arr);
    const result = summarizeToolOutput('bash', input);
    assert.ok(result.length < input.length);
    assert.match(result, /Array\(50\)/);
  });

  it('returns single-line summary for TodoWrite tool', () => {
    const input = Array.from({ length: 20 }, (_, i) => `todo item ${i}`).join('\n');
    const result = summarizeToolOutput('TodoWrite', input);
    assert.ok(!result.includes('\n') || result.split('\n').length <= 3);
    assert.ok(result.length < input.length);
  });

  it('returns original when already short enough', () => {
    const input = 'just a few words';
    assert.equal(summarizeToolOutput('bash', input), input);
  });

  it('emits truncation message when summarization fires', async () => {
    const longLines = Array.from({ length: 100 }, (_, i) => `line ${String(i).padStart(4, '0')}: ${'a'.repeat(20)}`).join('\n');
    assert.ok(longLines.length > 800, 'test input must exceed MIN_CHARS_TO_SUMMARIZE');
    const client = new MockClient();
    const origWrite = process.stdout.write;
    let output = '';
    process.stdout.write = (chunk) => { output += String(chunk); return true; };

    try {
      await run(() => client, makeStdin({ tool_name: 'Bash', tool_output: longLines }));
    } finally {
      process.stdout.write = origWrite;
    }

    assert.equal(client.calls.length, 0, 'daemon should not be called when local truncation fires');
    const parsed = JSON.parse(output.trim());
    assert.match(parsed.content, /\[Engram\] Tool output truncated/);
    assert.match(parsed.content, /lines omitted/);
  });
});
