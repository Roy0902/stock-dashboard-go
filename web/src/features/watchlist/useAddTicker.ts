import { useMutation, useQueryClient } from '@tanstack/react-query';

import { addTicker, unwrap } from '@/api/client';
import type { ApiError } from '@/api/client';
import { queryKeys } from '@/api/queryKeys';
import type { Watchlist } from '@/api/types';

export function useAddTicker() {
  const queryClient = useQueryClient();

  return useMutation<Watchlist, ApiError, string>({
    mutationFn: async (ticker) => unwrap(await addTicker(ticker)),
    onSuccess: (watchlist) => {
      queryClient.setQueryData(queryKeys.watchlist, watchlist);
      void queryClient.invalidateQueries({ queryKey: queryKeys.quotes });
    },
  });
}
