#!/usr/bin/env node
import { createInterface } from 'readline';
import { DaemonClient } from '../lib/daemon-client.mjs';

const MIN_CHARS_TO_CHECK = 800; // ~200 tokens at ~4 chars/token
const OWN_TOOL_PREFIX = 'mcp__engram-ccode';

async function readStdin() {
  return new Promise((resolve) => {
    const lines = [];
    const rl = createInterface({ input: process.stdin, terminal: false });
    rl.on('line', (line) => lines.push(line));
    rl.on('close', () => resolve(lines.join('\n')));
    if (process.stdin.isTTY) resolve('');
  });
}

/**
 * Core hook logic. Injectable for testing:
 *   clientFactory() returns a DaemonClient-compatible object.
 *   stdinFn() returns the raw stdin string.
 */
export async function run(clientFactory = () => new DaemonClient(), stdinFn = readStdin) {
  const raw = await stdinFn();
  if (!raw.trim()) return;

  let payload;
  try { payload = JSON.parse(raw); } catch { return; }

  const toolName = typeof payload.tool_name === 'string' ? payload.tool_name : '';
  const toolOutput = typeof payload.tool_output === 'string' ? payload.tool_output : '';

  if (toolName.startsWith(OWN_TOOL_PREFIX)) return;
  if (!toolOutput || toolOutput.length < MIN_CHARS_TO_CHECK) return;

  const client = clientFactory();
  try {
    await client.call('engram.checkRedundancy', { content: toolOutput });
  } catch {
    // Daemon may not be running — fail silently, never surface to Claude
  } finally {
    client.disconnect();
  }
}

// Run only when executed directly as a hook, not when imported by tests.
if (new URL(import.meta.url).pathname === process.argv[1]) {
  run().then(() => process.exit(0)).catch(() => process.exit(0));
}
