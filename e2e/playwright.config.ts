import { defineConfig } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const providerPort = 4111;

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  timeout: 45_000,
  expect: { timeout: 7_000 },
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: [
    {
      command: 'node fixtures/provider-server.mjs',
      cwd: here,
      url: `http://127.0.0.1:${providerPort}/__health`,
      env: {
        ...(process.env as Record<string, string>),
        FAKE_PROVIDER_PORT: String(providerPort),
      },
      reuseExistingServer: false,
      timeout: 30_000,
    },
  ],
});
