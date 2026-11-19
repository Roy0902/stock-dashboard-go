import { generate } from 'otplib';
import { expect, type BrowserContext } from '@playwright/test';
import type { AppInstance } from './app-server';

export type OwnerCredentials = {
  username: string;
  password: string;
  secret: string;
  recoveryCodes: string[];
};

export const ownerUsername = 'owner';
export const ownerPassword = 'correct-horse-battery-staple';

type SetupResponse = {
  secret: string;
  recoveryCodes: string[];
};

type StatusResponse = {
  setupRequired: boolean;
  authenticated: boolean;
};

/**
 * Performs first-run setup against a freshly started app and leaves the session cookie in
 * the browser context's cookie jar. Every e2e app gets an isolated DATA_DIR, so setup runs
 * exactly once per test.
 */
export async function bootstrapAuth(
  app: AppInstance,
  context: BrowserContext,
): Promise<OwnerCredentials> {
  const statusResponse = await context.request.get(`${app.url}/api/auth/status`);
  expect(statusResponse.ok()).toBeTruthy();
  const status = (await statusResponse.json()) as StatusResponse;
  if (!status.setupRequired) {
    throw new Error('bootstrapAuth expected a fresh app without an owner');
  }

  const setupResponse = await context.request.post(`${app.url}/api/auth/setup`, {
    data: { username: ownerUsername, password: ownerPassword },
  });
  expect(setupResponse.status(), await setupResponse.text()).toBe(201);
  const setup = (await setupResponse.json()) as SetupResponse;

  const activationResponse = await context.request.post(
    `${app.url}/api/auth/2fa/activate`,
    // Activate with the previous 30-second step so the current step stays unused for
    // the login tests. Without this, replay protection rejects an immediate login.
    {
      data: {
        code: await generate({
          secret: setup.secret,
          epoch: Math.floor(Date.now() / 1000) - 30,
        }),
      },
    },
  );
  expect(activationResponse.status(), await activationResponse.text()).toBe(200);

  return {
    username: ownerUsername,
    password: ownerPassword,
    secret: setup.secret,
    recoveryCodes: setup.recoveryCodes,
  };
}

export async function currentTotp(secret: string): Promise<string> {
  return generate({ secret });
}

export async function signInWithPassword(
  context: BrowserContext,
  appUrl: string,
  credentials: Pick<OwnerCredentials, 'username' | 'password'>,
): Promise<void> {
  const response = await context.request.post(`${appUrl}/api/auth/login`, {
    data: { username: credentials.username, password: credentials.password },
  });
  expect(response.status(), await response.text()).toBe(200);
  const body = (await response.json()) as { status: string };
  expect(body.status).toBe('totp_required');
}
