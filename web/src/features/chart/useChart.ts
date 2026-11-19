import { useQuery } from '@tanstack/react-query';

import { getChart, unwrap } from '@/api/client';
import type { ApiError } from '@/api/client';
import { queryKeys } from '@/api/queryKeys';
import type { ChartSeries, PriceRange } from '@/api/types';

export function useChart(ticker: string, range: PriceRange) {
  return useQuery<ChartSeries, ApiError>({
    queryKey: queryKeys.chart(ticker, range),
    queryFn: async ({ signal }) => unwrap(await getChart(ticker, range, signal)),
    enabled: ticker.length > 0,
    staleTime: 60_000,
  });
}
