// Thin client for the Engram compression daemon.
// Connects via Unix socket using JSON-RPC 2.0.

import { connect } from "node:net";
import { homedir } from "node:os";
import { join } from "node:path";
import { access } from "node:fs/promises";

const DEFAULT_SOCKET = join(homedir(), ".engram", "engram.sock");

export class EngramError extends Error {
  constructor(message, code) {
    super(message);
    this.name = "EngramError";
    this.code = code;
  }
}

export class Engram {
  #socketPath;
  #requestId = 0;

  constructor(socketPath) {
    this.#socketPath = socketPath;
  }

  static async connect(socketPath) {
    const sock = socketPath || DEFAULT_SOCKET;
    try {
      await access(sock);
    } catch {
      throw new EngramError(`daemon socket not found: ${sock}`);
    }
    return new Engram(sock);
  }

  async #call(method, params) {
    return new Promise((resolve, reject) => {
      const conn = connect(this.#socketPath);
      let buffer = "";

      conn.on("connect", () => {
        const req = JSON.stringify({
          jsonrpc: "2.0",
          id: ++this.#requestId,
          method,
          params: params ?? null,
        });
        conn.write(req + "\n");
      });

      conn.on("data", (chunk) => {
        buffer += chunk.toString();
        const newlineIdx = buffer.indexOf("\n");
        if (newlineIdx === -1) return;

        const line = buffer.slice(0, newlineIdx);
        conn.destroy();

        try {
          const resp = JSON.parse(line);
          if (resp.error) {
            reject(new EngramError(resp.error.message, resp.error.code));
          } else {
            resolve(resp.result);
          }
        } catch (err) {
          reject(new EngramError(`invalid response: ${err.message}`));
        }
      });

      conn.on("error", (err) => {
        reject(new EngramError(`connection failed: ${err.message}`));
      });
    });
  }

  async compress(context) {
    return this.#call("engram.compress", context);
  }

  async deriveCodebook(content) {
    return this.#call("engram.deriveCodebook", { content });
  }

  async getStats() {
    return this.#call("engram.getStats");
  }

  async checkRedundancy(content) {
    return this.#call("engram.checkRedundancy", { content });
  }

  async generateReport() {
    return this.#call("engram.generateReport");
  }

  async close() {
    // Each call opens a fresh connection, so this is a no-op.
  }
}
