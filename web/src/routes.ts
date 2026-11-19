export const ROUTE_PATHS = {
  landing: '/',
  dashboard: '/dashboard',
  chart: '/chart/:ticker',
  setup: '/setup',
  login: '/login',
} as const;

export function chartPath(ticker: string): string {
  return ROUTE_PATHS.chart.replace(':ticker', encodeURIComponent(ticker));
}

/**
 * The single owner of post-sign-in navigation is the route guard, which reads the path the
 * guard itself stored in router state. Only in-app absolute paths are restored.
 */
export function resolvePostAuthPath(state: unknown): string {
  if (typeof state !== 'object' || state === null) {
    return ROUTE_PATHS.dashboard;
  }
  const from = (state as { from?: unknown }).from;
  if (
    typeof from !== 'string' ||
    !from.startsWith('/') ||
    from.startsWith('//') ||
    from.includes('\\')
  ) {
    return ROUTE_PATHS.dashboard;
  }
  return from;
}
