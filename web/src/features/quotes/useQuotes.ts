import { useQuery } from '@tanstack/react-query';

import { unwrap, getQuotes } from '@/api/client';
import { queryKeys } from '@/api/queryKeys';
import type { ApiError } from '@/api/client';
import type { QuotesResponse } from '@/api/types';
import { useDocumentVisible } from '@/features/quotes/useDocumentVisible';

const QUOTE_POLL_INTERVAL_MS = 60_000;

export function useQuotes() {
  const isVisible = useDocumentVisible();

  return useQuery<QuotesResponse, ApiError>({
    queryKey: queryKeys.quotes,
    queryFn: async ({ signal }) => unwrap(await getQuotes(signal)),
    // One polling mechanism: the interval exists only while the document is visible, so a
    // hidden tab stops polling and a visible one resumes on the next 60 s tick.
    refetchInterval: isVisible ? QUOTE_POLL_INTERVAL_MS : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: false,
  });
}
