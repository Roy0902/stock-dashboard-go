/**
 * English copy for the whole client. Every user-visible string lives here so a new
 * locale can be added without touching components.
 */
export const en = {
  localeTag: 'en-US',
  currency: 'USD',
  document: {
    title: 'Stock dashboard',
    description: 'A local stock watchlist with quotes and daily price charts.',
  },
  app: {
    skipToContent: 'Skip to content',
    reload: 'Reload',
  },
  brand: {
    name: 'Stock dashboard',
    homeLabel: 'Stock dashboard home',
  },
  header: {
    descriptor: 'Quotes and daily charts for the tickers you follow.',
    signOut: 'Sign out',
  },
  landing: {
    eyebrow: 'Local stock dashboard',
    title: 'Your watchlist, quotes and daily charts in one calm place.',
    lede: 'Track the US tickers you care about with a private, local tool. No cloud account, no telemetry, and no data leaves this machine except the quote requests you configure.',
    signIn: 'Sign in',
    hint: 'Password and authenticator code required.',
    featuresLabel: 'What the dashboard includes',
    features: [
      {
        title: 'Live quotes',
        body: 'Latest price, change and provider for every ticker, refreshed while the page is open.',
      },
      {
        title: 'Daily charts',
        body: 'Thirty days by default, with 1M, 3M and 1Y ranges from Twelve Data.',
      },
      {
        title: 'Private by design',
        body: 'Loopback-only, separate accounts, each with a password and authenticator code.',
      },
    ],
    footnote: (host: string) =>
      `Runs on ${host}. Quotes are cached locally; charts use the daily close.`,
  },
  dashboard: {
    title: 'Watchlist',
    empty: 'Your watchlist is empty. Add a ticker to begin.',
  },
  watchlist: {
    listLabel: 'Watchlist',
    columns: {
      ticker: 'Ticker',
      price: 'Price',
      change: 'Change',
      direction: 'Direction',
      provider: 'Provider',
      updated: 'Updated',
    },
    priceUnavailable: 'Price unavailable',
    emptyValue: '—',
    noProvider: 'none',
    viewChart: 'View chart',
    loading: 'Loading watchlist',
    loadingPrice: 'Loading price',
  },
  addTicker: {
    label: 'Ticker',
    placeholder: 'e.g. AAPL',
    submit: 'Add ticker',
    submitPending: 'Adding…',
  },
  chart: {
    back: 'Back',
    unavailable: 'Chart unavailable',
    noData: 'No data for this range',
    rangeLabel: 'Range',
    source: (source: string) => `Source: ${source}`,
    accessibleLabel: (ticker: string, range: string) => `${ticker} price chart, ${range} range`,
    description: (values: {
      firstDate: string;
      lastDate: string;
      low: string;
      lowDate: string;
      high: string;
      highDate: string;
    }) =>
      `Daily closing prices from ${values.firstDate} to ${values.lastDate}. ` +
      `Low ${values.low} on ${values.lowDate}; high ${values.high} on ${values.highDate}.`,
  },
  quotes: {
    lastUpdated: (clock: string) => `Last updated ${clock}`,
    staleSuffix: (value: string) => `${value} (stale)`,
  },
  auth: {
    localOnlyNote: 'Local-only · Password + authenticator',
    headings: {
      signIn: 'Sign in',
      twoFactor: 'Two-factor authentication',
      recovery: 'Use a recovery code',
      createAccount: 'Create account',
      authenticator: 'Set up authenticator',
    },
    ledes: {
      signIn: 'Use your password and authenticator code to continue.',
      twoFactor: 'Enter the 6-digit code from your authenticator app.',
      recovery: 'Enter one of the recovery codes you saved during setup.',
    },
    fields: {
      username: 'Username',
      password: 'Password',
      confirmPassword: 'Confirm password',
      verificationCode: 'Verification code',
      recoveryCode: 'Recovery code',
    },
    actions: {
      signIn: 'Sign in',
      continue: 'Continue',
      useRecoveryCode: 'Use a recovery code',
      backToSignIn: 'Back to sign in',
      createAccount: 'Create account',
      copy: 'Copy',
      copied: 'Copied',
      tryAgain: 'Try again',
    },
    password: {
      show: 'Show password',
      hide: 'Hide password',
    },
    codeExpiry: (seconds: number) => {
      if (seconds < 60) {
        return `Code expires in ${seconds} ${seconds === 1 ? 'second' : 'seconds'}.`;
      }
      const minutes = Math.ceil(seconds / 60);
      return `Code expires in ${minutes} ${minutes === 1 ? 'minute' : 'minutes'}.`;
    },
    switch: {
      prompt: 'New here?',
      createAccount: 'Set up your account',
    },
    setup: {
      qrTitle: 'Authenticator QR code',
      scanQr: 'Scan this QR code with your authenticator app.',
      manualKey: 'Or enter this setup key manually:',
      saveCodes: 'Save these recovery codes.',
      savedCodes: 'I have saved these recovery codes',
      recoveryCodesLabel: 'Recovery codes',
    },
  },
  errors: {
    network: 'Data provider is unavailable. Try again shortly.',
    generic: 'Something went wrong. Try again shortly.',
    sessionExpired: 'Your session expired. Sign in again.',
    invalidCode: 'Invalid verification code.',
    invalidRecoveryCode: 'Invalid recovery code.',
    missingCredentials: 'Enter your username and password.',
    username: 'Use 3-32 lowercase letters, numbers, dots, dashes or underscores.',
    weakPassword: 'Use at least 12 characters.',
    passwordMismatch: 'Passwords do not match.',
  },
};

export type Copy = typeof en;
