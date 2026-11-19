import { describe, expect, it } from 'vitest';

import { ApiError } from '@/api/client';
import { isRetryableQueryError, QUERY_RETRY_COUNT } from '@/api/queryKeys';

function apiError(code: string, status: number): ApiError {
  return new ApiError('message', code, status);
}

describe('isRetryableQueryError', () => {
  it('retries network and 5xx failures until the retry cap', () => {
    for (const attempt of [0, 1]) {
      expect(isRetryableQueryError(attempt, apiError('NETWORK_ERROR', 0))).toBe(true);
      expect(isRetryableQueryError(attempt, apiError('INTERNAL_ERROR', 500))).toBe(true);
      expect(isRetryableQueryError(attempt, apiError('PROVIDER_UNAVAILABLE', 502))).toBe(true);
      expect(isRetryableQueryError(attempt, apiError('UPSTREAM_GATEWAY_ERROR', 599))).toBe(true);
    }

    expect(isRetryableQueryError(QUERY_RETRY_COUNT, apiError('NETWORK_ERROR', 0))).toBe(false);
  });

  it('never retries a permanent missing API key', () => {
    for (const attempt of [0, 1]) {
      expect(isRetryableQueryError(attempt, apiError('MISSING_API_KEY', 503))).toBe(false);
    }
  });

  it('treats an auth request downgraded from a network failure as retryable', () => {
    expect(isRetryableQueryError(0, apiError('INTERNAL_ERROR', 0))).toBe(true);
  });

  it('never retries a 4xx answer', () => {
    for (const status of [400, 401, 404, 409, 429]) {
      expect(isRetryableQueryError(0, apiError('INVALID_REQUEST', status))).toBe(false);
    }
  });

  it('does not retry a malformed success body or a non-API error', () => {
    expect(isRetryableQueryError(0, apiError('INTERNAL_ERROR', 200))).toBe(false);
    expect(isRetryableQueryError(0, new Error('boom'))).toBe(false);
  });
});
