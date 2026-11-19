import { describe, expect, it } from 'vitest';

import {
  formatChartDate,
  formatClock,
  formatCurrency,
  formatPercent,
  formatPrice,
  formatSigned,
  formatTimestamp,
} from '@/lib/format';

describe('formatPrice', () => {
  it('always shows two decimals', () => {
    expect(formatPrice(190.12)).toBe('190.12');
    expect(formatPrice(190)).toBe('190.00');
    expect(formatPrice(0)).toBe('0.00');
  });

  it('groups thousands without a currency symbol', () => {
    expect(formatPrice(1234.56)).toBe('1,234.56');
  });
});

describe('formatSigned', () => {
  it('prefixes positive values with a plus', () => {
    expect(formatSigned(1.24)).toBe('+1.24');
    expect(formatSigned(10)).toBe('+10.00');
  });

  it('groups thousands like the price column', () => {
    expect(formatSigned(1234.56)).toBe('+1,234.56');
  });

  it('keeps the minus for negative values', () => {
    expect(formatSigned(-10)).toBe('-10.00');
    expect(formatSigned(-0.5)).toBe('-0.50');
  });

  it('renders zero without a sign', () => {
    expect(formatSigned(0)).toBe('0.00');
    expect(formatSigned(-0)).toBe('0.00');
  });
});

describe('formatPercent', () => {
  it('adds a sign and a percent suffix', () => {
    expect(formatPercent(0.66)).toBe('+0.66%');
    expect(formatPercent(-5)).toBe('-5.00%');
    expect(formatPercent(0)).toBe('0.00%');
  });

  it('returns null when no percentage is available', () => {
    expect(formatPercent(null)).toBeNull();
  });
});

describe('formatTimestamp', () => {
  it('renders the requested dd-MMM-yyyy format in UTC', () => {
    expect(formatTimestamp('2026-11-16T14:30:00Z', false)).toBe('16-Nov-2026');
    expect(formatTimestamp('2026-01-05T00:00:00Z', false)).toBe('05-Jan-2026');
  });

  it('uses the UTC date even when local time is on the next day', () => {
    expect(formatTimestamp('2026-11-17T21:00:00Z', false)).toBe('17-Nov-2026');
  });

  it('appends the stale suffix', () => {
    expect(formatTimestamp('2026-11-16T14:00:00Z', true)).toBe('16-Nov-2026 (stale)');
  });

  it('falls back to the raw value when the timestamp is not parseable', () => {
    expect(formatTimestamp('not-a-date', false)).toBe('not-a-date');
  });
});

describe('formatClock', () => {
  it('renders 24-hour local time with padded parts', () => {
    expect(formatClock(new Date(2026, 0, 5, 9, 5, 7))).toBe('09:05:07');
    expect(formatClock(new Date(2026, 0, 5, 23, 59, 0))).toBe('23:59:00');
  });
});

describe('formatCurrency', () => {
  it('formats dollar amounts for the accessible chart description', () => {
    expect(formatCurrency(190.5)).toBe('$190.50');
    expect(formatCurrency(0)).toBe('$0.00');
  });
});

describe('formatChartDate', () => {
  it('formats ISO dates without timezone drift', () => {
    expect(formatChartDate('2026-10-17')).toBe('Oct 17');
  });

  it('returns unparseable input unchanged', () => {
    expect(formatChartDate('not-a-date')).toBe('not-a-date');
  });

  it('does not roll a day that does not exist in that month', () => {
    expect(formatChartDate('2026-02-31')).toBe('2026-02-31');
    expect(formatChartDate('2026-13-01')).toBe('2026-13-01');
  });

  it('rejects 29 February in a non-leap year but formats a real leap day', () => {
    expect(formatChartDate('2026-02-29')).toBe('2026-02-29');
    expect(formatChartDate('2028-02-29')).toBe('Feb 29');
  });
});
