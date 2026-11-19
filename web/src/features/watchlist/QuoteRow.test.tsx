import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import type { Quote } from '@/api/types';
import { QuoteRow } from '@/features/watchlist/QuoteRow';
import { makeQuote } from '@/test/fixtures';

function renderRow(quote: Quote | null, pending = false) {
  render(
    <MemoryRouter>
      <ul>
        <QuoteRow ticker="AAPL" quote={quote} pending={pending} />
      </ul>
    </MemoryRouter>,
  );

  return screen.getByRole('listitem', { name: 'AAPL' });
}

describe('QuoteRow', () => {
  it('renders a positive quote with sign, percent, provider and timestamp', () => {
    const row = renderRow(makeQuote());

    expect(within(row).getByText('190.12')).toBeInTheDocument();
    const change = within(row).getByText('+1.24');
    expect(change).toHaveClass('quote-row__change--up');
    expect(within(row).getByText('+0.66%')).toHaveClass('quote-row__percent--up');
    expect(within(row).getByText('up')).toBeInTheDocument();
    expect(within(row).getByText('Finnhub')).toBeInTheDocument();
    expect(within(row).getByText('16-Nov-2026')).toBeInTheDocument();
  });

  it('renders a negative quote with a minus sign and the down class', () => {
    const row = renderRow(
      makeQuote({ price: 190, change: -10, changePercent: -5, direction: 'down' }),
    );

    const change = within(row).getByText('-10.00');
    expect(change).toHaveClass('quote-row__change--down');
    expect(within(row).getByText('-5.00%')).toHaveClass('quote-row__percent--down');
    expect(within(row).getByText('down')).toBeInTheDocument();
  });

  it('renders an unchanged quote with the neutral class and direction word', () => {
    const row = renderRow(
      makeQuote({ price: 100, change: 0, changePercent: 0, direction: 'unchanged' }),
    );

    const change = within(row).getByText('0.00');
    expect(change).toHaveClass('quote-row__change--unchanged');
    expect(within(row).getByText('0.00%')).toHaveClass('quote-row__percent--unchanged');
    expect(within(row).getByText('unchanged')).toBeInTheDocument();
  });

  it('labels every value cell for assistive technology', () => {
    const row = renderRow(makeQuote());

    for (const label of ['Price', 'Change', 'Direction', 'Provider', 'Updated']) {
      expect(within(row).getByText(label)).toHaveClass('sr-only');
    }
  });

  it('appends the stale suffix to the raw timestamp', () => {
    const row = renderRow(
      makeQuote({ timestamp: '2026-11-16T14:00:00Z', stale: true, provider: 'Twelve Data' }),
    );

    expect(within(row).getByText('16-Nov-2026 (stale)')).toBeInTheDocument();
    expect(within(row).getByText('Twelve Data')).toBeInTheDocument();
  });

  it('hides the percentage when changePercent is null', () => {
    const row = renderRow(makeQuote({ changePercent: null, price: 50, change: 0 }));

    expect(within(row).queryByText(/%/)).not.toBeInTheDocument();
    expect(row.querySelector('.quote-row__percent')).toBeNull();
    expect(within(row).getByText('50.00')).toBeInTheDocument();
    expect(within(row).getByText('0.00')).toBeInTheDocument();
  });

  it('shows zero price and change with no percentage for a zero previous close', () => {
    const row = renderRow(
      makeQuote({
        price: 0,
        change: 0,
        changePercent: null,
        direction: 'unchanged',
      }),
    );

    expect(within(row).getByText('0.00', { selector: '.quote-row__price' })).toBeInTheDocument();
    expect(within(row).getByText('0.00', { selector: '.quote-row__change' })).toBeInTheDocument();
    expect(row.querySelector('.quote-row__percent')).toBeNull();
    expect(within(row).getByText('unchanged')).toBeInTheDocument();
  });

  it('shows an unavailable row without any price number', () => {
    const row = renderRow(null);

    expect(within(row).getByText('Price unavailable')).toBeInTheDocument();
    expect(within(row).getByText('none')).toBeInTheDocument();
    expect(within(row).queryByText(/^\d+\.\d{2}$/)).not.toBeInTheDocument();
    expect(within(row).queryByText(/^(up|down|unchanged)$/)).not.toBeInTheDocument();
    expect(within(row).getByRole('link', { name: 'View chart' })).toHaveAttribute(
      'href',
      '/chart/AAPL',
    );
  });

  it('shows a pending price skeleton instead of the unavailable copy after an add', () => {
    const row = renderRow(null, true);

    expect(row).toHaveAttribute('aria-busy', 'true');
    expect(within(row).getByText('Loading price')).toHaveClass('sr-only');
    expect(within(row).queryByText('Price unavailable')).not.toBeInTheDocument();
    expect(row.querySelector('.quote-row__price--pending')).not.toBeNull();
    expect(within(row).getByRole('link', { name: 'View chart' })).toHaveAttribute(
      'href',
      '/chart/AAPL',
    );
  });

  it('links to the ticker chart from inside the row', () => {
    const row = renderRow(makeQuote());

    expect(within(row).getByRole('link', { name: 'View chart' })).toHaveAttribute(
      'href',
      '/chart/AAPL',
    );
  });
});
