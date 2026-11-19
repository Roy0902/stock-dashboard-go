import { act, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ApiError,
  getAuthStatus,
  getChart,
  getQuotes,
  getWatchlist,
  login,
  loginRecovery,
  loginTotp,
} from '@/api/client';
import type * as ApiClientModule from '@/api/client';
import { App } from '@/App';
import { OWNER_USER, makeUnauthenticatedStatus } from '@/test/authFixtures';
import { renderWithProviders } from '@/test/render';

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof ApiClientModule>();
  return {
    ...actual,
    getAuthStatus: vi.fn(),
    login: vi.fn(),
    loginTotp: vi.fn(),
    loginRecovery: vi.fn(),
    getWatchlist: vi.fn(),
    getQuotes: vi.fn(),
    getChart: vi.fn(),
  };
});

const getAuthStatusMock = vi.mocked(getAuthStatus);
const loginMock = vi.mocked(login);
const loginTotpMock = vi.mocked(loginTotp);
const loginRecoveryMock = vi.mocked(loginRecovery);
const getWatchlistMock = vi.mocked(getWatchlist);
const getQuotesMock = vi.mocked(getQuotes);
const getChartMock = vi.mocked(getChart);

async function submitCredentials(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByRole('heading', { name: 'Sign in' });
  await user.type(screen.getByRole('textbox', { name: 'Username' }), 'owner');
  await user.type(screen.getByLabelText('Password'), 'longenough12');
  await user.click(screen.getByRole('button', { name: 'Sign in' }));
}

async function reachTotpStep(user: ReturnType<typeof userEvent.setup>) {
  await submitCredentials(user);
  return screen.findByRole('heading', { name: 'Two-factor authentication' });
}

beforeEach(() => {
  getAuthStatusMock.mockReset();
  loginMock.mockReset();
  loginTotpMock.mockReset();
  loginRecoveryMock.mockReset();
  getWatchlistMock.mockReset();
  getQuotesMock.mockReset();
  getChartMock.mockReset();

  getAuthStatusMock.mockResolvedValue({ ok: true, value: makeUnauthenticatedStatus() });
  loginMock.mockResolvedValue({
    ok: true,
    value: { status: 'totp_required', expiresInSeconds: 300 },
  });
  loginTotpMock.mockResolvedValue({ ok: true, value: { user: OWNER_USER } });
  loginRecoveryMock.mockResolvedValue({ ok: true, value: { user: OWNER_USER } });
  getWatchlistMock.mockResolvedValue({ ok: true, value: { tickers: [] } });
  getQuotesMock.mockResolvedValue({ ok: true, value: { quotes: [] } });
  getChartMock.mockResolvedValue({
    ok: true,
    value: {
      ticker: 'AAPL',
      range: '1M',
      source: 'Twelve Data',
      points: [{ date: '2026-10-17', close: 189.5 }],
    },
  });
});

describe('LoginPage credentials', () => {
  it('shows the invalid-credentials copy verbatim in a single alert', async () => {
    const user = userEvent.setup();
    loginMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Invalid username or password.', 'INVALID_CREDENTIALS', 401),
    });

    renderWithProviders(<App />, { route: '/login' });
    await submitCredentials(user);

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Invalid username or password.');
    expect(screen.getAllByRole('alert')).toHaveLength(1);
    expect(screen.getByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
    expect(loginMock).toHaveBeenCalledWith('owner', 'longenough12');
    expect(screen.queryByText('Your session expired. Sign in again.')).not.toBeInTheDocument();
  });

  it('always shows the sign-up prompt', async () => {
    renderWithProviders(<App />, { route: '/login' });

    await screen.findByRole('heading', { name: 'Sign in' });
    expect(screen.getByText(/New here\?/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Set up your account' })).toHaveAttribute(
      'href',
      '/setup',
    );
    expect(screen.getByRole('textbox', { name: 'Username' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument();
  });

  it('trims the username before submitting', async () => {
    const user = userEvent.setup();
    loginMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Invalid username or password.', 'INVALID_CREDENTIALS', 401),
    });

    renderWithProviders(<App />, { route: '/login' });
    await screen.findByRole('heading', { name: 'Sign in' });
    await user.type(screen.getByRole('textbox', { name: 'Username' }), '  owner  ');
    await user.type(screen.getByLabelText('Password'), 'longenough12');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(loginMock).toHaveBeenCalledWith('owner', 'longenough12');
  });

  it('marks the empty credential fields and focuses the first invalid input', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/login' });
    await screen.findByRole('heading', { name: 'Sign in' });

    await user.click(screen.getByRole('button', { name: 'Sign in' }));

    const username = screen.getByRole('textbox', { name: 'Username' });
    const password = screen.getByLabelText('Password');
    expect(screen.getAllByText('Enter your username and password.')).toHaveLength(2);
    expect(username).toHaveAttribute('aria-invalid', 'true');
    expect(password).toHaveAttribute('aria-invalid', 'true');
    expect(username).toHaveAccessibleDescription('Enter your username and password.');
    expect(username).toHaveFocus();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(loginMock).not.toHaveBeenCalled();
  });

  it('reveals the password without losing its value', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/login' });
    await screen.findByRole('heading', { name: 'Sign in' });

    const password = screen.getByLabelText('Password');
    expect(password).toHaveAttribute('type', 'password');

    const reveal = screen.getByRole('button', { name: 'Show password' });
    expect(reveal).toHaveAttribute('aria-pressed', 'false');

    await user.type(password, 'longenough12');
    await user.click(reveal);

    expect(password).toHaveAttribute('type', 'text');
    expect(reveal).toHaveAttribute('aria-pressed', 'true');
    expect(reveal).toHaveAccessibleName('Hide password');
    expect(password).toHaveValue('longenough12');
  });
});

describe('LoginPage two-factor step', () => {
  it('auto-submits a numeric one-time code and completes the session', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/login' });

    const heading = await reachTotpStep(user);
    expect(heading).toHaveFocus();

    const input = screen.getByRole('textbox', { name: 'Verification code' });
    expect(input).toHaveAttribute('autocomplete', 'one-time-code');
    expect(input).toHaveAttribute('inputmode', 'numeric');
    expect(input).toHaveAttribute('pattern', '[0-9]{6}');
    expect(input).toHaveAccessibleDescription(
      'Enter the 6-digit code from your authenticator app. Code expires in 5 minutes.',
    );
    expect(screen.getByText('Code expires in 5 minutes.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Use a recovery code' })).toBeInTheDocument();

    await user.type(input, '123456');

    expect(await screen.findByRole('heading', { name: 'Watchlist' })).toBeInTheDocument();
    expect(loginTotpMock).toHaveBeenCalledTimes(1);
    expect(loginTotpMock).toHaveBeenCalledWith('123456');
    expect(screen.getByText(OWNER_USER.username)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument();
  });

  it('submits a pasted six-digit code once', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);

    const input = screen.getByRole('textbox', { name: 'Verification code' });
    await user.click(input);
    await user.paste('123456');

    expect(loginTotpMock).toHaveBeenCalledTimes(1);
    expect(loginTotpMock).toHaveBeenCalledWith('123456');
    expect(await screen.findByRole('heading', { name: 'Watchlist' })).toBeInTheDocument();
  });

  it('shows an invalid verification code and stays on the step', async () => {
    const user = userEvent.setup();
    loginTotpMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Invalid verification code.', 'INVALID_CODE', 401),
    });

    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '000000');

    expect(await screen.findByRole('alert')).toHaveTextContent('Invalid verification code.');
    expect(screen.getByRole('heading', { name: 'Two-factor authentication' })).toBeInTheDocument();
    expect(screen.getAllByRole('alert')).toHaveLength(1);
  });

  it('re-announces an identical error when the same code is submitted again', async () => {
    const user = userEvent.setup();
    loginTotpMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Invalid verification code.', 'INVALID_CODE', 401),
    });

    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '000000');

    const firstAlert = await screen.findByRole('alert');
    expect(firstAlert).toHaveTextContent('Invalid verification code.');

    await user.click(screen.getByRole('button', { name: 'Continue' }));

    const secondAlert = await screen.findByRole('alert');
    expect(secondAlert).toHaveTextContent('Invalid verification code.');
    expect(secondAlert).not.toBe(firstAlert);
    expect(loginTotpMock).toHaveBeenCalledTimes(2);
  });

  it('returns to sign in when the challenge expired', async () => {
    const user = userEvent.setup();
    loginTotpMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Your sign-in attempt expired. Sign in again.', 'CHALLENGE_EXPIRED', 401),
    });

    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '123456');

    const heading = await screen.findByRole('heading', { name: 'Sign in' });
    expect(heading).toHaveFocus();
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Your sign-in attempt expired. Sign in again.',
    );
  });
});

describe('LoginPage recovery step', () => {
  it('toggles the recovery input and signs in with one code', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);

    await user.click(screen.getByRole('button', { name: 'Use a recovery code' }));

    const recoveryInput = screen.getByRole('textbox', { name: 'Recovery code' });
    expect(recoveryInput).toHaveFocus();
    expect(screen.getByRole('button', { name: 'Back to sign in' })).toBeInTheDocument();

    await user.type(recoveryInput, 'AAAA-BBBB-CCCC-DDDD');
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(await screen.findByRole('heading', { name: 'Watchlist' })).toBeInTheDocument();
    expect(loginRecoveryMock).toHaveBeenCalledWith('AAAABBBBCCCCDDDD');
  });

  it.each([
    ['spaced', 'AAAA BBBB CCCC DDDD'],
    ['lowercase', 'aaaa bbbb cccc dddd'],
    ['hyphenless', 'AAAABBBBCCCCDDDD'],
  ])('sends the canonical code for %s recovery input', async (_label, input) => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);
    await user.click(screen.getByRole('button', { name: 'Use a recovery code' }));

    await user.type(screen.getByRole('textbox', { name: 'Recovery code' }), input);
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(await screen.findByRole('heading', { name: 'Watchlist' })).toBeInTheDocument();
    expect(loginRecoveryMock).toHaveBeenCalledWith('AAAABBBBCCCCDDDD');
    expect(screen.queryByText('Invalid recovery code.')).not.toBeInTheDocument();
  });

  it('ignores a late TOTP response after switching to recovery', async () => {
    const user = userEvent.setup();
    let resolveTotp: (value: Awaited<ReturnType<typeof loginTotp>>) => void = () => undefined;
    loginTotpMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveTotp = resolve;
        }),
    );

    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '123456');

    await user.click(screen.getByRole('button', { name: 'Use a recovery code' }));
    const recoveryInput = screen.getByRole('textbox', { name: 'Recovery code' });
    await user.type(recoveryInput, 'AAAA BBBB CCCC DDDD');

    await act(async () => {
      resolveTotp({
        ok: false,
        error: new ApiError('Invalid verification code.', 'INVALID_CODE', 401),
      });
      await Promise.resolve();
    });

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Use a recovery code' })).toBeInTheDocument();
    expect(recoveryInput).toHaveValue('AAAA BBBB CCCC DDDD');

    await user.click(screen.getByRole('button', { name: 'Continue' }));
    expect(await screen.findByRole('heading', { name: 'Watchlist' })).toBeInTheDocument();
    expect(loginRecoveryMock).toHaveBeenCalledWith('AAAABBBBCCCCDDDD');
  });

  it('shows the server recovery error verbatim', async () => {
    const user = userEvent.setup();
    loginRecoveryMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Invalid recovery code.', 'INVALID_RECOVERY_CODE', 401),
    });

    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);
    await user.click(screen.getByRole('button', { name: 'Use a recovery code' }));
    await user.type(screen.getByRole('textbox', { name: 'Recovery code' }), 'AAAA-BBBB-CCCC-DDDD');
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Invalid recovery code.');
    expect(screen.getByRole('textbox', { name: 'Recovery code' })).toBeInTheDocument();
    expect(screen.getAllByRole('alert')).toHaveLength(1);
  });

  it('goes back to the credential step and focuses its heading', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/login' });
    await reachTotpStep(user);

    await user.click(screen.getByRole('button', { name: 'Use a recovery code' }));
    await user.click(screen.getByRole('button', { name: 'Back to sign in' }));

    const heading = screen.getByRole('heading', { name: 'Sign in' });
    expect(heading).toHaveFocus();
    expect(screen.queryByRole('textbox', { name: 'Recovery code' })).not.toBeInTheDocument();
  });
});

describe('LoginPage deep-link restore', () => {
  it('returns to the requested chart after signing in', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/chart/AAPL' });

    await submitCredentials(user);
    await screen.findByRole('heading', { name: 'Two-factor authentication' });
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '123456');

    expect(await screen.findByRole('heading', { name: 'AAPL' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Watchlist' })).not.toBeInTheDocument();
    expect(getChartMock).toHaveBeenCalledWith('AAPL', '1M', expect.any(AbortSignal));
  });
});
