import { describe, expect, it } from 'vitest';

import { resolvePostAuthPath, ROUTE_PATHS } from '@/routes';

describe('resolvePostAuthPath', () => {
  it.each([
    ['protocol-relative', { from: '//evil.com' }],
    ['absolute URL', { from: 'https://evil.com' }],
    ['backslash escape', { from: '/\\evil.com' }],
    ['non-string from', { from: 42 }],
    ['missing from', { other: '/dashboard' }],
    ['non-object state', 'not-state'],
  ])('falls back to the dashboard for %s', (_label, state) => {
    expect(resolvePostAuthPath(state)).toBe(ROUTE_PATHS.dashboard);
  });

  it('accepts a normal in-app deep link', () => {
    expect(resolvePostAuthPath({ from: '/chart/AAPL?range=1M' })).toBe('/chart/AAPL?range=1M');
    expect(resolvePostAuthPath({ from: '/dashboard' })).toBe('/dashboard');
  });
});
