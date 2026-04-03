import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:net';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

import { DaemonClient } from '../lib/daemon-client.mjs';

describe('DaemonClient', () => {
  let mockServer;
  let socketPath;
  let tempDir;

  beforeEach((_, done) => {
    tempDir = mkdtempSync(join(tmpdir(), 'engram-test-'));
    socketPath = join(tempDir, 'test.sock');

    mockServer = createServer((conn) => {
      let buffer = '';
      conn.on('data', (chunk) => {
        buffer += chunk.toString();
        const lines = buffer.split('\n');
        buffer = lines.pop();

        for (const line of lines) {
          if (!line.trim()) continue;
          const req = JSON.parse(line);
          const resp = {
            jsonrpc: '2.0',
            id: req.id,
            result: { method: req.method, echo: true },
          };
          conn.write(JSON.stringify(resp) + '\n');
        }
      });
    });

    mockServer.listen(socketPath, done);
  });

  afterEach(() => {
    mockServer?.close();
    try {
      rmSync(tempDir, { recursive: true });
    } catch {
      // ignore
    }
  });

  it('should connect to a Unix socket', async () => {
    const client = new DaemonClient(socketPath);
    await client.connect();
    assert.equal(client.connected, true);
    client.disconnect();
  });

  it('should send JSON-RPC calls and receive responses', async () => {
    const client = new DaemonClient(socketPath);
    await client.connect();

    const result = await client.call('engram.getStats', {});
    assert.equal(result.method, 'engram.getStats');
    assert.equal(result.echo, true);

    client.disconnect();
  });

  it('should handle multiple concurrent calls', async () => {
    const client = new DaemonClient(socketPath);
    await client.connect();

    const results = await Promise.all([
      client.call('engram.getStats', {}),
      client.call('engram.deriveCodebook', { content: 'test' }),
      client.call('engram.checkRedundancy', { content: 'test' }),
    ]);

    assert.equal(results.length, 3);
    assert.equal(results[0].method, 'engram.getStats');
    assert.equal(results[1].method, 'engram.deriveCodebook');
    assert.equal(results[2].method, 'engram.checkRedundancy');

    client.disconnect();
  });

  it('should report disconnected after disconnect', async () => {
    const client = new DaemonClient(socketPath);
    await client.connect();
    assert.equal(client.connected, true);
    client.disconnect();
    assert.equal(client.connected, false);
  });

  it('should fail to connect to nonexistent socket', async () => {
    const client = new DaemonClient('/tmp/nonexistent-engram-test.sock');
    await assert.rejects(() => client.connect(), /ENOENT|ECONNREFUSED|timed out/);
  });
});
