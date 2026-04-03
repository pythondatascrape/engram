import { connect } from 'node:net';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { EventEmitter } from 'node:events';

const DEFAULT_SOCKET = join(homedir(), '.engram', 'engram.sock');
const CONNECT_TIMEOUT_MS = 5000;
const CALL_TIMEOUT_MS = 30000;

export class DaemonClient extends EventEmitter {
  #socket = null;
  #buffer = '';
  #nextId = 1;
  #pending = new Map();
  #socketPath;
  #connected = false;

  constructor(socketPath = DEFAULT_SOCKET) {
    super();
    this.#socketPath = socketPath;
  }

  async connect() {
    if (this.#connected) return;

    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        reject(new Error(`Connection to ${this.#socketPath} timed out after ${CONNECT_TIMEOUT_MS}ms`));
      }, CONNECT_TIMEOUT_MS);

      this.#socket = connect(this.#socketPath, () => {
        clearTimeout(timer);
        this.#connected = true;
        resolve();
      });

      this.#socket.setEncoding('utf8');

      this.#socket.on('data', (chunk) => {
        this.#buffer += chunk;
        this.#processBuffer();
      });

      this.#socket.on('error', (err) => {
        clearTimeout(timer);
        this.#connected = false;
        for (const [, entry] of this.#pending) {
          clearTimeout(entry.timer);
          entry.reject(new Error(`Socket error: ${err.message}`));
        }
        this.#pending.clear();
        reject(err);
      });

      this.#socket.on('close', () => {
        this.#connected = false;
        for (const [, entry] of this.#pending) {
          clearTimeout(entry.timer);
          entry.reject(new Error('Socket closed'));
        }
        this.#pending.clear();
        this.emit('disconnected');
      });
    });
  }

  async call(method, params = {}) {
    if (!this.#connected) {
      await this.connect();
    }

    const id = this.#nextId++;

    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.#pending.delete(id);
        reject(new Error(`RPC call "${method}" timed out after ${CALL_TIMEOUT_MS}ms`));
      }, CALL_TIMEOUT_MS);

      this.#pending.set(id, { resolve, reject, timer });

      const request = JSON.stringify({ jsonrpc: '2.0', id, method, params });
      this.#socket.write(request + '\n');
    });
  }

  #processBuffer() {
    const lines = this.#buffer.split('\n');
    this.#buffer = lines.pop();

    for (const line of lines) {
      if (!line.trim()) continue;
      try {
        const msg = JSON.parse(line);
        if (msg.id != null && this.#pending.has(msg.id)) {
          const entry = this.#pending.get(msg.id);
          this.#pending.delete(msg.id);
          clearTimeout(entry.timer);
          if (msg.error) {
            entry.reject(new Error(msg.error.message || JSON.stringify(msg.error)));
          } else {
            entry.resolve(msg.result);
          }
        }
      } catch {
        // Malformed JSON — skip
      }
    }
  }

  get connected() {
    return this.#connected;
  }

  disconnect() {
    if (this.#socket) {
      this.#socket.destroy();
      this.#socket = null;
      this.#connected = false;
    }
  }
}
