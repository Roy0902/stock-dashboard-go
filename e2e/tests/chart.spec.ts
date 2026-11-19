import { expect, test } from '../fixtures/test';
import { copy } from '../fixtures/app';

test.describe('opening a chart from a quote row', () => {
  test.use({ appOptions: { watchlist: ['AAPL'] } });

  test('shows the labelled chart and its source', async ({ page, app }) => {
    await page.goto(app.url);

    await page
      .getByRole('listitem', { name: 'AAPL' })
      .getByRole('link', { name: 'View chart' })
      .click();

    await expect(page).toHaveURL(/\/chart\/AAPL$/);
    await expect(page.getByRole('heading', { name: 'AAPL' })).toBeVisible();
    await expect(
      page.getByRole('img', { name: 'AAPL price chart, 1M range' }),
    ).toBeVisible();
    await expect(page.getByText(copy.source, { exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/dashboard');
  });
});

test('the range selector defaults to 1M and switches to 3M', async ({ page, app }) => {
  await page.goto(`${app.url}/chart/AAPL`);

  const range = page.getByRole('group', { name: 'Range' });
  await expect(range).toBeVisible();
  await expect(range.getByRole('button', { name: '1M' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await expect(range.getByRole('button', { name: '3M' })).toBeVisible();
  await expect(range.getByRole('button', { name: '1Y' })).toBeVisible();

  await range.getByRole('button', { name: '3M' }).click();
  await expect(
    page.getByRole('img', { name: 'AAPL price chart, 3M range' }),
  ).toBeVisible();
});

test.describe('unavailable chart', () => {
  test.use({ appOptions: { mode: 'all-down' } });

  test('keeps navigation usable', async ({ page, app }) => {
    await page.goto(`${app.url}/chart/AAPL`);

    await expect(page.getByRole('alert')).toHaveText(copy.chartUnavailable);
    await expect(page.getByRole('link', { name: 'Back' })).toBeVisible();
    await expect(page.getByRole('group', { name: 'Range' })).toBeVisible();
  });
});

test('a chart can be opened for a ticker outside the watchlist', async ({ page, app }) => {
  await page.goto(`${app.url}/chart/MSFT`);
  await expect(page.getByRole('img', { name: 'MSFT price chart, 1M range' })).toBeVisible();
});

