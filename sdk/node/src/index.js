// Thin client for the Engram compression daemon.
// Maintains a pool of persistent Unix socket connections for concurrent calls.

import { connect as netConnect } from "node:net";
import { homedir } from "node:os";
import { join } from "node:path";

const DEFAULT_SOCKET = join(homedir(), ".engram", "engram.sock");
const DEFAULT_POOL_SIZE = 4;

export class EngramError extends Error {
  constructor(message, code) {
    super(message);
    this.name = "EngramError";
    this.code = code;
  }
}

/** A single persistent daemon connection with newline-delimited JSON-RPC. */
class Conn {
  #sock;
  #buffer = "";
  #pending = new Map(); // id → { resolve, reject }
  #idCounter = 0;
  #dead = false;

  constructor(sock) {
    this.#sock = sock;

    sock.on("data", (chunk) => {
      this.#buffer += chunk.toString();
      let idx;
      while ((idx = this.#buffer.indexOf("\n")) !== -1) {
        const line = this.#buffer.slice(0, idx);
        this.#buffer = this.#buffer.slice(idx + 1);
        this.#handleLine(line);
      }
    });

    sock.on("error", () => this.#fail("connection error"));
    sock.on("close", () => this.#fail("connection closed"));
  }

  #fail(msg) {
    this.#dead = true;
    for (const { reject } of this.#pending.values()) {
      reject(new EngramError(msg));
    }
    this.#pending.clear();
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

  get dead() {
    return this.#dead;
  }

  get busy() {
    return this.#pending.size > 0;
  }

  call(method, params) {
    return new Promise((resolve, reject) => {
      const id = ++this.#idCounter;
      this.#pending.set(id, { resolve, reject });
      this.#sock.write(
        JSON.stringify({ jsonrpc: "2.0", id, method, params: params ?? null }) +
          "\n"
      );
    });
  }

  destroy() {
    this.#dead = true;
    this.#sock.destroy();
  }
}

function dial(socketPath) {
  return new Promise((resolve, reject) => {
    const sock = netConnect(socketPath);
    sock.on("connect", () => resolve(new Conn(sock)));
    sock.on("error", (err) =>
      reject(new EngramError(`daemon not reachable: ${err.message}`))
    );
  });
}

export class Engram {
  #socketPath;
  #idle = []; // available Conn instances
  #maxSize;

  constructor(socketPath, firstConn, maxSize) {
    this.#socketPath = socketPath;
    this.#idle = [firstConn];
    this.#maxSize = maxSize;
  }

  static async connect(socketPath, poolSize = DEFAULT_POOL_SIZE) {
    const sock = socketPath || DEFAULT_SOCKET;
    const first = await dial(sock);
    return new Engram(sock, first, poolSize);
  }

  async #get() {
    // Grab an idle, live connection.
    while (this.#idle.length > 0) {
      const cn = this.#idle.pop();
      if (!cn.dead) return cn;
    }
    // Pool empty — dial a new one.
    return dial(this.#socketPath);
  }

  #put(cn) {
    if (cn.dead) return;
    if (this.#idle.length < this.#maxSize) {
      this.#idle.push(cn);
    } else {
      cn.destroy();
    }
  }

  async #call(method, params) {
    const cn = await this.#get();
    try {
      const result = await cn.call(method, params);
      this.#put(cn);
      return result;
    } catch (err) {
      cn.destroy();
      throw err;
    }
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
    for (const cn of this.#idle) {
      cn.destroy();
    }
    this.#idle = [];
  }
}
