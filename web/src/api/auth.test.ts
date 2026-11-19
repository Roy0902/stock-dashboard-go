import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ApiError,
  activateTotp,
  getAuthStatus,
  getMe,
  login,
  loginRecovery,
  loginTotp,
  logout,
  setUnauthorizedHandler,
  setupAccount,
} from '@/api/client';
import { copy } from '@/locale';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

const fetchMock = vi.fn<typeof fetch>();

const AUTH_USER = { username: 'owner', totpEnabled: true };

function makeRecoveryCodes(): string[] {
  return Array.from(
    { length: 10 },
    (_, index) => `AAAA-BBBB-CCCC-${String(index).padStart(4, '0')}`,
  );
}

function makeSetupMaterial() {
  return {
    username: 'owner',
    secret: 'JBSWY3DPEHPK3PXP',
    otpauthUrl:
      'otpauth://totp/Stock%20Dashboard:owner?secret=JBSWY3DPEHPK3PXP&issuer=Stock%20Dashboard&algorithm=SHA1&digits=6&period=30',
    recoveryCodes: makeRecoveryCodes(),
  };
}

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  setUnauthorizedHandler(null);
  vi.unstubAllGlobals();
});

describe('auth is same-origin and cookie-backed', () => {
  it('sends same-origin credentials and parses the authenticated status', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({ setupRequired: false, authenticated: true, user: AUTH_USER }),
    );

    const result = await getAuthStatus();

    expect(result).toEqual({
      ok: true,
      value: { setupRequired: false, authenticated: true, user: AUTH_USER },
    });
    const [path, init] = fetchMock.mock.calls[0] ?? [];
    expect(path).toBe('/api/auth/status');
    expect(init?.credentials).toBe('same-origin');
    expect(init?.cache).toBe('no-store');
  });

  it('parses an unauthenticated status without a user', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ setupRequired: false, authenticated: false }));

    const result = await getAuthStatus();

    expect(result).toEqual({
      ok: true,
      value: { setupRequired: false, authenticated: false },
    });
  });

  it('rejects an authenticated status with no user instead of trusting it', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ setupRequired: false, authenticated: true }));

    const result = await getAuthStatus();

    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.error.message).toBe(copy.errors.generic);
    expect(result.error.code).toBe('INTERNAL_ERROR');
  });

  it('maps an auth network failure to the generic internal-error copy', async () => {
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));

    const result = await getAuthStatus();

    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.error.message).toBe(copy.errors.generic);
    expect(result.error.code).toBe('INTERNAL_ERROR');
  });
});

describe('setup and activation', () => {
  it('posts the setup credentials and parses the setup material', async () => {
    fetchMock.mockResolvedValue(jsonResponse(makeSetupMaterial(), 201));

    const result = await setupAccount('owner', 'correct horse battery');

    expect(result).toEqual({ ok: true, value: makeSetupMaterial() });
    const [path, init] = fetchMock.mock.calls[0] ?? [];
    expect(path).toBe('/api/auth/setup');
    expect(init?.method).toBe('POST');
    expect(init?.body).toBe(
      JSON.stringify({ username: 'owner', password: 'correct horse battery' }),
    );
  });

  it('rejects setup material without exactly ten recovery codes', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({ ...makeSetupMaterial(), recoveryCodes: ['AAAA-BBBB-CCCC-DDDD'] }, 201),
    );

    const result = await setupAccount('owner', 'correct horse battery');

    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.error.message).toBe(copy.errors.generic);
  });

  it('posts the activation code and parses the user', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ user: AUTH_USER }));

    const result = await activateTotp('123456');

    expect(result).toEqual({ ok: true, value: { user: AUTH_USER } });
    const [path, init] = fetchMock.mock.calls[0] ?? [];
    expect(path).toBe('/api/auth/2fa/activate');
    expect(init?.body).toBe(JSON.stringify({ code: '123456' }));
  });
});

describe('login endpoints', () => {
  it('parses the TOTP challenge', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ status: 'totp_required', expiresInSeconds: 300 }));

    const result = await login('owner', 'correct horse battery');

    expect(result).toEqual({
      ok: true,
      value: { status: 'totp_required', expiresInSeconds: 300 },
    });
    const [path, init] = fetchMock.mock.calls[0] ?? [];
    expect(path).toBe('/api/auth/login');
    expect(init?.body).toBe(
      JSON.stringify({ username: 'owner', password: 'correct horse battery' }),
    );
  });

  it('posts the TOTP code and parses the user', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ user: AUTH_USER }));

    const result = await loginTotp('123456');

    expect(result).toEqual({ ok: true, value: { user: AUTH_USER } });
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/auth/login/totp');
  });

  it('posts the recovery code and parses the user', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ user: AUTH_USER }));

    const result = await loginRecovery('AAAA-BBBB-CCCC-DDDD');

    expect(result).toEqual({ ok: true, value: { user: AUTH_USER } });
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/auth/login/recovery');
    expect(fetchMock.mock.calls[0]?.[1]?.body).toBe(
      JSON.stringify({ code: 'AAAA-BBBB-CCCC-DDDD' }),
    );
  });

  it('parses the current user', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ user: AUTH_USER }));

    const result = await getMe();

    expect(result).toEqual({ ok: true, value: { user: AUTH_USER } });
  });
});

describe('logout', () => {
  it('accepts an empty 204 response', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 204 }));

    const result = await logout();

    expect(result).toEqual({ ok: true, value: null });
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe('POST');
  });
});

describe('auth errors', () => {
  it('keeps the server copy and code verbatim', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({ error: 'Invalid recovery code.', code: 'INVALID_RECOVERY_CODE' }, 401),
    );

    const result = await loginRecovery('AAAA-BBBB-CCCC-DDDD');

    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.error).toBeInstanceOf(ApiError);
    expect(result.error.message).toBe('Invalid recovery code.');
    expect(result.error.code).toBe('INVALID_RECOVERY_CODE');
    expect(result.error.status).toBe(401);
  });

  it('announces only UNAUTHENTICATED 401 responses', async () => {
    const handler = vi.fn();
    setUnauthorizedHandler(handler);

    fetchMock.mockResolvedValue(
      jsonResponse({ error: 'Authentication required.', code: 'UNAUTHENTICATED' }, 401),
    );
    await getMe();

    expect(handler).toHaveBeenCalledTimes(1);
  });

  it('does not announce a failed sign-in as an expired session', async () => {
    const handler = vi.fn();
    setUnauthorizedHandler(handler);

    fetchMock.mockResolvedValue(
      jsonResponse({ error: 'Invalid username or password.', code: 'INVALID_CREDENTIALS' }, 401),
    );
    await login('owner', 'wrong password');

    expect(handler).not.toHaveBeenCalled();
  });
});
