import { ApiError } from '@/api/client';
import type { PriceRange } from '@/api/types';

export const queryKeys = {
  watchlist: ['watchlist'] as const,
  quotes: ['quotes'] as const,
  chart: (ticker: string, range: PriceRange) => ['chart', ticker, range] as const,
};

/** Two retries after the first failure, matching the review's resilience target. */
export const QUERY_RETRY_COUNT = 2;

/**
 * Only transient failures are retried. A 4xx means the request itself was wrong, so a retry
 * would produce the same answer; a malformed success body is `INTERNAL_ERROR` with the
 * response status and is also not retried. Auth requests downgrade a network failure to
 * `INTERNAL_ERROR` with status 0, which stays retryable.
 */
export function isRetryableQueryError(failureCount: number, error: unknown): boolean {
  if (failureCount >= QUERY_RETRY_COUNT) return false;
  if (!(error instanceof ApiError)) return false;
  // A missing key is a configuration problem: retrying cannot fix it.
  if (error.code === 'MISSING_API_KEY') return false;
  if (error.code === 'NETWORK_ERROR' || error.status >= 500) return true;
  return error.status === 0 && error.code === 'INTERNAL_ERROR';
}
