import { expect, test } from '../fixtures/test';
import { addViaUi, copy, tickerRow } from '../fixtures/app';

test('first use shows the empty-watchlist message', async ({ page, app }) => {
  await page.goto(app.url);

  await expect(page.getByRole('alert')).toHaveText(copy.empty);
  await expect(page.getByRole('listitem')).toHaveCount(0);
  await expect(page.getByRole('textbox', { name: 'Ticker' })).toHaveAttribute(
    'placeholder',
    'e.g. AAPL',
  );
  await expect(page.getByRole('button', { name: 'Add ticker' })).toBeEnabled();
});

test('a lowercase ticker is canonicalised and its quote is rendered', async ({
  page,
  app,
}) => {
  await page.goto(app.url);

  await addViaUi(page, 'aapl');

  const row = tickerRow(page, 'AAPL');
  await expect(row).toBeVisible();
  await expect(row.getByText('190.12', { exact: true })).toBeVisible();
  await expect(row.getByText('+1.24', { exact: true })).toBeVisible();
  await expect(row.getByText('+0.66%', { exact: true })).toBeVisible();
  await expect(row.getByText('up', { exact: true })).toBeVisible();
  await expect(row.getByText('Finnhub', { exact: true })).toBeVisible();
  await expect(row.getByRole('link', { name: 'View chart' })).toHaveAttribute(
    'href',
    '/chart/AAPL',
  );
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(page.getByRole('status')).toHaveText(/^Last updated \d{2}:\d{2}:\d{2}$/);
});

test('invalid and duplicate adds surface the exact copy', async ({ page, app }) => {
  await page.goto(app.url);

  await addViaUi(page, '1AAPL');
  await expect(page.getByRole('alert')).toHaveText(copy.invalid);

  await addViaUi(page, 'AAPL');
  await expect(tickerRow(page, 'AAPL')).toBeVisible();

  await addViaUi(page, 'aapl');
  await expect(page.getByRole('alert')).toHaveText('AAPL is already in your watchlist.');
  await expect(page.getByRole('listitem')).toHaveCount(1);
});

test('an unknown symbol is refused without adding a row', async ({ page, app }) => {
  await page.goto(app.url);

  await addViaUi(page, 'ZZZZ');
  await expect(page.getByRole('alert')).toHaveText(copy.unknown);
  await expect(page.getByRole('listitem')).toHaveCount(0);
});

test.describe('with a cold cache', () => {
  test.describe('Twelve Data fallback', () => {
    test.use({ appOptions: { watchlist: ['AAPL'], mode: 'finnhub-down' } });

    test('Twelve Data supplies the quote when Finnhub fails', async ({ page, app }) => {
      await page.goto(app.url);

      const row = tickerRow(page, 'AAPL');
      await expect(row.getByText('190.12', { exact: true })).toBeVisible();
      await expect(row.getByText('Twelve Data', { exact: true })).toBeVisible();
      await expect(row.getByText('up', { exact: true })).toBeVisible();
    });
  });

  test.describe('both providers down', () => {
    test.use({ appOptions: { watchlist: ['AAPL'], mode: 'all-down' } });

    test('a row with no usable quote degrades honestly and never leaks detail', async ({
      page,
      app,
    }) => {
      await page.goto(app.url);

      const row = tickerRow(page, 'AAPL');
      await expect(row.getByText(copy.unavailable, { exact: true })).toBeVisible();
      await expect(row.getByText('none', { exact: true })).toBeVisible();
      await expect(row.getByText('190.12')).toHaveCount(0);
      await expect(page.getByRole('alert')).toHaveText(copy.providerUnavailable);
      await expect(page.locator('body')).not.toContainText('SECRET-XYZ');
    });
  });

  test.describe('neutral quote', () => {
    test.use({ appOptions: { watchlist: ['TSLA'] } });

    test('a zero change is neutral and still states direction', async ({ page, app }) => {
      await page.goto(app.url);

      const row = tickerRow(page, 'TSLA');
      await expect(row.getByText('0.00', { exact: true }).first()).toBeVisible();
      await expect(row.getByText('unchanged', { exact: true })).toBeVisible();
    });
  });

  test.describe('missing API keys', () => {
    test.use({ appOptions: { watchlist: ['AAPL'], keys: false } });

    test('the dashboard names both missing keys and stays running', async ({
      page,
      app,
    }) => {
      await page.goto(app.url);

      await expect(page.getByRole('alert')).toHaveText(copy.missingKey);
      await expect(page.getByRole('heading', { name: 'Watchlist' })).toBeVisible();
    });
  });
});
