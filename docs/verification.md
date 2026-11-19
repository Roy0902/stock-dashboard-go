# Verification record

Date: 2026-11-18. Host: Windows, Go 1.27.4, Node 24.22.0, npm 11.19.1.

Behaviour is verified against `docs/contracts.md`, which restates
`../spec/implementation-spec.md` v1.6 and the frozen plan under
`../spec/frozen-plan/`.

## Automated gates

| Gate | Command | Result |
| :-- | :-- | :-- |
| Go formatting | `gofmt -l .` | clean (no output) |
| Go static analysis | `go vet ./...` | clean |
| Go tests | `go test ./... -count=1` | all packages pass |
| Web lint | `npm run lint` | 0 errors, 0 warnings |
| Web types | `npm run typecheck` | 0 errors |
| Web formatting | `npm run format:check` | all files formatted |
| Web unit tests | `npm run test` | 20 files, 150 tests pass |
| Web build | `npm run build` | 140 modules, 111.1 kB gzip |
| Acceptance | `npm test` (in `e2e/`) | 33 tests pass, 1 worker |
| E2E types | `npm run typecheck` (in `e2e/`) | 0 errors |

`scripts/verify.ps1` runs every gate above except the Playwright suite; add `-WithE2E` to
include it. There is no root `package.json`, so the command is the PowerShell script itself:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1
```

Exact commands and expected output per gate: `docs/quality-gates.md`.

The race detector is not available on this host: `CGO_ENABLED=0` and no C compiler.
`go test -race` must be run on a toolchain with cgo before release to a multi-writer
environment.

## Final review dispositions (2026-11-18)

| Action from `docs/reviews/final-review.md` | Disposition |
| :-- | :-- |
| 1. Reconcile the colour contract | Fixed: `contracts.md` names the semantic tokens and both theme values with AA ratios; `verification.md` updated. |
| 2. Publish the nine new UI strings | Published in `contracts.md` §6 under "UI labels and hints". Product owner may revise; no server copy changed. |
| 3. Fix `contracts-auth.md` §9 and delete `SETUP_COMPLETE` | Fixed: closed-notice copy removed, routes/states match D-34, code path deleted from domain and HTTP mapping, contract row removed. |
| 4. Close the two auth nits and strengthen two tests | Fixed: busy `verifySlots` releases the reserved failure and global slot; lock-order comment corrected; route classification and real concurrent-setup tests strengthened. |
| 5. Refresh measured state and harden `set-module.mjs` | Fixed: coverage table above, quality-gates baseline, review index, script resolves its own root and skips reports. |

Residual items (deferred with rationale): TOTP seeds remain plaintext in SQLite and `-race`
cannot run on this host (`CGO_ENABLED=0`). Both are recorded in the final review and above.

## Coverage highlights

Go statement coverage from `go test -cover`:

Measured 2026-11-18 after the hardening pass (`go test ./... -cover -count=1`):

| Package | Coverage |
| :-- | --: |
| `internal/config` | 94.0% |
| `internal/api` | 92.8% |
| `internal/domain` | 89.1% |
| `internal/auth` | 87.8% |
| `internal/providers` | 86.9% |
| `internal/store` | 79.7% |
| `internal/webui` | 76.0% |
| `internal/app` | 73.2% |
| `cmd/dashboard` | 0.0% |
| `cmd/healthcheck` | 0.0% |

Four packages lost coverage while new code landed (`app`, `webui`, `config`, `providers`);
the new paths are tested but the denominator grew. `cmd/` remains untested and is tracked in the
final review.

Acceptance scenarios exercised end to end:

- Auth: first-run setup with QR and recovery codes, uppercase-username refusal, generic
  credential errors, mandatory TOTP, wrong TOTP, recovery single use, logout, session expiry
  copy, rate limiting, protected pages/APIs, and authenticated regression of every original
  dashboard/chart scenario.
- Watchlist: empty state, canonical add, invalid, duplicate, unknown symbol, list order,
  removal and `NOT_IN_WATCHLIST`, persistence in `stock-dashboard.db`.
- Multi-user: two accounts register with their own TOTP, coexist in separate sessions, and
  cannot see each other's watchlists; duplicate usernames return `USERNAME_TAKEN`.
- Quotes: full row shape, sign and direction, Twelve Data fallback, both providers down,
  zero change, `502` provider copy, missing-key message, `503` endpoints.
- Chart: route from a row, 1M default, 3M switch, accessible name, source label, unavailable
  state, deep link for an unlisted ticker, invalid range, ordering.
- Platform: `/healthz` without provider traffic, unknown API envelope, SPA deep link,
  loopback-only bind, no upstream detail in the page.

Unit tests additionally cover PBKDF2-HMAC-SHA256 and legacy Argon2id round-trips and malformed
hashes, legacy-hash upgrade on login, username/password
bounds, TOTP ±1 step and replay across restarts, challenge and enrolment expiry/attempt
caps, rate-limit reset, session idle/sliding/absolute expiry, cookie flags, Origin checks,
transactional SQLite writes and legacy JSON import, recovery normalisation, rounding
boundaries, staleness, cache hits, sliding-window budgets, malformed provider bodies and
config validation.

## Manual checks

Screenshots were reviewed at 1440×1050 (dark and light) and 390×900 (dark):

- no horizontal scroll at 390 px;
- numeric columns stay aligned with tabular figures;
- focus rings and 44 px targets are present;
- both themes keep the status, provider and timestamp readable;
- the chart renders currency ticks, first/last dates and an area gradient in both themes;
- auth pages were compared with public sign-in pages from Vercel, Linear, GitHub, Stripe,
  Google and Notion (`docs/auth-ui.md`); the applied hierarchy is brand → heading → form →
  action → account prompt → quiet footer, without a marketing header;
- login fields, the first-run setup prompt, and the 2FA step were checked in dark and light
  at desktop and 390 px widths.

## Copy and path portability

- Every user-visible string is read from `web/src/locale/en.ts` through `copy`; there are no
  per-feature copy modules or JSX string literals. Document title, description and `lang`
  are applied from the same locale at boot.
- All HTTP endpoints are built in `web/src/api/endpoints.ts` from `import.meta.env.BASE_URL`;
  the router basename uses the same value, so a sub-path deployment is configuration-only.
- `index.html` resolves the favicon and entry script through `%BASE_URL%`; the HTML shell
  contains no product copy.
- `web/src/api/endpoints.test.ts` pins the root defaults, rebasing and encoding.
- The landing footnote now takes the runtime host instead of `127.0.0.1`, and the feature
  list no longer claims a single owner.

## Code review follow-up

An independent review (2026-11-18) focused on code smells and contract drift. Dispositions:

| Finding | Disposition |
| :-- | :-- |
| No remove affordance (R1) | Kept API-only, matching the approved D-8 decision; documented as D-13. |
| 15-minute last-known quote (R2) | Intentional per spec §7; documented as D-14 and covered by service tests. |
| Key gate before empty watchlist (R3) | Kept; heuristic clarified in contracts §3 and pinned by a test. |
| Chart error codes/copy (R4) | Documented `INVALID_TICKER` as D-16; chart keeps server copy. |
| No single-flight (R5) | Fixed with striped per-ticker locks and a concurrency regression test. |
| Duplicated key policy (R6) | Kept: the API gate gives one honest message; adapters stay defensive. |
| Cancellation ignored (R7) | Fixed: dispatch, observe and budget reserve honour `ctx`. |
| `lastKnown` growth (R8) | Fixed: `DELETE` calls `Service.Forget`; test covers cache + last-known. |
| Missing `Retry-After` (R9) | Fixed: 429 now carries the header; test asserts it. |
| Cross-site POST surface (R10) | Fixed: non-JSON content types are refused before decoding. |
| Hidden column labels (R11) | Fixed: header is exposed to AT, visually hidden below 900 px. |
| 2 s timeout untested (R12) | Fixed: slow-provider test asserts a ~2 s unavailable failure. |
| 55/7 budgets unverified (R13) | Fixed: app-level tests pin both budgets end to end. |
| Weak concurrency test (R14) | Fixed: asserts parallelism, not only the upper bound. |
| Token/colour duplication (R15) | Component colours are literal and documented; direction dots use tokens. |
| `unsafe-inline` CSP (R16) | Removed; header test asserts its absence. |
| Invented shell copy (R17) | Documented as engineering copy in contracts §6. |
| Fail-closed store (R18) | Kept intentionally: never silently discard user data; README explains recovery. |
| Dead variant/export (R19) | Removed unused button variant, utility class and export. |
| Duplicated e2e helpers (R20) | Consolidated into `fixtures/app-server.ts`; serial mode documented. |
| Chart geometry (R21) | Deferred; component tests cover 30-point and single-point series. |
| Empty-path copy (R22) | Deferred; the route cannot match an empty segment. |

## Second review follow-up (2026-11-18)

A three-agent review (one code-smell pass, two method/edge-case passes) produced these
dispositions. All fixes below are covered by the Go or Vitest suites and by the full gates.

| Finding | Disposition |
| :-- | :-- |
| Normative docs still described a single owner (D-18/D-23/D-30/D-32) | Fixed: contract sections, decision consequences and verification prose now describe multi-account behavior; stale rows point to D-33/D-34. |
| D-19 told operators to reset `users.json` | Fixed: recovery loss now points at deleting the account row in SQLite, with cascading recovery/watchlist rows. |
| Verification recorded Argon2 as 256 MiB/t=6/p=8 | Fixed: records the shipped 64 MiB/t=3/p=4 and references `password.go` (later superseded by D-42). |
| Quotes mapped upstream 401/403 to 502 | Fixed: wholly credential-failed batches return `503 MISSING_API_KEY`; partial success stays `200`. |
| Logout always emitted a clearing `sd_session` | Fixed: clears it only when presented; always clears `sd_challenge`/`sd_enroll`. |
| `/api/auth/me` authenticated twice and owned a dead 401 | Fixed: `requireSession` injects the account (including TOTP state); the handler only serializes it. |
| Chart dates were accepted and sorted as arbitrary strings | Fixed: `YYYY-MM-DD` is parsed and calendar-validated; malformed provider dates become `FailureUnavailable`. |
| Login loaded recovery hashes it never reads | Fixed: username lookup uses the non-recovery projection; recovery flows use the full projection. |
| Provider attribution used two index-linked slices | Fixed: one ordered source/name pair list drives verification and observation. |
| Recovery input normalization was validated but not submitted | Fixed: one `normalizeRecoveryCode` is used for validation and the request. |
| A late TOTP response could set an error on the recovery step | Fixed: step and request tokens discard stale responses. |
| A repeated password hint was announced twice | Fixed: fields omit a hint that duplicates the current error, visually and in `aria-describedby`. |
| First mount focused the auth heading instead of the skip link | Fixed: heading focus starts at the first step/material transition. |
| First-run `/chart/...` deep links were dropped | Fixed: the login guard passes state to `/setup`; activation returns to the requested path. |
| Only one added ticker stayed pending | Fixed: pending adds are tracked per ticker and pruned by a newer quote batch or an error. |
| `formatChartDate` rolled invalid calendar dates over | Fixed: UTC year/month/day must round-trip; otherwise the raw value is kept. |
| A permanent `503 MISSING_API_KEY` was retried | Fixed: that code is never retried; other 5xx remain retryable. |
| Bootstrap only treated two codes as unavailable | Fixed: status 0 and every 5xx are unavailable; 4xx stays answered. |
| `resolvePostAuthPath` accepted `/\` | Fixed: any backslash is rejected in addition to `//`. |
| Cross-tab cache clearing was only asserted indirectly | Fixed: a test proves the old watchlist/quotes are cleared and refetched. |
| Landing copy hardcoded the 60 s cache TTL | Fixed: copy no longer states a fixed TTL. |
| Dead auth CSS, unused auth barrel and repeated brand SVG | Fixed: `.auth-visually-hidden`/`.auth-account` removed, the barrel deleted, and one `BrandMark` component used by the header, auth panel and splash. |
| jsdom `scrollTo` noise in tests | Fixed: shared setup stubs it. |

Deferred as behavior-neutral refactors, not defects: extracting the shared Login/Setup form
scaffolding, deduplicating the TOTP/recovery handlers and the two legacy-import skeletons,
collapsing the auth test stubs, and consolidating the duplicated loopback/config defaults.
Each is covered by current tests; changing them now would add regression risk without
changing observable behavior.

## Password hashing change (2026-11-19)

D-42 replaces Argon2id for **new** passwords with PBKDF2-HMAC-SHA256, 600,000 iterations,
16-byte salt and 32-byte key, encoded as `$pbkdf2-sha256$i=600000$...`.

- Legacy Argon2id records still verify. After a successful password login the record is
  rehashed to PBKDF2; a failed upgrade is ignored and retried on the next login.
- Unknown usernames run a dummy PBKDF2 verify, so timing still does not reveal existence.
- The global per-minute budget and the two-slot verification semaphore remain; PBKDF2 uses
  no per-request 64 MiB work area.
- Unit tests cover PBKDF2 round-trips, fresh salts, malformed hashes, legacy Argon2
  verification and the login-time upgrade, including the following TOTP step.
- Existing accounts need no reset: the first successful login upgrades them. Until then the
  stored Argon2id hash is verified by the retained legacy path.

## Authentication extension (user-auth)

- Frozen plan: `../spec/user-auth/user-auth.md` (`88e2ff2c…`), 25 scenarios.
- Original `stock-watchlist` plan and manifest verified unchanged.
- Runtime: first visit shows `/setup`; the first account is persisted only after the first
  TOTP code. Sessions are in memory (12 h absolute, 30 min idle).
  `stock-dashboard.db` is the only persisted file after migration.
- Routing: anonymous visitors always see the public landing page at `/` with
  a `Sign in` action; `/dashboard` and `/chart/*` redirect to `/login` and return to the
  requested page after sign-in. Authenticated visitors at `/`, `/login` or `/setup` land on
  `/dashboard`. The sign-in page always ends with `New here? Set up your account`; `/setup`
  always opens the Create account form, so additional accounts can be registered (D-34).
- Coverage: see the table above (`internal/auth` 87.8%, `internal/api` 92.8%), 38 auth
  unit tests and 10 Playwright
  scenarios.
- Manual smoke: setup QR/recovery screen, TOTP login, wrong-password copy, recovery-code
  single use, logout, session-expiry copy, and authenticated dashboard/chart regression were
  exercised through the isolated acceptance apps.
- SQLite migration smoke: a copy of the live `users.json` + `watchlist.json` was opened by a
  temporary server with the new binary; it produced `stock-dashboard.db`, renamed both
  legacy files `.migrated`, reported `setupRequired: false`, kept `AAPL` in the database and
  returned `401` for the watchlist without a session. The live directory was not touched
  during that check.
- Live migration (2026-11-18 15:16): the running server was restarted against the real data
  directory. It created `stock-dashboard.db` (32 KiB) and renamed both JSON files
  `.migrated`. `/api/auth/status` reports `setupRequired: false`, `/api/watchlist` returns
  `401`, the owner record loads through SQLite validation, and a fresh backup is kept under
  `.dev/data-backup-20261118-151557`.

### Auth review follow-up

A second independent review focused on the authentication slice. Dispositions:

| Finding | Disposition |
| :-- | :-- |
| Setup/activation race (F-01) | Fixed with a setup mutex spanning `BeginSetup`/`Activate`, plus a blocking-store concurrency test. |
| DNS-rebinding via unchecked `Host` (F-02) | Fixed with a loopback-only host guard before the router; tests cover evil and loopback hosts. |
| Rate window reset after the 60 s block (F-03) | Fixed: failure timestamps survive the block, with a window regression test. |
| Password max conflict (F-04) | Superseded by the product decision: only a 12-character minimum, no user-facing maximum (D-31). |
| Windows `0600` (F-05) | Documented as advisory on Windows; POSIX chmod remains in the write path. |
| `lastTotpStep` example unit (F-06) | Corrected to a real TOTP step (`58783333`). |
| Wrong copy on malformed code bodies (F-07) | Fixed: each endpoint returns its own code (`INVALID_CODE`/`INVALID_RECOVERY_CODE`). |
| Over-long password copy (F-08) | Left as `WEAK_PASSWORD`; there is no UI maximum (D-31 removed it) and the server accepts any password within the 4 KiB body cap. |
| Default allow-all auth in API tests (F-09) | Deny-without-session coverage remains explicit for all five protected routes. |
| Malformed recovery code burns attempts (F-10) | Fixed: client shape validation mirrors the server; malformed shapes no longer consume attempts. |
| Argon2 caps too generous (F-11) | Record corrected: hashing used 64 MiB, t=3, p=4 at that time (`server/internal/auth/password.go`); superseded by D-42, which moves new hashes to PBKDF2. |
| Raw invalid username in logs (F-12) | Fixed: only valid usernames reach auth log events. |
| Cookie expiry clock / deep-link return / `getMe` / lock scope / singular copy (F-13–F-19) | Deferred as nits; documented here and in decisions where behaviour matters. |
| Vite dev proxy origin (review test suggestion) | Fixed by setting `changeOrigin: false`; POST login through `:5173` now returns 401, not 403. |

### Final review residual follow-up

A third independent review (final-review residual list) closed the security/quality leftovers
that did not need a product decision. Dispositions:

| Finding | Disposition |
| :-- | :-- |
| Cross-site logout (residual 1) | Fixed: `sd_session` is expired only when the request presented it; tests cover absent, empty and unknown cookies. |
| `.migrated` file permissions (residual 2) | Fixed: a successful migration renames, then sets `0600`; a chmod failure logs a bounded warning. The file is retained, never deleted silently. |
| `AUTH_COOKIE_SECURE` parser mismatch (residual 7) | Fixed: only `true`/`false` (case-insensitive) are accepted, matching the error message. `1`/`0`/`t`/`f` now fail startup. |
| Provider aggregate attribution (residual 9) | Fixed: aggregate failures keep the provider that actually failed, so a Twelve-Data-only outage is not logged as Finnhub. |
| `DefaultMessage(RATE_LIMITED)` grammar (residual 10) | Fixed: the one-second window uses `1 second.`; `NewRateLimited` keeps the same numeric copy with correct pluralisation. |
| Weak route/concurrency tests (Top-5 #4) | Fixed: the route test fails on any unclassified `/api` pattern; concurrent setup uses a blocking random reader as a barrier instead of a sleep. |
| `Update` account-not-found (residual 8) | Fixed: `store.Users.Update` returns the sentinel `auth.ErrUserNotFound`; TOTP/recovery paths answer `401 CHALLENGE_EXPIRED` instead of `500`. |
| TOTP seeds encrypted at rest (residual 3) | Deferred: needs a key-management product decision (key source, rotation, backup). Plaintext seeds remain documented in the security review. |
| `go test -race` (residual 13) | Deferred: this host builds with `CGO_ENABLED=0`; run it on a cgo toolchain before release. No code change can substitute. |

## Container image

- Built `stock-dashboard:local` with the multi-stage Dockerfile (Node client, static Go
  binaries, Alpine 3.23 runtime with a CA bundle and a dedicated non-root user).
- Image: 35.03 MB, user `65532:65532`, read-only root, `/data` volume, `/tmp` tmpfs,
  bundled `/app/healthcheck`. The runtime includes BusyBox and `apk` for debugging; no
  source, tests, `node_modules`, tzdata or docs are added.
- Container smoke on `127.0.0.1:3200`: `/healthz` and `/` 200, unauthenticated watchlist
  401, health `healthy`, healthcheck exit 0.
- Auth smoke: account setup + TOTP activation, duplicate username `409 USERNAME_TAKEN`,
  second account with its own session, logout/login, and persistence across a container
  restart with the same volume. `/data/stock-dashboard.db` was 45,056 bytes after two
  accounts.
- `docker compose config --quiet` passes.
- Full details: `docs/docker.md`.

## Known limitations and decisions

- The watchlist has no remove control in the UI. `DELETE /api/watchlist/{ticker}` is
  implemented and tested; the frozen plan has no UI scenario for removal (D-8 in the
  reference run).
- Change text uses semantic tokens. Dark uses `#16a34a`/`#dc2626`/`#6b7280`; light uses
  `#15803d`/`#b91c1c`/`#4b5563`, which meet AA on white. The sign and the direction word
  still carry the meaning, so colour is decoration. The contract names the tokens, not a
  single hex value.
- Quote timestamps render as `dd-MMM-yyyy` (UTC) at the product owner's request, deviating
  from the frozen plan's ISO text. The API contract and `<time dateTime>` remain ISO-8601
  (D-17).
- The app supports multiple accounts with per-account watchlists and open registration
  (D-34). There is no email reset. Losing both the authenticator and all recovery codes for
  one account requires deleting that `users` row; its recovery codes and watchlist cascade.
  The frozen single-owner plans are superseded for this part.
- The frozen plan SCN-001 expects `/` to show `Set up your account`. At the product owner's
  request (D-30) `/` always shows the landing page with `Sign in`; registration is always
  reachable from `/login`. The rest of the flow is unchanged.
- The frozen plan REQ-003 says 12–128 characters. The product owner requested a
  12-character minimum only (D-31); longer passwords are accepted and the UI no longer
  caps input. The plan is unchanged.
- Sessions are in memory: restarting the server requires signing in again (D-20).
- TOTP replay protection consumes the activation step as well, so a login within the same
  30-second step as activation is refused by design (D-22). Recovery codes remain available.
- Windows reports synthetic POSIX mode bits; `stock-dashboard.db` is created with `0600`,
  but the mode is only enforceable on POSIX filesystems.
- The frontend poll interval pauses only while the document is hidden. A visible but
  unfocused window keeps polling, matching "while the page is open".
- `npm audit --omit=dev` reports 0 production vulnerabilities. Dev-only advisories from
  `picomatch` (via Vite) have no patched release; the suggested fix is a breaking Vite
  downgrade and was not applied.
