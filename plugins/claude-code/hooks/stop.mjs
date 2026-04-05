#!/usr/bin/env node
/**
 * Engram Stop hook — updates per-session compression stats after every response.
 *
 * Writes to ~/.engram/sessions/<session_id>.json so each terminal session tracks
 * its own orig/comp/saved independently, preventing cross-session contamination
 * when multiple Claude Code sessions run simultaneously.
 */
import { createInterface } from 'readline';
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { homedir } from 'os';
import { fileURLToPath } from 'url';

async function readStdin() {
  return new Promise((resolve) => {
    const lines = [];
    const rl = createInterface({ input: process.stdin, terminal: false });
    rl.on('line', (line) => lines.push(line));
    rl.on('close', () => resolve(lines.join('\n')));
    if (process.stdin.isTTY) resolve('');
  });
}

/** Walk up from startDir collecting CLAUDE.md paths, closest-first. */
function findClaudeMd(startDir) {
  let current = startDir;
  while (current && current !== dirname(current)) {
    const candidate = join(current, 'CLAUDE.md');
    if (existsSync(candidate)) return candidate;
    current = dirname(current);
  }
  return null;
}

/** Read the codebook path from engram.yaml in projectDir (if present). */
function findCodebook(projectDir) {
  const yamlPath = join(projectDir, 'engram.yaml');
  if (!existsSync(yamlPath)) return null;
  try {
    const content = readFileSync(yamlPath, 'utf8');
    const match = content.match(/^\s*codebook:\s*(.+)$/m);
    if (!match) return null;
    const rel = match[1].trim().replace(/^["']|["']$/g, '');
    const full = join(projectDir, rel);
    return existsSync(full) ? full : null;
  } catch {
    return null;
  }
}

export async function run(stdinFn = readStdin) {
  const raw = await stdinFn();
  if (!raw.trim()) return;

  let payload;
  try { payload = JSON.parse(raw); } catch { return; }

  const sessionId = typeof payload.session_id === 'string' ? payload.session_id : '';
  if (!sessionId) return;

  const sessionsDir = join(homedir(), '.engram', 'sessions');
  mkdirSync(sessionsDir, { recursive: true });
  const sessionFile = join(sessionsDir, `${sessionId}.json`);

  // Load existing session stats or initialize from CLAUDE.md / codebook sizes.
  let stats;
  if (existsSync(sessionFile)) {
    try { stats = JSON.parse(readFileSync(sessionFile, 'utf8')); } catch { stats = null; }
  }

  if (!stats || !stats.orig_per_call) {
    // First call for this session — derive compression constants.
    const projectDir = process.env.CLAUDE_PROJECT_DIR || process.cwd();
    const claudeMd = findClaudeMd(projectDir);
    if (!claudeMd) return;

    const origChars = readFileSync(claudeMd).length;
    const origTokens = Math.max(1, Math.floor(origChars / 4));

    const codebook = findCodebook(projectDir);
    let compTokens;
    if (codebook) {
      compTokens = Math.max(1, Math.floor(readFileSync(codebook).length / 4));
    } else {
      // Fallback: dimensions serialize to "project=<name>" — roughly 3-4 tokens.
      compTokens = 4;
    }

    stats = {
      session_id: sessionId,
      orig_per_call: origTokens,
      comp_per_call: compTokens,
      saved_per_call: Math.max(0, origTokens - compTokens),
      turns: 0,
      total_orig: 0,
      total_comp: 0,
      total_saved: 0,
    };
  }

  stats.turns += 1;
  stats.total_orig  = stats.orig_per_call  * stats.turns;
  stats.total_comp  = stats.comp_per_call  * stats.turns;
  stats.total_saved = stats.saved_per_call * stats.turns;

  // Context window tracking: record what the context would have been without
  // Engram (baseline = actual + identity savings this turn) vs what it is with Engram.
  const totalInputTokens = payload.context_window?.total_input_tokens;
  if (typeof totalInputTokens === 'number' && totalInputTokens > 0) {
    stats.ctx_comp = totalInputTokens;
    stats.ctx_orig = totalInputTokens + stats.total_saved;
  }

  const tmp = `${sessionFile}.tmp`;
  writeFileSync(tmp, JSON.stringify(stats, null, 2) + '\n', { mode: 0o600 });
  // Atomic rename so partial writes never corrupt the file.
  const { renameSync } = await import('fs');
  renameSync(tmp, sessionFile);

  // Write PID → session_id mapping so statusline-command.sh can identify
  // the right session file. The statusLine payload lacks session_id, but
  // both this hook and the statusline script share the same Claude Code
  // parent PID, making process.ppid a stable per-terminal key.
  const pidDir = join(sessionsDir, 'by-pid');
  mkdirSync(pidDir, { recursive: true });
  writeFileSync(join(pidDir, String(process.ppid)), sessionId + '\n', { mode: 0o600 });
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  run().then(() => process.exit(0)).catch(() => process.exit(0));
}
