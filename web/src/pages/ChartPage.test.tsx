import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiError, getAuthStatus, getChart } from '@/api/client';
import type * as ApiClientModule from '@/api/client';
import type { PriceRange } from '@/api/types';
import { App } from '@/App';
import { makeChartSeries } from '@/test/fixtures';
import { renderWithProviders } from '@/test/render';

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof ApiClientModule>();
  return {
    ...actual,
    getAuthStatus: vi.fn(),
    getChart: vi.fn(),
  };
});

const getChartMock = vi.mocked(getChart);
const getAuthStatusMock = vi.mocked(getAuthStatus);

function resolveChart(range: PriceRange) {
  getChartMock.mockResolvedValue({
    ok: true,
    value: makeChartSeries({ range }),
  });
}

beforeEach(() => {
  getChartMock.mockReset();
  getAuthStatusMock.mockReset();
  getAuthStatusMock.mockResolvedValue({
    ok: true,
    value: {
      setupRequired: false,
      authenticated: true,
      user: { username: 'owner', totpEnabled: true },
    },
  });
  resolveChart('1M');
});

describe('ChartPage', () => {
  it('uppercases the ticker and defaults to the 1M range', async () => {
    renderWithProviders(<App />, { route: '/chart/aapl' });

    const chart = await screen.findByRole('img', { name: 'AAPL price chart, 1M range' });
    expect(screen.getByRole('heading', { level: 1, name: 'AAPL' })).toBeInTheDocument();
    expect(chart.querySelector('title')).toHaveTextContent('AAPL price chart, 1M range');
    expect(chart.querySelector('desc')).toHaveTextContent('Daily closing prices');
    expect(screen.getByText('Source: Twelve Data')).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Range' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '1M' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: '3M' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: '1Y' })).toHaveAttribute('aria-pressed', 'false');
    expect(getChartMock).toHaveBeenCalledWith('AAPL', '1M', expect.anything());
  });

  it('links back to the dashboard', async () => {
    renderWithProviders(<App />, { route: '/chart/AAPL' });

    await screen.findByRole('img', { name: /AAPL price chart/ });
    expect(screen.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/dashboard');
    expect(screen.getByRole('link', { name: 'Skip to content' })).toHaveAttribute(
      'href',
      '#main-content',
    );
  });

  it('requests a new range and updates the accessible label', async () => {
    const user = userEvent.setup();
    getChartMock.mockImplementation((_ticker, range) =>
      Promise.resolve({ ok: true, value: makeChartSeries({ range }) }),
    );

    renderWithProviders(<App />, { route: '/chart/AAPL' });
    await screen.findByRole('img', { name: 'AAPL price chart, 1M range' });

    await user.click(screen.getByRole('button', { name: '3M' }));

    expect(
      await screen.findByRole('img', { name: 'AAPL price chart, 3M range' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '3M' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: '1M' })).toHaveAttribute('aria-pressed', 'false');
    expect(getChartMock).toHaveBeenCalledWith('AAPL', '3M', expect.anything());
  });

  it('shows the no-data state for a successful empty series', async () => {
    getChartMock.mockResolvedValue({
      ok: true,
      value: makeChartSeries({ range: '1M', points: [] }),
    });

    renderWithProviders(<App />, { route: '/chart/AAPL' });

    expect(await screen.findByText('No data for this range')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('No data for this range');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Range' })).toBeInTheDocument();
  });

  it('shows the Chart unavailable alert when the series fails', async () => {
    getChartMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Chart unavailable', 'CHART_UNAVAILABLE', 502),
    });

    renderWithProviders(<App />, { route: '/chart/AAPL' });

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Chart unavailable');
    expect(screen.getAllByRole('alert')).toHaveLength(1);
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/dashboard');
    expect(screen.getByRole('group', { name: 'Range' })).toBeInTheDocument();
  });

  it('uses other server chart errors verbatim', async () => {
    getChartMock.mockResolvedValue({
      ok: false,
      error: new ApiError('Symbol not found.', 'UNKNOWN_SYMBOL', 404),
    });

    renderWithProviders(<App />, { route: '/chart/ZZZZ' });

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Symbol not found.');
    expect(screen.queryByText('Source: Twelve Data')).not.toBeInTheDocument();
  });
});
