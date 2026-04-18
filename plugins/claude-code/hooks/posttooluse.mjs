#!/usr/bin/env node
import { createInterface } from 'readline';
import { fileURLToPath } from 'url';
import { DaemonClient } from '../lib/daemon-client.mjs';

const MIN_CHARS_TO_SUMMARIZE = 800; // ~200 tokens at ~4 chars/token
const OWN_TOOL_PREFIX = 'mcp__engram-ccode';
const HEAD_TAIL_LINES = 40; // lines to keep from head and tail for Bash/Read outputs
const MAX_JSON_KEYS = 20;   // top-level keys to show for large JSON objects

/**
 * Summarize a tool output string using per-tool strategies.
 * Returns the summarized string, or the original if no strategy applies or
 * the summary would not be shorter.
 */
export function summarizeToolOutput(toolName, toolOutput) {
  const lower = toolName.toLowerCase();

  // Large JSON: show top-level keys with value types/counts
  const trimmed = toolOutput.trim();
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      const parsed = JSON.parse(trimmed);
      if (Array.isArray(parsed)) {
        const sample = parsed.slice(0, 3).map((v) =>
          typeof v === 'object' && v !== null ? Object.keys(v) : typeof v,
        );
        const summary = `[Array(${parsed.length}), sample keys: ${JSON.stringify(sample)}]`;
        if (summary.length < toolOutput.length) return summary;
      } else if (typeof parsed === 'object' && parsed !== null) {
        const keys = Object.keys(parsed).slice(0, MAX_JSON_KEYS);
        const extra = Object.keys(parsed).length - keys.length;
        const pairs = keys.map((k) => {
          const v = parsed[k];
          if (Array.isArray(v)) return `${k}:Array(${v.length})`;
          if (typeof v === 'object' && v !== null) return `${k}:Object(${Object.keys(v).length})`;
          return `${k}:${typeof v}`;
        });
        const suffix = extra > 0 ? ` …+${extra} keys` : '';
        const summary = `{${pairs.join(', ')}${suffix}}`;
        if (summary.length < toolOutput.length) return summary;
      }
    } catch { /* not valid JSON, fall through */ }
  }

  // TodoWrite / Edit / Write: single-line delta summary
  if (/todo|edit|write/i.test(lower)) {
    const lines = toolOutput.split('\n').filter((l) => l.trim());
    const summary = lines.slice(0, 3).join(' | ');
    if (summary.length < toolOutput.length) return summary;
  }

  // Bash / Read / default line-oriented output: head + tail
  const lines = toolOutput.split('\n');
  if (lines.length > HEAD_TAIL_LINES * 2) {
    const head = lines.slice(0, HEAD_TAIL_LINES).join('\n');
    const tail = lines.slice(-HEAD_TAIL_LINES).join('\n');
    const omitted = lines.length - HEAD_TAIL_LINES * 2;
    return `${head}\n… [${omitted} lines omitted] …\n${tail}`;
  }

  return toolOutput;
}

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
  if (!toolOutput || toolOutput.length < MIN_CHARS_TO_SUMMARIZE) return;

  // Local truncation — always fires when output exceeds threshold, no daemon needed
  const summary = summarizeToolOutput(toolName, toolOutput);
  if (summary !== toolOutput) {
    const savedChars = toolOutput.length - summary.length;
    process.stdout.write(JSON.stringify({
      content: `[Engram] Tool output truncated (saved ~${Math.round(savedChars / 4)} tokens). Summarized output:\n${summary}`,
    }) + '\n');
    return;
  }

  // Fallback: daemon redundancy check for outputs that didn't compress locally
  const client = clientFactory();
  try {
    const result = await client.call('engram.checkRedundancy', { content: toolOutput });
    if (result?.isRedundant) {
      const kind = typeof result.kind === 'string' && result.kind ? result.kind : 'redundant';
      process.stdout.write(JSON.stringify({
        content: `Engram redundancy check detected ${kind} tool output. Do not restate the full tool output. Prefer a concise delta-only summary and only quote the minimum necessary lines.`,
      }) + '\n');
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
