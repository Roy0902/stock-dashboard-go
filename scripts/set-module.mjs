#!/usr/bin/env node
/**
 * Rewrites the Go module path across the repository.
 *
 * The committed module path is `stock-dashboard/server`: neutral, but not a
 * publishable VCS path. A fork, a private mirror, or a team that vendors this
 * repo can adopt its own path with one command instead of editing imports.
 *
 * Usage:
 *   node scripts/set-module.mjs github.com/acme/stock-dashboard/server
 *   node scripts/set-module.mjs github.com/acme/stock-dashboard/server --dry-run
 *
 * Files touched: server/go.mod, every *.go file under server/, and any docs or
 * scripts that name the module path. node_modules, dist, .dev and data are skipped.
 */
import { readdir, readFile, writeFile } from 'node:fs/promises';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

// Resolve the repository root from the script location, so the command works
// from any working directory (a CI step, another checkout, C:\Windows, ...).
const root = fileURLToPath(new URL('..', import.meta.url));
const serverDir = join(root, 'server');
const goModPath = join(serverDir, 'go.mod');

const args = process.argv.slice(2);
const dryRun = args.includes('--dry-run');
const nextPath = args.find((arg) => !arg.startsWith('--'));

const MODULE_PATTERN = /^[a-z0-9][a-z0-9.-]*(\/[a-z0-9][a-z0-9._~-]*)+$/;
// Review reports are historical evidence: never rewrite them. Runtime state,
// vendored packages and build output are not source either.
const SKIP =
  /node_modules|\\dist\\|\/dist\/|\\\.dev\\|\.dev\/|\\data\\|\/data\/|playwright-report|test-results|\.git|\\docs\\reviews\\|\/docs\/reviews\//;
const EXTENSIONS = new Set(['.go', '.mod', '.md', '.ps1', '.cmd', '.mjs', '.json', '.yml', '.yaml']);

function usage(message) {
  if (message) console.error(`error: ${message}`);
  console.error('usage: node scripts/set-module.mjs <module-path> [--dry-run]');
  process.exit(2);
}

if (!nextPath) usage('missing module path');
if (!MODULE_PATTERN.test(nextPath)) {
  usage(`"${nextPath}" is not a valid module path (lowercase, slash-separated, no spaces)`);
}

const goMod = await readFile(goModPath, 'utf8');
const match = goMod.match(/^module\s+(\S+)$/m);
if (!match) usage('cannot find the module directive in server/go.mod');
const currentPath = match[1];

if (currentPath === nextPath) {
  console.log(`module path is already ${currentPath}`);
  process.exit(0);
}

const changed = [];
let replacements = 0;

async function walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (SKIP.test(full)) continue;
    if (entry.isDirectory()) {
      await walk(full);
      continue;
    }
    const dot = entry.name.lastIndexOf('.');
    if (dot < 0 || !EXTENSIONS.has(entry.name.slice(dot))) continue;
    const text = await readFile(full, 'utf8');
    if (!text.includes(currentPath)) continue;
    const count = text.split(currentPath).length - 1;
    changed.push({ file: relative(root, full), count });
    replacements += count;
    if (!dryRun) {
      await writeFile(full, text.split(currentPath).join(nextPath), 'utf8');
    }
  }
}

await walk(root);

if (changed.length === 0) {
  console.log(`no files reference ${currentPath}`);
  process.exit(0);
}

for (const item of changed.sort((a, b) => a.file.localeCompare(b.file))) {
  console.log(`${dryRun ? 'would update' : 'updated'} ${item.count}\t${item.file}`);
}
console.log(
  `${dryRun ? 'dry run' : 'done'}: ${changed.length} file(s), ${replacements} reference(s) ` +
    `${currentPath} -> ${nextPath}`,
);
if (!nextPath.split('/')[0].includes('.')) {
  console.log(
    'note: the first element has no dot, so the module builds locally but is not go-gettable. ' +
      'That is fine for an application; use a domain path if you publish it.',
  );
}
if (!dryRun) {
  console.log('next: cd server && gofmt -l . && go build ./... && go test ./...');
}
