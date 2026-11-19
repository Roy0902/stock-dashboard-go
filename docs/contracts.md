# Contracts (normative for this implementation)

This file restates the externally observable contract that both the Go API and the React
client must satisfy. Behaviour originates in `../spec/implementation-spec.md`
v1.6 and the approved workflow artifacts. Where this file and the spec differ, the spec wins.

## 1. Runtime

| Item | Value |
| :-- | :-- |
| Bind address | `BIND_HOST`, default `127.0.0.1` (container image sets `0.0.0.0`) |
| Port | `PORT`, default `3000` |
| Data directory | `DATA_DIR`, default `./data` |
| Persisted state | `$DATA_DIR/stock-dashboard.db` (SQLite); legacy JSON imported once and renamed |
| API prefix | `/api` |
| Health | `GET /healthz` -> `200 {"status":"ok"}`, touches no provider |

Environment:

- `FINNHUB_API_KEY` - primary quotes.
- `TWELVEDATA_API_KEY` - fallback quotes and the only chart source.
- `BIND_HOST` - listen IP, default `127.0.0.1`; the container image uses `0.0.0.0`.
- `ALLOWED_HOSTS` - comma-separated `Host` allowlist, default
  `127.0.0.1,localhost,::1`. Entries are hostnames or IP literals, never a scheme or port.
- `FINNHUB_RPM` / `TWELVEDATA_RPM` - per-minute upstream budgets, defaults `55` and `7`.
- `UPSTREAM_TIMEOUT` - per-request upstream timeout, default `2s`.
- `QUOTE_CACHE_TTL` - quote cache lifetime, default `1m`.
- `FINNHUB_BASE_URL` - default `https://finnhub.io`.
- `TWELVEDATA_BASE_URL` - default `https://api.twelvedata.com`.

Both API keys are required for quote and chart data. When either is absent the process still
starts and the two data endpoints answer `503 MISSING_API_KEY`.

## 1a. Authentication

The product supports multiple accounts with mandatory TOTP, each with its own watchlist. The
full contract lives in [`contracts-auth.md`](contracts-auth.md); the original frozen auth
plan is `../spec/user-auth/user-auth.md` (`88e2ff2c…`), superseded for
multi-user by §11 of the auth contract.

- Public: `/healthz`, static assets, `/api/auth/status`, `/api/auth/setup`,
  `/api/auth/login`, `/api/auth/login/totp`, `/api/auth/login/recovery`, `/api/auth/logout`.
  Activation needs the enrollment cookie.
- Session: `/api/auth/me` and every watchlist, quote and chart endpoint.
- `/setup` is always the Create account page for anonymous visitors; `/login` always offers
  `New here? Set up your account`. Session cookies are `HttpOnly`, `SameSite=Strict`;
  tokens never reach JavaScript.
- Persisted state: `$DATA_DIR/stock-dashboard.db` (SQLite). Legacy `users.json` and
  `watchlist.json` are imported once and renamed `.migrated`. Each account has its own
  watchlist and recovery codes.

## 2. Error envelope

Every API error is exactly:

```json
{ "error": "<user-visible copy>", "code": "<STABLE_CODE>" }
```

| Code | Status | Copy |
| :-- | :-- | :-- |
| `INVALID_TICKER` | 400 | `Enter a valid ticker, e.g. AAPL.` |
| `DUPLICATE_TICKER` | 409 | `<TICKER> is already in your watchlist.` |
| `UNKNOWN_SYMBOL` | 404 | `Symbol not found.` |
| `NOT_IN_WATCHLIST` | 404 | `<TICKER> is not in your watchlist.` |
| `INVALID_RANGE` | 400 | `Range must be 1M, 3M or 1Y.` |
| `MISSING_API_KEY` | 503 | `Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.` |
| `PROVIDER_UNAVAILABLE` | 502 | `Data provider is unavailable. Try again shortly.` |
| `RATE_LIMITED` | 429 | `Too many requests. Try again in N seconds.` |
| `CHART_UNAVAILABLE` | 502 | `Chart unavailable` |
| `INTERNAL_ERROR` | 500 | `Something went wrong. Try again shortly.` |
| `REQUEST_TOO_LARGE` | 413 | `Request body is too large.` |

A request body over 4 KiB returns `413 REQUEST_TOO_LARGE`; malformed smaller bodies keep
the endpoint-specific `400` code.

`DUPLICATE_TICKER` and `NOT_IN_WATCHLIST` copy is engineering copy (spec is silent).

## 3. Endpoints

### `GET /api/watchlist`

`200 {"tickers":["AAPL","MSFT"]}` - insertion order, canonical uppercase.

### `POST /api/watchlist`

Request `{"ticker":"aapl"}`; case-insensitive input.

`201 {"tickers":["AAPL","MSFT"]}` with the full list after insertion.

Check order: format -> duplicate -> provider verification.

- format: after trimming and uppercasing, `^[A-Z][A-Z0-9]{0,9}$`
- duplicate: already present -> `409 DUPLICATE_TICKER`
- provider: Finnhub first, Twelve Data second. An unknown symbol from either provider ->
  `404 UNKNOWN_SYMBOL`; both failing for another reason -> `502 PROVIDER_UNAVAILABLE`
  (credential failures follow §4: `503 MISSING_API_KEY`); budget refusal -> `429 RATE_LIMITED`.

### `DELETE /api/watchlist/{ticker}`

`200 {"tickers":["MSFT"]}`. Unknown entry -> `404 NOT_IN_WATCHLIST`. A malformed path
(not `^[A-Za-z][A-Za-z0-9]{0,9}$` after trimming) -> `400 INVALID_TICKER` and no store write.

### `GET /api/quotes`

`200`:

```json
{
  "quotes": [
    {
      "ticker": "AAPL",
      "price": 190.12,
      "change": 1.24,
      "changePercent": 0.66,
      "direction": "up",
      "provider": "Finnhub",
      "timestamp": "2026-11-16T14:30:00Z",
      "stale": false
    }
  ]
}
```

- One entry per listed ticker that has a usable quote; failures are omitted, never fatal.
- Finnhub first, Twelve Data second; provider attribution is `Finnhub`, `Twelve Data` or `none`.
- `change = round2(price - previousClose)`;
  `changePercent = round2(change / previousClose * 100)`.
- `previousClose == 0` -> `price: 0`, `change: 0`, `changePercent: null`,
  `direction: "unchanged"`.
- `previousClose` missing or negative -> price as supplied, `change: 0`,
  `changePercent: null`, `direction: "unchanged"`.
- `stale` is `now - timestamp > 15 minutes`.
- The API always returns `timestamp` as ISO-8601; `dd-MMM-yyyy` is a display format only
  (see §6).
- Quotes are cached in memory for 60 seconds per ticker.
- A partial batch is `200`: tickers without a usable quote are omitted.
- If every needed ticker was refused by a local budget and none was served ->
  `429 RATE_LIMITED`.
- If none was served and at least one ticker failed because all providers errored ->
  `502 PROVIDER_UNAVAILABLE`. An upstream `401`/`403` is not an outage: it is a credential
  failure and follows the §4 rule (`503 MISSING_API_KEY`).
- A cancelled refresh is never a clean empty `200`; with nothing served the batch is `502`
  (§4).
- When providers fail, a previously successful observation younger than 15 minutes is
  still served (spec §7, "cached data is served regardless of provider availability").
  SCN-022's cold-cache case therefore shows `Price unavailable`; an older observation is
  omitted.
- Without API keys the `503 MISSING_API_KEY` gate runs before the watchlist is read, so a
  keyless empty watchlist also answers 503. The empty-state copy needs keys configured.
- An empty watchlist with keys configured is `200 {"quotes":[]}`.

### `GET /api/chart/{ticker}?range=1M`

`200`:

```json
{
  "ticker": "AAPL",
  "range": "1M",
  "source": "Twelve Data",
  "points": [{ "date": "2026-10-17", "close": 189.5 }]
}
```

- `range` is `1M` (default, 30 points), `3M` (90), `1Y` (365); anything else ->
  `400 INVALID_RANGE`.
- A malformed ticker path is `400 INVALID_TICKER`; a well-formed but unknown symbol is
  `404 UNKNOWN_SYMBOL`. The chart page renders the returned copy.
- Points are ordered oldest to newest.
- The ticker does not need to be in the watchlist.
- Twelve Data `time_series` is the only source.
- Unknown symbol -> `404 UNKNOWN_SYMBOL`; any other provider failure -> `502 CHART_UNAVAILABLE`
  (credential failures follow §4: `503 MISSING_API_KEY`).

## 4. Provider mapping

Finnhub quote: `GET {base}/api/v1/quote?symbol=<T>&token=<key>`
`c` price, `d` change, `dp` changePercent, `pc` previousClose, `t` unix seconds.
`c == 0 && t == 0` means unknown symbol. `d`/`dp`/`pc` may be absent (`pc` null).

Twelve Data quote: `GET {base}/quote?symbol=<T>&apikey=<key>`
`close` price, `change` change, `percent_change` changePercent, `previous_close`
previousClose, `timestamp` unix seconds.

Twelve Data series: `GET {base}/time_series?symbol=<T>&interval=1day&outputsize=<N>&apikey=<key>`
`values[].datetime` -> date, `values[].close` -> close. Values arrive newest first.

A body with `error` or `status == "error"` is a provider failure. Non-2xx is a provider
failure. Unknown shapes must never panic.

- Upstream `429` maps to `429 RATE_LIMITED`; `Retry-After` is parsed from the upstream
  header (seconds or HTTP date) and echoed.
- Upstream `401`/`403` is a distinct credential failure, reported as
  `503 MISSING_API_KEY`; provider text is never returned or logged.
- A cancelled request context is distinct from an outage and never becomes a clean empty
  batch. When nothing was served, the batch fails with `502 PROVIDER_UNAVAILABLE`.
- One bounded, jittered retry is allowed for idempotent GETs on a transient transport
  failure; timeouts are not retried.

## 5. Provider limits

| Provider | Calls / minute | Notes |
| :-- | :-- | :-- |
| Finnhub | 55 | primary quotes |
| Twelve Data | 7 | fallback quotes and charts |

Before an upstream call the local sliding-window budget is checked. If no slot is free, wait
for the next slot when the wait is at most 2 seconds; otherwise refuse with `429`.

Upstream timeout: 2 seconds per request. `FINNHUB_RPM`, `TWELVEDATA_RPM`,
`UPSTREAM_TIMEOUT` and `QUOTE_CACHE_TTL` override these defaults.

## 6. UI contract (role, name, copy)

| Element | Role / text | Notes |
| :-- | :-- | :-- |
| Ticker input | `textbox` named `Ticker` | placeholder `e.g. AAPL` |
| Add control | `button` named `Add ticker` | disabled while a request is in flight |
| Message slot | `alert` | exactly one visible message |
| Empty state | `alert`: `Your watchlist is empty. Add a ticker to begin.` | |
| Ticker row | `listitem` accessible name = ticker | exactly one per listed ticker |
| Price | text `190.12` | 2 decimals |
| Change | text `+1.24` / `-10.00` / `0.00` | semantic tokens `--color-up` / `--color-down` / `--color-neutral`; dark `#16a34a` / `#dc2626` / `#6b7280`, light `#15803d` / `#b91c1c` / `#4b5563` (AA on white) |
| Direction | text `up` / `down` / `unchanged` | never colour-only |
| Provider | `Finnhub` / `Twelve Data` / `none` | |
| Timestamp | `dd-MMM-yyyy` in UTC, plus ` (stale)` when stale | `<time dateTime>` keeps the ISO-8601 value |
| Unavailable | `Price unavailable` | provider `none` |
| Last updated | `status`: `Last updated HH:MM:SS` | local time, 24h |
| Chart link | `link` named `View chart` | inside its row |
| Chart page heading | ticker text | |
| Range group | `group` named `Range` | buttons `1M`, `3M`, `1Y`; `1M` selected |
| Chart | `img` aria-label `<TICKER> price chart, <RANGE> range` | custom SVG |
| Source | `Source: Twelve Data` | |
| Chart unavailable | `alert`: `Chart unavailable` | dashboard stays usable |
| Back | `link` named `Back` | points to `/dashboard` |

The client renders server-derived numbers; it never recomputes change or percent.
Colour is decoration only: the sign and the direction word carry the meaning.

Shell copy (engineering copy, not part of the frozen acceptance contract):

| Element | Copy |
| :-- | :-- |
| Product name | `Stock dashboard` |
| Tagline | `Quotes and daily charts for the tickers you follow.` |
| Skip link | `Skip to content` |
| Placeholder shell (client not built) | `Stock Dashboard` plus a build instruction |

The placeholder shell is only reachable when the embedded client is absent; it is not part
of the user-facing product.

UI labels and hints (shipped in `web/src/locale/en.ts`; not server copy):

| Element | Copy |
| :-- | :-- |
| Password reveal, hidden | `Show password` |
| Password reveal, shown | `Hide password` |
| Bootstrap retry action | `Try again` |
| Render-error recovery action | `Reload` |
| Two-factor challenge expiry | `Code expires in N minutes.` / `Code expires in N seconds.` |
| Watchlist loading announcement | `Loading watchlist` |
| Just-added row pending | `Loading price` |
| Successful but empty chart series | `No data for this range` |
| Add control while pending | `Adding…` |

## 7. Routes

| Path | Render |
| :-- | :-- |
| `/` | public landing page when an account exists; first run redirects to `/setup`; primary action `Sign in`; authenticated visitors go to `/dashboard` |
| `/dashboard` | dashboard (session required) |
| `/login` | sign-in. The form is always followed by `New here? Set up your account` |
| `/setup` | create-account form. Always reachable: multiple accounts are supported (D-34), and first run redirects here. |
| `/chart/:ticker` | chart page, ticker uppercased (session required) |

Unknown non-API paths return the SPA shell. `/api/*` unknown paths return
`404 {"error":"Not found.","code":"NOT_FOUND"}`.
