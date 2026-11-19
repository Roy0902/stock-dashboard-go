# Final Review

Independent, read-only verification of the Go hardening pass and the two React/CSS passes,
against `docs/reviews/{go-code-review,security-review,web-code-review,web-copy-proposals}.md`,
`AGENTS.md`, `docs/quality-gates.md` and decisions D-29, D-34, D-36…D-41.

Everything below was measured in this checkout on 2026-11-18, not taken from a report.

Gates I ran myself:

| Gate | Result |
| :-- | :-- |
| `gofmt -l .` / `go build ./...` / `go vet ./...` | clean, exit 0 |
| `go test ./... -count=1` | all 8 packages `ok` |
| `go test ./... -cover` | api 91.8, auth 87.7, config 94.1, domain 87.6, providers 85.7, store 79.6, webui 76.0, app 73.2, both `cmd/` 0.0 |
| `npm run typecheck` / `lint` / `format:check` | clean |
| `npm run test` | 19 files, 131 tests pass |
| `npm run build` | 139 modules, 110.76 kB gzip |
| `npm test` (e2e) | 33 passed, 31.5 s, 1 worker |
| `node scripts/set-module.mjs stock-dashboard/server --dry-run` | "module path is already stock-dashboard/server" |

## Verdict

**Fix-first — documentation only. No code blocker found; do not merge until the doc pass in
"Top 5" is done.**

The hardening claims are real, not cosmetic: the auth guard is now an allowlist attached per
route and a test walks the route table; `auth.Service.mu` no longer wraps SQLite; Argon2 is
metered on both public paths and setup no longer hashes under `setupMu`; attempts are
reserved before verification; provider throttling, bad keys and cancellation are separated;
one decoder returns `413`; a cancelled batch is flagged; store reads are logged and bounded;
`ALLOWED_HOSTS` and the provider tuning knobs are real settings. I could not construct a
bypass or a data-leak path through any of it, and the new tests assert behaviour, not mocks,
including under 20-way concurrency.

The blockers are procedural, and they are the checklist's own:

1. `contracts.md:220` still pins `#16a34a / #dc2626 / #6b7280` as the UI contract, and
   `verification.md:202-204` states that changing that hue "would violate the observable
   contract" — while the shipped light theme now uses `#15803d / #b91c1c / #4b5563`. The
   change is correct (5.0:1, 6.4:1, 7.6:1 on white); the documents were not updated, so the
   repo currently claims its own release violates the contract.
2. Nine new user-visible strings ship without a `docs/contracts.md` entry
   (`web-copy-proposals.md` says so explicitly), which is a `quality-gates.md` "Definition of
   done" line.
3. `contracts-auth.md` §9 still describes the closed sign-up notice that §11 and the code
   removed (`contracts-auth.md:222,240` vs `:308`; `SetupRoute.tsx:9-22` always renders
   `SetupPage`).

None of those is a code change, so the engineering work can ship as soon as the documents
match it.

Severity totals (introduced by this pass): **0 Critical, 0 High, 2 Medium, 13 Low**, plus 6
nits. Pre-existing items worth tracking are listed separately and are *not* counted.

## Verified fixes

| Claim | Evidence | Verdict |
| :-- | :-- | :-- |
| Auth guard inverted to a public allowlist, per-route wrapping | `api/router.go:12-20` (`public bool` on the single route table), `:57-70` (guard attached while registering), `:40-43` (catch-all public, documented as not reading account state); `api/middleware.go:203-218`. Test: `api/api_test.go:811-854` walks `routeDefinitions`, asserts the public/guarded split and drives every guarded pattern with `denyAllAuth` (401). `isProtectedAPIPath` is gone (no references remain). | **Real.** Fail-closed now: a new route that forgets `public: true` is guarded by default, and a new *public* route fails the test because `wantPublic[pattern]` is false. |
| `auth.Service.mu` no longer held across SQLite; pruning amortised | `auth/service.go:573-598` answers entirely from `sessionEntry` (no store call); `:119-126` carries `username`/`totpEnabled` in the entry; `:610-632` builds it; `maybePruneLocked` `:762-769` runs 1 in 64 lock acquisitions. `store/users.go:279-282` is the hash-free projection. | **Real.** `/api/quotes` now does zero SQL: `Authenticate` (memory) + `WatchlistStore.List` (snapshot). Residual: a session is no longer revalidated against the account row, so an externally deleted account keeps working until the session expires — acceptable, there is no delete path (see residual list). |
| Argon2 metered; no hashing under `setupMu` | `auth/service.go:206-207` (`setupSlot` cap 1, `verifySlots` cap 2), `:269-273` (hash outside `setupMu`, second setup refused with 429), `:434-446` (verify slots), `:709-727` (global 60/min budget). Tests: `auth/service_test.go:776-864`. | **Real.** `TestGlobalAuthBudgetCapsDistinctUsernames` proves the per-username limiter can no longer be multiplied by inventing names. |
| Attempts are atomic (reserve before verify) | `auth/service.go:429-431` (password), `:660` (`challenge.attempts++` before `ValidateTOTP`), `:360-362` (enrollment). Tests: `service_test.go:776-803` (≤5 store reads for a 20-way burst), `:805-836` (≤5 for TOTP). | **Real and deterministic**, because `rateLimitLocked` and `recordFailureLocked` run under the same `mu` hold. |
| `ALLOWED_HOSTS` replaces the hardcoded literal | `config/config.go:78-81,180-238` (validates scheme/path/port, dedupes, rejects empty); `api/middleware.go:87-89,154-186` (per-request map); wired `app/app.go:115`; documented `.env.example:13`, `README.md:141`. Test `api/api_test.go:856-875`, `config/config_test.go:155-184`. | **Real.** `/healthz` is now allowed on a configured name and 403 on an unlisted one; tests use the API, not the constants. |
| Provider failures normalised | `providers/httpclient.go:186-205` (`429`→`FailureRateLimited` + `Retry-After`, `401/403`→`FailureAuth`), `:49-65` (one jittered retry, never on timeout/cancel), `:129-145`; `twelvedata.go:143-169` uses it; `service.go:326-349` separates `FailureCanceled`; `api/errors.go:65-80` maps auth→`503 MISSING_API_KEY`, rate limit→`429`. Tests `adapters_test.go:312,336`, `httpclient_test.go:114,139`. | **Real.** Upstream throttling is no longer reported to the user as "provider unavailable", and no provider text leaves the adapter (`Failure.Error()` is provider+kind only). |
| One JSON decoder, `413` for oversize | `api/decode.go:13-42` (content type, `MaxBytesReader`, `DisallowUnknownFields`, single value, `errors.As(*http.MaxBytesError)`→`REQUEST_TOO_LARGE`); callers `api/handlers.go:142-151`, `api/auth.go:306-310`; status map `api/errors.go:28`; copy `domain/errors.go:101-102`. Tests `api_test.go:781-786,923-928`. | **Real.** `POST /api/watchlist` and `/api/auth/setup` with 5 KB now return `413` with the contract copy; small malformed bodies keep their endpoint code. |
| Batch cancellation is not a clean empty 200 | `providers/service.go:156-168` (labelled `dispatch:`, undispatched indices marked `canceled`), `:197-209` (flags only when nothing served), `:290-296` (pre-cancelled context short-circuits); `api/handlers.go:85-99` maps it to `502`. Tests `providers/service_test.go:564-580` (0 provider calls), `api/api_test.go:877-882`. | **Real.** The shadowed-`break` reading is gone and the semantic hole is closed. |
| Store errors are logged and bounded; reads fail closed | `store/users.go:16-21,58-111,226-240` (bounded warning, 30 s grace, nil after it), `store/watchlist.go:13-59` (snapshot + warning), `store/store.go:87` (`0o700`), `:147-171` (regular-file check, chmod warning), `:220-227` (`PRAGMA quick_check`). Test `store/store_errors_test.go:13-43`. | **Real, with two gaps** listed below (`Count` never fails closed; first `List` read failure returns empty). |
| Provider limits defined once and configurable | `providers/types.go:24-41` (`DefaultLimits`); `config/config.go:82-97,240-262`; `app/app.go:59-71`; `cmd/dashboard/main.go:26-30,96-101` (drain = `WriteTimeout` + 5 s, `Warn` not fatal). | **Real.** Adapter constructors keep fallbacks but the composition root always passes explicit budgets, so the duplicated policy is now a single source in practice. |
| API hardening details | `api/respond.go:10-24` (marshal first, envelope on failure); `api/handlers.go:59-64` (canonicalise before `Remove`); `api/middleware.go:253-256` (`Unwrap`), `:105-113` (no append after a partial write); `auth/service.go:542-549` (full recovery scan). Tests `api_test.go:788-798,800-809,884-906`. | **Real.** All five D-41 items are implemented and asserted. |
| Static assets (bonus, not claimed) | `webui/webui.go:101-137` (`ServeContent`, ETag, `Range`, `HEAD`, `no-store` on `index.html`). | **Real** and an improvement; the review's Low finding is closed. |

## Introduced findings

### [Medium] The UI contract and the verification record now contradict the shipped colours
`docs/contracts.md:220` — `docs/verification.md:202-204` — `web/src/styles/tokens.css:111-113`, `web/src/features/watchlist/QuoteRow.css:97-112`
The contrast fix is correct and I verified the ratios (light `#15803d` 5.0:1, `#b91c1c` 6.4:1, `#4b5563` 7.6:1 on `#ffffff`; the old `#16a34a` is 3.3:1). But `contracts.md` still lists the three dark hexes as *the* UI contract, and `verification.md` states that changing the hue "would violate the observable contract". A reader following the DoD would conclude the release is non-conformant, and the CSS-source guard in `styles.test.ts:38-40` now enshrines the new values in a fourth place.
**Why it matters:** colour is the one place this project documents exact values, and the documents are the acceptance record.
**Fix:** change `contracts.md:220` to name the tokens (`var(--color-up/down/neutral)`, AA in both themes) and rewrite `verification.md:202-204` to record the new values and the measured ratios. **Effort:** S.

### [Medium] Nine new user-visible strings ship without a contract entry
`web/src/locale/en.ts:14,126,129-130,132-138,66-67,73,78` vs `docs/contracts.md` (no match for `Show password`, `Hide password`, `Try again`, `Reload`, `Code expires in …`, `Loading watchlist`, `Loading price`, `No data for this range`, `Adding…`)
`web-copy-proposals.md:30-34` says outright: "None of the strings above are in `docs/contracts.md` yet. They need product approval." `AGENTS.md` ("Copy: use the exact strings in `docs/contracts.md`") and `quality-gates.md` ("New user-visible copy comes from `docs/contracts.md`, not from the implementation") make this a DoD failure, not a nit.
**Why it matters:** the proposal file is an audit trail, not the contract; the next implementer has no approved source for these strings and may "fix" them.
**Fix:** product-approve the table and move the rows into `contracts.md` §6 / `contracts-auth.md` §9, recording the approval in `verification.md`. **Effort:** S.

### [Low] `auth.Service`'s lock-order comment is false
`server/internal/auth/service.go:157-163` — the doc says the store mutex "is only taken while `mu` is not held … so an auth lock can never be held across store/SQLite I/O", but `BeginSetup` holds `setupMu` across `s.users.FindByUsername` (`:295`) and `Activate` holds it across `s.users.Create` (`:396`).
**Why it matters:** this is exactly the invariant a future maintainer will trust when adding a store call; with `busy_timeout(5000)` and one connection, a slow write blocks *all* registration for up to 5 s. It is bounded, not a deadlock (order is still `setupMu → store.mu`, never the reverse).
**Fix:** either amend the comment to "`mu` is never held across store I/O; `setupMu` may be, and must be released before any new store call", or move the `Create` outside `setupMu` (it already has a unique-username constraint to serialise on). **Effort:** S.

### [Low] A busy verification semaphore is recorded as a wrong password
`server/internal/auth/service.go:429-436` — `recordFailureLocked` runs before `tryAcquire(s.verifySlots)`, and the refusal path returns `RATE_LIMITED` without releasing the reserved slot.
**Why it matters:** a *correct* password submitted while two verifications are in flight consumes one of the five failure slots. Two such refusals plus three real typos put a legitimate user behind the 60 s block. This is a fairness regression introduced by the otherwise-correct "reserve before verify" ordering.
**Fix:** acquire the slot first and release the reserved failure slot on a semaphore refusal (a small `undoFailureLocked`), or refuse with a distinct retry-budget that does not touch `attempts`. **Effort:** S.

### [Low] `/api/auth/me` is guarded twice and its own 401 branch is unreachable
`server/internal/api/router.go:38` (no `public`, so `requireSession` wraps it) and `server/internal/api/auth.go:186-197` (re-implements the check and clears the cookie again).
**Why it matters:** `Authenticate` runs twice per request, two identical clearing `Set-Cookie` headers are emitted on failure, and a reader cannot tell which layer owns the answer. The leftover is a fossil of the old blocklist design (the review specifically called it out at `auth.go:190-201`).
**Fix:** collapse the handler to "read the session user from context and return it", or mark the route public and keep the explicit check — not both. **Effort:** S.

### [Low] `Login` can log a password typed into the username field (pre-existing, still reachable)
`server/internal/api/auth.go:114,232-238` — `loggableUsername` filters by *shape*, so a 3–32 char lowercase password that happens to match `^[a-z0-9][a-z0-9._-]{2,31}$` is written to the JSON log on a failed login.
**Why it matters:** the security review rated this Low and it was in its Top-5 ("Log the submitted username only when it resolves to an existing account"), but the pass did not touch it. Note the code has not changed here; I flag it because the pass claimed the log surface as hardened.
**Fix:** log `username_present:false` for unknown names, or a keyed hash. **Effort:** S.

### [Low] The new route-table test cannot catch the failure mode it was written for
`server/internal/api/api_test.go:811-837`
`wantPublic` is a hand-maintained map and the assertion is `route.public != wantPublic[pattern]`. For a new **guarded** route, `wantPublic[pattern]` is `false` and `route.public` is `false`, so it passes silently — which is the safe direction, but it means the test does not fail when someone adds a route *without thinking about the split*. Only the second half (the `denyAllAuth` sweep at `:839-853`) actually proves the guard.
**Why it matters:** this is the regression net for D-36; it should fail loudly on any new route, not just careless public ones.
**Fix:** fail on any `/api` pattern absent from `wantPublic` (`if _, ok := wantPublic[pattern]; !ok { t.Fatalf(...) }`) so adding a route is a deliberate, reviewed act. **Effort:** S.

### [Low] `TestSecondConcurrentSetupIsRefused` is white-box and cannot fail on the bug it targets
`server/internal/auth/service_test.go:838-849` — the test pushes a value straight into `service.setupSlot` and asserts the next `BeginSetup` returns 429.
**Why it matters:** if `BeginSetup` regressed to hashing *under* `setupMu`, this test would still pass; it only proves the channel check exists. D-37's real claim ("second concurrent setup returns 429 instead of queueing") is untested at the concurrency level.
**Fix:** start two real `BeginSetup` calls with a `Rand` reader that blocks on the first read (or with production-cost params), and assert the second returns 429 before the first completes. **Effort:** S.

### [Low] `styles.test.ts` asserts CSS source text, not the property it guards
`web/src/styles/styles.test.ts:10-41` — greps three stylesheets for the literal `@media (max-width: 960px)`, forbids `overflow-x: hidden` by regex, and asserts exact hex strings in the light block.
**Why it matters:** the review asked for the *measured* properties (no overflow at 900–960 px, AA contrast). The test stays green if someone raises a track minimum (overflow returns at a different width), and it pins a magic number in four files. It also cannot see whether `.sr-only`/`display:contents` still produces seven grid items.
**Fix:** keep the cheap hex guard, add one Playwright viewport test at 900 px asserting the "View chart" link is inside the viewport and has a non-zero box. **Effort:** M.

### [Low] Two 9-method auth stubs that differ only in return values
`server/internal/api/authstub_test.go:9-81` — `allowAllAuth` and `denyAllAuth` duplicate `Status/BeginSetup/Activate/Login/CompleteTOTP/CompleteRecovery/Logout/Authenticate`.
**Why it matters:** 80 lines that must be edited twice when `AuthService` grows; the added `TestEveryAPIRouteIsPublicOrGuarded` only needed a flag.
**Fix:** one struct with `session *auth.Account`; derive both stubs. **Effort:** S.

### [Low] `ChartView` has a dead state branch
`web/src/features/chart/ChartView.tsx:34-35` — `series === undefined` cannot be reached after `chart.isPending` and `chart.isError` have both been excluded (`useQuery` guarantees `data` once neither holds; `unwrap` throws instead of returning `undefined`).
**Why it matters:** it renders `Chart unavailable` for a state that cannot exist, which hides a future real "no data" case behind the wrong copy.
**Fix:** delete the branch. **Effort:** S.

### [Low] A cancelled batch answers `502`, while the contract says it is distinct from an outage
`server/internal/api/handlers.go:86-90` vs `docs/contracts.md:192-193` ("A cancelled request context is distinct from an outage and never becomes a clean empty batch").
**Why it matters:** the new behaviour is defensible (the client is usually gone), but the contract sentence now overstates what a caller can observe: cancellation and a provider outage are identical (`502 PROVIDER_UNAVAILABLE`).
**Fix:** reword the contract to "is never reported as a clean empty batch; an abandoned request yields no user-visible state, and any surviving response is `502`". **Effort:** S.

### [Low] "Quotes are cached for 60 seconds" is now configuration-dependent copy
`web/src/locale/en.ts:45-46` — the landing footnote interpolates the host but hard-codes 60 s, while `QUOTE_CACHE_TTL` (`config/config.go:94`) now changes it. The review asked for this sentence to be dropped; it survived and became less true.
**Why it matters:** first-sentence copy on the landing page can be wrong on a default override, and it is not in `contracts.md`.
**Fix:** drop the TTL clause (`Runs on {host}. Quotes refresh while the page is open.`) or have the API expose the TTL. **Effort:** S.

### [Low] `set-module.mjs` is root-only, rewrites historical reviews, and under-documents its skip list
`scripts/set-module.mjs:25` (`root = process.cwd()`), `:31` (`SKIP`), `:39`; `README.md:124`.
Running from `server/` gives `Error: ENOENT … server\server\go.mod` with a Node stack trace (I reproduced it). The dry run from the root rewrites `docs/reviews/go-code-review.md` (1 reference) and `scripts/set-module.mjs` (3 references) — the former contradicts `docs/reviews/README.md:19` ("Keep reports as written. Corrections belong in the disposition, not in an edited report"). `\.git` in `SKIP` also matches `.github`, and `e2e/.tmp` is not skipped, so neither is covered by the README promise ("never touches `node_modules`, `dist`, `.dev` or `data`").
**Why it matters:** adopters are told this is a one-command, safe rewrite; it silently edits the audit trail and fails unhelpfully from a subdirectory.
**Fix:** resolve the root from `import.meta.url` like `scripts/*.ps1` do, add `docs/reviews` and `e2e/.tmp` to `SKIP`, read `.go`, `.mod`, `.ps1`, `.cmd`, `.mjs`, `.yml` only, and update `README.md:124`. **Effort:** S.

### [Low] `SETUP_COMPLETE` is dead but still mapped in three places
`server/internal/api/errors.go:38`, `server/internal/domain/errors.go:35,117` — `contracts-auth.md:275` states "`SETUP_COMPLETE` is no longer returned by setup", and nothing produces it.
**Why it matters:** it is the leftover of the single-owner rule, and a future handler copying the code table will believe it is reachable.
**Fix:** delete the constant, the map row and the copy case, and remove the row from `contracts-auth.md:182`. **Effort:** S.

### Nits
- `server/internal/store/users.go:99-111` — `Count` returns `lastCount` with no grace check, so the "fails closed after 30 s" sentence at `:16-21` is only true for `FindByID`/`FindByUsername`.
- `server/internal/store/watchlist.go:34-38` — a first-read failure returns the zero watchlist (an empty list) and is only distinguishable by the warning; the snapshot cannot be wrong because `Open` seeds every existing account, but a newly created account is unseeded because `Users.Create` does not seed `Watchlist.last`.
- `server/internal/api/middleware.go:137` still logs `r.URL.Path` unbounded and `cmd/dashboard/main.go:25` keeps `MaxHeaderBytes = 1 << 20`; the security review's amplification finding is unaddressed.
- `server/internal/auth/service.go:31-34` — the 60/min global auth budget is a new user-visible `429` that has no number in `contracts-auth.md` (§7 only says "a global per-minute budget").
- `web/src/features/auth/AuthForms.css:213-215` — `.auth-account` is still dead CSS; `web/src/features/auth/index.ts` is still an unused barrel (both called out in the web review, both still present).
- `npm run test` prints five `Not implemented: Window's scrollTo()` lines: `App.tsx:29` is exercised by tests outside `App.test.tsx`, which do not stub it.

## Test-quality audit

**The two e2e edits are honest and do not weaken coverage.**

- **TOTP auto-submit** — `e2e/tests/auth.spec.ts:59-64` fills the six-digit field and asserts the `Watchlist` heading, with a comment explaining why no click follows. Because Playwright's `fill` delivers one `onChange` with six digits, this exercises `VerificationCodeInput.tsx:35-46`'s `onComplete` path and *fails* if auto-submit is removed (the heading never appears). The old click path is still covered at unit level (`web/src/features/auth/LoginPage.test.tsx:242` clicks `Continue` after an error). Net: the e2e suite now tests the new behaviour instead of a control that is no longer the primary path — and the auto-submit is exactly what the review asked for (`web-code-review.md:95-100`).
- **First-run redirect** — `e2e/tests/auth.spec.ts:196-205` sets `bootstrap:false` and asserts `toHaveURL(/\/setup$/)` *before* the `Create account` heading. That is strictly stronger than the previous landing-page assertion: it would catch `LandingRoute` dropping the `setupRequired` branch (`LandingRoute.tsx:18-20`), and `AppAuth.test.tsx:69-78` independently asserts the marketing heading and the `Sign in` link are absent.
- `SetupPage`'s auto-submit is correctly gated on the checkbox (`SetupPage.tsx:275-281`), which is why the e2e order (fill code → check box → click) still succeeds; that ordering is not a hidden workaround but the real behaviour, and it also incidentally covers the no-auto-submit-yet path.

What is strong: the Go concurrency tests are deterministic rather than sleep-based (`service_test.go:776-836` counts store reads under a real 20-way burst); `store_errors_test.go` forces a real closed DB instead of a mock; the new API tests drive the router (`api_test.go:811-854`) and the real `writeJSON`/`recoverMiddleware`; `AppAuth.test.tsx:250-307` asserts the *unavailable* phase is not "signed out" and that retry re-fetches, and `:315-363` uses a fake `BroadcastChannel` to assert cross-tab behaviour.

Gaps in the new paths (none blocking):

- No test for the `!confirmed` branch of `SetupPage`'s auto-submit (typing six digits before ticking the box must *not* activate); it is the one ordering the implementation deliberately supports.
- No test that a refused `verifySlots` does not consume a failure slot (the finding above would be caught by it).
- No test for `Count()` falling back, and no test for a first `WatchlistStore.List` failure.
- No e2e for `413` (Go-level `api_test.go:781,923` is good, but the browser path is unverified).
- Pre-existing timing sensitivity remains in `providers/service_test.go:529-562` (5 ms sleep, `maxInFlight >= 2`) and `providers/httpclient_test.go:75-95` (2.1 s sleep per run). Neither was touched; the 2.1 s sleep is 7 % of the whole Go suite.
- `styles.test.ts` (see above) is the one new test that checks source text rather than behaviour.

## Contract/doc drift

| Where | Document says | Reality | Verdict |
| :-- | :-- | :-- | :-- |
| `contracts.md:220`, `verification.md:202-204` | `#16a34a/#dc2626/#6b7280` are the contract, changing them is a violation | light theme uses `#15803d/#b91c1c/#4b5563` (`tokens.css:111-113`) | **Introduced.** Must be fixed before release. |
| `contracts-auth.md:222,223,238-240` | `/setup` after setup shows `Sign-up is closed on this installation.`; "the sign-up flow is closed after setup" | `SetupRoute.tsx:9-22` renders the open create-account form; §11 (`:308`) says the notice was removed; `AppAuth.test.tsx:89,113,128` assert its absence | Pre-existing (the web review flagged it at its own `:265`), still unresolved. §11 "supersedes" it, but §9 reads as current. |
| `contracts-auth.md:182,275`; `api/errors.go:38` | `SETUP_COMPLETE` exists / is never returned | dead code in three files | **Introduced leftover** of D-34. |
| `contracts.md:232` | Chart `Back` "points to `/`" | `ChartPage.tsx:14` → `/dashboard` (D-29) | Pre-existing; harmless because `/` redirects when authenticated. |
| `contracts.md:192` | cancellation "distinct from an outage" | `handlers.go:86-90` returns `502 PROVIDER_UNAVAILABLE` | **Introduced wording gap** (see Low finding). |
| `contracts-auth.md:59,205-209` | 5 failures/15 min, "a global per-minute budget" | 60/min hard-coded (`auth/service.go:31-34`), not named | Minor; the number is user-visible (429) and unstated. |
| `verification.md:41-50` | coverage api 90.0, app 78.1, auth 89.6, config 98.1, domain 88.2, providers 88.1, store 81.1, webui 81.1 | measured now: api 91.8, app 73.2, auth 87.7, config 94.1, domain 87.6, providers 85.7, store 79.6, webui 76.0 | Table not re-measured; four packages *lost* coverage while new code landed (app −4.9, webui −5.1, config −4.0, providers −2.4). |
| `verification.md:174` | "the UI caps at 128 and the server rejects longer" | no `maxLength`; `D-31` removed the maximum; server accepts long passwords (`domain/validate` + 4 KiB body cap) | Pre-existing and false. |
| `quality-gates.md:7` | baseline "12 files, 94 tests" | 19 files, 131 tests | Stale, hedged by "test counts grow"; refresh it. |
| `docs/reviews/README.md:21-24` | lists one current report | five reports exist (`go-code-review`, `security-review`, `web-code-review`, `web-copy-proposals`, this file) | Update the list. |
| `README.md:124` | script "never touches `node_modules`, `dist`, `.dev` or `data`" | also skips `playwright-report`/`test-results`, and does touch `docs/reviews`, `e2e/.tmp`, `.github` | Under-documented. |
| `.env.example`, `README.md:138-145`, `contracts.md:23,207-208`, `docker-compose.yml` | five new settings | all present, validated, and consistent with `DefaultLimits` | **No drift.** |
| `decisions.md` D-36…D-41 | six decisions | each describes the code as shipped (verified row by row above) | **No drift**, except the D-40 "fails closed" nuance (`Count`) and the D-37 comment-lie nit. |

`docs/contracts.md` was correctly extended with `413`/`REQUEST_TOO_LARGE` (`:72-74`), the
malformed-`DELETE` rule (`:101-102`) and the provider mapping (`:188-195`), so the pass did
do the contract work for the Go side; the gaps are the frontend copy and the colour row.

## Residual pre-existing issues worth tracking

Ordered by what I would actually do next; none introduced by this pass.

1. **Cross-site logout** — `api/middleware.go:215` and `api/auth.go:192` clear `sd_session` on every unauthenticated response, even when no cookie was presented. A hostile page can force a sign-out with `<img src="http://127.0.0.1:3000/api/watchlist">`. Security review Low #3 and Top-5 #4; still open.
2. **`.migrated` files keep secret material with inherited permissions** — `store/migrate.go:207-211` renames only (`_ = os.Rename`), never chmods or deletes; `store.go:87` fixed the directory to `0o700` but the legacy `users.json.migrated` still holds password hashes, TOTP seeds and recovery hashes. Security review Top-5 #5, half-done.
3. **TOTP seeds are plaintext in SQLite** — `store/store.go:39`; `0600` is advisory on Windows (`verification.md:171,221`). Unchanged; needs an explicit accept-or-fix decision.
4. **Store callbacks run under a mutex and an open transaction** — `store/users.go:169-212`, `store/watchlist.go:64-90`. Safe today; an undocumented invariant.
5. **`auth.User` is an exported persistence row with credential JSON tags** — `auth/service.go:46-58`; one `slog.Any`/`json.Marshal` away from leaking. The `Account` split shows the intent.
6. **Unknown-username timing / enumeration** — `auth/service.go:254` (setup 409 vs 201 + Argon2) and the dummy verify. Documented trade-off, worth a decision entry rather than silence.
7. **`AUTH_COOKIE_SECURE` parser vs message** — `config/config.go:117-127` accepts `1/t/T/TRUE` but the error says "must be true or false"; `docker-compose.yml:15` still pins `"false"` (security review Low).
8. **`Update`'s "account not found" is a bare error** — `store/users.go:184` → `500 INTERNAL_ERROR` through `auth` instead of a classified code (review nit `users.go:151`).
9. **Provider aggregates always blame Finnhub** — `providers/service.go:219,228,236,247,270-275`; a Twelve-Data-only failure logs and reports `Finnhub`. Misleading, tests-pass.
10. **`domain.DefaultMessage(CodeRateLimited)` returns "Try again in 1 seconds."** — `domain/errors.go:93-94`; only `NewRateLimited` produces grammatical copy.
11. **`PointsFor` returns 0 for an invalid range** — `domain/priceseries.go:44-45`; a future caller would request `outputsize=0`.
12. **Unbounded watchlist + global provider budgets** — `domain/watchlist.go`, `api/handlers.go:83-84`; one account can starve the shared 55/7 rpm.
13. **No `-race` run anywhere** — `CGO_ENABLED=0` on this host; now that `service.go` has two semaphores, a `sync.Mutex` and four maps, this is the highest-value missing gate (run it on a cgo toolchain before release).
14. **Unrequested UI gaps from the web review** — no copy-all/download for recovery codes (`RecoveryCodeList.tsx`), no route-level code splitting, light mode is media-query only. Product calls, not defects.
15. **`cmd/` has 0 % coverage** — `run()` owns signal handling and the shutdown drain that D-39 changed; `healthcheck` is untested.

## Top 5 actions before release

1. **Reconcile the colour contract** — `contracts.md:220` and `verification.md:202-204` must state the token names and the measured AA values, not the old hex literals. *(Docs; S.)*
2. **Approve and publish the nine new strings** into `contracts.md` §6 / `contracts-auth.md` §9, then record the approval in `verification.md`. *(Docs; S.)*
3. **Fix `contracts-auth.md` §9** (closed sign-up notice at `:222,223,238-240`) so it matches §11 and `SetupRoute.tsx`, and delete the dead `SETUP_COMPLETE` path (`api/errors.go:38`, `domain/errors.go:35,117`, `contracts-auth.md:182`). *(Docs + S code.)*
4. **Close the two new auth nits**: do not charge a failure slot when `verifySlots` is busy (`auth/service.go:429-436`), and correct the lock-order comment (`:157-163`). Then strengthen the two weak tests (`api_test.go:811-837` fail-on-unknown-route; `service_test.go:838-849` real concurrent setup). *(S.)*
5. **Refresh the measured state**: re-run `go test -cover` into `verification.md:41-50` (four packages regressed), update `quality-gates.md:7`, list all reports in `docs/reviews/README.md`, and harden `scripts/set-module.mjs` (root from `import.meta.url`, skip `docs/reviews` + `e2e/.tmp`, README skip-list). *(Docs + S.)*

Recommended before tagging, not blocking: emit the clearing `Set-Cookie` only when a session
cookie was presented (residual #1), and run `go test -race ./...` on a cgo toolchain
(residual #13).
