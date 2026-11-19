import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(here, '..');
const serverDir = path.join(root, '..', 'server');
const output = path.join(root, '.tmp', process.platform === 'win32' ? 'dashboard.exe' : 'dashboard');

const candidates = [
  process.env.GO_BINARY,
  'C:\\Program Files\\Go\\bin\\go.exe',
  'go',
].filter((candidate) => Boolean(candidate));

const go = candidates.find((candidate) => {
  if (candidate === 'go') return true;
  return existsSync(candidate);
});

if (!go) {
  throw new Error('Go toolchain not found. Set GO_BINARY or install Go.');
}

console.log('Building the web client...');
const webDir = path.join(root, '..', 'web');
execFileSync(process.execPath, [path.join(webDir, 'node_modules', 'vite', 'bin', 'vite.js'), 'build'], {
  cwd: webDir,
  stdio: 'inherit',
});

console.log('Building the server binary...');
execFileSync(go, ['build', '-o', output, './cmd/dashboard'], {
  cwd: serverDir,
  stdio: 'inherit',
});

console.log(`Server binary: ${output}`);
