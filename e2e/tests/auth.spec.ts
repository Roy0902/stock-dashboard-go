import { currentTotp, ownerPassword, ownerUsername } from '../fixtures/auth';
import { expect, test } from '../fixtures/test';

test.describe('signed out', () => {
  test.use({ appOptions: { authenticated: false } });

  test('an anonymous visit shows the landing page and links to sign in', async ({
    page,
    app,
  }) => {
    await page.goto(app.url);

    await expect(
      page.getByRole('heading', {
        name: 'Your watchlist, quotes and daily charts in one calm place.',
      }),
    ).toBeVisible();
    await page.getByRole('link', { name: 'Sign in' }).click();
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
    await expect(page.getByText('New here?')).toBeVisible();

    await page.getByRole('link', { name: 'Set up your account' }).click();
    await expect(page.getByRole('heading', { name: 'Create account' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Username' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Password', exact: true })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Confirm password' })).toBeVisible();
    await page.goBack();
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
  });

  test('protected pages and APIs require a session', async ({ page, app, context }) => {
    await page.goto(`${app.url}/chart/AAPL`);
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();

    const quotes = await context.request.get(`${app.url}/api/quotes`);
    expect(quotes.status()).toBe(401);
    expect(await quotes.json()).toMatchObject({ code: 'UNAUTHENTICATED' });
  });

  test('wrong credentials use the generic copy', async ({ page, app }) => {
    await page.goto(`${app.url}/login`);
    await page.getByRole('textbox', { name: 'Username' }).fill(ownerUsername);
    await page.getByRole('textbox', { name: 'Password' }).fill('wrong password value');
    await page.getByRole('button', { name: 'Sign in' }).click();

    await expect(page.getByRole('alert')).toHaveText('Invalid username or password.');
  });

  test('password then a TOTP code signs in', async ({ page, app, auth }) => {
    expect(auth).not.toBeNull();
    await page.goto(`${app.url}/login`);
    await page.getByRole('textbox', { name: 'Username' }).fill(ownerUsername);
    await page.getByRole('textbox', { name: 'Password' }).fill(ownerPassword);
    await page.getByRole('button', { name: 'Sign in' }).click();

    await expect(
      page.getByRole('heading', { name: 'Two-factor authentication' }),
    ).toBeVisible();
    await page
      .getByRole('textbox', { name: 'Verification code' })
      .fill(await currentTotp(auth!.secret));

    // Six digits auto-submit; the Continue control unmounts with the challenge step.
    await expect(page.getByRole('heading', { name: 'Watchlist' })).toBeVisible();
  });

  test('an invalid TOTP code is refused', async ({ page, app, auth }) => {
    expect(auth).not.toBeNull();
    await page.goto(`${app.url}/login`);
    await page.getByRole('textbox', { name: 'Username' }).fill(ownerUsername);
    await page.getByRole('textbox', { name: 'Password' }).fill(ownerPassword);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
      page.getByRole('heading', { name: 'Two-factor authentication' }),
    ).toBeVisible();

    const code = await currentTotp(auth!.secret);
    const wrong = `${code.slice(0, 5)}${String((Number(code.slice(-1)) + 1) % 10)}`;
    await page.getByRole('textbox', { name: 'Verification code' }).fill(wrong);

    await expect(page.getByRole('alert')).toHaveText('Invalid verification code.');
  });

  test('a recovery code signs in once', async ({ page, app, auth }) => {
    expect(auth).not.toBeNull();
    const recoveryCode = auth!.recoveryCodes[0] ?? '';
    expect(recoveryCode).not.toBe('');

    await page.goto(`${app.url}/login`);
    await page.getByRole('textbox', { name: 'Username' }).fill(ownerUsername);
    await page.getByRole('textbox', { name: 'Password' }).fill(ownerPassword);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.getByRole('button', { name: 'Use a recovery code' }).click();
    await page.getByRole('textbox', { name: 'Recovery code' }).fill(recoveryCode);
    await page.getByRole('button', { name: 'Continue' }).click();
    await expect(page.getByRole('heading', { name: 'Watchlist' })).toBeVisible();

    await page.getByRole('button', { name: 'Sign out' }).click();
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();

    await page.getByRole('textbox', { name: 'Username' }).fill(ownerUsername);
    await page.getByRole('textbox', { name: 'Password' }).fill(ownerPassword);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.getByRole('button', { name: 'Use a recovery code' }).click();
    await page.getByRole('textbox', { name: 'Recovery code' }).fill(recoveryCode);
    await page.getByRole('button', { name: 'Continue' }).click();
    await expect(page.getByRole('alert')).toHaveText('Invalid recovery code.');
  });

  test('password failures are rate limited', async ({ page, app }) => {
    await page.goto(`${app.url}/login`);

    for (let attempt = 0; attempt < 5; attempt += 1) {
      await page.getByRole('textbox', { name: 'Username' }).fill(ownerUsername);
      await page.getByRole('textbox', { name: 'Password' }).fill(`wrong-password-${attempt}`);
      await page.getByRole('button', { name: 'Sign in' }).click();
      await expect(page.getByRole('alert')).toHaveText('Invalid username or password.');
    }

    await page.getByRole('textbox', { name: 'Username' }).fill(ownerUsername);
    await page.getByRole('textbox', { name: 'Password' }).fill('wrong-password-final');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.getByRole('alert')).toContainText('Too many requests.');
  });
});

test.describe('authenticated shell', () => {
  test('sign out revokes the session', async ({ page, app, context }) => {
    await page.goto(app.url);
    await expect(page.getByRole('heading', { name: 'Watchlist' })).toBeVisible();
    await page.getByRole('button', { name: 'Sign out' }).click();

    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
    const quotes = await context.request.get(`${app.url}/api/quotes`);
    expect(quotes.status()).toBe(401);
  });

  test('a session that ends mid-use shows the expiry copy', async ({ page, app, context }) => {
    await page.goto(app.url);
    await expect(page.getByRole('heading', { name: 'Watchlist' })).toBeVisible();
    await context.clearCookies();

    await page.getByRole('textbox', { name: 'Ticker' }).fill('AAPL');
    await page.getByRole('button', { name: 'Add ticker' }).click();

    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
    await expect(page.getByRole('alert')).toHaveText('Your session expired. Sign in again.');
  });
});

test.describe('multi-user accounts', () => {
  test('two accounts keep separate watchlists', async ({ page, app, context, auth }) => {
    expect(auth).not.toBeNull();
    const ownerCode = async () => currentTotp(auth!.secret);

    await context.request.post(`${app.url}/api/watchlist`, { data: { ticker: 'AAPL' } });
    await context.request.post(`${app.url}/api/auth/logout`);

    const secondPassword = 'correct-horse-battery-staple';
    const setup = await context.request.post(`${app.url}/api/auth/setup`, {
      data: { username: 'second', password: secondPassword },
    });
    expect(setup.status(), await setup.text()).toBe(201);
    const material = (await setup.json()) as { secret: string };
    const activate = await context.request.post(`${app.url}/api/auth/2fa/activate`, {
      data: { code: await currentTotp(material.secret) },
    });
    expect(activate.status(), await activate.text()).toBe(200);

    await page.goto(`${app.url}/dashboard`);
    await expect(page.getByRole('heading', { name: 'Watchlist' })).toBeVisible();
    await expect(page.getByRole('listitem', { name: 'AAPL' })).toHaveCount(0);
    await expect(page.getByRole('alert')).toHaveText('Your watchlist is empty. Add a ticker to begin.');

    await context.request.post(`${app.url}/api/watchlist`, { data: { ticker: 'MSFT' } });
    await page.reload();
    await expect(page.getByRole('listitem', { name: 'MSFT' })).toBeVisible();
    await context.request.post(`${app.url}/api/auth/logout`);

    const login = await context.request.post(`${app.url}/api/auth/login`, {
      data: { username: ownerUsername, password: ownerPassword },
    });
    expect(login.status(), await login.text()).toBe(200);
    const totp = await context.request.post(`${app.url}/api/auth/login/totp`, {
      data: { code: await ownerCode() },
    });
    expect(totp.status(), await totp.text()).toBe(200);

    await page.goto(`${app.url}/dashboard`);
    await expect(page.getByRole('listitem', { name: 'AAPL' })).toBeVisible();
    await expect(page.getByRole('listitem', { name: 'MSFT' })).toHaveCount(0);
  });
});

test.describe('first-run setup', () => {
  test.use({ appOptions: { bootstrap: false, authenticated: false } });

  test('creates the owner with a QR code and recovery codes', async ({ page, app }) => {
    await page.goto(app.url);
    // First run goes straight to account creation (D-29); the landing page is for
    // installations that already have an owner.
    await expect(page).toHaveURL(/\/setup$/);
    await expect(page.getByRole('heading', { name: 'Create account' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Username' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Password', exact: true })).toBeVisible();

    await page.getByRole('textbox', { name: 'Username' }).fill(ownerUsername);
    await page.getByRole('textbox', { name: 'Password', exact: true }).fill(ownerPassword);
    await page.getByRole('textbox', { name: 'Confirm password' }).fill(ownerPassword);

    const setupResponse = page.waitForResponse(
      (response) =>
        response.url().endsWith('/api/auth/setup') && response.request().method() === 'POST',
    );
    await page.getByRole('button', { name: 'Create account' }).click();
    const setup = (await (await setupResponse).json()) as {
      secret: string;
      recoveryCodes: string[];
    };
    expect(setup.recoveryCodes).toHaveLength(10);

    await expect(page.getByRole('img', { name: 'Authenticator QR code' })).toBeVisible();
    for (const code of setup.recoveryCodes) {
      await expect(page.getByText(code, { exact: true })).toBeVisible();
    }

    await page.getByRole('textbox', { name: 'Verification code' }).fill(
      await currentTotp(setup.secret),
    );
    await page.getByRole('checkbox', { name: 'I have saved these recovery codes' }).check();
    await page.getByRole('button', { name: 'Continue' }).click();

    await expect(page.getByRole('heading', { name: 'Watchlist' })).toBeVisible();
  });
});

