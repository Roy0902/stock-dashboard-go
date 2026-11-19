import { readdir, stat } from 'node:fs/promises';
import path from 'node:path';
import { test, expect } from '../fixtures/test';
import { providerUrl } from '../fixtures/app-server';

test('healthz answers without touching providers', async ({ context, app }) => {
  const before = await context.request.get(`${providerUrl}/__stats`);
  const beforeBody = (await before.json()) as { requests: number };

  const response = await context.request.get(`${app.url}/healthz`);
  expect(response.status()).toBe(200);
  expect(await response.json()).toEqual({ status: 'ok' });

  const after = await context.request.get(`${providerUrl}/__stats`);
  const afterBody = (await after.json()) as { requests: number };
  expect(afterBody.requests).toBe(beforeBody.requests);
});

test('watchlist contract: canonical add, list order and removal', async ({ context, app }) => {
  const added = await context.request.post(`${app.url}/api/watchlist`, { data: { ticker: 'msft' } });
  expect(added.status()).toBe(201);
  expect(await added.json()).toEqual({ tickers: ['MSFT'] });

  const second = await context.request.post(`${app.url}/api/watchlist`, { data: { ticker: 'aapl' } });
  expect(second.status()).toBe(201);
  expect(await second.json()).toEqual({ tickers: ['MSFT', 'AAPL'] });

  const removed = await context.request.delete(`${app.url}/api/watchlist/MSFT`);
  expect(removed.status()).toBe(200);
  expect(await removed.json()).toEqual({ tickers: ['AAPL'] });

  const missing = await context.request.delete(`${app.url}/api/watchlist/MSFT`);
  expect(missing.status()).toBe(404);
  expect(await missing.json()).toMatchObject({ code: 'NOT_IN_WATCHLIST' });
});

test('invalid and duplicate adds expose stable codes', async ({ context, app }) => {
  const invalid = await context.request.post(`${app.url}/api/watchlist`, { data: { ticker: 'AA PL' } });
  expect(invalid.status()).toBe(400);

  const first = await context.request.post(`${app.url}/api/watchlist`, { data: { ticker: 'AAPL' } });
  expect(first.status()).toBe(201);

  const duplicate = await context.request.post(`${app.url}/api/watchlist`, { data: { ticker: 'aapl' } });
  expect(duplicate.status()).toBe(409);
  expect(await duplicate.json()).toEqual({
    error: 'AAPL is already in your watchlist.',
    code: 'DUPLICATE_TICKER',
  });
});

test.describe('quotes with a listed ticker', () => {
  test.use({ appOptions: { watchlist: ['AAPL'] } });

  test('exposes server-derived change, percent and attribution', async ({
    context,
    app,
  }) => {
    const response = await context.request.get(`${app.url}/api/quotes`);
    expect(response.status()).toBe(200);
    const body = (await response.json()) as {
      quotes: Array<Record<string, unknown>>;
    };
    expect(body.quotes).toHaveLength(1);
    expect(body.quotes[0]).toMatchObject({
      ticker: 'AAPL',
      price: 190.12,
      change: 1.24,
      changePercent: 0.66,
      direction: 'up',
      provider: 'Finnhub',
      stale: false,
    });
  });

  test('only the SQLite database is persisted', async ({ context, app }) => {
    await context.request.get(`${app.url}/api/quotes`);
    await context.request.get(`${app.url}/api/chart/AAPL`);

    const entries = (await readdir(app.dataDir)).sort();
    // The e2e fixture seeds a legacy watchlist.json; it is imported once and renamed.
    expect(entries.filter((entry) => !entry.endsWith('.migrated'))).toEqual([
      'stock-dashboard.db',
    ]);
    expect(entries).not.toContain('watchlist.json');
    expect(entries).not.toContain('users.json');

    const database = await stat(path.join(app.dataDir, 'stock-dashboard.db'));
    expect(database.size).toBeGreaterThan(0);
  });
});

test('chart contract: ranges, ordering and validation', async ({ context, app }) => {
  const ok = await context.request.get(`${app.url}/api/chart/AAPL?range=1M`);
  expect(ok.status()).toBe(200);
  const body = (await ok.json()) as {
    ticker: string;
    range: string;
    source: string;
    points: Array<{ date: string; close: number }>;
  };
  expect(body).toMatchObject({ ticker: 'AAPL', range: '1M', source: 'Twelve Data' });
  expect(body.points).toHaveLength(30);
  const dates = body.points.map((point) => point.date);
  expect([...dates].sort()).toEqual(dates);

  const invalid = await context.request.get(`${app.url}/api/chart/AAPL?range=2M`);
  expect(invalid.status()).toBe(400);
  expect(await invalid.json()).toEqual({
    error: 'Range must be 1M, 3M or 1Y.',
    code: 'INVALID_RANGE',
  });
});

test('unknown symbols are refused for both add and chart', async ({ context, app }) => {
  const add = await context.request.post(`${app.url}/api/watchlist`, { data: { ticker: 'ZZZZ' } });
  expect(add.status()).toBe(404);
  expect(await add.json()).toMatchObject({ code: 'UNKNOWN_SYMBOL' });

  const chart = await context.request.get(`${app.url}/api/chart/ZZZZ`);
  expect(chart.status()).toBe(404);
  expect(await chart.json()).toMatchObject({ code: 'UNKNOWN_SYMBOL' });
});

test('an unknown api path returns the error envelope', async ({ context, app }) => {
  const response = await context.request.get(`${app.url}/api/nope`);
  expect(response.status()).toBe(404);
  expect(await response.json()).toMatchObject({ code: 'NOT_FOUND' });
});

test('the SPA shell answers a deep link', async ({ context, app }) => {
  const response = await context.request.get(`${app.url}/chart/AAPL`);
  expect(response.status()).toBe(200);
  expect(await response.text()).toContain('<div id="root">');
});

test.describe('without API keys', () => {
  test.use({ appOptions: { watchlist: ['AAPL'], keys: false } });

  test('quote and chart endpoints refuse while health stays green', async ({
    context,
    app,
  }) => {
    const quotes = await context.request.get(`${app.url}/api/quotes`);
    expect(quotes.status()).toBe(503);
    expect(await quotes.json()).toMatchObject({ code: 'MISSING_API_KEY' });

    const chart = await context.request.get(`${app.url}/api/chart/AAPL`);
    expect(chart.status()).toBe(503);
    expect(await chart.json()).toMatchObject({ code: 'MISSING_API_KEY' });

    const health = await context.request.get(`${app.url}/healthz`);
    expect(health.status()).toBe(200);
  });
});


