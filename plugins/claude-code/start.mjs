#!/usr/bin/env node
import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (!existsSync(resolve(__dirname, 'node_modules'))) {
  execFileSync('npm', ['install', '--silent'], {
    cwd: __dirname,
    stdio: 'pipe',
    timeout: 60000,
  });
}

await import(resolve(__dirname, 'server.mjs'));
