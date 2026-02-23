#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { createRequire } from 'node:module';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const entry = join(__dirname, '..', 'src', 'index.ts');
const args = process.argv.slice(2);

let tsxBin;
try {
  const require = createRequire(join(__dirname, '..', 'package.json'));
  tsxBin = join(dirname(require.resolve('tsx/package.json')), 'dist', 'cli.mjs');
} catch {
  tsxBin = join(__dirname, '..', 'node_modules', '.bin', 'tsx');
}

try {
  execFileSync('node', [tsxBin, entry, ...args], { stdio: 'inherit' });
} catch (err) {
  if (err.message && !err.status) {
    console.error('nano-brain: failed to start —', err.message);
  }
  process.exit(err.status || 1);
}
