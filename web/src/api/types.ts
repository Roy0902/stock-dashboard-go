export const PROVIDERS = ['Finnhub', 'Twelve Data', 'none'] as const;
export type Provider = (typeof PROVIDERS)[number];

export const DIRECTIONS = ['up', 'down', 'unchanged'] as const;
export type Direction = (typeof DIRECTIONS)[number];

export const PRICE_RANGES = ['1M', '3M', '1Y'] as const;
export type PriceRange = (typeof PRICE_RANGES)[number];

export interface Watchlist {
  tickers: string[];
}

export interface Quote {
  ticker: string;
  price: number;
  change: number;
  changePercent: number | null;
  direction: Direction;
  provider: Provider;
  timestamp: string;
  stale: boolean;
}

export interface QuotesResponse {
  quotes: Quote[];
}

export interface ChartPoint {
  date: string;
  close: number;
}

export interface ChartSeries {
  ticker: string;
  range: PriceRange;
  source: string;
  points: ChartPoint[];
}

export interface ApiErrorPayload {
  error: string;
  code: string;
}

export interface AuthUser {
  username: string;
  totpEnabled: boolean;
}

export interface AuthStatus {
  setupRequired: boolean;
  authenticated: boolean;
  user?: AuthUser;
}

export interface SetupMaterial {
  username: string;
  secret: string;
  otpauthUrl: string;
  recoveryCodes: string[];
}

export interface LoginChallenge {
  status: 'totp_required';
  expiresInSeconds: number;
}

export interface AuthUserResponse {
  user: AuthUser;
}
