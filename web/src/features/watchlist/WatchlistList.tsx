import type { Quote } from '@/api/types';
import { QuoteRow } from '@/features/watchlist/QuoteRow';
import { copy } from '@/locale';
import './WatchlistList.css';

const EMPTY_PENDING: ReadonlySet<string> = new Set();

type WatchlistListProps = {
  tickers: string[];
  quotesByTicker: ReadonlyMap<string, Quote>;
  busy: boolean;
  pendingTickers?: ReadonlySet<string>;
};

export function WatchlistList({
  tickers,
  quotesByTicker,
  busy,
  pendingTickers = EMPTY_PENDING,
}: WatchlistListProps) {
  return (
    <div className="watchlist" aria-busy={busy}>
      <span className="watchlist__activity" data-busy={busy} aria-hidden="true" />
      <div className="watchlist__head">
        <span>{copy.watchlist.columns.ticker}</span>
        <span className="watchlist__head-numeric">{copy.watchlist.columns.price}</span>
        <span className="watchlist__head-numeric">{copy.watchlist.columns.change}</span>
        <span>{copy.watchlist.columns.direction}</span>
        <span>{copy.watchlist.columns.provider}</span>
        <span>{copy.watchlist.columns.updated}</span>
        <span />
      </div>
      <ul className="watchlist__list" aria-label={copy.watchlist.listLabel}>
        {tickers.map((ticker) => (
          <QuoteRow
            key={ticker}
            ticker={ticker}
            quote={quotesByTicker.get(ticker) ?? null}
            pending={pendingTickers.has(ticker)}
          />
        ))}
      </ul>
    </div>
  );
}
