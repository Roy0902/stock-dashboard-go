import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { PriceChart } from '@/features/chart/PriceChart';
import { makePoints } from '@/test/fixtures';

describe('PriceChart', () => {
  it('renders an accessible, smoothed line with axes and extremes', () => {
    const { container } = render(<PriceChart ticker="AAPL" range="1M" points={makePoints(30)} />);

    const chart = screen.getByRole('img', { name: 'AAPL price chart, 1M range' });
    expect(chart).toBeInTheDocument();

    const line = container.querySelector('.price-chart__line');
    expect(line?.getAttribute('d')).toMatch(/^M .*C /);

    const dates = container.querySelectorAll('.price-chart__dates text');
    expect(dates).toHaveLength(2);
    expect(dates[0]?.textContent).toBe('Oct 1');
    expect(dates[1]?.textContent).toBe('Oct 30');

    const yLabels = container.querySelectorAll('.price-chart__grid text');
    expect(yLabels.length).toBeGreaterThanOrEqual(3);
    for (const label of yLabels) {
      expect(label.textContent).toMatch(/^[\d,]+\.\d{2}$/);
    }

    expect(container.querySelectorAll('.price-chart__extreme-dot')).toHaveLength(2);
    expect(container.querySelector('.price-chart__area')).not.toBeNull();
    expect(chart.querySelector('desc')?.textContent).toContain('Low $180.00');
  });

  it('uses three value ticks when the measured width is below 400 px', () => {
    const rect = {
      width: 320,
      height: 0,
      top: 0,
      left: 0,
      right: 320,
      bottom: 0,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    } as DOMRect;
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(rect);

    const { container } = render(<PriceChart ticker="AAPL" range="1M" points={makePoints(30)} />);

    expect(screen.getByRole('img', { name: 'AAPL price chart, 1M range' })).toBeInTheDocument();
    const ticks = container.querySelectorAll('.price-chart__grid text');
    expect(ticks.length).toBeGreaterThan(0);
    expect(ticks.length).toBeLessThanOrEqual(3);
  });

  it('handles a single point without a line segment or area', () => {
    const { container } = render(<PriceChart ticker="MSFT" range="3M" points={makePoints(1)} />);

    expect(screen.getByRole('img', { name: 'MSFT price chart, 3M range' })).toBeInTheDocument();
    expect(container.querySelector('.price-chart__line')?.getAttribute('d')).toMatch(/^M /);
    expect(container.querySelector('.price-chart__area')).toBeNull();
  });
});
