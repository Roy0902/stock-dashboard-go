import type { ChartPoint, ChartSeries, Quote } from '@/api/types';

export function makeQuote(overrides: Partial<Quote> = {}): Quote {
  return {
    ticker: 'AAPL',
    price: 190.12,
    change: 1.24,
    changePercent: 0.66,
    direction: 'up',
    provider: 'Finnhub',
    timestamp: '2026-11-16T14:30:00Z',
    stale: false,
    ...overrides,
  };
}

export function makePoints(count = 30): ChartPoint[] {
  const start = Date.UTC(2026, 9, 1);
  return Array.from({ length: count }, (_, index) => {
    const date = new Date(start + index * 86_400_000).toISOString().slice(0, 10);
    return {
      date,
      close: 180 + index * 0.5,
    };
  });
}

export function makeChartSeries(overrides: Partial<ChartSeries> = {}): ChartSeries {
  return {
    ticker: 'AAPL',
    range: '1M',
    source: 'Twelve Data',
    points: makePoints(),
    ...overrides,
  };
}
