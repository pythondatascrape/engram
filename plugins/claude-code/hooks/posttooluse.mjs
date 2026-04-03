#!/usr/bin/env node
import { createInterface } from 'readline';

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

async function main() {
  try {
    const raw = await readStdin();
    if (!raw.trim()) process.exit(0);

    let payload;
    try {
      payload = JSON.parse(raw);
    } catch {
      process.exit(0);
    }

    const toolName = typeof payload.tool_name === 'string' ? payload.tool_name : '';
    const toolOutput = typeof payload.tool_output === 'string' ? payload.tool_output : '';

    if (toolName.startsWith(OWN_TOOL_PREFIX)) process.exit(0);
    if (!toolOutput || toolOutput.length < MIN_CHARS_TO_CHECK) process.exit(0);

    const estimatedTokens = Math.ceil(toolOutput.length / 4);

    const message = {
      message: `Tool output detected (~${estimatedTokens} tokens) from "${toolName || 'unknown'}". Consider running mcp__engram-ccode__check_redundancy to detect repeated identity context.`,
    };

    process.stdout.write(JSON.stringify(message) + '\n');
  } catch {
    process.exit(0);
  }
}

main();
