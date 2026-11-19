import { afterEach, describe, expect, it, vi } from 'vitest';

import { ApiError, addTicker, getChart, getQuotes, getWatchlist } from '@/api/client';
import { copy } from '@/locale';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

const fetchMock = vi.fn<typeof fetch>();

function installFetch(): void {
  vi.stubGlobal('fetch', fetchMock);
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('getWatchlist', () => {
  it('reads the ticker list', async () => {
    installFetch();
    fetchMock.mockResolvedValue(jsonResponse({ tickers: ['AAPL', 'MSFT'] }));

    const result = await getWatchlist();

    expect(result).toEqual({ ok: true, value: { tickers: ['AAPL', 'MSFT'] } });
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/watchlist',
      expect.objectContaining({ method: 'GET', cache: 'no-store' }),
    );
  });
});

describe('addTicker', () => {
  it('posts the untouched user input and returns the full list', async () => {
    installFetch();
    fetchMock.mockResolvedValue(jsonResponse({ tickers: ['AAPL'] }, 201));

    const result = await addTicker('aapl');

    expect(result).toEqual({ ok: true, value: { tickers: ['AAPL'] } });
    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(init?.method).toBe('POST');
    expect(init?.body).toBe(JSON.stringify({ ticker: 'aapl' }));
  });
});

describe('error handling', () => {
  it('uses the server error copy and code verbatim', async () => {
    installFetch();
    fetchMock.mockResolvedValue(
      jsonResponse(
        { error: 'Too many requests. Try again in 42 seconds.', code: 'RATE_LIMITED' },
        429,
      ),
    );

    const result = await getQuotes();

    expect(result.ok).toBe(false);
    if (result.ok) {
      return;
    }
    expect(result.error).toBeInstanceOf(ApiError);
    expect(result.error.message).toBe('Too many requests. Try again in 42 seconds.');
    expect(result.error.code).toBe('RATE_LIMITED');
    expect(result.error.status).toBe(429);
  });

  it('maps a network failure to the provider-unavailable copy', async () => {
    installFetch();
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));

    const result = await getQuotes();

    expect(result.ok).toBe(false);
    if (result.ok) {
      return;
    }
    expect(result.error.message).toBe(copy.errors.network);
    expect(result.error.code).toBe('NETWORK_ERROR');
  });

  it('rejects an unexpected success body instead of trusting it', async () => {
    installFetch();
    fetchMock.mockResolvedValue(jsonResponse({ quotes: 'not-an-array' }));

    const result = await getQuotes();

    expect(result.ok).toBe(false);
    if (result.ok) {
      return;
    }
    expect(result.error.message).toBe(copy.errors.generic);
    expect(result.error.code).toBe('INTERNAL_ERROR');
  });

  it('falls back to generic copy when an error body has no envelope', async () => {
    installFetch();
    fetchMock.mockResolvedValue(jsonResponse({ detail: 'boom' }, 500));

    const result = await getQuotes();

    expect(result.ok).toBe(false);
    if (result.ok) {
      return;
    }
    expect(result.error.message).toBe(copy.errors.generic);
    expect(result.error.code).toBe('INTERNAL_ERROR');
  });

  it('rethrows aborted requests so TanStack Query can ignore them', async () => {
    installFetch();
    const abortError = new Error('aborted');
    abortError.name = 'AbortError';
    fetchMock.mockRejectedValue(abortError);

    await expect(getQuotes()).rejects.toThrow('aborted');
  });
});

describe('getChart', () => {
  it('encodes the range and parses points', async () => {
    installFetch();
    fetchMock.mockResolvedValue(
      jsonResponse({
        ticker: 'AAPL',
        range: '3M',
        source: 'Twelve Data',
        points: [
          { date: '2026-10-17', close: 189.5 },
          { date: '2026-10-18', close: 190.2 },
        ],
      }),
    );

    const result = await getChart('AAPL', '3M');

    expect(result.ok).toBe(true);
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/chart/AAPL?range=3M');
  });
});
