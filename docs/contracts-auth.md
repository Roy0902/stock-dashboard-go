# Authentication contract (extension)

Normative for the `user-auth` extension. Sources:
`../spec/implementation-spec.md` §11 (v1.3, append-only) and the frozen plan
`../spec/user-auth/user-auth.md` (`88e2ff2c…`). Where this file and those
differ, the spec wins.

Any number of accounts may exist. Each account has its own password, TOTP secret, recovery
codes, sessions and watchlist; registration is open. §11 records the multi-user extension
(2026-11-18) that supersedes the single-owner wording in the frozen plans and in earlier
revisions of this file.

## 1. Configuration

| Variable | Default | Meaning |
| :-- | :-- | :-- |
| `AUTH_COOKIE_SECURE` | `false` | Set `Secure` on auth cookies; enable behind HTTPS |
| `ALLOWED_HOSTS` | `127.0.0.1,localhost,::1` | Accepted `Host` values; comma-separated hostnames or IPs |

No auth-disable switch. Account sessions are always enforced.

## 2. Persistence

`$DATA_DIR/stock-dashboard.db`, a SQLite database created with mode `0600`:

```sql
watchlist(user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
          position INTEGER NOT NULL, ticker TEXT NOT NULL,
          PRIMARY KEY (user_id, position), UNIQUE (user_id, ticker))
users(id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE,
      password_hash TEXT NOT NULL, totp_secret TEXT NOT NULL,
      totp_enabled INTEGER NOT NULL, last_totp_step INTEGER NOT NULL,
      created_at TEXT NOT NULL, updated_at TEXT NOT NULL)
recovery_codes(user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
               code_hash TEXT NOT NULL, PRIMARY KEY (user_id, code_hash))
```

- `PRAGMA user_version = 2` (version 1 databases migrate, §11); `foreign_keys=ON`,
  `busy_timeout=5000`, `journal_mode=DELETE`, `synchronous=FULL`, one open connection.
- Every watchlist mutation, account create/update and recovery-code consumption is one
  transaction. Recovery hashes are replaced atomically with the account row.
- Any number of accounts may exist. Each account has its own password hash, TOTP secret,
  recovery codes and watchlist; each watchlist preserves insertion order.
- Legacy `watchlist.json` / `users.json` are imported once when the matching table is empty
  and renamed `<name>.json.migrated`; corrupt legacy files fail startup by name. If the
  database already has data, legacy files are left untouched. If a migrated v1 database has
  watchlist rows but no account, the first account created claims those rows (§11).
- No production code writes JSON state after migration.
- Sessions, challenges, pending setups and rate limits live in memory only.

## 3. Crypto parameters

| Concern | Rule |
| :-- | :-- |
| Password hash | PBKDF2-HMAC-SHA256, 600,000 iterations, salt 16 bytes, key 32 bytes, PHC string (`$pbkdf2-sha256$i=...`); legacy Argon2id hashes still verify and are rehashed on the next successful password login |
| Password length | at least 12 characters (runes); no composition rules and no user-facing maximum |
| Username | trim; must already be lowercase; `^[a-z0-9][a-z0-9._-]{2,31}$`; uppercase is rejected |
| TOTP | RFC 6238, SHA-1, 6 digits, 30 s period, ±1 step, replay rejected by `lastTotpStep` |
| TOTP issuer | `Stock Dashboard`, account = username |
| Session token | 32 random bytes, base64url (43 chars); server stores SHA-256 hex |
| Challenge token | 32 random bytes, base64url; SHA-256 stored; 5 minutes; 5 code attempts |
| Enrollment token | 32 random bytes, base64url; SHA-256 stored; 15 minutes; 5 code attempts |
| Recovery code | 10 codes, 10 random bytes each, base32 uppercase grouped `XXXX-XXXX-XXXX-XXXX`; SHA-256 stored; single use |
| Rate limit | 5 failed password attempts per username in 15 min → 429 for 60 s |

## 4. Cookies

| Cookie | Value | Path | Lifetime | Flags |
| :-- | :-- | :-- | :-- | :-- |
| `sd_session` | session token | `/` | 12 h absolute, 30 min idle | HttpOnly, SameSite=Strict, Secure if configured |
| `sd_challenge` | challenge token | `/api/auth` | 5 min | HttpOnly, SameSite=Strict, Secure if configured |
| `sd_enroll` | pending setup token | `/api/auth` | 15 min | HttpOnly, SameSite=Strict, Secure if configured |

Clearing uses `Max-Age=0` plus an epoch `Expires`. Never expose token values in JSON or JS.

## 5. Flow

1. No accounts yet: `GET /api/auth/status` → `setupRequired:true`. The SPA shows `/setup`.
2. `POST /api/auth/setup` (available at any time, §11) validates and stores a **pending
   setup in memory**. It returns the secret, otpauth URI and ten recovery codes, and sets
   `sd_enroll`. No account is written yet.
3. `POST /api/auth/2fa/activate` verifies a code against the pending secret, writes the
   account row, sets `lastTotpStep`, clears `sd_enroll`, creates a session and sets
   `sd_session`.
4. Later: `POST /api/auth/login` verifies username+password and sets `sd_challenge`.
5. `POST /api/auth/login/totp` or `/login/recovery` consumes the challenge, sets
   `sd_session`, clears `sd_challenge`.
6. `POST /api/auth/logout` revokes the current session and clears the cookie.
7. Every protected API request without a valid session → 401 `UNAUTHENTICATED`.

Pending setup can be replaced by submitting `/setup` again (fresh secret and codes).
A process restart discards it and `setupRequired` stays true.

## 6. API

All error bodies are `{"error":"<copy>","code":"<CODE>"}` with `cache-control: no-store`.

### `GET /api/auth/status`

Public.

```json
{ "setupRequired": false, "authenticated": true,
  "user": { "username": "owner", "totpEnabled": true } }
```

`user` is omitted when unauthenticated. No cookie is set.

### `POST /api/auth/setup`

Public, always available; registers a new account for a unique username (§11). Body
`{ "username": string, "password": string }`.

`201`:

```json
{
  "username": "owner",
  "secret": "JBSWY3DPEHPK3PXP",
  "otpauthUrl": "otpauth://totp/Stock%20Dashboard:owner?secret=...&issuer=Stock%20Dashboard&algorithm=SHA1&digits=6&period=30",
  "recoveryCodes": ["AAAA-BBBB-CCCC-DDDD", "..."]
}
```

Errors: `400 INVALID_USERNAME`, `400 WEAK_PASSWORD`, `400 INVALID_REQUEST`,
`409 USERNAME_TAKEN`, `403 CROSS_ORIGIN_REQUEST`.

### `POST /api/auth/2fa/activate`

Requires `sd_enroll`. Body `{ "code": "123456" }`.

`200 {"user":{"username":"owner","totpEnabled":true}}` + `sd_session`; clears `sd_enroll`.

Errors: `400 INVALID_REQUEST`, `401 INVALID_CODE`, `401 ENROLLMENT_EXPIRED`,
`429 RATE_LIMITED`.

### `POST /api/auth/login`

Public. Body `{ "username": string, "password": string }`.

`200 {"status":"totp_required","expiresInSeconds":300}` + `sd_challenge`.

Errors: `400 INVALID_REQUEST`, `401 INVALID_CREDENTIALS`, `429 RATE_LIMITED`
with `Retry-After`.

### `POST /api/auth/login/totp`

Requires `sd_challenge`. Body `{ "code": string }`.

`200 {"user":{...}}` + `sd_session`; clears `sd_challenge`.

Errors: `400 INVALID_REQUEST`, `401 INVALID_CODE`, `401 CHALLENGE_EXPIRED`,
`429 RATE_LIMITED`.

### `POST /api/auth/login/recovery`

Requires `sd_challenge`. Body `{ "code": string }` (`XXXX-XXXX-XXXX-XXXX`, case-insensitive,
hyphens optional).

`200 {"user":{...}}` + `sd_session`; consumes the code.

Errors: `400 INVALID_REQUEST`, `401 INVALID_RECOVERY_CODE`, `401 CHALLENGE_EXPIRED`,
`429 RATE_LIMITED`.

### `POST /api/auth/logout`

`204`, clears `sd_session`, revokes the token. Idempotent; works without a session.

### `GET /api/auth/me`

`200 {"user":{...}}` or `401 UNAUTHENTICATED`.

### Protected endpoints

`/api/watchlist*`, `/api/quotes`, `/api/chart/*` require `sd_session`.
`/healthz` and the static client stay public.

## 7. Error codes and copy

| Code | Status | Copy |
| :-- | :-- | :-- |
| `UNAUTHENTICATED` | 401 | `Authentication required.` |
| `INVALID_CREDENTIALS` | 401 | `Invalid username or password.` |
| `INVALID_CODE` | 401 | `Invalid verification code.` |
| `INVALID_RECOVERY_CODE` | 401 | `Invalid recovery code.` |
| `CHALLENGE_EXPIRED` | 401 | `Your sign-in attempt expired. Sign in again.` |
| `ENROLLMENT_EXPIRED` | 401 | `Setup timed out. Start again.` |
| `INVALID_USERNAME` | 400 | `Use 3-32 lowercase letters, numbers, dots, dashes or underscores.` |
| `WEAK_PASSWORD` | 400 | `Use at least 12 characters.` |
| `INVALID_REQUEST` | 400 | `Enter your username and password.` (missing code → `INVALID_CODE`) |
| `CROSS_ORIGIN_REQUEST` | 403 | `Cross-origin requests are not allowed.` |
| `RATE_LIMITED` | 429 | `Too many requests. Try again in N seconds.` + `Retry-After` (a one-second window reads `1 second.`) |
| `REQUEST_TOO_LARGE` | 413 | `Request body is too large.` |
| `INTERNAL_ERROR` | 500 | `Something went wrong. Try again shortly.` |

Session expiry in the SPA: `Your session expired. Sign in again.`

## 8. Security rules

- CSRF: for every unsafe method (`POST`, `PUT`, `PATCH`, `DELETE`) under `/api`, if `Origin`
  is present it must equal `http://<Host>` or `https://<Host>`; otherwise
  `403 CROSS_ORIGIN_REQUEST`. `Origin: null` is rejected. SameSite=Strict is also set.
- Session cookies are cleared only when the request presented `sd_session`. A request with
  no session cookie never receives a clearing `Set-Cookie`, so a cross-site `<img>` cannot
  force a sign-out. A presented invalid/expired cookie is still expired.
- Content type: auth POSTs and watchlist POST require `application/json` when a type is
  present; non-JSON is refused.
- Unknown usernames run a dummy PBKDF2 verify so timing does not reveal existence.
- All token, code and hash comparisons use `crypto/subtle`.
- Never log or return passwords, secrets, codes, recovery codes, token values or hashes.
  Log auth events as `event`, `username`, `request_id`.
- TOTP step replay is persisted; a persistence failure fails the login closed.
- Password and TOTP verification reserve an attempt before the expensive check, so
  concurrent bursts cannot exceed 5 password failures per 15 minutes or 5 code attempts per
  challenge. A global per-minute budget and a small verification semaphore bound
  unauthenticated password-hashing work; excess setup/login work is refused with `429` rather
  than queued. A second concurrent setup is refused with `429`.
- Password hashes, TOTP secrets and recovery hashes never appear in `/api/auth/*` responses
  except the setup secret and recovery codes returned once.

## 9. Frontend

Routes:

| Route | Access |
| :-- | :-- |
| `/` | public landing page; authenticated visitors go to `/dashboard`; the primary action is always `Sign in` |
| `/dashboard` | session; anonymous visitors go to `/login` |
| `/chart/:ticker` | session; anonymous visitors go to `/login` |
| `/setup` | create-account form; always reachable (D-34). First run redirects here. |
| `/login` | anonymous visitors. Always shows the username/password form and `New here? Set up your account`; registration stays open |

Unauthenticated visits to a protected page redirect to `/login` with the requested path in
router state; a successful sign-in returns there. `Sign out` clears the session and goes to
`/login`.

States:

- Initial status loading → neutral splash (no copy invented beyond the brand).
- `setupRequired` → setup form.
- Setup step 1: username, password, confirm password, `Create account`.
- Setup step 2: QR (`role="img"`, accessible name `Authenticator QR code`), setup key,
  ten recovery codes, checkbox `I have saved these recovery codes`, code input,
  `Continue`.
- Login step 1: username, password, `Sign in`, always followed by `New here?` and a
  `Set up your account` link. The link always opens the create-account form.
- `/setup` after setup: heading `Create account`; a new account may be added (D-34).
- Login step 2 heading `Two-factor authentication`, numeric `Verification code` with
  `autocomplete="one-time-code"`, `Continue`, button `Use a recovery code`.
- Recovery step: `Recovery code` input, `Continue`, `Back to sign in`.
- Authenticated shell: header shows username and a `Sign out` button.
- Session expiry: login screen with `role="alert"` copy `Your session expired. Sign in again.`

Accessibility: every input labelled; error region `role="alert"`; headings receive focus
after step changes; the QR's accessible name is exactly `Authenticator QR code` and the account
is named in the visible heading; recovery codes are a list with copy buttons and an
`aria-live` confirmation.

The QR is generated client-side from `otpauthUrl`. No secret or recovery code is written to
`localStorage`, `sessionStorage`, the URL or logs.

## 10. Tests

- Go: auth primitives, stores, service, API flows, protected middleware, cookie flags,
  rate limits, replay, recovery, CSRF, no-secret logging.
- Vitest: setup/login/TOTP/recovery forms, auth gating, 401 expiry, logout, header.
- Playwright: untouched regression suite runs authenticated through a fixture that performs
  first-run setup and TOTP with a deterministic clock; dedicated auth specs cover setup,
  login, 2FA, recovery, logout and guards.
- Every existing acceptance scenario must keep its expectations under the new session.

## 11. Multi-user extension (2026-11-18)

The product owner removed the single-owner limit. The rules below supersede the
single-owner statements earlier in this file and in the frozen plans.

- Any number of accounts may exist. Each account has its own username, password hash, TOTP
  secret, recovery codes, sessions and a **separate watchlist**.
- `POST /api/auth/setup` is always available, not only on first run. It registers a new
  pending account for a unique username and returns the same TOTP/recovery material.
- A duplicate username returns `409 USERNAME_TAKEN` with `That username is already taken.`
  The `SETUP_COMPLETE` code was removed from the domain and the HTTP mapping.
- `GET /api/auth/status` keeps `setupRequired`, now meaning "no accounts exist yet". The SPA
  does not use it to close registration.
- Login, TOTP, recovery, logout, cookies, rate limits and error copy are unchanged except
  that usernames resolve to their own account.
- Every watchlist, quote and chart call uses the authenticated account:
  - `GET/POST /api/watchlist` and `DELETE /api/watchlist/{ticker}` read and mutate only the
    caller's list.
  - `GET /api/quotes` prices only the caller's tickers.
  - A new account starts with an empty watchlist.
- Existing single-owner data migrates to the first account:
  - The current owner keeps the current watchlist.
  - If a migrated database has watchlist rows but no account, the first account created
    claims those rows.
- SQLite schema version 2:
  ```sql
  watchlist(
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    ticker TEXT NOT NULL,
    PRIMARY KEY (user_id, position),
    UNIQUE (user_id, ticker)
  )
  ```
  Rows migrated from version 1 are assigned to the single v1 owner, or left with
  `user_id IS NULL` so the first new account can claim them. Users and recovery codes keep
  their existing tables.
- Sessions and challenges carry the account ID internally; the public `user` object still
  contains only `username` and `totpEnabled`.
- Frontend routes:
  - `/setup` is the public **Create account** page whenever the visitor is anonymous.
    Authenticated visitors go to `/dashboard`.
  - `/login` always shows the sign-in form and `New here? Set up your account`.
  - The closed-sign-up notice is removed.
- Registration copy: heading `Create account`; server errors are shown verbatim.
- Privacy: no endpoint lists users; a session only sees its own username and watchlist.
