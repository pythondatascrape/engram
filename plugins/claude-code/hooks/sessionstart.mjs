#!/usr/bin/env node
import { readFileSync } from 'fs';
import { dirname, join } from 'path';

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

function main() {
  try {
    const projectDir = process.env.CLAUDE_PROJECT_DIR;
    if (!projectDir) process.exit(0);

    const files = collectClaudeMd(projectDir);
    if (files.length === 0) process.exit(0);

    const combinedContent = files.map((f) => f.content).join('\n');
    if (!combinedContent.trim()) process.exit(0);

    const fileList = files.map((f) => f.path).join(', ');

    const message = {
      message: `Found CLAUDE.md with ${combinedContent.length} chars (${files.length} file(s): ${fileList}). Deriving codebook for identity compression. Please call mcp__engram-ccode__derive_codebook with the CLAUDE.md content, then call mcp__engram-ccode__compress_identity to compress the project identity for this session.`,
    };

    process.stdout.write(JSON.stringify(message) + '\n');
  } catch {
    process.exit(0);
  }
}

main();
