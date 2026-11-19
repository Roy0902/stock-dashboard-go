import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { AuthStatus } from '@/api/types';
import { App } from '@/App';
import {
  makeAuthenticatedStatus,
  makeSetupRequiredStatus,
  makeUnauthenticatedStatus,
} from '@/test/authFixtures';
import { makeQuote } from '@/test/fixtures';
import { renderWithProviders } from '@/test/render';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

function requestPath(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input;
  if (input instanceof URL) return input.pathname;
  return input.url;
}

type FetchStubOptions = {
  status: AuthStatus;
  sessionExpired?: boolean;
};

function installFetch({ status, sessionExpired = false }: FetchStubOptions) {
  const fetchMock = vi.fn<typeof fetch>((input) => {
    const path = requestPath(input);
    if (path === '/api/auth/status') {
      return Promise.resolve(jsonResponse(status));
    }
    if (path === '/api/auth/logout') {
      return Promise.resolve(new Response(null, { status: 204 }));
    }
    if (path.startsWith('/api/watchlist') || path.startsWith('/api/quotes')) {
      if (sessionExpired) {
        return Promise.resolve(
          jsonResponse({ error: 'Authentication required.', code: 'UNAUTHENTICATED' }, 401),
        );
      }
      return Promise.resolve(
        path.startsWith('/api/watchlist')
          ? jsonResponse({ tickers: [] })
          : jsonResponse({ quotes: [] }),
      );
    }
    return Promise.resolve(jsonResponse({ error: 'Not found.', code: 'NOT_FOUND' }, 404));
  });

  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

beforeEach(() => {
  vi.unstubAllGlobals();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('route gating', () => {
  it('sends a first-run visit straight to account setup', async () => {
    installFetch({ status: makeSetupRequiredStatus() });

    renderWithProviders(<App />, { route: '/' });

    expect(await screen.findByRole('heading', { name: 'Create account' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /one calm place/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Sign in/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Sign out' })).not.toBeInTheDocument();
  });

  it('shows the create-account form while no accounts exist', async () => {
    installFetch({ status: makeSetupRequiredStatus() });

    renderWithProviders(<App />, { route: '/setup' });

    expect(await screen.findByRole('heading', { name: 'Create account' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Username' })).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
    expect(screen.getByLabelText('Confirm password')).toBeInTheDocument();
    expect(screen.queryByText('Sign-up is closed on this installation.')).not.toBeInTheDocument();
  });

  it('sends a first-run sign-in visit to account setup', async () => {
    installFetch({ status: makeSetupRequiredStatus() });

    renderWithProviders(<App />, { route: '/login' });

    expect(await screen.findByRole('heading', { name: 'Create account' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Username' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Sign in' })).not.toBeInTheDocument();
    expect(screen.queryByText(/New here\?/)).not.toBeInTheDocument();
  });

  it('sends an unauthenticated dashboard to login and keeps sign-up open at /setup', async () => {
    installFetch({ status: makeUnauthenticatedStatus() });

    const first = renderWithProviders(<App />, { route: '/dashboard' });
    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
    first.unmount();

    renderWithProviders(<App />, { route: '/setup' });
    expect(await screen.findByRole('heading', { name: 'Create account' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Username' })).toBeInTheDocument();
    expect(screen.queryByText('Sign-up is closed on this installation.')).not.toBeInTheDocument();
  });

  it('keeps the sign-up option visible after setup and opens registration', async () => {
    const user = userEvent.setup();
    installFetch({ status: makeUnauthenticatedStatus() });

    renderWithProviders(<App />, { route: '/login' });

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
    expect(screen.getByText(/New here\?/)).toBeInTheDocument();
    await user.click(screen.getByRole('link', { name: 'Set up your account' }));

    expect(await screen.findByRole('heading', { name: 'Create account' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Username' })).toBeInTheDocument();
    expect(screen.queryByText('Sign-up is closed on this installation.')).not.toBeInTheDocument();
  });

  it('shows the landing page to an anonymous visitor and links to sign in', async () => {
    const user = userEvent.setup();
    installFetch({ status: makeUnauthenticatedStatus() });

    renderWithProviders(<App />, { route: '/' });

    expect(
      await screen.findByRole('heading', {
        name: 'Your watchlist, quotes and daily charts in one calm place.',
      }),
    ).toBeInTheDocument();
    const signIn = screen.getByRole('link', { name: /Sign in/ });
    expect(signIn).toHaveAttribute('href', '/login');

    await user.click(signIn);
    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
  });

  it('sends an authenticated auth-page visit to the dashboard', async () => {
    installFetch({ status: makeAuthenticatedStatus() });

    const first = renderWithProviders(<App />, { route: '/login' });
    expect(await screen.findByRole('heading', { name: 'Watchlist' })).toBeInTheDocument();
    first.unmount();

    renderWithProviders(<App />, { route: '/setup' });
    expect(await screen.findByRole('heading', { name: 'Watchlist' })).toBeInTheDocument();
  });
});

describe('session expiry', () => {
  it('announces an expired session and shows the login screen', async () => {
    installFetch({ status: makeAuthenticatedStatus(), sessionExpired: true });

    renderWithProviders(<App />, { route: '/' });

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Your session expired. Sign in again.');
    expect(screen.getByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
    expect(screen.getAllByRole('alert')).toHaveLength(1);
    expect(screen.queryByRole('button', { name: 'Sign out' })).not.toBeInTheDocument();
  });

  it('does not announce expiry on a normal first visit', async () => {
    installFetch({ status: makeUnauthenticatedStatus() });

    renderWithProviders(<App />, { route: '/' });

    expect(
      await screen.findByRole('heading', {
        name: 'Your watchlist, quotes and daily charts in one calm place.',
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Your session expired. Sign in again.')).not.toBeInTheDocument();
    expect(screen.queryAllByRole('alert')).toHaveLength(0);
  });
});

describe('authenticated header', () => {
  it('shows the username and signs out through the API', async () => {
    const user = userEvent.setup();
    const fetchMock = installFetch({ status: makeAuthenticatedStatus() });

    renderWithProviders(<App />, { route: '/' });
    await screen.findByRole('heading', { name: 'Watchlist' });

    expect(screen.getByText('owner')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Sign out' }));

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Sign out' })).not.toBeInTheDocument();
    expect(screen.queryByText('owner')).not.toBeInTheDocument();

    const logoutCall = fetchMock.mock.calls.find(
      ([input]) => requestPath(input) === '/api/auth/logout',
    );
    expect(logoutCall).toBeDefined();
    expect(logoutCall?.[1]?.method).toBe('POST');
    expect(logoutCall?.[1]?.credentials).toBe('same-origin');
    await waitFor(() => {
      expect(screen.getByRole('main')).toHaveFocus();
    });
  });
});

type ChannelListener = (message: MessageEvent<unknown>) => void;

type PostedMessage = {
  event: string;
  origin: string;
};

class FakeBroadcastChannel {
  static open: FakeBroadcastChannel[] = [];
  onmessage: ChannelListener | null = null;
  posted: PostedMessage[] = [];
  closed = false;

  constructor() {
    FakeBroadcastChannel.open.push(this);
  }

  postMessage(message: PostedMessage): void {
    this.posted.push(message);
  }

  close(): void {
    this.closed = true;
  }
}

function openSubscriber(): FakeBroadcastChannel {
  const channel = FakeBroadcastChannel.open.find((candidate) => !candidate.closed);
  if (channel === undefined) {
    throw new Error('no open broadcast channel');
  }
  return channel;
}

describe('auth bootstrap failure', () => {
  it('offers a retry instead of pretending the session is signed out', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>(() =>
        Promise.resolve(
          jsonResponse(
            { error: 'Something went wrong. Try again shortly.', code: 'INTERNAL_ERROR' },
            500,
          ),
        ),
      ),
    );

    renderWithProviders(<App />, { route: '/' });

    expect(await screen.findByText('Something went wrong. Try again shortly.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /one calm place/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Sign in' })).not.toBeInTheDocument();
  });

  it('treats an unusual 5xx status as unavailable', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>(() =>
        Promise.resolve(
          jsonResponse({ error: 'Gateway timeout.', code: 'UPSTREAM_GATEWAY_TIMEOUT' }, 598),
        ),
      ),
    );

    renderWithProviders(<App />, { route: '/dashboard' });

    expect(await screen.findByRole('button', { name: 'Try again' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Sign in' })).not.toBeInTheDocument();
  });

  it('treats a rejected status request as unavailable, not signed out', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>(() => Promise.reject(new Error('offline'))),
    );

    renderWithProviders(<App />, { route: '/dashboard' });

    expect(await screen.findByRole('button', { name: 'Try again' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Sign in' })).not.toBeInTheDocument();
  });

  it('re-checks the session when the retry panel is used', async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        jsonResponse(
          { error: 'Something went wrong. Try again shortly.', code: 'INTERNAL_ERROR' },
          500,
        ),
      )
      .mockResolvedValueOnce(jsonResponse(makeUnauthenticatedStatus()));
    vi.stubGlobal('fetch', fetchMock);

    renderWithProviders(<App />, { route: '/' });
    await user.click(await screen.findByRole('button', { name: 'Try again' }));

    expect(
      await screen.findByRole('heading', {
        name: 'Your watchlist, quotes and daily charts in one calm place.',
      }),
    ).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});

describe('cross-tab account changes', () => {
  beforeEach(() => {
    FakeBroadcastChannel.open = [];
    vi.stubGlobal('BroadcastChannel', FakeBroadcastChannel);
  });

  it('tells other tabs when this tab signs out', async () => {
    const user = userEvent.setup();
    installFetch({ status: makeAuthenticatedStatus() });

    renderWithProviders(<App />, { route: '/' });
    await screen.findByRole('heading', { name: 'Watchlist' });
    await user.click(screen.getByRole('button', { name: 'Sign out' }));
    await screen.findByRole('heading', { name: 'Sign in' });

    const posted = FakeBroadcastChannel.open.flatMap((channel) => channel.posted);
    expect(posted.map((message) => message.event)).toContain('signed-out');
    expect(posted.every((message) => message.origin.length > 0)).toBe(true);
  });

  it('signs this tab out when another tab signs out', async () => {
    installFetch({ status: makeAuthenticatedStatus() });

    renderWithProviders(<App />, { route: '/dashboard' });
    await screen.findByRole('heading', { name: 'Watchlist' });
    const subscriber = openSubscriber();

    act(() => {
      subscriber.onmessage?.(
        new MessageEvent('message', { data: { event: 'signed-out', origin: 'other-tab' } }),
      );
    });

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument();
  });

  it('re-checks the session when another tab switches accounts', async () => {
    const fetchMock = installFetch({ status: makeAuthenticatedStatus() });

    renderWithProviders(<App />, { route: '/dashboard' });
    await screen.findByRole('heading', { name: 'Watchlist' });
    const subscriber = openSubscriber();

    act(() => {
      subscriber.onmessage?.(
        new MessageEvent('message', { data: { event: 'account-changed', origin: 'other-tab' } }),
      );
    });

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.filter(([input]) => requestPath(input) === '/api/auth/status'),
      ).toHaveLength(2);
    });
  });

  it('clears the previous account watchlist and quotes when another tab switches accounts', async () => {
    let account: 'first' | 'second' = 'first';
    const fetchMock = vi.fn<typeof fetch>((input) => {
      const path = requestPath(input);
      if (path === '/api/auth/status') {
        return Promise.resolve(jsonResponse(makeAuthenticatedStatus()));
      }
      if (path.startsWith('/api/watchlist')) {
        return Promise.resolve(
          jsonResponse({ tickers: account === 'first' ? ['AAPL'] : ['MSFT'] }),
        );
      }
      if (path.startsWith('/api/quotes')) {
        return Promise.resolve(
          jsonResponse({
            quotes:
              account === 'first' ? [makeQuote()] : [makeQuote({ ticker: 'MSFT', price: 420 })],
          }),
        );
      }
      return Promise.resolve(jsonResponse({ error: 'Not found.', code: 'NOT_FOUND' }, 404));
    });
    vi.stubGlobal('fetch', fetchMock);

    renderWithProviders(<App />, { route: '/dashboard' });
    const aaplRow = await screen.findByRole('listitem', { name: 'AAPL' });
    expect(within(aaplRow).getByText('190.12')).toBeInTheDocument();
    const subscriber = openSubscriber();

    account = 'second';
    act(() => {
      subscriber.onmessage?.(
        new MessageEvent('message', { data: { event: 'account-changed', origin: 'other-tab' } }),
      );
    });

    const msftRow = await screen.findByRole('listitem', { name: 'MSFT' });
    expect(within(msftRow).getByText('420.00')).toBeInTheDocument();
    expect(screen.queryByRole('listitem', { name: 'AAPL' })).not.toBeInTheDocument();

    expect(
      fetchMock.mock.calls.filter(([input]) => requestPath(input) === '/api/watchlist'),
    ).toHaveLength(2);
    expect(
      fetchMock.mock.calls.filter(([input]) => requestPath(input) === '/api/quotes'),
    ).toHaveLength(2);
  });
});
