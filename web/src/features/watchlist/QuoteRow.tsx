import { Link } from 'react-router-dom';

import type { Quote } from '@/api/types';
import { chartPath } from '@/routes';
import { formatPercent, formatPrice, formatSigned, formatTimestamp } from '@/lib/format';
import { copy } from '@/locale';
import './QuoteRow.css';

type QuoteRowProps = {
  ticker: string;
  quote: Quote | null;
  pending?: boolean;
};

export function QuoteRow({ ticker, quote, pending = false }: QuoteRowProps) {
  const state = quote !== null ? 'quoted' : pending ? 'pending' : 'unavailable';

  return (
    <li
      className={`quote-row quote-row--${state}`}
      aria-label={ticker}
      aria-busy={state === 'pending' ? true : undefined}
    >
      <span className="quote-row__ticker">{ticker}</span>

      {state === 'pending' ? (
        <>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.price}</span>
            <span className="quote-row__price quote-row__price--pending">
              <span className="quote-row__pending-bar" aria-hidden="true" />
              <span className="sr-only">{copy.watchlist.loadingPrice}</span>
            </span>
          </div>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.change}</span>
            <span className="quote-row__change-group">
              <span
                className="quote-row__pending-bar quote-row__pending-bar--short"
                aria-hidden="true"
              />
            </span>
          </div>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.direction}</span>
            <span
              className="quote-row__pending-bar quote-row__pending-bar--short quote-row__pending-bar--direction"
              aria-hidden="true"
            />
          </div>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.provider}</span>
            <span
              className="quote-row__pending-bar quote-row__pending-bar--provider"
              aria-hidden="true"
            />
          </div>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.updated}</span>
            <span
              className="quote-row__pending-bar quote-row__pending-bar--short quote-row__pending-bar--timestamp"
              aria-hidden="true"
            />
          </div>
        </>
      ) : state === 'unavailable' ? (
        <>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.price}</span>
            <span className="quote-row__price quote-row__price--unavailable">
              {copy.watchlist.priceUnavailable}
            </span>
          </div>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.change}</span>
            <span className="quote-row__change-group" aria-hidden="true">
              <span className="quote-row__empty">{copy.watchlist.emptyValue}</span>
            </span>
          </div>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.direction}</span>
            <span className="quote-row__direction quote-row__direction--empty" aria-hidden="true">
              {copy.watchlist.emptyValue}
            </span>
          </div>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.provider}</span>
            <span className="quote-row__provider">{copy.watchlist.noProvider}</span>
          </div>
          <div className="quote-row__field">
            <span className="sr-only">{copy.watchlist.columns.updated}</span>
            <span className="quote-row__timestamp quote-row__timestamp--empty" aria-hidden="true">
              {copy.watchlist.emptyValue}
            </span>
          </div>
        </>
      ) : (
        quote !== null && (
          <>
            <div className="quote-row__field">
              <span className="sr-only">{copy.watchlist.columns.price}</span>
              <span className="quote-row__price">{formatPrice(quote.price)}</span>
            </div>
            <div className="quote-row__field">
              <span className="sr-only">{copy.watchlist.columns.change}</span>
              <span className="quote-row__change-group">
                <span className={`quote-row__change quote-row__change--${quote.direction}`}>
                  {formatSigned(quote.change)}
                </span>
                {quote.changePercent === null ? null : (
                  <span className={`quote-row__percent quote-row__percent--${quote.direction}`}>
                    {formatPercent(quote.changePercent)}
                  </span>
                )}
              </span>
            </div>
            <div className="quote-row__field">
              <span className="sr-only">{copy.watchlist.columns.direction}</span>
              <span className={`quote-row__direction quote-row__direction--${quote.direction}`}>
                <span className="quote-row__direction-dot" aria-hidden="true" />
                {quote.direction}
              </span>
            </div>
            <div className="quote-row__field">
              <span className="sr-only">{copy.watchlist.columns.provider}</span>
              <span className="quote-row__provider">{quote.provider}</span>
            </div>
            <div className="quote-row__field">
              <span className="sr-only">{copy.watchlist.columns.updated}</span>
              <time className="quote-row__timestamp" dateTime={quote.timestamp}>
                {formatTimestamp(quote.timestamp, quote.stale)}
              </time>
            </div>
          </>
        )
      )}

      <Link className="quote-row__link" to={chartPath(ticker)}>
        {copy.watchlist.viewChart}
        <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
          <path
            d="M3 8h9M8.5 3.5 13 8l-4.5 4.5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </Link>
    </li>
  );
}
