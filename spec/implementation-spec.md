# Stock Dashboard — Implementation Spec

Status: Draft, awaiting Phase 1 review
Version: v1.6 (2026-11-18) — rev. 7: adds §15 container image (append-only; §1–§14 unchanged)
Related: `../spec-to-test-workflow/stock-dashboard/00-ingestion/`

This document is the normative source of expected behavior for the project. Expected
values may be taken from this file or from the user-approved intent diagram only.

## 1. Purpose

A local, single-user stock dashboard. The user keeps a watchlist of US tickers and sees
the latest quote for each ticker, plus a daily price chart per ticker. Quotes come from a
free market-data provider. The app runs on the developer's machine, stores its watchlist
locally, and is usable as a real personal tool, not a demo.

## 2. Scope

### 2.1 In scope

- A web UI served by a Node.js process on `http://127.0.0.1:3000`.
- A local JSON watchlist that survives restarts.
- Add, list and remove tickers.
- Latest quote per ticker: price, change, change percent, provider, timestamp.
- Per-ticker daily price chart.
- Provider fallback and rate-limit handling.
- API-key configuration through environment variables.

### 2.2 Out of scope

- User accounts, authentication, or multi-user support.
- Cloud, VPS or Docker deployment; remote access.
- Portfolio holdings, cost basis, profit/loss, or trading.
- Alerts, notifications, or background jobs.
- Non-US markets.
- Intraday charts or real-time streaming.
- Paid data plans and any feature that requires them.

## 3. Environment and configuration

| Variable | Required | Behavior |
| :-- | :-- | :-- |
| `FINNHUB_API_KEY` | Yes for quote data | Primary quote provider key. |
| `TWELVEDATA_API_KEY` | Yes for charts and fallback | Fallback source and chart source. |
| `PORT` | No | Server port. Default `3000`. |
| `DATA_DIR` | No | Directory for `watchlist.json`. Default `./data`. |

Rules:

- Keys are read from the environment only. They never appear in logs, API responses, the
  UI, or the repository.
- If `FINNHUB_API_KEY` or `TWELVEDATA_API_KEY` is missing at startup, the server still
  starts and the UI shows `Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.`
- The server listens on `127.0.0.1` only.

## 4. Functional behavior

### 4.1 Watchlist

- On first start the watchlist is empty and the UI shows `Your watchlist is empty. Add a ticker to begin.`
- A ticker is normalised to uppercase before it is stored or compared.
- A ticker is valid when it matches `^[A-Z][A-Z0-9]{0,9}$` after normalisation. Anything else is
  rejected.
- The watchlist has no cap (user decision 2026-11-16, rev. 2).
- Adding an existing ticker does not create a duplicate.
- Removal is permanent for that watchlist entry.
- The watchlist file is `$DATA_DIR/watchlist.json`; its content survives a server restart.

### 4.2 Quotes

- The dashboard shows one row per watchlist ticker.
- Each row shows: ticker, price, change, change percent, provider, quote timestamp, and a visible
  direction word `up`, `down` or `unchanged`.
- `change = price - previousClose`, rounded to 2 decimals. `changePercent = change / previousClose * 100`,
  rounded to 2 decimals.
- When `previousClose` is zero, price and change are shown as `0` and no percentage is shown
  (user decision 2026-11-16, rev. 3).
- When `previousClose` is missing or negative, the price is shown as supplied, change is shown as
  supplied or `0`, and no percentage is shown.
- Positive values carry a `+` prefix and are shown in green (`#16a34a`); negative values in red
  (`#dc2626`); zero in neutral grey (`#6b7280`).
- The default provider order is Finnhub first, Twelve Data second.
- If Finnhub fails for a ticker, Twelve Data is used for that ticker and the row's provider reads
  `Twelve Data`.
- If both providers fail for a ticker, the row shows `Price unavailable` and provider `none`.
- If the last successful quote for a ticker is older than 15 minutes, the row shows the timestamp
  with the suffix ` (stale)`.
- Quotations are cached for 60 seconds. Repeated refreshes inside 60 seconds reuse the cache and
  do not call a provider.
- The UI refreshes quotes every 60 seconds while the page is open.
- The UI shows the last refresh time as `Last updated HH:MM:SS` in local time.

### 4.3 Chart

- Each ticker has a `View chart` control and a chart page at `/chart/<TICKER>`.
- The chart is a daily close-price line for the last 30 trading days, from Twelve Data.
- The chart page shows the ticker, the chart, and `Source: Twelve Data`.
- A range selector offers `1M`, `3M`, `1Y`; `1M` is the default.
- If chart data is unavailable, the page shows `Chart unavailable` and the dashboard stays usable.
- The chart page has a `Back` link to the dashboard.

### 4.4 Errors and messages

Exact user-visible copy:

| Situation | Copy |
| :-- | :-- |
| Empty watchlist | `Your watchlist is empty. Add a ticker to begin.` |
| Invalid ticker format | `Enter a valid ticker, e.g. AAPL.` |
| Duplicate add | `AAPL is already in your watchlist.` |
| Unknown symbol | `Symbol not found.` |
| Both providers failed | `Price unavailable` |
| Missing keys | `Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.` |
| Upstream 5xx / timeout | `Data provider is unavailable. Try again shortly.` |
| Rate limited | `Too many requests. Try again in N seconds.` |
| Chart unavailable | `Chart unavailable` |

- Raw provider messages, HTTP traces or stack traces are never shown in the UI.
- The UI never renders an API key.

## 5. API contract

All responses are JSON. All errors use `{ "error": "<copy>", "code": "<CODE>" }`; the
`code` and HTTP status are fixed below.

| Method | Path | Success | Errors |
| :-- | :-- | :-- | :-- |
| `GET` | `/api/watchlist` | `200 { "tickers": ["AAPL"] }` | none |
| `POST` | `/api/watchlist` | `201 { "tickers": ["AAPL","MSFT"] }` | `400 INVALID_TICKER`, `409 DUPLICATE_TICKER`, `404 UNKNOWN_SYMBOL` |
| `DELETE` | `/api/watchlist/:ticker` | `200 { "tickers": ["MSFT"] }` | `404 NOT_IN_WATCHLIST` |
| `GET` | `/api/quotes` | `200 { "quotes": [ { "ticker": "AAPL", "price": 190.12, "change": 1.24, "changePercent": 0.66, "direction": "up", "provider": "Finnhub", "timestamp": "2026-11-16T14:30:00Z", "stale": false } ] }` | `503 MISSING_API_KEY` || `GET` | `/api/chart/:ticker?range=1M` | `200 { "ticker": "AAPL", "range": "1M", "source": "Twelve Data", "points": [ { "date": "2026-10-17", "close": 189.5 } ] }` | `400 INVALID_RANGE`, `404 UNKNOWN_SYMBOL`, `502 CHART_UNAVAILABLE` |

Behavior:

- `POST /api/watchlist` accepts `{ "ticker": "aapl" }` and stores `AAPL`.
- `GET /api/quotes` omits tickers with no quote and does not fail the whole request for one
  failed ticker.
- When no percentage can be computed, the quote object carries `"changePercent": null`.
- `range` accepts `1M`, `3M`, `1Y`; any other value is `400 INVALID_RANGE`.
- `404 UNKNOWN_SYMBOL` is returned when the provider reports the symbol does not exist.
- When no API key is configured, quote and chart endpoints return `503 MISSING_API_KEY`.
- Upstream timeouts (2 seconds) and non-2xx provider responses map to `502 PROVIDER_UNAVAILABLE`
  for quotes and `502 CHART_UNAVAILABLE` for charts.

## 6. Provider mapping

### 6.1 Finnhub (primary quotes)

`GET https://finnhub.io/api/v1/quote?symbol=<TICKER>&token=<key>`

Map `c` to price, `d` to change, `dp` to changePercent, `pc` to previousClose, `t` (Unix seconds)
to timestamp. `c = 0` and `t = 0` mean the symbol is unknown.

### 6.2 Twelve Data (fallback quotes and charts)

`GET https://api.twelvedata.com/quote?symbol=<TICKER>&apikey=<key>`

Map `close` to price, `change` to change, `percent_change` to changePercent, `previous_close`
to previousClose, `timestamp` (Unix seconds) to timestamp.

`GET https://api.twelvedata.com/time_series?symbol=<TICKER>&interval=1day&outputsize=<N>&apikey=<key>`

Map each `values[]` entry: `datetime` to date, `close` to close. `N` = 30 for `1M`, 90 for `3M`,
365 for `1Y`. The most recent point is first; the chart plots oldest to newest.

- A provider response with an `error` or `status: "error"` field counts as a provider failure.
- HTTP non-2xx also counts as a provider failure.
- Response handling is defensive: unknown shape is a provider failure, never a crash.

## 7. Rate limits and caching

- At most one upstream call per 60 seconds per ticker for quotes; a batch refresh deduplicates
  calls within the same request.
- Before an upstream call, the app checks its own local limiter: 55 calls/minute for Finnhub,
  7 calls/minute for Twelve Data. A request that would exceed a limit waits for the next slot
  (up to 2 seconds) or fails with `429 RATE_LIMITED` and copy `Too many requests. Try again in N seconds.`
- Cached data is served regardless of provider availability.

## 8. Local-only and data

- The watchlist is the only persisted data: `$DATA_DIR/watchlist.json`.
- Quotes and charts are in-memory caches only; they are not persisted.
- No telemetry or analytics.
- The only outbound requests are to `finnhub.io` and `twelvedata.com`.

## 9. Accessibility

- Every form control has an accessible name.
- Color is not the only carrier of meaning: change values also carry `+`/`-` and the ticker row
  text states up/down.
- The chart has an accessible text alternative naming the ticker and the range.

## 10. Testability requirements

- The server reads provider base URLs from `FINNHUB_BASE_URL` and `TWELVEDATA_BASE_URL`, defaulting
  to the real endpoints, so acceptance tests can point at a local fake provider.
- The app can be started with `DATA_DIR` pointing at a temporary directory.
- `GET /healthz` returns `200 { "status": "ok" }` and touches no provider.

## 11. Authentication (extension, 2026-11-18)

This section extends v1.2. Sections 1–10 remain in force except where stated here.

### 11.1 Scope change

- In scope: a first-run owner account, password login, mandatory TOTP second factor,
  recovery codes, sessions, logout, and protection of every data endpoint and page.
- This supersedes the first clause of §2.2 ("User accounts, authentication") for the single
  owner account. Multiple users, roles, sharing, invitations, password reset by email and
  self-service registration after setup remain out of scope.
- The watchlist, quotes and charts stay single-tenant: they belong to the owner account.

### 11.2 Account model

- Exactly one owner account exists. `$DATA_DIR/users.json` is the only auth state file.
- A username is 3–32 characters and must already be lowercase, matching
  `^[a-z0-9][a-z0-9._-]{2,31}$`. Uppercase input is rejected with `INVALID_USERNAME`; it
  is not silently folded. Login compares the stored lowercase name exactly.
- A password must contain at least 12 characters. There is no composition rule and no
  user-facing maximum; the 4 KiB request-body limit bounds the input.
- Passwords are stored only as Argon2id hashes (memory 64 MiB, iterations 3, parallelism 4,
  salt 16 bytes, key 32 bytes) in PHC string form. Plaintext never reaches a log or response.
- The TOTP secret and recovery-code hashes are stored in `users.json`; the file is written
  atomically with mode `0600` (best effort on Windows, where POSIX modes are advisory).
- Account setup is not persisted until the first TOTP code is verified. An abandoned setup
  leaves no account and can be restarted.

### 11.3 First-run setup

- While no owner exists, `/` always shows the landing page with a `Sign in` action. The
  sign-in page always shows the username/password form and `New here? Set up your account`.
  `/dashboard` and `/chart/*` redirect to `/login`; `/api/auth/status` reports
  `setupRequired: true`.
- After setup, the sign-in page still shows `New here? Set up your account`. Opening
  `/setup` then shows `Create account` with `Sign-up is closed on this installation.` and a
  `Back to sign in` link; it never reopens account creation or overwrites the owner.
- Once an owner exists, anonymous visitors see a public landing page at `/` with a
  `Sign in` call to action. `/dashboard`, `/chart/*` and every other protected page
  redirect to `/login`; after sign-in the user returns to the requested page.
- Authenticated visitors who open `/`, `/login` or `/setup` are sent to `/dashboard`.
- `POST /api/auth/setup` validates the username and password, creates a pending setup
  (15 minutes) and returns the TOTP secret, the `otpauth://` URI and ten recovery codes.
  It does not create a session and does not persist the account yet.
- The UI shows a QR code generated from the `otpauth://` URI, the setup key for manual
  entry, and the recovery codes once. The user must confirm the codes are saved.
- `POST /api/auth/2fa/activate` with a valid code persists the account, marks TOTP enabled
  and creates the first session.
- Setup is refused with `409 SETUP_COMPLETE` once an owner exists. There is no public
  registration endpoint.

### 11.4 Login and second factor

- Step 1: `POST /api/auth/login` with username and password establishes a pending
  2FA challenge (5 minutes) and returns `status: "totp_required"`. It does not create a
  session.
- Step 2: `POST /api/auth/login/totp` with a six-digit code completes the login and creates
  a session. TOTP is SHA-1, six digits, 30-second period, accepted with ±1 step skew.
  A code whose time step was already used is rejected.
- Recovery: step 2 can instead use `POST /api/auth/login/recovery` with a single-use
  recovery code. A used code is removed.
- Failed password, TOTP and recovery attempts use one generic failure copy per kind, never
  reveal whether a username exists, and are rate limited: five failures for one username
  in fifteen minutes returns `429 RATE_LIMITED` for 60 seconds before another attempt.
- A pending challenge allows at most five code attempts, then it is destroyed. The next
  attempt returns `401 CHALLENGE_EXPIRED`; a destroyed challenge is never re-usable.
- Pending setup allows at most five activation attempts, then it expires.

### 11.5 Sessions

- A session token is 32 random bytes, base64url encoded, sent only in the `sd_session`
  cookie. The server stores only its SHA-256 digest.
- `sd_session` is `HttpOnly`, `SameSite=Strict`, `Path=/`, and `Secure` when
  `AUTH_COOKIE_SECURE=true`. Absolute lifetime 12 hours; idle lifetime 30 minutes, extended
  on each authenticated request.
- Logout revokes the current session. Sessions live in memory, so a restart requires login
  again.
- Auth cookies: `sd_challenge` (5 min) and `sd_enroll` (15 min), both `HttpOnly`,
  `SameSite=Strict`, `Path=/api/auth`.
- Every protected API request without a valid session returns
  `401 UNAUTHENTICATED` with `Authentication required.` The SPA then shows the login screen
  with `Your session expired. Sign in again.` when the session ends mid-use.

### 11.6 API additions

All errors use the §5 envelope.

| Method | Path | Request | Success | Errors |
| :-- | :-- | :-- | :-- | :-- |
| `GET` | `/api/auth/status` | — | `200 { setupRequired, authenticated, user? }` | none |
| `POST` | `/api/auth/setup` | `{ username, password }` | `201 { username, secret, otpauthUrl, recoveryCodes }` | `400 INVALID_USERNAME`, `400 WEAK_PASSWORD`, `409 SETUP_COMPLETE` |
| `POST` | `/api/auth/2fa/activate` | `{ code }` | `200 { user }` + session | `401 INVALID_CODE`, `401 ENROLLMENT_EXPIRED` |
| `POST` | `/api/auth/login` | `{ username, password }` | `200 { status: "totp_required", expiresInSeconds }` | `400 INVALID_REQUEST`, `401 INVALID_CREDENTIALS`, `429 RATE_LIMITED` |
| `POST` | `/api/auth/login/totp` | `{ code }` | `200 { user }` + session | `401 INVALID_CODE`, `401 CHALLENGE_EXPIRED`, `429 RATE_LIMITED` |
| `POST` | `/api/auth/login/recovery` | `{ code }` | `200 { user }` + session | `401 INVALID_RECOVERY_CODE`, `401 CHALLENGE_EXPIRED`, `429 RATE_LIMITED` |
| `POST` | `/api/auth/logout` | — | `204` | none |
| `GET` | `/api/auth/me` | — | `200 { user }` | `401 UNAUTHENTICATED` |

`user` is `{ "username": "owner", "totpEnabled": true }`.

Every existing data endpoint (`/api/watchlist`, `/api/quotes`, `/api/chart/*`) requires a
valid session. `/healthz` and the static client stay public.

### 11.7 Copy

| Situation | Copy |
| :-- | :-- |
| Authentication required / expired | `Authentication required.` |
| Invalid username or password | `Invalid username or password.` |
| Invalid TOTP code | `Invalid verification code.` |
| Invalid recovery code | `Invalid recovery code.` |
| Pending challenge expired | `Your sign-in attempt expired. Sign in again.` |
| Pending setup expired | `Setup timed out. Start again.` |
| Cross-origin mutation | `Cross-origin requests are not allowed.` |
| Setup already complete | `Setup is already complete.` |
| Username format | `Use 3-32 lowercase letters, numbers, dots, dashes or underscores.` |
| Password too short | `Use at least 12 characters.` |
| Missing login fields | `Enter your username and password.` |
| Closed sign-up | `Sign-up is closed on this installation.` |
| Rate limited | `Too many requests. Try again in N seconds.` |
| Session ended in the SPA | `Your session expired. Sign in again.` |

UI labels: `Sign in`, `Sign out`, `Create account`, `Username`, `Password`,
`Confirm password`, `Two-factor authentication`, `Verification code`,
`Use a recovery code`, `Recovery code`, `Back to sign in`, `Set up authenticator`,
`Scan this QR code with your authenticator app.`, `Or enter this setup key manually:`,
`Save these recovery codes.`, `Continue`, `Copy`, `Copied`.

### 11.8 Security requirements

- Constant-time secret comparison; a dummy Argon2id verification runs for unknown usernames.
- No secret, password, TOTP code, recovery code, session token or cookie value is logged.
- Mutating requests with an `Origin` header must match the request host (CSRF defence);
  auth cookies are `SameSite=Strict`.
- Login, TOTP, recovery and setup responses never contain password or TOTP-secret hashes.
- The TOTP secret is returned only to the setup response and to a request holding the
  matching `sd_enroll` cookie.
- The server binds loopback only, as in §3. `AUTH_COOKIE_SECURE` defaults to false for
  plain-HTTP loopback and must be true when served over HTTPS.

### 11.9 Testability

- The account store is a plain JSON file under `DATA_DIR`, so tests use a temporary
  directory.
- The TOTP clock is injectable for unit tests; an accepted code cannot be replayed.
- Tests cover: first-run setup, duplicate setup, username and password validation, generic
  credential errors, mandatory TOTP, wrong code, challenge expiry, replay, recovery code
  single use, rate limiting, cookie flags, logout, session expiry, CSRF origin rejection,
  protected endpoints, and no secret leakage.

## 12. Quote timestamp display (extension, 2026-11-18)

The API keeps `timestamp` as ISO-8601. The dashboard displays it as
`dd-MMM-yyyy` in UTC, followed by ` (stale)` when stale, for example
`17-Nov-2026 (stale)`. The `<time datetime>` attribute keeps the ISO-8601 value.
This is a display-only rule and changes no §4.2 or §5 value.

## 13. SQLite persistence (extension, 2026-11-18)

This section supersedes the file names in §8 and the `users.json` references in §11.2.
All behavioural rules stay the same.

### 13.1 Storage

- The only persisted state is `$DATA_DIR/stock-dashboard.db`, a SQLite database opened by
  the server at startup. It is created with mode `0600` before the driver opens it.
- Tables:
  - `watchlist(position INTEGER PRIMARY KEY, ticker TEXT NOT NULL UNIQUE)` — insertion
    order preserved; one row per canonical ticker.
  - `users(id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT,
    totp_secret TEXT, totp_enabled INTEGER, last_totp_step INTEGER, created_at TEXT,
    updated_at TEXT)` — exactly one row.
  - `recovery_codes(user_id TEXT REFERENCES users(id) ON DELETE CASCADE, code_hash TEXT,
    PRIMARY KEY (user_id, code_hash))` — hashed, single-use.
- Schema changes are tracked with `PRAGMA user_version`; version 1 is the initial schema.
- Connections use `foreign_keys=ON`, `busy_timeout=5000`, `journal_mode=DELETE`,
  `synchronous=FULL` and a single open connection. Each mutation is one transaction.
- Quotes, charts, sessions, 2FA challenges, pending setup and rate limits remain in memory;
  they are never written to the database.
- The database is the only file in `$DATA_DIR` after migration, apart from
  `<name>.json.migrated` backups kept for operator recovery.

### 13.2 One-time migration from JSON

- On startup, if the corresponding table is empty and the legacy file exists:
  - `watchlist.json` (`{"tickers":[...]}`) is validated and imported.
  - `users.json` (`{"version":1,"users":[...]}`) is validated and imported with its
    recovery-code hashes.
- A successful import renames the source to `<name>.json.migrated` (best effort). A corrupt
  legacy file is an error naming the file, matching the previous fail-closed behaviour.
- If the database already contains data, the legacy file is not imported and is left alone.
- No production code writes JSON state after migration.

### 13.3 Operator notes

- Stop the server and copy `stock-dashboard.db` to back it up.
- Recovery when both the authenticator and all recovery codes are lost: use any SQLite
  client to delete the row from `users` (recovery codes cascade) and restart; the app
  returns to `/setup`. Deleting the whole database also clears the watchlist.
- The database file is local-only and must never be served or committed.

## 14. Multi-user accounts (extension, 2026-11-18)

This section supersedes the single-owner statements in ��11 and ��13. All security rules in
��11 remain.

### 14.1 Accounts

- Any number of accounts may exist. Each account has a unique username, its own Argon2id
  password hash, TOTP secret, recovery codes and sessions.
- `POST /api/auth/setup` is always available and creates a pending account for a unique
  username. A duplicate username returns `409 USERNAME_TAKEN` with
  `That username is already taken.` `SETUP_COMPLETE` is no longer returned by setup.
- `GET /api/auth/status` keeps `setupRequired`, now meaning "no accounts exist yet"; it is
  informational and does not close registration.
- Login, TOTP, recovery, rate limits, cookies, CSRF and error copy are unchanged except
  that the username resolves to its own account.
- No endpoint lists accounts or exposes user IDs. The public `user` object stays
  `{ username, totpEnabled }`.

### 14.2 Separate watchlists

- Each account owns its own watchlist. `GET/POST /api/watchlist`,
  `DELETE /api/watchlist/{ticker}` and `GET /api/quotes` operate only on the authenticated
  account's list.
- A new account starts with an empty watchlist.
- The current single owner and watchlist migrate unchanged to that account. If a migrated
  database contains watchlist rows but no account, the first account created claims them.

### 14.3 Storage

SQLite schema version 2 replaces the single watchlist table:

```sql
watchlist(
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  ticker TEXT NOT NULL,
  PRIMARY KEY (user_id, position),
  UNIQUE (user_id, ticker)
)
```

- Version 1 rows are copied to the v1 owner's ID when one exists, otherwise left with
  `user_id IS NULL` for the first account to claim.
- The users and recovery-code tables keep their shape; multiple user rows are allowed.
- Transactions, fail-closed migration, snapshots and the single database file remain.

### 14.4 UI

- `/setup` is the public **Create account** page whenever the visitor is anonymous;
  authenticated visitors go to `/dashboard`.
- `/login` always shows the credential form and `New here? Set up your account`.
- Duplicate usernames show `That username is already taken.` from the API.
- After activation the new account is signed in and sees its own empty dashboard.

## 15. Container image (extension, 2026-11-18)

This section adds an optional deployment form; the local process behaviour is unchanged.

- A multi-stage image builds the React client, embeds it in a static Go binary and runs on
  a distroless nonroot base.
- `BIND_HOST` defaults to `127.0.0.1`. The image sets `BIND_HOST=0.0.0.0` because a
  container must accept traffic from the host; the host port must be published on
  `127.0.0.1` only (`-p 127.0.0.1:3000:3000`).
- `DATA_DIR=/data`; the SQLite database lives on a mounted volume. The root filesystem may
  be read-only; `/tmp` is a tmpfs.
- API keys are runtime environment variables and are never copied into the image.
- The image includes a tiny `/app/healthcheck` binary that probes `/healthz` every 30 s.
- The app runs as UID/GID 65532 with no shell. Docker Compose maps the same volume and
  loopback port and reads keys from the environment.
