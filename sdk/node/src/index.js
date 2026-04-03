// Thin client for the Engram compression daemon.
// Holds a persistent Unix socket connection across calls.

import { connect } from "node:net";
import { homedir } from "node:os";
import { join } from "node:path";

const DEFAULT_SOCKET = join(homedir(), ".engram", "engram.sock");

export class EngramError extends Error {
  constructor(message, code) {
    super(message);
    this.name = "EngramError";
    this.code = code;
  }
}

export class Engram {
  #conn;
  #requestId = 0;
  #pending = new Map(); // id → { resolve, reject }
  #buffer = "";

  constructor(conn) {
    this.#conn = conn;

    conn.on("data", (chunk) => {
      this.#buffer += chunk.toString();
      let idx;
      while ((idx = this.#buffer.indexOf("\n")) !== -1) {
        const line = this.#buffer.slice(0, idx);
        this.#buffer = this.#buffer.slice(idx + 1);
        this.#handleLine(line);
      }
    });

    conn.on("error", (err) => {
      for (const { reject } of this.#pending.values()) {
        reject(new EngramError(`connection error: ${err.message}`));
      }
      this.#pending.clear();
    });

    conn.on("close", () => {
      for (const { reject } of this.#pending.values()) {
        reject(new EngramError("connection closed"));
      }
      this.#pending.clear();
    });
  }

  #handleLine(line) {
    let resp;
    try {
      resp = JSON.parse(line);
    } catch {
      return;
    }
    const p = this.#pending.get(resp.id);
    if (!p) return;
    this.#pending.delete(resp.id);
    if (resp.error) {
      p.reject(new EngramError(resp.error.message, resp.error.code));
    } else {
      p.resolve(resp.result);
    }
  }

  static connect(socketPath) {
    const sock = socketPath || DEFAULT_SOCKET;
    return new Promise((resolve, reject) => {
      const conn = connect(sock);
      conn.on("connect", () => resolve(new Engram(conn)));
      conn.on("error", (err) =>
        reject(new EngramError(`daemon not reachable: ${err.message}`))
      );
    });
  }

  #call(method, params) {
    return new Promise((resolve, reject) => {
      const id = ++this.#requestId;
      this.#pending.set(id, { resolve, reject });
      const req =
        JSON.stringify({
          jsonrpc: "2.0",
          id,
          method,
          params: params ?? null,
        }) + "\n";
      this.#conn.write(req);
    });
  }

  compress(context) {
    return this.#call("engram.compress", context);
  }

  deriveCodebook(content) {
    return this.#call("engram.deriveCodebook", { content });
  }

  getStats() {
    return this.#call("engram.getStats");
  }

  checkRedundancy(content) {
    return this.#call("engram.checkRedundancy", { content });
  }

  generateReport() {
    return this.#call("engram.generateReport");
  }

  async close() {
    if (this.#conn) {
      this.#conn.destroy();
      this.#conn = null;
    }
  }
}
