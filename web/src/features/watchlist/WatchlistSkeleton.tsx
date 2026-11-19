import { copy } from '@/locale';
import './WatchlistSkeleton.css';

type WatchlistSkeletonProps = {
  rows?: number;
};

export function WatchlistSkeleton({ rows = 3 }: WatchlistSkeletonProps) {
  const placeholders = Array.from({ length: Math.max(rows, 1) }, (_, index) => index);

  return (
    <div className="watchlist-skeleton" role="status">
      <span className="sr-only">{copy.watchlist.loading}</span>
      <div className="watchlist-skeleton__rows" aria-hidden="true">
        {placeholders.map((row) => (
          <div className="watchlist-skeleton__row" key={row}>
            <span className="watchlist-skeleton__cell watchlist-skeleton__cell--ticker" />
            <span className="watchlist-skeleton__cell watchlist-skeleton__cell--price" />
            <span className="watchlist-skeleton__cell watchlist-skeleton__cell--change" />
            <span className="watchlist-skeleton__cell watchlist-skeleton__cell--direction" />
            <span className="watchlist-skeleton__cell watchlist-skeleton__cell--provider" />
            <span className="watchlist-skeleton__cell watchlist-skeleton__cell--updated" />
            <span className="watchlist-skeleton__cell watchlist-skeleton__cell--link" />
          </div>
        ))}
      </div>
    </div>
  );
}
