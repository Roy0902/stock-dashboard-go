import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { App } from '@/App';
import { makeUnauthenticatedStatus } from '@/test/authFixtures';
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

function installFetch() {
  const fetchMock = vi.fn<typeof fetch>((input) => {
    if (requestPath(input) === '/api/auth/status') {
      return Promise.resolve(jsonResponse(makeUnauthenticatedStatus()));
    }
    return Promise.resolve(jsonResponse({ error: 'Not found.', code: 'NOT_FOUND' }, 404));
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

beforeEach(() => {
  vi.unstubAllGlobals();
  // jsdom has no scrolling; the implementation is asserted where it matters.
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function mainElement(): HTMLElement {
  return screen.getByRole('main');
}

describe('App route focus', () => {
  it('leaves initial focus on the document so the skip link comes first', async () => {
    installFetch();
    renderWithProviders(<App />, { route: '/login' });

    const heading = await screen.findByRole('heading', { name: 'Sign in' });
    expect(document.body).toHaveFocus();
    expect(heading).not.toHaveFocus();
    expect(mainElement()).not.toHaveFocus();
  });

  it('moves focus to #main-content and resets scroll after a route change', async () => {
    const user = userEvent.setup();
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined);
    installFetch();
    renderWithProviders(<App />, { route: '/login' });

    await screen.findByRole('heading', { name: 'Sign in' });
    expect(mainElement()).not.toHaveFocus();

    await user.click(screen.getByRole('link', { name: 'Stock dashboard home' }));
    expect(
      await screen.findByRole('heading', {
        name: 'Your watchlist, quotes and daily charts in one calm place.',
      }),
    ).toBeInTheDocument();

    expect(mainElement()).toHaveFocus();
    expect(scrollTo).toHaveBeenCalledWith(0, 0);
  });

  it('moves route-change focus to main when an auth page mounts', async () => {
    const user = userEvent.setup();
    installFetch();
    renderWithProviders(<App />, { route: '/login' });

    await screen.findByRole('heading', { name: 'Sign in' });
    await user.click(screen.getByRole('link', { name: 'Set up your account' }));

    const heading = await screen.findByRole('heading', { name: 'Create account' });
    expect(heading).not.toHaveFocus();
    expect(mainElement()).toHaveFocus();
  });
});
