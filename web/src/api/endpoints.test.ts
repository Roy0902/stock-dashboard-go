import { afterEach, describe, expect, it, vi } from 'vitest';

import { API_ENDPOINTS, appPath } from '@/api/endpoints';

afterEach(() => {
  vi.unstubAllEnvs();
});

describe('endpoint portability', () => {
  it('keeps the root API paths by default', () => {
    expect(API_ENDPOINTS.watchlist).toBe('/api/watchlist');
    expect(API_ENDPOINTS.quotes).toBe('/api/quotes');
    expect(API_ENDPOINTS.auth.status).toBe('/api/auth/status');
    expect(API_ENDPOINTS.chart('AAPL', '3M')).toBe('/api/chart/AAPL?range=3M');
  });

  it('rebases every path on the deployment BASE_URL', () => {
    vi.stubEnv('BASE_URL', '/dashboard/');

    expect(appPath('api/watchlist')).toBe('/dashboard/api/watchlist');
    expect(appPath('/api/watchlist')).toBe('/dashboard/api/watchlist');
  });

  it('encodes tickers and ranges in the chart path', () => {
    expect(API_ENDPOINTS.chart('BRK.B/X', '1Y')).toBe('/api/chart/BRK.B%2FX?range=1Y');
  });
});
