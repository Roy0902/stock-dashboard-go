import { useMemo, useState } from 'react';

import type { Quote } from '@/api/types';
import { AddTickerForm } from '@/features/watchlist/AddTickerForm';
import { WatchlistList } from '@/features/watchlist/WatchlistList';
import { WatchlistSkeleton } from '@/features/watchlist/WatchlistSkeleton';
import { useWatchlist } from '@/features/watchlist/useWatchlist';
import { LastUpdated } from '@/features/quotes/LastUpdated';
import { useQuotes } from '@/features/quotes/useQuotes';
import { Alert } from '@/ui/Alert';
import { PageHeader } from '@/ui/PageHeader';
import { copy } from '@/locale';
import './DashboardPage.css';

/** Drop adds settled by a quote (or by a failed batch) and keep the rest pending. */
function prunePendingAdds(
  pendingAdds: ReadonlyMap<string, number>,
  quotesByTicker: ReadonlyMap<string, Quote>,
  quotesFailed: boolean,
  dataUpdatedAt: number,
): ReadonlyMap<string, number> {
  if (pendingAdds.size === 0) return pendingAdds;

  const next = new Map<string, number>();
  for (const [ticker, since] of pendingAdds) {
    if (!quotesByTicker.has(ticker) && !quotesFailed && dataUpdatedAt <= since) {
      next.set(ticker, since);
    }
  }
  return next.size === pendingAdds.size ? pendingAdds : next;
}

export function DashboardPage() {
  const watchlist = useWatchlist();
  const quotes = useQuotes();
  const [formError, setFormError] = useState<string | null>(null);
  const [pendingAdds, setPendingAdds] = useState<ReadonlyMap<string, number>>(() => new Map());

  const tickers = watchlist.data?.tickers ?? [];
  const quotesByTicker = useMemo(() => {
    const index = new Map<string, Quote>();
    for (const quote of quotes.data?.quotes ?? []) {
      index.set(quote.ticker, quote);
    }
    return index;
  }, [quotes.data]);

  const requestError = formError ?? quotes.error?.message ?? watchlist.error?.message ?? null;
  const isEmpty = watchlist.isSuccess && tickers.length === 0;
  const message = requestError ?? (isEmpty ? copy.dashboard.empty : null);
  const showSkeleton = watchlist.isPending || (tickers.length > 0 && quotes.isPending);

  // Every just-added ticker stays pending until a later quotes response (or an error)
  // settles it; a second add must not replace the first.
  const activePendingAdds = useMemo(
    () => prunePendingAdds(pendingAdds, quotesByTicker, quotes.isError, quotes.dataUpdatedAt),
    [pendingAdds, quotesByTicker, quotes.isError, quotes.dataUpdatedAt],
  );
  const pendingTickers = useMemo(() => new Set(activePendingAdds.keys()), [activePendingAdds]);

  return (
    <div className="dashboard">
      <PageHeader title={copy.dashboard.title}>
        <LastUpdated at={showSkeleton ? 0 : quotes.dataUpdatedAt} />
      </PageHeader>

      <AddTickerForm
        onError={setFormError}
        onAdded={(ticker) => {
          setPendingAdds((current) => {
            const next = new Map(
              prunePendingAdds(current, quotesByTicker, quotes.isError, quotes.dataUpdatedAt),
            );
            next.set(ticker, quotes.dataUpdatedAt);
            return next;
          });
        }}
      />

      {message === null ? null : (
        <Alert
          tone={requestError === null ? 'neutral' : 'error'}
          className={requestError === null ? 'dashboard__empty' : undefined}
        >
          {message}
        </Alert>
      )}

      {showSkeleton ? (
        <WatchlistSkeleton rows={Math.max(tickers.length, 3)} />
      ) : tickers.length > 0 ? (
        <WatchlistList
          tickers={tickers}
          quotesByTicker={quotesByTicker}
          busy={quotes.isFetching}
          pendingTickers={pendingTickers}
        />
      ) : null}
    </div>
  );
}
