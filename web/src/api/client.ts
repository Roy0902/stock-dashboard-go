import { API_ENDPOINTS } from '@/api/endpoints';
import type {
  ApiErrorPayload,
  AuthStatus,
  AuthUser,
  AuthUserResponse,
  ChartPoint,
  ChartSeries,
  Direction,
  LoginChallenge,
  PriceRange,
  Provider,
  Quote,
  QuotesResponse,
  SetupMaterial,
  Watchlist,
} from '@/api/types';
import { DIRECTIONS, PRICE_RANGES, PROVIDERS } from '@/api/types';
import { copy } from '@/locale';

export const UNAUTHENTICATED_CODE = 'UNAUTHENTICATED';

type UnauthorizedHandler = () => void;

let unauthorizedHandler: UnauthorizedHandler | null = null;

export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  unauthorizedHandler = handler;
}

function notifyUnauthorized(status: number, code: string): void {
  if (status === 401 && code === UNAUTHENTICATED_CODE) {
    unauthorizedHandler?.();
  }
}

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
  }
}

export type ApiResult<T> = { ok: true; value: T } | { ok: false; error: ApiError };

export function unwrap<T>(result: ApiResult<T>): T {
  if (!result.ok) {
    throw result.error;
  }
  return result.value;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isProvider(value: unknown): value is Provider {
  return typeof value === 'string' && (PROVIDERS as readonly string[]).includes(value);
}

function isDirection(value: unknown): value is Direction {
  return typeof value === 'string' && (DIRECTIONS as readonly string[]).includes(value);
}

function isPriceRange(value: unknown): value is PriceRange {
  return typeof value === 'string' && (PRICE_RANGES as readonly string[]).includes(value);
}

function isNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function parseErrorPayload(body: unknown): ApiErrorPayload | null {
  if (!isRecord(body)) return null;
  const { error, code } = body;
  if (typeof error !== 'string' || typeof code !== 'string') return null;
  return { error, code };
}

function parseWatchlist(body: unknown): Watchlist | null {
  if (!isRecord(body) || !Array.isArray(body.tickers)) return null;
  if (!body.tickers.every((ticker): ticker is string => typeof ticker === 'string')) {
    return null;
  }
  return { tickers: body.tickers };
}

function parseQuote(value: unknown): Quote | null {
  if (!isRecord(value)) return null;
  const { ticker, price, change, changePercent, direction, provider, timestamp, stale } = value;
  if (typeof ticker !== 'string') return null;
  if (!isNumber(price) || !isNumber(change)) return null;
  if (changePercent !== null && !isNumber(changePercent)) return null;
  if (!isDirection(direction) || !isProvider(provider)) return null;
  if (typeof timestamp !== 'string' || typeof stale !== 'boolean') return null;
  return {
    ticker,
    price,
    change,
    changePercent,
    direction,
    provider,
    timestamp,
    stale,
  };
}

function parseQuotes(body: unknown): QuotesResponse | null {
  if (!isRecord(body) || !Array.isArray(body.quotes)) return null;
  const quotes: Quote[] = [];
  for (const entry of body.quotes) {
    const quote = parseQuote(entry);
    if (quote === null) return null;
    quotes.push(quote);
  }
  return { quotes };
}

function parsePoint(value: unknown): ChartPoint | null {
  if (!isRecord(value)) return null;
  const { date, close } = value;
  if (typeof date !== 'string' || !isNumber(close)) return null;
  return { date, close };
}

function parseChart(body: unknown): ChartSeries | null {
  if (!isRecord(body)) return null;
  const { ticker, range, source, points } = body;
  if (typeof ticker !== 'string' || !isPriceRange(range) || typeof source !== 'string') {
    return null;
  }
  if (!Array.isArray(points)) return null;
  const parsed: ChartPoint[] = [];
  for (const entry of points) {
    const point = parsePoint(entry);
    if (point === null) return null;
    parsed.push(point);
  }
  return { ticker, range, source, points: parsed };
}

function parseAuthUser(value: unknown): AuthUser | null {
  if (!isRecord(value)) return null;
  const { username, totpEnabled } = value;
  if (typeof username !== 'string' || username.length === 0) return null;
  if (typeof totpEnabled !== 'boolean') return null;
  return { username, totpEnabled };
}

function parseAuthStatus(body: unknown): AuthStatus | null {
  if (!isRecord(body)) return null;
  const { setupRequired, authenticated, user } = body;
  if (typeof setupRequired !== 'boolean' || typeof authenticated !== 'boolean') return null;
  if (user === undefined) {
    return authenticated ? null : { setupRequired, authenticated };
  }
  const parsedUser = parseAuthUser(user);
  if (parsedUser === null) return null;
  return { setupRequired, authenticated, user: parsedUser };
}

function parseSetupMaterial(body: unknown): SetupMaterial | null {
  if (!isRecord(body)) return null;
  const { username, secret, otpauthUrl, recoveryCodes } = body;
  if (typeof username !== 'string' || username.length === 0) return null;
  if (typeof secret !== 'string' || secret.length === 0) return null;
  if (typeof otpauthUrl !== 'string' || !otpauthUrl.startsWith('otpauth://')) return null;
  if (!Array.isArray(recoveryCodes) || recoveryCodes.length !== 10) return null;
  if (!recoveryCodes.every((code): code is string => typeof code === 'string' && code.length > 0)) {
    return null;
  }
  return { username, secret, otpauthUrl, recoveryCodes };
}

function parseLoginChallenge(body: unknown): LoginChallenge | null {
  if (!isRecord(body)) return null;
  const { status, expiresInSeconds } = body;
  if (status !== 'totp_required' || !isNumber(expiresInSeconds)) return null;
  return { status, expiresInSeconds };
}

function parseAuthUserResponse(body: unknown): AuthUserResponse | null {
  if (!isRecord(body)) return null;
  const user = parseAuthUser(body.user);
  if (user === null) return null;
  return { user };
}

function isAbortError(error: unknown): boolean {
  if (typeof DOMException !== 'undefined' && error instanceof DOMException) {
    return error.name === 'AbortError';
  }
  return error instanceof Error && error.name === 'AbortError';
}

async function request<T>(
  path: string,
  init: RequestInit,
  parse: (body: unknown) => T | null,
): Promise<ApiResult<T>> {
  let response: Response;
  try {
    const headers = new Headers(init.headers);
    headers.set('accept', 'application/json');
    response = await fetch(path, {
      ...init,
      headers,
      cache: 'no-store',
      credentials: 'same-origin',
    });
  } catch (error) {
    if (isAbortError(error)) throw error;
    return {
      ok: false,
      error: new ApiError(copy.errors.network, 'NETWORK_ERROR', 0),
    };
  }

  let body: unknown = null;
  try {
    body = await response.json();
  } catch (error) {
    if (isAbortError(error)) throw error;
    body = null;
  }

  if (!response.ok) {
    const payload = parseErrorPayload(body);
    if (payload !== null) {
      notifyUnauthorized(response.status, payload.code);
      return {
        ok: false,
        error: new ApiError(payload.error, payload.code, response.status),
      };
    }
    return {
      ok: false,
      error: new ApiError(copy.errors.generic, 'INTERNAL_ERROR', response.status),
    };
  }

  const parsed = parse(body);
  if (parsed === null) {
    return {
      ok: false,
      error: new ApiError(copy.errors.generic, 'INTERNAL_ERROR', response.status),
    };
  }
  return { ok: true, value: parsed };
}

export function getWatchlist(signal?: AbortSignal): Promise<ApiResult<Watchlist>> {
  return request(API_ENDPOINTS.watchlist, { method: 'GET', signal }, parseWatchlist);
}

export function addTicker(ticker: string): Promise<ApiResult<Watchlist>> {
  return request(
    API_ENDPOINTS.watchlist,
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ ticker }),
    },
    parseWatchlist,
  );
}

export function getQuotes(signal?: AbortSignal): Promise<ApiResult<QuotesResponse>> {
  return request(API_ENDPOINTS.quotes, { method: 'GET', signal }, parseQuotes);
}

export function getChart(
  ticker: string,
  range: PriceRange,
  signal?: AbortSignal,
): Promise<ApiResult<ChartSeries>> {
  return request(API_ENDPOINTS.chart(ticker, range), { method: 'GET', signal }, parseChart);
}

async function authRequest<T>(
  path: string,
  init: RequestInit,
  parse: (body: unknown) => T | null,
): Promise<ApiResult<T>> {
  const result = await request(path, init, parse);
  if (!result.ok && result.error.code === 'NETWORK_ERROR') {
    return {
      ok: false,
      error: new ApiError(copy.errors.generic, 'INTERNAL_ERROR', 0),
    };
  }
  return result;
}

function postJson(body: unknown): RequestInit {
  return {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body),
  };
}

export function getAuthStatus(signal?: AbortSignal): Promise<ApiResult<AuthStatus>> {
  return authRequest(API_ENDPOINTS.auth.status, { method: 'GET', signal }, parseAuthStatus);
}

export function setupAccount(
  username: string,
  password: string,
): Promise<ApiResult<SetupMaterial>> {
  return authRequest(
    API_ENDPOINTS.auth.setup,
    postJson({ username, password }),
    parseSetupMaterial,
  );
}

export function activateTotp(code: string): Promise<ApiResult<AuthUserResponse>> {
  return authRequest(API_ENDPOINTS.auth.activateTotp, postJson({ code }), parseAuthUserResponse);
}

export function login(username: string, password: string): Promise<ApiResult<LoginChallenge>> {
  return authRequest(
    API_ENDPOINTS.auth.login,
    postJson({ username, password }),
    parseLoginChallenge,
  );
}

export function loginTotp(code: string): Promise<ApiResult<AuthUserResponse>> {
  return authRequest(API_ENDPOINTS.auth.loginTotp, postJson({ code }), parseAuthUserResponse);
}

export function loginRecovery(code: string): Promise<ApiResult<AuthUserResponse>> {
  return authRequest(API_ENDPOINTS.auth.loginRecovery, postJson({ code }), parseAuthUserResponse);
}

export async function logout(): Promise<ApiResult<null>> {
  const result = await authRequest(API_ENDPOINTS.auth.logout, { method: 'POST' }, () => true);
  return result.ok ? { ok: true, value: null } : result;
}

export function getMe(signal?: AbortSignal): Promise<ApiResult<AuthUserResponse>> {
  return authRequest(API_ENDPOINTS.auth.me, { method: 'GET', signal }, parseAuthUserResponse);
}
