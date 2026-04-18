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
import { fileURLToPath } from 'url';
import { DaemonClient } from '../lib/daemon-client.mjs';

async function readStdin() {
  if (process.stdin.isTTY) return '';
  return new Promise((resolve) => {
    const lines = [];
    const rl = createInterface({ input: process.stdin, terminal: false });
    rl.on('line', (line) => lines.push(line));
    rl.on('close', () => resolve(lines.join('\n')));
  });
}

export function extractSessionId(payload) {
  if (!payload || typeof payload !== 'object') return '';
  if (typeof payload.session_id === 'string' && payload.session_id) return payload.session_id;
  if (typeof payload.sessionId === 'string' && payload.sessionId) return payload.sessionId;
  if (payload.session && typeof payload.session === 'object') {
    if (typeof payload.session.id === 'string' && payload.session.id) return payload.session.id;
    if (typeof payload.session.session_id === 'string' && payload.session.session_id) return payload.session.session_id;
  }
  return '';
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
    const resp = await fetch(`http://localhost:${port}/internal/register-session`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionId }),
      signal: AbortSignal.timeout(1000),
    });
    if (!resp.ok) return;
  } catch {
    // Proxy not running or port file absent — not an error for this hook.
  }
}

function estimateTokens(text) {
  return Math.max(1, Math.floor((text?.length ?? 0) / 4));
}

export async function buildCompressedIdentity(clientFactory, content) {
  const client = clientFactory();
  try {
    const derived = await client.call('engram.deriveCodebook', { content });
    const dimensions = derived?.codebook ?? {};
    const compressed = await client.call('engram.compressIdentity', {
      dimensions,
      originalTokens: estimateTokens(content),
    });
    return compressed?.block ?? '';
  } finally {
    client.disconnect();
  }
}

export async function run(
  stdinFn = readStdin,
  registerSessionFn = registerSession,
  collectClaudeMdFn = collectClaudeMd,
  clientFactory = () => new DaemonClient(),
) {
  const raw = await stdinFn();
  let sessionId = '';
  if (raw.trim()) {
    try {
      const payload = JSON.parse(raw);
      sessionId = extractSessionId(payload);
    } catch {
      // ignore malformed stdin
    }
  }

  // Always register with proxy so context stats are attributed to the correct
  // session UUID even when there are no CLAUDE.md files to inject.
  await registerSessionFn(sessionId);

  const projectDir = process.env.CLAUDE_PROJECT_DIR;
  if (!projectDir) return;

  const files = collectClaudeMdFn(projectDir);
  if (files.length === 0) return;

  const combinedContent = files.map((f) => f.content).join('\n');
  if (!combinedContent.trim()) return;

  let identityBlock = '';
  try {
    identityBlock = await buildCompressedIdentity(clientFactory, combinedContent);
  } catch {
    // Daemon may not be running yet or compression may fail — keep hook fail-open.
  }

  const fileList = files.map((f) => f.path).join(', ');
  const derivedNote = identityBlock
    ? `Engram directly compressed the project identity for this session from ${files.length} file(s): ${fileList}.\n\nUse this compressed identity block as the session identity context:\n\n${identityBlock}`
    : `Found CLAUDE.md with ${combinedContent.length} chars (${files.length} file(s): ${fileList}). Engram could not directly compress identity in the hook, so please derive and compress the project identity for this session.`;
  const message = {
    message: derivedNote,
  };
  process.stdout.write(JSON.stringify(message) + '\n');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  run().then(() => process.exit(0)).catch(() => process.exit(0));
}
