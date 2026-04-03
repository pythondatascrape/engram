import { describe, it, before, after } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:net";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { Engram, EngramError } from "./index.js";

function createFakeDaemon(socketPath, responses) {
  const server = createServer((conn) => {
    let buffer = "";
    conn.on("data", (chunk) => {
      buffer += chunk.toString();
      const idx = buffer.indexOf("\n");
      if (idx === -1) return;
      const line = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 1);

      const req = JSON.parse(line);
      const result = responses[req.method] ?? {};
      const resp = JSON.stringify({ jsonrpc: "2.0", id: req.id, result }) + "\n";
      conn.write(resp);
    });
  });

  return new Promise((resolve) => {
    server.listen(socketPath, () => resolve(server));
  });
}

describe("Engram Node SDK", () => {
  let tmpDir, sock, server;

  const responses = {
    "engram.compress": {
      compressed: "c:expert|t:formal",
      original_tokens: 500,
      compressed_tokens: 12,
    },
    "engram.deriveCodebook": {
      dimensions: [{ key: "expertise", type: "enum", values: ["novice", "expert"] }],
    },
    "engram.getStats": {
      sessions: 1,
      total_tokens_saved: 488,
      compression_ratio: 0.976,
    },
    "engram.checkRedundancy": {
      redundant: false,
      patterns: [],
    },
    "engram.generateReport": {
      report: "Session saved 488 tokens (97.6%)",
    },
  };

  before(async () => {
    tmpDir = await mkdtemp(join(tmpdir(), "engram-test-"));
    sock = join(tmpDir, "engram.sock");
    server = await createFakeDaemon(sock, responses);
  });

  after(async () => {
    server.close();
    await rm(tmpDir, { recursive: true });
  });

  it("compress", async () => {
    const client = await Engram.connect(sock);
    const result = await client.compress({ identity: "test", history: [], query: "hello" });
    assert.equal(result.compressed, "c:expert|t:formal");
    assert.equal(result.original_tokens, 500);
    await client.close();
  });

  it("deriveCodebook", async () => {
    const client = await Engram.connect(sock);
    const result = await client.deriveCodebook("content about expertise");
    assert.equal(result.dimensions.length, 1);
    assert.equal(result.dimensions[0].key, "expertise");
    await client.close();
  });

  it("getStats", async () => {
    const client = await Engram.connect(sock);
    const result = await client.getStats();
    assert.equal(result.total_tokens_saved, 488);
    await client.close();
  });

  it("checkRedundancy", async () => {
    const client = await Engram.connect(sock);
    const result = await client.checkRedundancy("some content");
    assert.equal(result.redundant, false);
    await client.close();
  });

  it("generateReport", async () => {
    const client = await Engram.connect(sock);
    const result = await client.generateReport();
    assert.ok(result.report.includes("488"));
    await client.close();
  });

  it("connect missing socket", async () => {
    await assert.rejects(
      () => Engram.connect("/tmp/nonexistent-engram-test.sock"),
      (err) => err instanceof EngramError
    );
  });
});
