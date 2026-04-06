#!/usr/bin/env node
/**
 * Engram SessionStart hook.
 *
 * 1. Collects CLAUDE.md files from the project tree and outputs a message
 *    asking Claude to derive and compress the project identity codebook.
 *
 * 2. Registers the real Claude session UUID with the local Engram proxy
 *    (via POST /internal/register-session) so the proxy can write
 *    <session_id>.ctx.json instead of the fallback proxy-<fingerprint>.ctx.json.
 *    This enables the statusline side-by-side context chart.
 *    Fails open — if the proxy is not running, this step is silently skipped.
 */
import { readFileSync } from 'fs';
import { dirname, join } from 'path';
import { homedir } from 'os';
import { createInterface } from 'readline';

async function readStdin() {
  if (process.stdin.isTTY) return '';
  return new Promise((resolve) => {
    const lines = [];
    const rl = createInterface({ input: process.stdin, terminal: false });
    rl.on('line', (line) => lines.push(line));
    rl.on('close', () => resolve(lines.join('\n')));
  });
}

function collectClaudeMd(startDir) {
  const results = [];
  let current = startDir;

  while (current && current !== dirname(current)) {
    const candidate = join(current, 'CLAUDE.md');
    try {
      const content = readFileSync(candidate, 'utf8');
      results.push({ path: candidate, content });
    } catch {
      // skip
    }
    current = dirname(current);
  }
  return results;
}

/**
 * Registers the Claude session UUID with the Engram proxy so the proxy
 * writes <session_id>.ctx.json for context-savings correlation.
 * Reads the proxy port from ~/.engram/proxy.port (written by `engram serve`).
 * Fails open on any error.
 */
async function registerSession(sessionId) {
  if (!sessionId) return;
  const portFile = join(homedir(), '.engram', 'proxy.port');
  try {
    const portStr = readFileSync(portFile, 'utf8').trim();
    const port = parseInt(portStr, 10);
    if (isNaN(port) || port < 1 || port > 65535) return;
    await fetch(`http://localhost:${port}/internal/register-session`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionId }),
      signal: AbortSignal.timeout(1000),
    });
  } catch {
    // Proxy not running or port file absent — not an error for this hook.
  }
}

async function main() {
  const raw = await readStdin();
  let sessionId = '';
  if (raw.trim()) {
    try {
      const payload = JSON.parse(raw);
      if (typeof payload.session_id === 'string') sessionId = payload.session_id;
    } catch {
      // ignore malformed stdin
    }
  }

  const projectDir = process.env.CLAUDE_PROJECT_DIR;
  if (!projectDir) process.exit(0);

  const files = collectClaudeMd(projectDir);
  if (files.length === 0) process.exit(0);

  const combinedContent = files.map((f) => f.content).join('\n');
  if (!combinedContent.trim()) process.exit(0);

  // Register with proxy before outputting the codebook message.
  // This fires before the first API request, so the proxy will have the
  // session ID ready when the message request arrives.
  await registerSession(sessionId);

  const fileList = files.map((f) => f.path).join(', ');
  const message = {
    message: `Found CLAUDE.md with ${combinedContent.length} chars (${files.length} file(s): ${fileList}). Deriving codebook for identity compression. Please call mcp__engram-ccode__derive_codebook with the CLAUDE.md content, then call mcp__engram-ccode__compress_identity to compress the project identity for this session.`,
  };
  process.stdout.write(JSON.stringify(message) + '\n');
}

main().catch(() => process.exit(0));
