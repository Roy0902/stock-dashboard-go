import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import type { Quote } from '@/api/types';
import { WatchlistList } from '@/features/watchlist/WatchlistList';
import { makeQuote } from '@/test/fixtures';

function renderList(quotes: Quote[], pendingTickers: ReadonlySet<string> = new Set()) {
  const quotesByTicker = new Map(quotes.map((quote) => [quote.ticker, quote]));
  return render(
    <MemoryRouter>
      <WatchlistList
        tickers={['AAPL', 'MSFT']}
        quotesByTicker={quotesByTicker}
        busy={false}
        pendingTickers={pendingTickers}
      />
    </MemoryRouter>,
  );
}

describe('WatchlistList', () => {
  it('keeps a column header for every value column', () => {
    const { container } = renderList([makeQuote(), makeQuote({ ticker: 'MSFT' })]);
    const head = container.querySelector('.watchlist__head');
    expect(head).not.toBeNull();

    for (const header of ['Ticker', 'Price', 'Change', 'Direction', 'Provider', 'Updated']) {
      expect(within(head as HTMLElement).getByText(header)).toBeInTheDocument();
    }
  });

  it('marks a just-added row as pending before its first quote', () => {
    renderList([makeQuote()], new Set(['MSFT']));

    const pending = screen.getByRole('listitem', { name: 'MSFT' });
    expect(pending).toHaveAttribute('aria-busy', 'true');
    expect(within(pending).getByText('Loading price')).toBeInTheDocument();
    expect(within(pending).queryByText('Price unavailable')).not.toBeInTheDocument();

    const quoted = screen.getByRole('listitem', { name: 'AAPL' });
    expect(quoted).not.toHaveAttribute('aria-busy');
  });
});
