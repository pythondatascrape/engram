#!/usr/bin/env node
import { createInterface } from 'readline';
import { fileURLToPath } from 'url';
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

export function extractToolFields(payload) {
  if (!payload || typeof payload !== 'object') {
    return { toolName: '', toolOutput: '' };
  }

  const toolNameCandidates = [
    payload.tool_name,
    payload.toolName,
    payload.name,
    payload.tool?.name,
  ];
  const toolOutputCandidates = [
    payload.tool_output,
    payload.toolOutput,
    payload.output,
    payload.result,
    payload.tool?.output,
    payload.tool_result?.output,
  ];

  const toolName = toolNameCandidates.find((v) => typeof v === 'string' && v) ?? '';
  const toolOutput = toolOutputCandidates.find((v) => typeof v === 'string' && v) ?? '';

  return { toolName, toolOutput };
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

  const { toolName, toolOutput } = extractToolFields(payload);

  if (toolName.startsWith(OWN_TOOL_PREFIX)) return;
  if (!toolOutput || toolOutput.length < MIN_CHARS_TO_CHECK) return;

  const client = clientFactory();
  try {
    const result = await client.call('engram.checkRedundancy', { content: toolOutput });
    if (result?.isRedundant) {
      const kind = typeof result.kind === 'string' && result.kind ? result.kind : 'redundant';
      const message = {
        message: `Engram redundancy check detected ${kind} tool output. Do not restate the full tool output. Prefer a concise delta-only summary and only quote the minimum necessary lines.`,
      };
      process.stdout.write(JSON.stringify(message) + '\n');
    }
  } catch {
    // Daemon may not be running — fail silently, never surface to Claude
  } finally {
    client.disconnect();
  }
}

// Run only when executed directly as a hook, not when imported by tests.
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  run().then(() => process.exit(0)).catch(() => process.exit(0));
}
