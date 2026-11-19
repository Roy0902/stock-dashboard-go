import type { PriceRange } from '@/api/types';

/**
 * All HTTP paths are derived from the deployment base so the client can be mounted
 * under a sub-path (Vite `base`) without editing call sites. BASE_URL always ends
 * with a slash; without a custom base it is `/`, so requests stay at `/api/...`.
 */
export function appPath(path: string): string {
  const base = import.meta.env.BASE_URL;
  const prefix = base.endsWith('/') ? base : `${base}/`;
  return `${prefix}${path.replace(/^\//, '')}`;
}

const API_ROOT = 'api';

export const API_ENDPOINTS = {
  watchlist: appPath(`${API_ROOT}/watchlist`),
  quotes: appPath(`${API_ROOT}/quotes`),
  chart: (ticker: string, range: PriceRange) =>
    appPath(`${API_ROOT}/chart/${encodeURIComponent(ticker)}?range=${encodeURIComponent(range)}`),
  auth: {
    status: appPath(`${API_ROOT}/auth/status`),
    setup: appPath(`${API_ROOT}/auth/setup`),
    activateTotp: appPath(`${API_ROOT}/auth/2fa/activate`),
    login: appPath(`${API_ROOT}/auth/login`),
    loginTotp: appPath(`${API_ROOT}/auth/login/totp`),
    loginRecovery: appPath(`${API_ROOT}/auth/login/recovery`),
    logout: appPath(`${API_ROOT}/auth/logout`),
    me: appPath(`${API_ROOT}/auth/me`),
  },
} as const;
