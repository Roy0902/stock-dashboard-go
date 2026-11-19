# Security Review

Date: 2026-11-18. Scope: `server/internal/{api,auth,config,store,app,providers,webui}`,
`server/cmd`, `Dockerfile`, `docker-compose.yml`, `scripts/`, `web/src/api` and the
auth UI, and the Playwright auth fixture. Read-only review: no source, test, config or
data file was modified; only this report was written.

Commands actually run: `go vet ./...` (clean), `go test ./... -count=1` (all packages
pass), `npm audit --omit=dev` (0 vulnerabilities), `go list -m -u all` (outdated modules
listed below). `govulncheck` is not installed on this host, so no Go CVE scan was run.
No live login or brute-force attempt was made.

## Verdict and risk level

**Overall risk: Medium, conditional on deployment.** The authentication and session core
is well built: Argon2id with production parameters, 256-bit hashed session tokens,
`HttpOnly`/`SameSite=Strict` cookies, persisted TOTP replay state, transactional single-use
recovery codes, generic login errors, a setup/activation mutex, and uniform session checks
on every data route. No critical or high-severity authentication bypass, SQL injection,
XSS or open redirect was found.

The material weaknesses are availability and at-rest handling rather than account
takeover: rate limits are check-then-act and can be exceeded by concurrent bursts;
unauthenticated Argon2 work is unbounded; "loopback-only" is enforced by a spoofable
`Host` header rather than the socket; TOTP seeds are stored in plaintext. Under the
strict documented deployment (loopback bind, published only on `127.0.0.1`, one trusted
local user) these are Low-to-Medium. If the container port is ever published on
`0.0.0.0`, several of them rise substantially in severity.

Counts: 0 Critical, 0 High, 4 Medium, 10 Low, 5 Info.

## Threat model assumptions

- Single-user (or small trusted multi-account) local tool; the operator is trusted.
- Server binds `127.0.0.1` by default (`server/internal/config/config.go:14`).
- The container sets `BIND_HOST=0.0.0.0` (inside the container only) and is published as
  `127.0.0.1:3000` (`Dockerfile:44-46`, `docker-compose.yml:10`). The host loopback is
  the trust boundary.
- No TLS in-process; HTTPS, if used, terminates at a local reverse proxy and
  `AUTH_COOKIE_SECURE=true` is expected.
- Browser threats are cross-site requests, DNS rebinding, clickjacking and XSS. A local
  process, another OS user, or anyone who can reach a mispublished port is *outside* the
  intended trust boundary but is still considered below.

## What is already good

- **Password hashing.** Argon2id, 16-byte salt, 32-byte key, 64 MiB, t=3, p=4, PHC output
  (`server/internal/auth/password.go:31-82`). Hash parsing has memory/time/thread caps so
  a tampered row cannot become a memory bomb (`password.go:110-147`).
- **Unknown-user timing.** A precomputed dummy hash is verified for unknown usernames
  (`server/internal/auth/service.go:164-191,362-367`). Password comparison is constant
  time (`password.go:86-93`).
- **TOTP.** 160-bit secret, SHA-1/6 digits/30 s, ±1 step, `crypto/subtle` comparison, and
  the matched step is persisted so a code cannot be replayed (`auth/totp.go:30-86`,
  `auth/service.go:415-441`). A persistence failure fails the login closed.
- **Recovery codes.** 10 codes of 10 random bytes, hashed, compared in constant time,
  consumed in the same transaction that updates the account (`auth/recovery.go:13-92`,
  `auth/service.go:451-499`, `store/users.go:136-178`). Malformed shapes do not burn an
  attempt (`service.go:457-462`).
- **Sessions.** 32 random bytes, only the SHA-256 digest is stored, new token on every
  login, 12 h absolute and 30 min sliding idle, server-side revocation on logout
  (`auth/tokens.go:12-33`, `auth/service.go:501-556`).
- **Cookies.** `HttpOnly`, `SameSite=Strict`, host-only (no `Domain`), explicit paths,
  `Secure` configurable, clearing uses `Max-Age=0` plus epoch `Expires`
  (`api/auth.go:272-297`). Auth state never reaches JS or browser storage.
- **First-run setup.** Pending in memory only, serialised by `setupMu` across
  `BeginSetup`/`Activate`, no account row before a valid TOTP code (`auth/service.go:153-155,
  212-343`).
- **CSRF/origin.** Unsafe `/api` methods with a present `Origin` must match the request
  host; `Origin: null` is rejected; JSON-only auth and watchlist mutations; no CORS
  headers; `SameSite=Strict` (`api/middleware.go:180-190,241-252`, `api/auth.go:306-323`,
  `api/handlers.go:143-164`).
- **DNS rebinding.** Non-loopback `Host` values are rejected before routing
  (`middleware.go:142-175`).
- **Authorization.** Every watchlist, quote and chart route is behind `requireSession`;
  handlers derive the user ID from the session only, so there is no IDOR surface
  (`middleware.go:195-237`, `api/handlers.go:19-123`). No endpoint lists users.
- **Input/output.** 4 KiB JSON bodies with `DisallowUnknownFields` and a single-value
  check; ticker grammar `^[A-Z][A-Z0-9]{0,9}$`; all SQL is parameterised; React escapes
  and there is no `dangerouslySetInnerHTML`, `innerHTML` or user-controlled URL; the
  post-login redirect accepts only a single-slash relative path
  (`api/handlers.go:140-164`, `domain/ticker.go:54-67`, `web/src/features/auth/LoginPage.tsx:25-31`).
- **Provider boundary.** 2 s timeout, 1 MiB response cap, keys never logged, failure
  messages reduced to stable codes (`providers/httpclient.go:17-70`, `api/errors.go:81-119`).
- **Headers.** CSP without `unsafe-inline`, `frame-ancestors 'none'`, `X-Frame-Options:
  DENY`, `nosniff`, `Referrer-Policy: no-referrer` (`middleware.go:48-85`).
- **Container.** Distroless nonroot, read-only root, `/data` volume, `/tmp` tmpfs, keys
  runtime-only, `.dockerignore` excludes `.env` and runtime state, healthcheck bundled
  (`Dockerfile`, `.dockerignore`, `docker-compose.yml`).
- **Tests and dependencies.** `go vet` and `go test ./...` pass; `npm audit --omit=dev`
  reports 0 production vulnerabilities.

## Findings

### [Medium] Concurrent bursts bypass the login, TOTP and enrollment rate limits

**Where:** `server/internal/auth/service.go:347-377` (password), `:403-419` (TOTP),
`:560-578` (challenge resolution), `:586-596` (attempt recording), `:275-314` (activation).

**Attack/failure scenario.** The rate limit and the attempt counter are read under
`s.mu`, then the expensive verification runs *after* the lock is released, and failures
are recorded even later. An attacker who can send JSON (a local process, or anyone if the
port is exposed) sends 100 parallel `POST /api/auth/login` requests for the same username:
all 100 pass `rateLimitLocked` at `:356-359` before any failure is recorded, and all 100
run Argon2 concurrently. The same check-then-act gap applies to TOTP: 100 parallel
`POST /api/auth/login/totp` calls all resolve the same challenge at `:570-576`, all see
`attempts == 0`, and all validate at `:415` before `failChallenge` increments at `:590`.
With the password known, this turns the 6-digit code and the 5-attempt cap into a
parallel guessing problem; without the password, it is an Argon2 memory amplifier.

**Impact.** The documented "5 failures per 15 minutes" and "5 code attempts per challenge"
caps are not effective against concurrency. It enables faster online password/TOTP
guessing and, more cheaply, memory exhaustion (64 MiB per in-flight verification).

**Concrete fix.** Serialise verification per username (a per-username mutex or a
single-flight gate) and reserve/increment the failure slot *before* running Argon2 or
`ValidateTOTP`; decrement only on success. For challenges, increment `attempts` before
validation, or hold the challenge lock across validate-and-consume. Add a global cap on
concurrent password verifications. Add a concurrency regression test that fires 20
parallel wrong passwords and asserts at most 5 Argon2 verifications.

**Effort:** M

### [Medium] Unauthenticated Argon2 work and pending enrollments are unbounded

**Where:** `server/internal/auth/service.go:186-191` (dummy hash), `:347-367` (login),
`:212-271` (setup), `:604-614` and `:657-679` (in-memory attempt map);
`server/internal/api/auth.go:44-71,103-128`.

**Attack/failure scenario.** `POST /api/auth/login` is public and, for any unknown
username, performs a full 64 MiB/t=3 Argon2id verification of the dummy hash
(`service.go:362-367`). There is no per-IP or global limit and no cap on in-flight
verifications, so a client can open hundreds of connections and force hundreds of
simultaneous 64 MiB hashes. Separately, `POST /api/auth/setup` is public with no rate
limit; each call computes Argon2id and inserts an enrollment entry retained for 15
minutes (`service.go:227-261`). `setupMu` serialises setup hashing, so the immediate CPU
cost is one at a time, but the queue is unbounded and every call adds an enrollment
holding a hash, a TOTP secret and ten recovery hashes. Login attempts with random
usernames also grow the `attempts` map (`:604-609`) until the 15-minute prune.

**Impact.** CPU/memory denial of service against the server and anything sharing the
host; a large pending-enrollment map; legitimate setup/activation delayed behind queued
work. Browser cross-site triggering is largely blocked by the JSON content-type and
Origin checks, so this requires a local process or an exposed port.

**Concrete fix.** Add a global/per-IP token bucket for `/api/auth/setup` and
`/api/auth/login`; cap concurrent Argon2 verifications with a semaphore; cap the number of
pending enrollments (and per-username entries). Return `429 RATE_LIMITED` before hashing.

**Effort:** M

### [Medium] "Loopback-only" is a Host-header check, not a network control

**Where:** `server/internal/api/middleware.go:142-175` (`hostGuard`, `loopbackHost`),
`:241-252` (`sameOrigin`); `Dockerfile:44-46` (`BIND_HOST=0.0.0.0`);
`docker-compose.yml:10` (loopback publish); `server/internal/config/config.go:127-138`.

**Attack/failure scenario.** `hostGuard` accepts any request whose `Host` header is
`127.0.0.1`, `localhost` or `::1`; it never inspects `RemoteAddr`. The image deliberately
listens on `0.0.0.0` inside the container. If an operator uses `-p 3000:3000` (or a
reverse proxy/Docker network forwards traffic), a remote client simply sends
`Host: 127.0.0.1:3000` and `Origin: http://127.0.0.1:3000` and passes both the host guard
and the CSRF origin check. The guard still does its stated job against DNS rebinding from
a browser (the browser cannot forge `Host`), but it is not an access-control boundary and
should not be described as "loopback-only enforcement".

**Impact.** On a mispublished port, anyone on the LAN can register accounts, consume the
shared Finnhub/Twelve Data budgets, create data and attempt logins. Existing accounts are
still protected by password + TOTP; there is no unauthenticated data read. Under the
documented compose file the risk is Low; the conditional misconfiguration is why this is
rated Medium.

**Concrete fix.** Keep the loopback publish as the primary control, and add a
`RemoteAddr` loopback check (with an explicit `TRUST_PROXY`/`TRUSTED_PROXY_CIDRS` escape
hatch for a local reverse proxy). Emit a startup warning when `BIND_HOST` is not loopback.
Update `docs/docker.md` to say the guard is defence-in-depth, not an ACL.

**Effort:** S

### [Medium] TOTP seeds are stored in plaintext; at-rest protection is a no-op on Windows

**Where:** `server/internal/store/store.go:24` (`dbFileMode 0600`), `:38`
(`totp_secret TEXT NOT NULL`), `:145-163` (`ensurePrivateFile`); `store/users.go:304-319`
(insert); `docs/verification.md:202-203` (Windows caveat).

**Attack/failure scenario.** The SQLite file contains each account's `totp_secret` in
clear text next to the Argon2 hash. Anyone who can read the database — a backup copied to
cloud storage, a volume snapshot, another local user on a POSIX host, or an OS account
with access on Windows where `chmod 0600` is advisory — obtains the second factor for
every account. The password hash still has to be cracked, but Argon2id at 64 MiB is
expensive, not impossible, and the TOTP seed makes a stolen password sufficient.

**Impact.** Loss of the second factor for all accounts from a single file read; defeats the
purpose of mandatory TOTP for at-rest compromise scenarios.

**Concrete fix.** Encrypt `totp_secret` (and, if desired, recovery hashes) with a key from
the environment or the OS keyring, using AES-GCM with a per-row nonce; or store the seed
in a separate keyring-backed store. At minimum, create `DATA_DIR` as `0700`, document disk
encryption for the volume/backups, and state explicitly in the README that Windows ACLs,
not `0600`, are the control there.

**Effort:** M (encryption) / S (directory mode + documentation)

### [Low] Migrated `users.json.migrated` keeps credential material with inherited permissions

**Where:** `server/internal/store/migrate.go:100,165,209-211` (`renameLegacy`),
`server/internal/store/store.go:79` (`MkdirAll(..., 0o755)`).

**Attack/failure scenario.** `renameLegacy` renames the legacy file after a successful
import and never deletes or tightens it. The file keeps whatever mode the previous Node
implementation created (typically `0644` under umask 022) and contains password hashes,
TOTP secrets and recovery hashes in JSON. On a POSIX host with multiple local users, or
after copying `DATA_DIR` to a share/backup, those secrets are readable. The database is
`0600`, but the `.migrated` file is not covered by `ensurePrivateFile`.

**Impact.** Credential and second-factor exposure from a file the operator may assume is
inert; stale secrets live indefinitely after migration.

**Concrete fix.** After a successful import, `chmod 0600` the file and either delete it or
move it to a clearly marked, operator-approved backup location. If retention is desired,
document it and include it in the backup/secret-handling guidance.

**Effort:** S

### [Low] Clearing the session cookie on every unauthenticated request enables cross-site logout

**Where:** `server/internal/api/middleware.go:195-210` (`requireSession` clears at `:208`);
`server/internal/api/auth.go:190-200` (`authMe` clears at `:196`), `:286-297`
(`clearAuthCookie`).

**Attack/failure scenario.** `requireSession` emits a `Set-Cookie: sd_session=...; Max-Age=0`
even when the request carried no `sd_session` cookie. A hostile page can include
`<img src="http://127.0.0.1:3000/api/watchlist">`; `SameSite=Strict` prevents the cookie
from being *sent*, but the response's clearing `Set-Cookie` is still processed for the
target host, deleting the victim's session cookie. Other third-party-cookie policies may
block this, so it is browser-dependent.

**Impact.** Forced sign-out / nuisance; no data disclosure and no privilege change. A
stolen session token is not revoked server-side, only removed from the browser.

**Concrete fix.** Only emit the clearing cookie when `sessionToken(r) != ""` (or when an
`sd_session` cookie was actually presented). Keep the 401 body unchanged. Add a test that
an unauthenticated request without a cookie does not set `Set-Cookie`.

**Effort:** S

### [Low] Registration confirms usernames and enables repeated account lockout

**Where:** `server/internal/auth/service.go:223-225` (`FindByUsername` → `UsernameTaken`),
`:604-643` (per-username failure window and block); `server/internal/api/auth.go:57-71`;
`docs/contracts-auth.md:265-268`.

**Attack/failure scenario.** `POST /api/auth/setup` returns `409 USERNAME_TAKEN` for an
existing account and `201` plus Argon2 work for a new one, so both the status code and the
response time enumerate usernames. The same knowledge lets an attacker submit five wrong
passwords for the owner's username every 15 minutes, forcing the documented 60-second
block (`service.go:611-613`) repeatedly. There is no per-IP component, so the block cannot
distinguish the attacker from the legitimate user.

**Impact.** Privacy leak (which usernames exist) and a cheap targeted availability attack
against a known account. Password/TOTP guessing itself remains bounded by the failure
window (see the concurrency finding).

**Concrete fix.** If enumeration must be avoided, make duplicate setup do comparable work
and return a generic success-shaped response (the account is still not created); or accept
this as a documented registration trade-off. Add a per-IP/global limiter to setup and
consider not letting unauthenticated failures lock an existing account (e.g. exponential
delay per source instead of a hard account block).

**Effort:** S (limiter) / M (uniform setup response)

### [Low] A password typed into the username field can be written to the auth log

**Where:** `server/internal/api/auth.go:118` (`loggableUsername(request.Username)`),
`:236-242` (`loggableUsername`), `:245-258` (`safeUsername`).

**Attack/failure scenario.** On a failed login, the submitted username is logged if it
matches the username grammar. A user who swaps the fields (password in "username",
password in "password") and whose password happens to be lowercase letters/digits/dots/
dashes/underscores, 3–32 characters, will have that password written to the JSON log as
`username`. The existing filter only drops values that are invalid *usernames*, not values
that are valid usernames but are actually secrets.

**Impact.** Credential material in `slog` output (`stdout`, `.dev/server.log`, container
logs), contrary to the project rule that passwords are never logged. Narrow trigger, but
real.

**Concrete fix.** Log the username only when it resolves to an existing account (or log a
fixed `"unknown"` marker for misses). Alternatively, log only a keyed hash of the
submitted username for abuse correlation. Keep `safeUsername` for control-character
stripping.

**Effort:** S

### [Low] The watchlist is unbounded and provider budgets are global

**Where:** `server/internal/domain/watchlist.go:5` ("There is no maximum size"),
`:19-31`; `server/internal/api/handlers.go:84-85`; `providers/service.go:94-159` and
`:253-262`; `providers/budget.go:11-14,65-88`.

**Attack/failure scenario.** Any account (registration is open) can add an unlimited number
of tickers. Each add performs provider verification, and `/api/quotes` then fans out over
the whole list with 8 workers and a 2 s per-call timeout; `/api/chart` has no cache and
consumes the shared 7/min Twelve Data budget. One account can exhaust the global Finnhub
(55/min) and Twelve Data (7/min) budgets, degrading every other account, and grow the
in-memory cache/`lastKnown` maps.

**Impact.** Authenticated denial of service against other accounts and against the
operator's provider quota; long requests and memory growth. No cross-account data access.

**Concrete fix.** Cap tickers per account (for example 60, matching the batch design), and
either give each account its own budget or add fair scheduling/priority so one account
cannot starve the others. Rate-limit `POST /api/watchlist` per account. Return a stable
error when the cap is reached.

**Effort:** M

### [Low] URL path and reflected ticker values are unbounded in logs and error bodies

**Where:** `server/internal/api/middleware.go:125-131` (logs `r.URL.Path`);
`server/internal/api/handlers.go:63-75` (`Remove(r.PathValue("ticker"))`);
`server/internal/domain/errors.go:135-137` (`NotInWatchlistMessage`);
`server/cmd/dashboard/main.go:25` (`MaxHeaderBytes = 1 MiB`).

**Attack/failure scenario.** The access log writes the full request path with no
truncation, and `DELETE /api/watchlist/<arbitrary>` echoes the path value into the
`NOT_IN_WATCHLIST` error. A client can send a path close to the 1 MiB header limit, causing
~1 MiB log lines and a ~1 MiB JSON response per request. The content is JSON-encoded and
React escapes it, so this is not XSS; it is log/response amplification.

**Impact.** Log flooding, disk growth, noisy incident response, and oversized responses.

**Concrete fix.** Truncate `path` in the access log to a fixed length (for example 200
bytes) and wrap ticker values in the error message with the same 10-character grammar
check used on input, or return a fixed message for malformed removals. Consider lowering
`MaxHeaderBytes` to 64 KiB.

**Effort:** S

### [Low] Cookie hardening: no `__Host-` prefix and `Secure` is off by default

**Where:** `server/internal/api/auth.go:272-283`; `server/internal/config/config.go:78-89`
(blank → `false`); `docker-compose.yml:15` (`AUTH_COOKIE_SECURE: "false"`).

**Attack/failure scenario.** The cookies are host-only and `HttpOnly`, which is good, but
they are not `__Host-` prefixed, and `Secure` is false unless the operator opts in. If the
app is placed behind an HTTPS reverse proxy and `AUTH_COOKIE_SECURE` is left at the compose
value, the session cookie is transmitted without the `Secure` attribute; on a host with a
misconfigured HTTP listener or a proxy hop over cleartext, it can be sent over HTTP. The
`__Host-` prefix would also harden against a future subdomain/cookie-injection mistake.

**Impact.** Session confidentiality depends on correct deployment configuration; no
in-process defect.

**Concrete fix.** Rename the session cookie to `__Host-sd_session` (the prefix requires
`Path=/` and no `Domain`, which the session cookie already satisfies) and fail startup if
`AUTH_COOKIE_SECURE` is false while TLS mode is configured. The challenge and enrollment
cookies deliberately use `Path=/api/auth`, so they cannot take the `__Host-` prefix
without widening their path; keep them host-only and document that. At minimum, remove the
explicit `false` from compose and document the requirement next to the port mapping.

**Effort:** S

### [Low] Origin validation is optional and scheme-agnostic

**Where:** `server/internal/api/middleware.go:180-190` (only when `Origin` is present),
`:241-252` (`sameOrigin` accepts both `http://` and `https://`).

**Attack/failure scenario.** A request without `Origin` is accepted, by design, for CLI
and tests. A browser client that omits `Origin` on an unsafe request would bypass the
check (modern browsers send it for POST, but the code cannot rely on that). Because
`sameOrigin` accepts either scheme against the same `Host`, a page served from
`http://localhost` can satisfy the check for an app actually served at
`https://localhost` if a local HTTP listener exists on the same host/port tuple. There is
no synchronizer CSRF token.

**Impact.** Defence-in-depth gap rather than a demonstrated exploit; `SameSite=Strict`
remains the primary control. Relevant if a local HTTP service or a permissive proxy is
present.

**Concrete fix.** Require an exact expected origin (configured `PUBLIC_ORIGIN`, or derive
scheme from `AUTH_COOKIE_SECURE`) and reject unsafe requests with no `Origin` when a
session cookie is present, or add a double-submit CSRF token for cookie-authenticated
mutations.

**Effort:** M

### [Low] Auth reads fail open to stale in-memory snapshots on database errors

**Where:** `server/internal/store/users.go:37-47` (`FindByID`), `:50-65`
(`FindByUsername`), `:69-79` (`Count`).

**Attack/failure scenario.** If a SQLite read errors (I/O error, lock timeout, corrupt
page), the store returns the last successful snapshot instead of failing. `Authenticate`
then continues to accept a session whose account row may have been deleted or changed
externally, and `Count` keeps `setupRequired` at the last value. Normal "row missing" is
handled correctly (`sql.ErrNoRows` → nil), so this only affects query errors; it is the
opposite of the "fail closed" posture used for TOTP persistence.

**Impact.** During a storage fault, revoked accounts can remain usable and authorization
decisions use stale data. Availability of the app is prioritized over correctness.

**Concrete fix.** For authentication decisions, return an error (and a 500/401 fail-closed)
on storage errors rather than the snapshot. Keep snapshots for display-only watchlist data
if desired. Log the storage error with the request ID.

**Effort:** S

### [Low] Sessions cannot be revoked globally; re-login does not invalidate older tokens

**Where:** `server/internal/auth/service.go:526-533` (`Logout` deletes one digest),
`:535-556` (`createSession` adds a new entry without removing the account's older
sessions), `:657-662` (prune on idle/absolute expiry).

**Attack/failure scenario.** Every successful login adds a session; logging out revokes
only the token presented. A stolen session token stays valid until the 12 h absolute cap
(or 30 min idle, which the SPA's polling keeps alive while open), and the operator has no
"sign out everywhere". A password/TOTP change path does not exist, so there is no
privilege-change rotation either.

**Impact.** Prolonged use of a stolen token; no way to respond to suspected theft short of
restarting the process.

**Concrete fix.** Keep a per-account session generation/version and delete all entries for
the account on logout-all or credential change. Optionally rotate/invalidate older
sessions on a new login, or expose a "sign out all devices" action. Add tests for the
generation check.

**Effort:** M

### [Info] Missing HSTS, Permissions-Policy and cross-origin isolation headers

**Where:** `server/internal/api/middleware.go:76-85`.

**Scenario/impact.** CSP, `X-Frame-Options`, `nosniff` and `Referrer-Policy` are present
and strong. There is no `Strict-Transport-Security` (appropriate for plain HTTP loopback),
no `Permissions-Policy`, and no `Cross-Origin-Opener-Policy`/`Cross-Origin-Resource-Policy`.
On an HTTPS deployment, a missing HSTS header allows a first-visit downgrade; CORP would
reduce cross-origin embedding of static assets.

**Concrete fix.** Add `Permissions-Policy` (camera/microphone/geolocation disabled) now;
add HSTS when `AUTH_COOKIE_SECURE=true` (or behind a TLS proxy), and consider
`Cross-Origin-Resource-Policy: same-origin`.

**Effort:** S

### [Info] Open registration means authentication gates data, not access

**Where:** `server/internal/auth/service.go:209-271`; `docs/contracts-auth.md:258-303`
(D-34).

**Scenario/impact.** `POST /api/auth/setup` is always available, so anyone who can reach
the app can create an account, use the operator's provider keys, and store a watchlist.
Account isolation is correct (no endpoint lists users; each watchlist is keyed by the
session user), so this is a product/quota decision rather than a data breach. It becomes
more relevant if the port is exposed.

**Concrete fix.** If this is not intended beyond the local operator, add an invite token
or an allowlist controlled by an environment variable, or keep the loopback publish as a
hard requirement and state it in the README next to the registration description.

**Effort:** M

### [Info] Provider base URLs may use cleartext HTTP

**Where:** `server/internal/config/config.go:102-125` (accepts `http` and `https`,
defaults are HTTPS).

**Scenario/impact.** `FINNHUB_BASE_URL` and `TWELVEDATA_BASE_URL` are overridable. If set
to an `http://` URL (other than the documented test overrides), the API key in the query
string is sent in cleartext. The defaults and tests are HTTPS.

**Concrete fix.** Require `https` for non-loopback hosts, or gate `http` behind an
explicit `ALLOW_INSECURE_PROVIDER_URLS` development flag.

**Effort:** S

### [Info] Dependency and base-image freshness

**Where:** `server/go.mod:5-9`; `Dockerfile:3-4,7,18,33`.

**Scenario/impact.** `golang.org/x/crypto` v0.45.0 (latest v0.59.0) and
`modernc.org/sqlite` v1.44.3 (latest v1.62.0) are well behind; the SQLite gap includes
upstream fixes. The Dockerfile uses floating tags (`node:24-alpine`, `golang:1.27-alpine`,
`gcr.io/distroless/static-debian12:nonroot`), so builds are not reproducible and can pick
up an unexpected base image. `npm audit --omit=dev` is clean; the documented dev-only
Vite/picomatch advisory remains.

**Concrete fix.** Pin base images by digest (or at least minor version), bump
`x/crypto` and `modernc.org/sqlite`, and run `govulncheck ./...` in CI (it is not installed
here). Re-run `npm audit` after dependency updates.

**Effort:** S

### [Info] In-memory sessions and no persistent audit trail

**Where:** `server/internal/auth/service.go:146-162,657-679`; `docs/decisions.md:24`
(D-20).

**Scenario/impact.** Restarting the process invalidates all sessions (good for revocation,
bad for availability). Auth events are emitted to `slog` with `event`, `username` and
`request_id` but are not persisted, so there is no durable record of failed logins or
recovery-code use. This is acceptable for a local tool; it limits after-the-fact
investigation.

**Concrete fix.** If the app is ever exposed beyond loopback, add an append-only auth
event table (event, timestamp, outcome, request ID; no secrets) and a bounded retention
policy.

**Effort:** M

## Dependency and image notes

- Go: `go vet` clean, all tests pass. Module graph is healthy but behind: `x/crypto`
  v0.45.0 → v0.59.0, `modernc.org/sqlite` v1.44.3 → v1.62.0, `x/sys` v0.38.0 → v0.50.0,
  `x/net` v0.47.0 → v0.63.0 (transitive). `github.com/pquerna/otp` v1.5.0 is current.
  `govulncheck` is absent, so this is not a CVE clearance.
- npm: production dependency tree reports 0 vulnerabilities (`@tanstack/react-query`,
  `qrcode.react`, `react`, `react-dom`, `react-router-dom`). Dev-only advisories are
  documented in `docs/verification.md:206-208`.
- Image: multi-stage, `CGO_ENABLED=0`, `-trimpath`, stripped; distroless
  `static-debian12:nonroot`; UID/GID 65532; read-only root; `/data` volume; `/tmp` tmpfs.
  No keys in build args; `.dockerignore` excludes `.env`, `data/`, `.dev/` and logs.
- Image gaps: floating base tags, no digest pinning, no SBOM, no `cap_drop: ALL` /
  `no-new-privileges` in compose. None grant code execution by themselves; they reduce
  reproducibility and defense in depth.
- TLS is not terminated in-process. The cookie `Secure` flag is the only transport
  control the app owns; see the cookie finding.

## Verification steps a human should run

1. **Baseline gates (already run here).**
   `cd server; go vet ./...; go test ./... -count=1`.
   `cd web; npm audit --omit=dev` (expect 0).
2. **CVE scan.** Install `govulncheck` and run `govulncheck ./...` in `server/`; review
   `go list -m -u all` and bump `x/crypto` and `modernc.org/sqlite`.
3. **Rate-limit concurrency.** Against a throwaway `DATA_DIR`, send 20 parallel
   `POST /api/auth/login` requests with the same username and a wrong password. Count
   `401` vs `429`; the current code allows the whole burst to hash. Repeat for
   `/api/auth/login/totp` with one valid challenge and wrong codes, and inspect
   `challenge.attempts` behavior through the response codes.
4. **Setup DoS/queue.** Send repeated `POST /api/auth/setup` requests with distinct valid
   usernames while watching process RSS; confirm pending enrollments expire after 15
   minutes and that legitimate activation is not blocked behind the queue.
5. **Cross-site logout.** While signed in, open a page on another origin containing
   `<img src="http://127.0.0.1:3000/api/watchlist">`, then check whether `sd_session` was
   cleared in the browser. Confirm the fix suppresses `Set-Cookie` when no session cookie
   was sent.
6. **CSRF matrix.** Cross-origin form POST to `/api/auth/login` and `/api/watchlist`
   → `403 CROSS_ORIGIN_REQUEST`; `Origin: null` → 403; `Content-Type: text/plain` → 400;
   no `Origin` with JSON → allowed (documented). Verify no `Access-Control-Allow-Origin`
   appears on any response.
7. **Cookie flags.** With `AUTH_COOKIE_SECURE=true`, sign in over HTTPS and confirm
   `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`, no `Domain`. Check the challenge and
   enrollment cookies use `/api/auth` and expire in 5/15 minutes. Confirm logout clears
   all three (currently only `sd_session` is cleared).
8. **TOTP replay and window.** Use a code twice → second response `401 INVALID_CODE`;
   restart the server and replay the same code → still rejected (persisted
   `last_totp_step`). Use a code from the previous 30 s step after a login → rejected.
9. **At-rest secrets.** On a POSIX host, `stat -c '%a %n' data/*` after migration: the DB
   should be `600`, `DATA_DIR` ideally `700`, and `users.json.migrated` should not be
   world-readable. On Windows, inspect ACLs (`icacls`) and confirm only the operator
   account can read the data directory.
10. **Container exposure.** `docker compose config`; confirm `127.0.0.1:3000:3000`. Then
    deliberately run `-p 3000:3000` on a disposable host and try a request with
    `Host: 127.0.0.1:3000` from another machine; it should be rejected by a network
    control, not just the header check.
11. **Log hygiene.** Run a full setup/login/recovery/logout flow, then grep
    `.dev/server.log` for the password, TOTP secret, recovery codes, session tokens and
    hashes. Repeat with a password typed into the username field to confirm the logging
    edge case.
12. **Redirect/XSS spot checks.** Visit `/login` with router state `from=//evil.example`
    and `from=https://evil.example`; both must land on `/dashboard`. Add a ticker named
    `<img src=x onerror=alert(1)>` via the API; it must be rejected by the ticker grammar
    and never rendered as HTML.

## Top 5 fixes

1. **Make rate limiting atomic.** Serialise verification per username (or reserve the
   attempt before hashing) so the 5/15-minute password cap and 5-attempt challenge cap
   hold under concurrency. Add a 20-parallel regression test.
2. **Bound unauthenticated Argon2 work.** Add per-IP/global limiters and a verification
   semaphore for `/api/auth/login` and `/api/auth/setup`, and cap pending enrollments.
3. **Stop logging secrets through the username field.** Log the submitted username only
   when it resolves to an existing account; otherwise log `"unknown"`.
4. **Tighten cookie lifecycle.** Emit the clearing `Set-Cookie` only when a session cookie
   was presented, clear `sd_challenge`/`sd_enroll` on logout, and use `__Host-sd_session`
   plus `Secure` whenever TLS is in play.
5. **Harden secrets at rest and the image supply chain.** `0700` the data directory,
   `chmod 0600`/remove `.migrated` files, encrypt TOTP seeds (or document disk
   encryption), pin base images by digest and bump `x/crypto` and `modernc.org/sqlite`.
