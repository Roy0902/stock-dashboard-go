import { copy } from '@/locale';

const decimalFormatter = new Intl.NumberFormat(copy.localeTag, {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const currencyFormatter = new Intl.NumberFormat(copy.localeTag, {
  style: 'currency',
  currency: copy.currency,
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const chartDateFormatter = new Intl.DateTimeFormat(copy.localeTag, {
  month: 'short',
  day: 'numeric',
  timeZone: 'UTC',
});

const monthFormatter = new Intl.DateTimeFormat(copy.localeTag, {
  month: 'short',
  timeZone: 'UTC',
});

// One number convention everywhere: grouped, two decimals, no currency symbol.
export function formatPrice(value: number): string {
  return decimalFormatter.format(value);
}

export function formatSigned(value: number): string {
  if (value === 0) {
    return decimalFormatter.format(0);
  }
  if (value > 0) {
    return `+${decimalFormatter.format(value)}`;
  }
  return decimalFormatter.format(value);
}

export function formatPercent(value: number | null): string | null {
  if (value === null) {
    return null;
  }
  return `${formatSigned(value)}%`;
}

// Display format requested by the product owner: dd-MMM-yyyy (UTC). The API and the
// <time dateTime> attribute keep the ISO-8601 value.
export function formatTimestamp(timestamp: string, stale: boolean): string {
  const parsed = new Date(timestamp);
  const month = Number.isNaN(parsed.getTime())
    ? ''
    : monthFormatter.format(Date.UTC(parsed.getUTCFullYear(), parsed.getUTCMonth(), 1));
  const display = Number.isNaN(parsed.getTime())
    ? timestamp
    : `${String(parsed.getUTCDate()).padStart(2, '0')}-${month}-${parsed.getUTCFullYear()}`;
  return stale ? copy.quotes.staleSuffix(display) : display;
}

export function formatClock(date: Date): string {
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');
  return `${hours}:${minutes}:${seconds}`;
}

// Accessible chart description keeps the currency symbol; visible axis labels use
// formatPrice so the dashboard and the chart share one number convention.
export function formatCurrency(value: number): string {
  return currencyFormatter.format(value);
}

export function formatChartDate(date: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
  if (match === null) {
    return date;
  }
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const parsed = new Date(Date.UTC(year, month - 1, day));
  if (
    Number.isNaN(parsed.getTime()) ||
    parsed.getUTCFullYear() !== year ||
    parsed.getUTCMonth() !== month - 1 ||
    parsed.getUTCDate() !== day
  ) {
    return date;
  }
  return chartDateFormatter.format(parsed);
}
