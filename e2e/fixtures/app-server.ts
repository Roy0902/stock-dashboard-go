import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { createServer } from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(here, '..');
const serverDir = path.join(root, '..', 'server');
const binary = path.join(root, '.tmp', process.platform === 'win32' ? 'dashboard.exe' : 'dashboard');

export type ProviderMode = 'ok' | 'finnhub-down' | 'twelve-down' | 'all-down';

export type StartOptions = {
  watchlist?: string[];
  mode?: ProviderMode;
  /** Set false to start without provider keys (SCN-045/046). */
  keys?: boolean;
};

export type AppInstance = {
  url: string;
  dataDir: string;
  stop: () => Promise<void>;
};

export const providerUrl = 'http://127.0.0.1:4111';

export async function setProviderMode(mode: ProviderMode): Promise<void> {
  const response = await fetch(`${providerUrl}/__control`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ mode }),
  });
  if (!response.ok) {
    throw new Error(`provider control failed: ${response.status}`);
  }
}

export async function startApp(options: StartOptions = {}): Promise<AppInstance> {
  if (!existsSync(binary)) {
    throw new Error(
      `Server binary missing at ${binary}. Run "npm run build:app" before "playwright test".`,
    );
  }

  await setProviderMode(options.mode ?? 'ok');

  const dataDir = await mkdtemp(path.join(os.tmpdir(), 'stock-dashboard-e2e-'));
  await mkdir(dataDir, { recursive: true });
  if (options.watchlist && options.watchlist.length > 0) {
    await writeFile(
      path.join(dataDir, 'watchlist.json'),
      `${JSON.stringify({ tickers: options.watchlist }, null, 2)}\n`,
      'utf8',
    );
  }

  const port = await freePort();
  const withKeys = options.keys !== false;
  const child = spawn(binary, [], {
    cwd: serverDir,
    env: {
      ...process.env,
      PORT: String(port),
      DATA_DIR: dataDir,
      FINNHUB_API_KEY: withKeys ? 'test-finnhub-key' : '',
      TWELVEDATA_API_KEY: withKeys ? 'test-twelve-key' : '',
      FINNHUB_BASE_URL: providerUrl,
      TWELVEDATA_BASE_URL: providerUrl,
    },
    stdio: 'pipe',
  });

  const app: AppInstance = {
    url: `http://127.0.0.1:${port}`,
    dataDir,
    stop: async () => {
      await stopChild(child);
      await rm(dataDir, { recursive: true, force: true });
    },
  };

  let stderr = '';
  child.stderr?.on('data', (chunk: Buffer) => {
    stderr += chunk.toString();
  });

  try {
    await waitForHealth(`${app.url}/healthz`, child);
  } catch (error) {
    await app.stop();
    throw new Error(`app did not start: ${String(error)}\n${stderr}`);
  }
  return app;
}

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const probe = createServer();
    probe.once('error', reject);
    probe.listen(0, '127.0.0.1', () => {
      const address = probe.address();
      const port = typeof address === 'object' && address !== null ? address.port : 0;
      probe.close(() => resolve(port));
    });
  });
}

async function waitForHealth(url: string, child: ChildProcess): Promise<void> {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`process exited with code ${child.exitCode}`);
    }
    try {
      const response = await fetch(url);
      if (response.ok) return;
    } catch {
      // not listening yet
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error('health check timed out');
}

function stopChild(child: ChildProcess): Promise<void> {
  if (child.exitCode !== null) return Promise.resolve();
  return new Promise((resolve) => {
    child.once('exit', () => resolve());
    child.kill();
    setTimeout(() => {
      if (child.exitCode === null) child.kill('SIGKILL');
      resolve();
    }, 3_000);
  });
}
