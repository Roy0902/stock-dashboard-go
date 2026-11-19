import { useQuery } from '@tanstack/react-query';

import { getWatchlist, unwrap } from '@/api/client';
import type { ApiError } from '@/api/client';
import { queryKeys } from '@/api/queryKeys';
import type { Watchlist } from '@/api/types';

export function useWatchlist() {
  return useQuery<Watchlist, ApiError>({
    queryKey: queryKeys.watchlist,
    queryFn: async ({ signal }) => unwrap(await getWatchlist(signal)),
    staleTime: 30_000,
  });
}
