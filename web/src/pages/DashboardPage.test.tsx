import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, addTicker, getQuotes, getWatchlist } from '@/api/client';
import type * as ApiClientModule from '@/api/client';
import { makeQuote } from '@/test/fixtures';
import { renderWithProviders } from '@/test/render';
import { DashboardPage } from '@/pages/DashboardPage';

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof ApiClientModule>();
  return {
    ...actual,
    getWatchlist: vi.fn(),
    getQuotes: vi.fn(),
    addTicker: vi.fn(),
  };
});

const getWatchlistMock = vi.mocked(getWatchlist);
const getQuotesMock = vi.mocked(getQuotes);
const addTickerMock = vi.mocked(addTicker);

beforeEach(() => {
  getWatchlistMock.mockReset();
  getQuotesMock.mockReset();
  addTickerMock.mockReset();
  getWatchlistMock.mockResolvedValue({ ok: true, value: { tickers: [] } });
  getQuotesMock.mockResolvedValue({ ok: true, value: { quotes: [] } });
});

afterEach(() => {
  vi.useRealTimers();
});

describe('DashboardPage', () => {
  it('shows the exact empty-state alert and no rows', async () => {
    renderWithProviders(<DashboardPage />);

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Your watchlist is empty. Add a ticker to begin.');
    expect(screen.getAllByRole('alert')).toHaveLength(1);
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
  });

  it('renders exactly one named row per watchlist ticker', async () => {
    getWatchlistMock.mockResolvedValue({ ok: true, value: { tickers: ['AAPL', 'MSFT'] } });
    getQuotesMock.mockResolvedValue({
      ok: true,
      value: {
        quotes: [
          makeQuote(),
          makeQuote({
            ticker: 'MSFT',
            price: 420,
            change: -2.5,
            changePercent: -0.59,
            direction: 'down',
            provider: 'Twelve Data',
          }),
        ],
      },
    });

    renderWithProviders(<DashboardPage />);

    const rows = await screen.findAllByRole('listitem');
    expect(rows).toHaveLength(2);
    expect(screen.getByRole('listitem', { name: 'AAPL' })).toBeInTheDocument();
    expect(screen.getByRole('listitem', { name: 'MSFT' })).toBeInTheDocument();
    expect(within(screen.getByRole('listitem', { name: 'MSFT' })).getByText('-2.50')).toHaveClass(
      'quote-row__change--down',
    );
  });

  it('reports the last update time in a status element', async () => {
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date(2026, 10, 16, 14, 30, 0));
    getWatchlistMock.mockResolvedValue({ ok: true, value: { tickers: ['AAPL'] } });
    getQuotesMock.mockResolvedValue({ ok: true, value: { quotes: [makeQuote()] } });

    renderWithProviders(<DashboardPage />);

    expect(await screen.findByText('Last updated 14:30:00')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('Last updated 14:30:00');
  });

  it('shows the server error copy verbatim when quotes fail', async () => {
    const cases = [
      {
        copy: 'Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.',
        code: 'MISSING_API_KEY',
        status: 503,
      },
      {
        copy: 'Data provider is unavailable. Try again shortly.',
        code: 'PROVIDER_UNAVAILABLE',
        status: 502,
      },
      {
        copy: 'Too many requests. Try again in 30 seconds.',
        code: 'RATE_LIMITED',
        status: 429,
      },
    ] as const;

    for (const item of cases) {
      getWatchlistMock.mockResolvedValue({ ok: true, value: { tickers: ['AAPL'] } });
      getQuotesMock.mockResolvedValue({
        ok: false,
        error: new ApiError(item.copy, item.code, item.status),
      });

      const { unmount } = renderWithProviders(<DashboardPage />);
      const alert = await screen.findByRole('alert');
      expect(alert).toHaveTextContent(item.copy);
      expect(screen.getAllByRole('alert')).toHaveLength(1);
      expect(screen.getByRole('listitem', { name: 'AAPL' })).toHaveTextContent('Price unavailable');
      unmount();
      getWatchlistMock.mockReset();
      getQuotesMock.mockReset();
    }
  });

  it('renders an unavailable row when the batch omits a ticker', async () => {
    getWatchlistMock.mockResolvedValue({ ok: true, value: { tickers: ['AAPL', 'MSFT'] } });
    getQuotesMock.mockResolvedValue({ ok: true, value: { quotes: [makeQuote()] } });

    renderWithProviders(<DashboardPage />);

    const row = await screen.findByRole('listitem', { name: 'MSFT' });
    expect(within(row).getByText('Price unavailable')).toBeInTheDocument();
    expect(within(row).getByText('none')).toBeInTheDocument();
    expect(within(row).queryByText(/^\d+\.\d{2}$/)).not.toBeInTheDocument();
  });

  it('shows add-form errors from the API in the single alert slot', async () => {
    const user = userEvent.setup();
    const cases = [
      {
        input: '1AAPL',
        copy: 'Enter a valid ticker, e.g. AAPL.',
        code: 'INVALID_TICKER',
        status: 400,
      },
      {
        input: 'aapl',
        copy: 'AAPL is already in your watchlist.',
        code: 'DUPLICATE_TICKER',
        status: 409,
      },
      {
        input: 'ZZZZ',
        copy: 'Symbol not found.',
        code: 'UNKNOWN_SYMBOL',
        status: 404,
      },
    ] as const;

    for (const item of cases) {
      addTickerMock.mockResolvedValue({
        ok: false,
        error: new ApiError(item.copy, item.code, item.status),
      });

      const { unmount } = renderWithProviders(<DashboardPage />);
      await screen.findByText('Your watchlist is empty. Add a ticker to begin.');
      await user.type(screen.getByRole('textbox', { name: 'Ticker' }), item.input);
      await user.click(screen.getByRole('button', { name: 'Add ticker' }));

      const alert = await screen.findByRole('alert');
      expect(alert).toHaveTextContent(item.copy);
      expect(screen.getAllByRole('alert')).toHaveLength(1);
      unmount();
    }
  });

  it('adds a ticker, clears the field and refreshes the row', async () => {
    const user = userEvent.setup();
    getQuotesMock
      .mockResolvedValueOnce({ ok: true, value: { quotes: [] } })
      .mockResolvedValue({ ok: true, value: { quotes: [makeQuote()] } });
    addTickerMock.mockResolvedValue({ ok: true, value: { tickers: ['AAPL'] } });

    renderWithProviders(<DashboardPage />);
    await screen.findByText('Your watchlist is empty. Add a ticker to begin.');

    const input = screen.getByRole('textbox', { name: 'Ticker' });
    await user.type(input, 'aapl');
    await user.click(screen.getByRole('button', { name: 'Add ticker' }));

    const row = await screen.findByRole('listitem', { name: 'AAPL' });
    expect(row).toHaveTextContent('190.12');
    await waitFor(() => {
      expect(input).toHaveValue('');
    });
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('keeps two rapidly added tickers pending until the batch answers', async () => {
    const user = userEvent.setup();
    let resolveQuotes: (value: Awaited<ReturnType<typeof getQuotes>>) => void = () => undefined;
    const pendingQuotes = new Promise<Awaited<ReturnType<typeof getQuotes>>>((resolve) => {
      resolveQuotes = resolve;
    });
    getQuotesMock.mockImplementationOnce(() =>
      Promise.resolve({ ok: true, value: { quotes: [] } }),
    );
    getQuotesMock.mockImplementation(() => pendingQuotes);
    addTickerMock
      .mockResolvedValueOnce({ ok: true, value: { tickers: ['AAPL'] } })
      .mockResolvedValueOnce({ ok: true, value: { tickers: ['AAPL', 'MSFT'] } });

    renderWithProviders(<DashboardPage />);
    await screen.findByText('Your watchlist is empty. Add a ticker to begin.');

    const input = screen.getByRole('textbox', { name: 'Ticker' });
    await user.type(input, 'aapl');
    await user.click(screen.getByRole('button', { name: 'Add ticker' }));

    const aapl = await screen.findByRole('listitem', { name: 'AAPL' });
    expect(aapl).toHaveAttribute('aria-busy', 'true');
    expect(within(aapl).getByText('Loading price')).toBeInTheDocument();
    await waitFor(() => {
      expect(input).toHaveValue('');
    });

    await user.type(input, 'msft');
    await user.click(screen.getByRole('button', { name: 'Add ticker' }));

    const msft = await screen.findByRole('listitem', { name: 'MSFT' });
    expect(msft).toHaveAttribute('aria-busy', 'true');
    expect(within(msft).getByText('Loading price')).toBeInTheDocument();
    expect(aapl).toHaveAttribute('aria-busy', 'true');
    expect(within(aapl).getByText('Loading price')).toBeInTheDocument();

    await act(async () => {
      resolveQuotes({
        ok: true,
        value: { quotes: [makeQuote(), makeQuote({ ticker: 'MSFT', price: 420 })] },
      });
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(within(aapl).getByText('190.12')).toBeInTheDocument();
      expect(within(msft).getByText('420.00')).toBeInTheDocument();
    });
    expect(aapl).not.toHaveAttribute('aria-busy');
    expect(msft).not.toHaveAttribute('aria-busy');
  });

  it('shows a pending price cell on a new ticker until its first quote arrives', async () => {
    const user = userEvent.setup();
    getWatchlistMock.mockResolvedValue({ ok: true, value: { tickers: [] } });
    getQuotesMock.mockResolvedValueOnce({ ok: true, value: { quotes: [] } });
    addTickerMock.mockResolvedValue({ ok: true, value: { tickers: ['AAPL'] } });

    let resolveQuotes: (value: Awaited<ReturnType<typeof getQuotes>>) => void = () => undefined;
    getQuotesMock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveQuotes = resolve;
        }),
    );

    renderWithProviders(<DashboardPage />);
    await screen.findByText('Your watchlist is empty. Add a ticker to begin.');

    await user.type(screen.getByRole('textbox', { name: 'Ticker' }), 'aapl');
    await user.click(screen.getByRole('button', { name: 'Add ticker' }));

    const row = await screen.findByRole('listitem', { name: 'AAPL' });
    expect(within(row).queryByText('Price unavailable')).not.toBeInTheDocument();
    expect(row).toHaveAttribute('aria-busy', 'true');
    expect(within(row).getByText('Loading price')).toBeInTheDocument();

    await act(async () => {
      resolveQuotes({ ok: true, value: { quotes: [makeQuote()] } });
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(within(row).getByText('190.12')).toBeInTheDocument();
    });
    expect(within(row).queryByText('Price unavailable')).not.toBeInTheDocument();
  });
});
