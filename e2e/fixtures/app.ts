import type { Locator, Page } from '@playwright/test';

export async function addViaUi(page: Page, ticker: string): Promise<void> {
  await page.getByRole('textbox', { name: 'Ticker' }).fill(ticker);
  await page.getByRole('button', { name: 'Add ticker' }).click();
}

export function tickerRow(page: Page, ticker: string): Locator {
  return page.getByRole('listitem', { name: ticker });
}

export const copy = {
  empty: 'Your watchlist is empty. Add a ticker to begin.',
  invalid: 'Enter a valid ticker, e.g. AAPL.',
  unknown: 'Symbol not found.',
  unavailable: 'Price unavailable',
  chartUnavailable: 'Chart unavailable',
  source: 'Source: Twelve Data',
  missingKey: 'Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.',
  providerUnavailable: 'Data provider is unavailable. Try again shortly.',
} as const;
