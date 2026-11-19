import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ApiError,
  activateTotp,
  getAuthStatus,
  getChart,
  getQuotes,
  getWatchlist,
  setupAccount,
} from '@/api/client';
import type * as ApiClientModule from '@/api/client';
import { App } from '@/App';
import {
  OWNER_USER,
  RECOVERY_CODES,
  SETUP_SECRET,
  makeSetupMaterial,
  makeSetupRequiredStatus,
} from '@/test/authFixtures';
import { makeChartSeries } from '@/test/fixtures';
import { renderWithProviders } from '@/test/render';

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof ApiClientModule>();
  return {
    ...actual,
    getAuthStatus: vi.fn(),
    setupAccount: vi.fn(),
    activateTotp: vi.fn(),
    getWatchlist: vi.fn(),
    getQuotes: vi.fn(),
    getChart: vi.fn(),
  };
});

const getAuthStatusMock = vi.mocked(getAuthStatus);
const setupAccountMock = vi.mocked(setupAccount);
const activateTotpMock = vi.mocked(activateTotp);
const getWatchlistMock = vi.mocked(getWatchlist);
const getQuotesMock = vi.mocked(getQuotes);
const getChartMock = vi.mocked(getChart);

async function fillSetupForm(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByRole('textbox', { name: 'Username' }), 'owner');
  await user.type(screen.getByLabelText('Password'), 'longenough12');
  await user.type(screen.getByLabelText('Confirm password'), 'longenough12');
}

async function reachVerifyStep(user: ReturnType<typeof userEvent.setup>) {
  await screen.findByRole('heading', { name: 'Create account' });
  await fillSetupForm(user);
  await user.click(screen.getByRole('button', { name: 'Create account' }));
  return screen.findByRole('heading', { name: 'Set up authenticator' });
}

function fieldOf(input: HTMLElement): HTMLElement {
  const field = input.closest('.auth-form__field');
  if (!(field instanceof HTMLElement)) {
    throw new Error('field container missing');
  }
  return field;
}

beforeEach(() => {
  getAuthStatusMock.mockReset();
  setupAccountMock.mockReset();
  activateTotpMock.mockReset();
  getWatchlistMock.mockReset();
  getQuotesMock.mockReset();
  getChartMock.mockReset();

  getAuthStatusMock.mockResolvedValue({ ok: true, value: makeSetupRequiredStatus() });
  setupAccountMock.mockResolvedValue({ ok: true, value: makeSetupMaterial() });
  activateTotpMock.mockResolvedValue({ ok: true, value: { user: OWNER_USER } });
  getWatchlistMock.mockResolvedValue({ ok: true, value: { tickers: [] } });
  getQuotesMock.mockResolvedValue({ ok: true, value: { quotes: [] } });
  getChartMock.mockResolvedValue({ ok: true, value: makeChartSeries() });
});

describe('SetupPage validation', () => {
  it('mirrors the server username, password and confirmation rules', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/setup' });

    await screen.findByRole('heading', { name: 'Create account' });
    const usernameInput = screen.getByRole('textbox', { name: 'Username' });
    const passwordInput = screen.getByLabelText('Password');
    const confirmInput = screen.getByLabelText('Confirm password');
    const submit = screen.getByRole('button', { name: 'Create account' });

    await user.click(submit);

    const usernameMessage = screen.getByText(
      'Use 3-32 lowercase letters, numbers, dots, dashes or underscores.',
    );
    expect(usernameMessage).toBeInTheDocument();
    expect(usernameInput).toHaveAttribute('aria-invalid', 'true');
    expect(usernameInput).toHaveAccessibleDescription(
      'Use 3-32 lowercase letters, numbers, dots, dashes or underscores.',
    );
    expect(usernameInput).toHaveFocus();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();

    await user.type(usernameInput, 'ab');
    await user.type(passwordInput, 'longenough12');
    await user.type(confirmInput, 'longenough12');
    await user.click(submit);

    expect(usernameInput).toHaveAttribute('aria-invalid', 'true');
    expect(setupAccountMock).not.toHaveBeenCalled();

    await user.clear(usernameInput);
    await user.type(usernameInput, 'Owner');
    await user.click(submit);

    expect(usernameInput).toHaveAttribute('aria-invalid', 'true');
    expect(setupAccountMock).not.toHaveBeenCalled();

    await user.clear(usernameInput);
    await user.type(usernameInput, 'owner');
    await user.clear(passwordInput);
    await user.clear(confirmInput);
    await user.click(submit);

    expect(
      screen.getByText('Use at least 12 characters.', { selector: '.auth-form__field-error' }),
    ).toBeInTheDocument();
    expect(passwordInput).toHaveAttribute('aria-invalid', 'true');
    expect(passwordInput).toHaveFocus();

    await user.type(passwordInput, 'longenough12');
    await user.type(confirmInput, 'longenough13');
    await user.click(submit);

    expect(screen.getByText('Passwords do not match.')).toBeInTheDocument();
    expect(confirmInput).toHaveAttribute('aria-invalid', 'true');
    expect(confirmInput).toHaveFocus();
    expect(setupAccountMock).not.toHaveBeenCalled();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('states the password rule up front and offers a reveal toggle', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/setup' });

    await screen.findByRole('heading', { name: 'Create account' });

    const password = screen.getByLabelText('Password');
    expect(password).toHaveAttribute('minlength', '12');
    expect(password).toHaveAccessibleDescription('Use at least 12 characters.');

    const reveal = within(fieldOf(password)).getByRole('button', { name: 'Show password' });
    expect(reveal).toHaveAttribute('aria-pressed', 'false');

    await user.type(password, 'longenough12');
    await user.click(reveal);

    expect(password).toHaveAttribute('type', 'text');
    expect(reveal).toHaveAttribute('aria-pressed', 'true');
    expect(reveal).toHaveAccessibleName('Hide password');
    expect(password).toHaveValue('longenough12');
  });

  it('describes an invalid password once when the hint repeats the error', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/setup' });

    await screen.findByRole('heading', { name: 'Create account' });
    await user.type(screen.getByRole('textbox', { name: 'Username' }), 'owner');
    await user.click(screen.getByRole('button', { name: 'Create account' }));

    const password = screen.getByLabelText('Password');
    expect(password).toHaveAttribute('aria-invalid', 'true');
    expect(password).toHaveAccessibleDescription('Use at least 12 characters.');
    expect(screen.getAllByText('Use at least 12 characters.')).toHaveLength(1);
  });

  it('shows the duplicate-username server error verbatim', async () => {
    const user = userEvent.setup();
    setupAccountMock.mockResolvedValue({
      ok: false,
      error: new ApiError('That username is already taken.', 'USERNAME_TAKEN', 409),
    });

    renderWithProviders(<App />, { route: '/setup' });
    await screen.findByRole('heading', { name: 'Create account' });
    await fillSetupForm(user);
    await user.click(screen.getByRole('button', { name: 'Create account' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('That username is already taken.');
    expect(screen.getByRole('heading', { name: 'Create account' })).toBeInTheDocument();
  });
});

describe('SetupPage authenticator material', () => {
  it('shows the QR, setup key and ten copyable recovery codes', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/setup' });

    const heading = await reachVerifyStep(user);

    expect(heading).toHaveFocus();
    expect(setupAccountMock).toHaveBeenCalledWith('owner', 'longenough12');
    expect(screen.getByRole('img', { name: 'Authenticator QR code' })).toBeInTheDocument();
    expect(screen.getByText('Scan this QR code with your authenticator app.')).toBeInTheDocument();
    expect(screen.getByText('Or enter this setup key manually:')).toBeInTheDocument();
    expect(screen.getByText(SETUP_SECRET)).toBeInTheDocument();

    const list = screen.getByRole('list', { name: 'Recovery codes' });
    expect(within(list).getAllByRole('listitem')).toHaveLength(10);
    for (const code of RECOVERY_CODES) {
      expect(within(list).getByText(code)).toBeInTheDocument();
    }

    const copyButtons = within(list).getAllByRole('button', { name: 'Copy' });
    expect(copyButtons).toHaveLength(10);
    const firstCopyButton = copyButtons[0];
    expect(firstCopyButton).toBeDefined();
    if (firstCopyButton === undefined) return;

    await user.click(firstCopyButton);
    expect(await navigator.clipboard.readText()).toBe(RECOVERY_CODES[0]);
    expect(screen.getAllByText('Copied').length).toBeGreaterThan(0);

    const checkbox = screen.getByRole('checkbox', {
      name: 'I have saved these recovery codes',
    });
    const continueButton = screen.getByRole('button', { name: 'Continue' });
    expect(checkbox).not.toBeChecked();
    expect(continueButton).toBeDisabled();

    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '123456');
    expect(continueButton).toBeDisabled();

    await user.click(checkbox);
    expect(continueButton).toBeEnabled();
  });

  it('activates the account and opens the dashboard', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/setup' });

    await reachVerifyStep(user);
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '123456');
    await user.click(screen.getByRole('checkbox', { name: 'I have saved these recovery codes' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(await screen.findByRole('heading', { name: 'Watchlist' })).toBeInTheDocument();
    expect(activateTotpMock).toHaveBeenCalledWith('123456');
    expect(screen.getByText(OWNER_USER.username)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument();
  });

  it('restores a requested deep link after first-run activation', async () => {
    const user = userEvent.setup();
    renderWithProviders(<App />, { route: '/chart/AAPL' });

    await reachVerifyStep(user);
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '123456');
    await user.click(screen.getByRole('checkbox', { name: 'I have saved these recovery codes' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(await screen.findByRole('heading', { level: 1, name: 'AAPL' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Watchlist' })).not.toBeInTheDocument();
    expect(getChartMock).toHaveBeenCalledWith('AAPL', '1M', expect.any(AbortSignal));
  });

  it('keeps the server activation error verbatim', async () => {
    const user = userEvent.setup();
    activateTotpMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Invalid verification code.', 'INVALID_CODE', 401),
    });

    renderWithProviders(<App />, { route: '/setup' });
    await reachVerifyStep(user);
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '000000');
    await user.click(screen.getByRole('checkbox', { name: 'I have saved these recovery codes' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Invalid verification code.');
    expect(screen.getByRole('heading', { name: 'Set up authenticator' })).toBeInTheDocument();
    expect(screen.getAllByRole('alert')).toHaveLength(1);
  });

  it('returns to the setup form when the pending setup expired', async () => {
    const user = userEvent.setup();
    activateTotpMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Setup timed out. Start again.', 'ENROLLMENT_EXPIRED', 401),
    });

    renderWithProviders(<App />, { route: '/setup' });
    await reachVerifyStep(user);
    await user.type(screen.getByRole('textbox', { name: 'Verification code' }), '123456');
    await user.click(screen.getByRole('checkbox', { name: 'I have saved these recovery codes' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    await screen.findByRole('textbox', { name: 'Username' });
    const heading = screen.getByRole('heading', { name: 'Create account' });
    expect(heading).toHaveFocus();
    expect(screen.getByRole('alert')).toHaveTextContent('Setup timed out. Start again.');
    await waitFor(() => {
      expect(screen.queryByRole('textbox', { name: 'Verification code' })).not.toBeInTheDocument();
    });
    expect(getAuthStatusMock).toHaveBeenCalled();
  });
});
