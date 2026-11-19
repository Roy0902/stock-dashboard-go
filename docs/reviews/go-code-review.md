# Go Code Review

Reviewed `server/` only (cmd, internal/api, internal/app, internal/auth, internal/config, internal/domain, internal/providers, internal/store, internal/webui) against `AGENTS.md`, `README.md`, `docs/architecture.md`, `docs/contracts.md`, `docs/contracts-auth.md` and `docs/decisions.md`.

Baseline in this checkout: `gofmt -l .` prints nothing, `go vet ./...` is clean, `go test ./... -count=1` passes (api 90.0%, auth 89.6%, config 98.1%, domain 88.2%, providers 88.1%, store 81.1%, webui 81.1%, app 78.1%, both `cmd/` packages 0.0%). `go test -race` cannot run here: `CGO_ENABLED=0` and `-race requires cgo`.

## Verdict

The layering is disciplined and the contract surface is unusually well covered by tests; there is no forbidden import, no leaked key or token, and the money paths (quote math, TOTP replay, migration, transactional storage) are correct. What a senior reviewer would push back on is not the domain code but the edges: authorisation is enforced by a hand-maintained list of *protected* paths, so it fails open, and two locks (`auth.Service.mu` and the store mutexes) are held across SQLite I/O and user callbacks, which serialises every authenticated request. Public Argon2 work is also unmetered, so the login/setup endpoints are the cheapest way to consume the process.

## What is already good

- `domain` imports only `errors`/`fmt`/`math`/`sort`/`strings`/`time`: no `net/http`, `os`, `database`, `context`, `auth` or `providers`. Derived quote fields are computed exactly once (`domain/quote.go:60-93`) and the client is not trusted to recompute them.
- Dependency direction matches `docs/architecture.md` exactly: `api -> {auth, domain, providers}`, `store -> {auth, domain}`, `providers -> domain`, `webui -> stdlib`.
- One status mapping table (`api/errors.go:16-42`) and one `fail` funnel (`api/errors.go:81-112`); every domain error carries stable copy from `domain/errors.go:80-127`.
- Secrets hygiene is real and tested: keys never leave `providers` (`providers/types.go:20-30`), URLs are never logged (`api/middleware.go:124-131`), the access log strips query strings, panic values and provider text are deliberately not logged, and `TestAuthResponsesAndLogsHoldNoSecrets` proves it.
- Auth primitives are sound: Argon2id PHC with guard rails (`auth/password.go:110-147`), 256-bit hashed tokens (`auth/tokens.go`), ±1-step TOTP with persisted replay rejection (`auth/service.go:421-441`), single-use recovery codes, constant-time secret comparison, dummy-verify for unknown users, and fail-closed persistence.
- SQLite is used carefully: prepared/parameterised statements everywhere, one transaction per mutation, `PRAGMA` verification at open (`store/store.go:184-212`), schema-version guard, and migration that fails closed on a corrupt legacy file.
- JSON bodies are capped at 4 KiB with `DisallowUnknownFields` and a single-value check; cookies are `HttpOnly` + `SameSite=Strict`; there is an Origin guard and a loopback Host guard.
- Tests are overwhelmingly table-driven or subtest-driven with real fixtures, and coverage is high on the contract-critical packages.

## Findings

### [High] Authorisation is a blocklist of protected paths, so it fails open
- **Where:** `server/internal/api/middleware.go:226-237`, wired at `server/internal/api/middleware.go:195-211` and `server/internal/api/router.go:23-41`
- **Smell:** `requireSession` only guards paths that `isProtectedAPIPath` enumerates (`/api/watchlist*`, `/api/quotes`, `/api/chart/*`). Every other `/api` route is public by default. `/api/auth/me` has to re-implement the check (`api/auth.go:190-201`), and the router's catch-all is also unguarded.
- **Why it matters:** The security invariant is "new routes are public unless someone remembers to edit a switch in another file". `TestProtectedEndpointsDenyWithoutSession` (`api/auth_test.go:838-869`) only asserts the routes that exist today, so a future `GET /api/portfolio/export` would ship unauthenticated and stay green. I found no current bypass, but this is the shape of an eventual one.
- **Fix:** Invert the rule: keep a public allowlist (`/healthz`, `/api/auth/status`, `/api/auth/setup`, `/api/auth/login*`, `/api/auth/logout`, `/api/auth/2fa/activate`, SPA) and guard everything else under `/api`. Better still, attach the guard per route in the table (`mux.Handle("GET /api/watchlist", h.session(h.listWatchlist))`) so a route cannot be registered without it, and add a test that walks the registered patterns and fails for any `/api` pattern not wrapped or allowlisted.
- **Effort:** S

### [High] Unauthenticated Argon2 work is unmetered and serialised behind `setupMu`
- **Where:** `server/internal/auth/service.go:212-271` (lock at `:213`, hash at `:227`), `server/internal/auth/service.go:347-373` (dummy verify at `:365`)
- **Smell:** `BeginSetup` runs Argon2id at 64 MiB/t=3 while holding the package-wide `setupMu`, and is reachable by anyone because registration is permanently open (D-34). `Login` runs a full dummy hash for every unknown username. The only limiter is per-username and only for password failures (`:354-360`), so N distinct usernames cost N × ~100 ms CPU and N × 64 MiB transient allocation.
- **Why it matters:** Setup/activation is the one code path where an unauthenticated request owns a global lock, so one concurrent caller also stalls every other registration. Any local process, or a container port published wider than `127.0.0.1`, can pin cores and spike memory without ever guessing a credential.
- **Fix:** Put setup and password verification behind a small global semaphore plus a cheap global/IP budget (the existing `auth.Budget`-style window is fine), and cap concurrent Argon2 work (`make(chan struct{}, 2)` wrapping the hash call). Do not hold `setupMu` while hashing: hash first, then take `setupMu` only for the map mutation and for the Activate/Create window. Add a test that asserts the second concurrent begin is refused with `429`, not queued.
- **Effort:** M

### [High] `auth.Service.mu` is held across SQLite reads, and `pruneLocked` scans all state per request
- **Where:** `server/internal/auth/service.go:502-523` (`s.mu.Lock()` at `:509`, `users.FindByID` at `:516`), `server/internal/auth/service.go:657-679`; the per-request caller is `server/internal/api/middleware.go:201-206`
- **Smell:** `Authenticate` takes the service-wide mutex, walks four maps, then calls into the store, which takes `Users.mu` and runs two SQL queries (`store/users.go:41-45` and `:268`) on a pool capped at one connection (`store/store.go:92-93`). So every authenticated request holds `auth.mu` → `Users.mu` → the single DB connection, and goroutines that are not even touching auth queue behind it.
- **Why it matters:** This is the classic "mutex held across I/O" bug: a slow or blocked SQLite call (busy timeout is 5 s, `store/store.go:170`) freezes all sessions, `/api/auth/status` included. It also invents a lock ordering (`auth.mu → store.mu`) that is not documented and is only safe because no store callback currently calls back into `auth`; one future callback would deadlock. The map scan makes the cost O(sessions + challenges + enrollments + attempts) on every poll.
- **Fix:** Copy the `sessionEntry` under the lock, release it, then load the account outside. Better, put the immutable `username` (already stored) and `TOTPEnabled` in `sessionEntry`, so `Authenticate` needs no DB read at all and only `CompleteTOTP`/`CompleteRecovery` touch the store. Move pruning to a background ticker (or amortise it every Nth call) and document the lock order.
- **Effort:** M

### [Medium] Store read paths swallow query errors with no log line, so corruption is invisible
- **Where:** `server/internal/store/watchlist.go:23-33`, `server/internal/store/users.go:37-47`, `:50-65`, `:69-79`
- **Smell:** `List`, `FindByID`, `FindByUsername` and `Count` all catch a query error and silently return a cached copy (or `nil`/`0`). The `err` value is dropped: no `slog`, no counter, no `errors.Is(sql.ErrConnDone)` distinction.
- **Why it matters:** The stated intent ("a transient error must not look like an empty watchlist") is good, but the implementation makes a *permanent* failure look like a healthy one forever: the dashboard serves stale data, `/api/auth/status` keeps saying `setupRequired:false`, and the only symptom is "nothing updates". It also silently continues authenticating from a cache when the credential table is unreadable, which is the wrong default for an auth path.
- **Fix:** Return `(value, error)` from the store ports (or add a `Health()`/error callback), log the first failure at `Warn` with the operation name (never the query text or arguments), and rate-limit that log. For the auth lookup specifically, fail closed after a bounded grace window rather than serving the cache indefinitely.
- **Effort:** M

### [Medium] The in-memory caches are never used for reads, so each poll is 3 SQL round trips
- **Where:** `server/internal/store/users.go:41-45`, `:268`, `server/internal/store/watchlist.go:27`, `server/internal/store/store.go:92-93`; call chain `server/internal/api/middleware.go:202` → `server/internal/api/handlers.go:84`
- **Smell:** `Open` seeds `Users.lastByID` and `WatchlistStore.last` and both are maintained on every write, but they are only consulted after a failed query. Every `GET /api/quotes` therefore performs: `users WHERE id` + `recovery_codes WHERE user_id` (both per request, and the recovery hashes are not needed to authenticate) + `watchlist WHERE user_id`, on a one-connection pool that also serialises the auth mutex.
- **Why it matters:** The cache exists, costs memory and code, and buys nothing on the hot path. The polling dashboard is the main traffic source, so this is pure latency and write-contention on a single connection for data that changes a few times a day.
- **Fix:** Serve `Authenticate` from the session entry (see the High finding) and drop `loadRecoveryHashes` from `FindByID` — it belongs to the login/TOTP paths. Serve `List` from the snapshot and invalidate on `Update`, re-reading only on startup or an explicit refresh. Keep the DB as the source of truth for writes.
- **Effort:** M

### [Medium] The Host allowlist is hardcoded to loopback, with no configuration hook
- **Where:** `server/internal/api/middleware.go:146-175` (allowlist literal at `:170`)
- **Smell:** `hostGuard` rejects any `Host` other than `127.0.0.1`, `localhost` and `::1`, including the rest of the loopback range (`127.0.0.2`, `[::2]`) and every DNS name. It runs for all paths, including `/healthz`, before panic recovery.
- **Why it matters:** This silently makes the app undeployable behind a reverse proxy, a `.local` hostname, or a container reached as `host.docker.internal` — all legitimate for a local tool — and the failure mode is a generic `403 CROSS_ORIGIN_REQUEST` on `/healthz`, which looks like a CSRF bug. It is also duplicated policy: the same three names are repeated in tests (`api/auth_test.go:769`) and in docs.
- **Fix:** Add `ALLOWED_HOSTS` (comma-separated, defaulting to the loopback set) to `config.Config` and pass it into `Deps`; validate entries the same way `parseBaseURL`/`parseBindHost` validate today. Move `recoverMiddleware` outside `hostGuard` so a panic in any guard still produces an envelope.
- **Effort:** S

### [Medium] JSON body decoding is duplicated, and an oversized body is reported as `INVALID_TICKER`
- **Where:** `server/internal/api/handlers.go:143-165` and `server/internal/api/auth.go:306-324`
- **Smell:** Two near-identical decoders (content-type check, `MaxBytesReader`, `DisallowUnknownFields`, single-value check) differ only in the failure code they return. Both map every decode failure — including `*http.MaxBytesError` — to a 4xx domain code, so a 5000-byte ticker body is reported as "Enter a valid ticker" rather than 413.
- **Why it matters:** Two copies of a security-relevant check drift: the next person who adds a body limit or a charset rule will patch one. And 400-for-too-large is misleading for clients and hides a spam/DoS signal in the access log.
- **Fix:** One `decodeJSON(w, r, dst, limit, failureCode)` helper in a shared file, with `var maxErr *http.MaxBytesError; if errors.As(err, &maxErr) { return 413 }`. Keep endpoint-specific copy only for malformed-but-small bodies.
- **Effort:** S

### [Medium] Provider failures are not normalised: no retry, upstream 429 becomes 502, context errors become 502
- **Where:** `server/internal/providers/httpclient.go:44-70` (single attempt), `server/internal/providers/twelvedata.go:143-166` (non-2xx → `FailureUnavailable` at `:159-161`), `server/internal/providers/budget.go:83-86` (raw `ctx.Err()`), `server/internal/providers/service.go:290-312` (anything that is not `*Failure` sets `unavailable = true`)
- **Smell:** A transient socket reset or one slow response immediately omits the ticker; Twelve Data's own `429`/`403` (rate limit / bad key) is classified as `FailureUnavailable`; a cancelled request is classified as a provider outage.
- **Why it matters:** The contract distinguishes `429` (local budget), `502` (providers down) and `404` (unknown symbol); collapsing upstream throttling into `502` tells the user "provider unavailable, try shortly" when the truth is "we are being throttled and a retry in N seconds would work". The local budget already has `RetryAfter`, so the information exists.
- **Fix:** Map `429` to `FailureRateLimited` with `Retry-After` (parse the header), map `401/403` to a distinct kind so an operator can see a bad key, add one bounded retry with jitter for idempotent GETs, and wrap `ctx.Err()` in a `Failure{Kind: FailureCanceled}` that the service turns into "no quote" rather than `Unavailable`.
- **Effort:** M

### [Medium] Batch dispatch relies on a shadowed `break` and turns cancellation into a silent empty 200
- **Where:** `server/internal/providers/service.go:148-159`
- **Smell:** `break` inside `select` breaks the `select`, not the `for`; the loop actually exits via the following `if ctx.Err() != nil { break }`. When the context is cancelled, `outcomes[index]` for undispatched tickers stay zero-valued (`quote == nil`, `refused == false`, `unavailable == false`), so the batch is returned as a clean, partly-empty success.
- **Why it matters:** The control flow reads as a bug even though it works, and the semantic result is wrong: a cancelled request looks like "these tickers simply have no quote". If the client is still reading (server-side cancel from `Shutdown`), it gets `200 {"quotes":[]}` with no flag, which is indistinguishable from the documented empty-watchlist case.
- **Fix:** Label the loop `dispatch:` and `break dispatch`, or return early on `ctx.Err()`. Mark undispatched indices as `unavailable = true` (or set a batch-level `Canceled` flag) so the caller never mistakes "never asked" for "asked and had no data". Add a test with a pre-cancelled context asserting the documented outcome.
- **Effort:** S

### [Medium] Rate limits, cache TTL, worker count and per-call timeouts are duplicated constants with no configuration
- **Where:** `server/internal/app/app.go:17-23` (55/7/2 s/60 s/8) versus `server/internal/providers/finnhub.go:30,33`, `server/internal/providers/twelvedata.go:32,35`, `server/internal/providers/httpclient.go:37`, plus `server/internal/providers/service.go:13-17` and `server/internal/providers/budget.go:11-14`
- **Smell:** The same four production numbers exist two or three times, and every one is a compile-time constant. `NewFinnhub`/`NewTwelveData` silently fall back to their own 55/7/2 s defaults when the composition root forgets to pass them.
- **Why it matters:** These are exactly the knobs an operator needs when a key is upgraded or a provider tightens its quota; today a change requires a rebuild, and a change in one copy can silently disagree with the other. The contract documents 55/7/2 s, so drift becomes a contract bug.
- **Fix:** Export them once from `providers` (e.g. `providers.DefaultLimits`) and reference that from `app`; add optional env overrides (`FINNHUB_RPM`, `TWELVEDATA_RPM`, `UPSTREAM_TIMEOUT`, `QUOTE_CACHE_TTL`) validated in `config.Load`. Delete the adapter-level defaults (nil budget/client should be a constructor error).
- **Effort:** S

### [Medium] `auth.User` is a persistence record with JSON tags for secrets, exported from the auth API
- **Where:** `server/internal/auth/service.go:36-48` (struct and tags), consumed by `server/internal/store/users.go:190-233` and `server/internal/store/migrate.go:122-125`
- **Smell:** The type that carries `PasswordHash`, `TOTPSecret` and `RecoveryCodeHashes` is both the auth package's published model and the store's row shape, and it is annotated for `encoding/json` so a legacy file can be decoded straight into it. `Account` (`:70-76`) exists to hide the fields, which is evidence the split was recognised but only half applied.
- **Why it matters:** Any future `json.Marshal`/`slog.Any` of a `User` writes credentials; the boundary that is supposed to guarantee "hashes never leave this layer" is a comment plus a second struct. It also forces `store` to import `auth` solely for this row type, which is the whole `store -> auth` edge.
- **Fix:** Move the persisted record to an unexported `store` struct (or a `store.UserRecord`) and translate to `auth.User` at the port. If the port must stay, remove the JSON tags and give the legacy importer its own DTO with explicit field names.
- **Effort:** M

### [Medium] Persistence ports run caller callbacks inside a mutex and an open transaction
- **Where:** `server/internal/store/users.go:136-178` (`fn(next)` at `:155`), `server/internal/store/watchlist.go:38-64` (`fn(current)` at `:52`), callers in `server/internal/auth/service.go:422-433`, `:464-481`, `server/internal/api/handlers.go:50-55`
- **Smell:** `Users.Update` and `WatchlistStore.Update` hold their mutex, hold a SQLite transaction, and then execute arbitrary caller-supplied code that may itself block or call back into the same ports.
- **Why it matters:** It is safe only while every callback stays trivial, which is an undocumented invariant. A callback that calls `List`/`FindByID` (natural for the next feature) deadlocks on a non-reentrant mutex, and a slow callback holds a write transaction, which with `busy_timeout=5000` and one connection blocks every reader.
- **Fix:** Do the read + validation first, then commit a value (not a closure): `Update(userID string, mutate func(*User) error)` is the closure form; a `Save(user)` with an optimistic `updated_at`/version check removes the callback entirely. At minimum, document "the callback must not call the store" and add a `-race`-time assertion.
- **Effort:** L

### [Low] `writeJSON` breaks the error envelope on marshal failure
- **Where:** `server/internal/api/respond.go:8-17`
- **Smell:** `json.Marshal` failure falls back to `http.Error`, i.e. `text/plain` with a trailing newline, instead of `{"error":...,"code":"INTERNAL_ERROR"}`, and nothing is logged.
- **Why it matters:** The contract says every API error is the envelope (`docs/contracts.md:47-66`) and `api` is the only place allowed to map errors. Today it is unreachable (all DTOs are plain scalars and providers reject NaN/Inf), so it is a latent contract hole rather than a live bug.
- **Fix:** Marshal into a `bytes.Buffer` first, and on error `h.fail(w, r, domainError(domain.CodeInternalError))` — which requires `writeJSON` to be a method or to take a logger. Add a test with a handler that returns `math.NaN()`.
- **Effort:** S

### [Low] Panic recovery can write a second status line, and the recorder cannot be unwrapped
- **Where:** `server/internal/api/middleware.go:87-109`, `:254-272`
- **Smell:** `recoverMiddleware` always calls `writeJSON(500)` even when the handler already wrote a header and part of the body; `statusRecorder` lacks `Unwrap() http.ResponseWriter`, so `http.ResponseController` (Flush, deadlines) cannot reach the real writer. It also logs `debug.Stack()` as a single attribute on every panic.
- **Why it matters:** A panic after a partial write produces a corrupted response (500 appended to a 200 body) plus a `superfluous WriteHeader` log; the stack attribute escapes newlines into one very large JSON line, which some log pipelines truncate exactly when it matters.
- **Fix:** Track "header sent" on the recorder and skip the body write if so (just close/abort), add `func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }`, and log the stack as a separate line or `slog.Group` of frames.
- **Effort:** S

### [Low] Database directory is world-readable and permissions failures are ignored
- **Where:** `server/internal/store/store.go:79` (`0o755`), `:145-163`
- **Smell:** The data dir is created `0755` while the file is `0600`; `os.Chmod` on an existing file is best-effort with `_ =`; a path that exists but is not a regular file falls through `os.Stat` and only fails later inside SQLite. There is no `PRAGMA integrity_check` or `quick_check` at startup.
- **Why it matters:** File names leak (an account directory listing), a chmod failure is silent, and a truncated database surfaces as a confusing driver error instead of a named startup failure — while `README.md:128` promises that a corrupt database fails startup loudly.
- **Fix:** `MkdirAll(dataDir, 0o700)`, log a warning when chmod fails, `os.Stat` + `Mode().IsRegular()` check in `ensurePrivateFile`, and run `PRAGMA quick_check` in `verifyPragmas`.
- **Effort:** S

### [Low] `DELETE /api/watchlist/{ticker}` echoes unvalidated path text in user copy
- **Where:** `server/internal/api/handlers.go:63-75`, `server/internal/domain/watchlist.go:35-50`
- **Smell:** The ticker is never canonicalised/validated before `Remove`; `Remove` normalises only (trims/uppercases) and any non-member becomes `NOT_IN_WATCHLIST` with that raw text interpolated into `"%s is not in your watchlist."` (`domain/errors.go:134-137`).
- **Why it matters:** `DELETE /api/watchlist/<5 KB of junk>` returns 404 with attacker-controlled text in the message (safe in JSON, but it lands in logs/UI copy and can contain control characters), and the endpoint spends a store transaction before discovering the input was never a ticker.
- **Fix:** Run `domain.Canonicalize` first and return `400 INVALID_TICKER` for malformed paths (consistent with `/api/chart`), then `Remove`; keep `NOT_IN_WATCHLIST` for well-formed non-members. Add a test for a 5000-char path value.
- **Effort:** S

### [Low] Dead code and unused exports in otherwise tight packages
- **Where:** `server/internal/domain/ticker.go:33-40` (`MustTicker`, tests only), `server/internal/domain/watchlist.go:12-16` (`NewWatchlist`, tests only), `server/internal/providers/budget.go:90-97` (`RetryAfter`, tests only), `server/internal/store/store.go:128-130` (`Path`, tests only), `server/internal/domain/quote.go:24` (`ProviderNone`, never produced by any adapter)
- **Smell:** Production packages export helpers that only tests use, plus an enum value no code path emits.
- **Why it matters:** It inflates the public surface (these are `internal/`, but still the package contract), makes `go doc` misleading, and `ProviderNone` invites a future caller to trust a value the API never sends.
- **Fix:** Move `MustTicker`/`NewWatchlist` into a `domain_test` helper file, unexport or delete `RetryAfter`/`Path`, and either emit `ProviderNone` for "unavailable" quotes or delete the constant.
- **Effort:** S

### [Low] Static assets are read whole into memory per request, with no validators
- **Where:** `server/internal/webui/webui.go:98-118`
- **Smell:** Every request does `io.ReadAll(file)` then `w.Write(data)`, sets `Content-Length` by hand and never sets `ETag`/`Last-Modified`, handles `Range`, or answers `If-None-Match`.
- **Why it matters:** The bundle is embedded and immutable, so the allocation is pure waste on every page load, and without a validator a client that ignores `max-age` (or a fresh tab after a deploy with a changed `index.html`) cannot revalidate cheaply.
- **Fix:** Use `http.ServeContent` with an `io.ReadSeeker` (or `io.Copy` + `Content-Length` for hashed assets) and an ETag derived from the build hash for `index.html`; keep the long cache only for `assets/`.
- **Effort:** S

### [Low] Shutdown budget (10 s) is shorter than the HTTP write timeout (60 s)
- **Where:** `server/cmd/dashboard/main.go:23-26`, `:93-100`
- **Smell:** `WriteTimeout` is 60 s and `ReadTimeout` 10 s, but `shutdownTimeout` is 10 s; when `Shutdown` hits its deadline, `run` returns the error and `main` exits with status 1.
- **Why it matters:** A slow `/api/quotes` batch (60 tickers × 2 s worst case, plus budget waits) can turn an orderly restart into a failed unit/docker stop, and the operator sees "shutdown: context deadline exceeded" for what is really "one request was slow".
- **Fix:** Derive `shutdownTimeout` from `WriteTimeout` (or make both configurable and assert `shutdown >= write` at startup), and log the drain timeout at `Warn` rather than returning it as a fatal error.
- **Effort:** S

### [Low] Cookie timestamps use the wall clock instead of the injected clock
- **Where:** `server/internal/api/auth.go:272-283` (`Expires: time.Now().Add(lifetime)` at `:278`)
- **Smell:** `auth.Service` takes an injectable `Now`, but the cookie writer calls `time.Now()` directly; the same code has no `__Host-`/`__Secure-` prefix option.
- **Why it matters:** Cookie expiry and server-side session expiry are computed from different clocks, so a test or an NTP-shifted host can produce a cookie that outlives (or under-lives) its server entry. It also makes the HTTP layer untestable for expiry without sleeping.
- **Fix:** Thread the clock through `Deps` (`Now func() time.Time`) and use it for `Expires`; use the `__Host-` prefix when `CookieSecure` is true, or document why not.
- **Effort:** S

### [Low] Password-hash parsing is lax and permits expensive rows
- **Where:** `server/internal/auth/password.go:110-147` (`fmt.Sscanf` at `:131`, caps at `:112-116`)
- **Smell:** `fmt.Sscanf("m=%d,t=%d,p=%d")` stops after the last verb and ignores trailing garbage, so `m=65536,t=3,p=4,junk` parses. The guard rails still allow 256 MiB × t=6, which is 4× the production cost, per attempt, from a row an operator can edit.
- **Why it matters:** A hand-edited or tampered row can make every login attempt allocate 256 MiB; the parse laxity means malformed hashes are accepted rather than rejected at startup where `validateUser` could catch them.
- **Fix:** Parse the parameter segment with an explicit split on `,` and `=` and reject anything unexpected; set the caps to the production values (or 2×) and validate on load, not just at verify time.
- **Effort:** S

### [Low] Recovery-code lookup leaks the matching index through an early `break`
- **Where:** `server/internal/auth/service.go:464-481` (`break` at `:472`)
- **Smell:** The loop compares every stored hash in constant time but exits at the first match, so the number of comparisons — and therefore roughly which code was used — is observable in timing.
- **Why it matters:** `docs/contracts-auth.md:199` says all code/hash comparisons use `crypto/subtle`; the comparison does, but the control flow reintroduces a (small) side channel that reveals how many codes remain. On a loopback tool this is close to academic, but the contract sentence is absolute.
- **Fix:** Scan the full slice, accumulate the match into an index with a constant-time select, and slice afterwards; or hash the presented code and look it up in a map after a single constant-time compare of the whole set.
- **Effort:** S

### [Low] `AUTH_COOKIE_SECURE` accepts values the error message denies
- **Where:** `server/internal/config/config.go:79-89`
- **Smell:** `strconv.ParseBool` accepts `1`, `t`, `T`, `TRUE`, `True`, etc., while the error says "must be true or false".
- **Why it matters:** Minor, but this is a security flag: an operator who sets `AUTH_COOKIE_SECURE=1` gets `Secure` cookies without having been told that `1` is legal, and the message documents a narrower grammar than the parser.
- **Fix:** Use an explicit switch on `strings.ToLower(raw)` with `"true"`/`"false"`, or change the message to "must be a boolean". Document the accepted values next to the flag in `README.md:155-156`.
- **Effort:** S

### [Low] `chartRange` has an unreachable branch, and nil dependencies panic at request time
- **Where:** `server/internal/api/handlers.go:129-138`, `server/internal/api/router.go:15-22`
- **Smell:** `values, present := r.URL.Query()["range"]` never yields `present == true` with `len(values) == 0` (a present key always has at least one value), so `:134-136` is dead. `NewRouter` accepts a `Deps` with nil `Watchlist`/`Quotes` and only fails when a request arrives.
- **Why it matters:** Dead branches hide the real rule (`?range=` is an empty value, not an absent key) and make the tests' intent ambiguous; a nil-dependency panic is recovered as a 500, which hides a wiring bug as an internal error.
- **Fix:** Delete the `len(values) == 0` branch and let `ParsePriceRange("")` produce `INVALID_RANGE`; validate `Deps` in `NewRouter` and return `(http.Handler, error)` (or panic at construction with a clear message, which is not a request path).
- **Effort:** S

### [Low] Usernames from failed logins are written to logs with only shape validation
- **Where:** `server/internal/api/auth.go:226-242`
- **Smell:** `loggableUsername` accepts any string that matches the username pattern, so an unauthenticated caller can write arbitrary 3–32 character values into the auth event log (subject only to the login rate limiter, which is keyed per username and therefore does not throttle new names).
- **Why it matters:** Log flooding and log-content injection (grep/pager noise, alert rules keyed on `username`) from an unauthenticated endpoint; it is bounded but entirely attacker-chosen.
- **Fix:** Log the submitted username only when it resolves to an account (i.e. after `Login` succeeds or returns `INVALID_CREDENTIALS` for a known name); otherwise log `username_present:false`. Add a global per-IP request budget for auth POSTs.
- **Effort:** S

## Appendix: additional nits

- `server/internal/auth/password.go:1` — package comment still says "the single-owner account"; superseded by D-34.
- `server/internal/providers/service.go:219,228,247,249` — aggregated failures are always attributed to `ProviderFinnhub`, even when only Twelve Data failed; misleading in logs/tests.
- `server/internal/domain/errors.go:93` — `DefaultMessage(CodeRateLimited)` returns "Try again in 1 seconds."; only `NewRateLimited` produces grammatically correct copy.
- `server/internal/store/users.go:151` — `errors.New("store: account not found")` should be a sentinel so `auth` can classify it instead of returning a 500.
- `server/internal/api/handlers.go:57` — store errors are passed through `mapProviderFailure`, which can only ever return them unchanged.
- `server/internal/api/dto.go:115` — chart `source` is hardcoded to `Twelve Data` rather than taken from the series result.
- `server/internal/api/middleware.go:21-25` — the string constant `requestIDHeader` shares a `const` block with the `contextKey` iota values; harmless today but it puts an unrelated type in the key namespace.
- `server/internal/domain/priceseries.go:36-47` — `PointsFor` returns `0` for an unsupported range instead of an error; a future caller would request `outputsize=0`.
- `server/internal/api/api_test.go:691` — reads `store.listCalls` directly instead of the fake's locked accessor (safe today, inconsistent).
- `server/internal/providers/httpclient.go:57-63` — body-size and decode failures collapse into `FailureUnavailable`, so an oversized response is indistinguishable from a network outage.
- `server/internal/auth/service.go:468-474` — the linear scan allocates nothing but is O(n) per recovery attempt; a map keyed by hash would be simpler once the timing issue is fixed.
- `server/internal/store/migrate.go:209-211` — `renameLegacy` is best-effort by design; log at `Warn` when the rename fails so a read-only data dir is visible.
- `server/internal/config/config.go:129-137` — `BIND_HOST` accepts only IP literals, so `localhost` (a name the Host guard happily accepts) is rejected at startup; the asymmetry between the two host rules is surprising.
- `server/internal/auth/recovery.go:66` — a `strings.NewReplacer` is built on every normalisation; hoist it to a package var.
- Test duplication: `testArgon2` appears in `api/auth_test.go:26`, `app/app_test.go:26`, `store/users_test.go:14`; `testClock` in `api/auth_test.go:30` and `auth/service_test.go:17`.

## Hardcoded / portability list

| Value | Where | Why it reduces portability |
| :-- | :-- | :-- |
| Module path `stock-dashboard/server` | `server/go.mod:1` | Not a resolvable VCS/org path; cannot be `go get`-ed, moved, or imported by a sibling module without a rewrite. |
| `127.0.0.1`, `localhost`, `::1` Host allowlist | `server/internal/api/middleware.go:170` | Any other loopback address, hostname, proxy or container alias gets 403; no `ALLOWED_HOSTS` override. |
| `127.0.0.1` + `/healthz` + default `3000` | `server/cmd/healthcheck/main.go:13-18` | Ignores `BIND_HOST`; breaks if the server is bound elsewhere or the health path changes. |
| Default port `3000`, `./data`, loopback host, provider hosts | `server/internal/config/config.go:11-19` | Relative `DATA_DIR` depends on the launch directory; provider hosts and ports are baked in (override exists only for base URLs). |
| 55 / 7 calls per minute | `server/internal/app/app.go:18-19`, `providers/finnhub.go:30`, `providers/twelvedata.go:32` | Duplicated and not env-driven; plan changes require a rebuild. |
| 2 s upstream timeout | `server/internal/app/app.go:20`, `providers/httpclient.go:37`, `providers/finnhub.go:33`, `providers/twelvedata.go:35` | Same, four copies. |
| 60 s cache TTL, 8 workers | `server/internal/app/app.go:21-22`, `providers/service.go:13-14` | Not tunable; worker count ignores available CPUs. |
| 1 min budget window, 2 s max wait | `server/internal/providers/budget.go:11-14` | Contract-visible behaviour, not configurable. |
| 1 MiB upstream body cap | `server/internal/providers/httpclient.go:19` | Fine default, but hardcoded. |
| 4 KiB request body caps | `server/internal/api/handlers.go:13`, `api/auth.go:23` | Duplicated; not configurable. |
| 5 s / 10 s / 60 s / 60 s / 1 MiB / 10 s server limits | `server/cmd/dashboard/main.go:20-27` | No env override; `shutdown` (10 s) is inconsistent with `writeTimeout` (60 s). |
| Session 12 h / idle 30 min, challenge 5 min, enrollment 15 min, 5 attempts, 5 failures/15 min, 60 s block | `server/internal/auth/service.go:17-30` | Product/ops policy frozen in code. |
| TOTP issuer `Stock Dashboard`, 30 s / 6 digits / ±1, 20-byte secret | `server/internal/auth/totp.go:18-26` | The issuer name is user-visible in every authenticator app and is not configurable. |
| Argon2id 64 MiB / t=3 / p=4 / 16 B salt / 32 B key | `server/internal/auth/password.go:32-38` | Cost cannot be raised on faster hardware without a code change; remembering which cost a stored hash used is handled, but new-hash policy is not configurable. |
| 10 recovery codes, 10 random bytes, user-ID prefix `usr_` | `server/internal/auth/recovery.go:13-17`, `auth/service.go:689-696` | Count/entropy/ID prefix are fixed; the ID prefix is persisted forever. |
| Cookie names `sd_session`, `sd_challenge`, `sd_enroll`; paths `/`, `/api/auth` | `server/internal/api/auth.go:15-21` | Collide with any other local app on the same host cookie jar; no `__Host-` prefix. |
| `stock-dashboard.db`, file mode `0600`, dir `0755` | `server/internal/store/store.go:22-28`, `:79` | Data file name and permissions fixed; the dir contradicts the file's privacy. |
| Legacy names `watchlist.json`, `users.json`, `.migrated` | `server/internal/store/migrate.go:17-21` | Migration input names fixed; fine, but undocumented outside README. |
| Asset cache ages `31536000` / `3600` | `server/internal/webui/webui.go:108-112` | Deploy policy baked into the binary. |
| CSP string | `server/internal/api/middleware.go:50-52` | Must be edited and recompiled for any new origin (e.g. a CDN or inline style from a future build). |
| Test credentials `correct horse battery`, `usr_test_owner`, `fin-key`/`td-key`, fixed clock `2026-11-18T03:00:00Z` | `api/auth_test.go:26,28,63`, `api/api_test.go:21`, `app/app_test.go:23,26,77,78`, `auth/service_test.go:15,111`, `store/users_test.go:14,31,41` | Repeated literals; a change to the password policy (12-char minimum) must be mirrored by hand in every file. |
| Fixed test URLs `http://127.0.0.1:3000` (≈20 occurrences) and ports `9001`/`9002` | `api/api_test.go:197,320`, `api/auth_test.go:93,761,770,785,793,798,804,813,824,830`, `config/config_test.go:44-45` | Tests hardcode the production port and host instead of using `httptest` URLs; they also mean the Host guard cannot be exercised for other values without editing each call. |
| Fake payload timestamps `1763303400` and `LastTOTPStep: 1763500000` | `app/app_test.go:51,55`, `store/users_test.go:46` | Magic numbers: the first is a fixed 2025 epoch, so every fake quote is already `stale` (no test asserts freshness), and the second is a Unix-seconds value stored in a 30-second-step column (≈1.76e9 steps, so every code would be rejected as replayed if that fixture ever logged in). |

## Test gaps

- **`cmd/` is untested (0.0% each).** `run()` in `server/cmd/dashboard/main.go` covers config load, listen failure, signal handling, shutdown ordering and the `Serve` error channel; none of it is exercised. `cmd/healthcheck` has no test for a non-200 or an unreachable server.
- **No race-detector run anywhere.** `CGO_ENABLED=0` on this host means `TestWatchlistConcurrentUpdates`, `TestBudgetConcurrentReservations`, `TestServiceSerialisesConcurrentWorkForOneTicker` and `TestSetupAndActivateAreSerialised` have never been checked under `-race`. `AGENTS.md:48-49` acknowledges this; the gap is real for the mutex-heavy store.
- **Store read-error fallbacks are untested.** No test forces `Users.FindByID`/`WatchlistStore.List` to return a query error and asserts the cached answer (or the missing log).
- **Cancellation is untested end to end.** There is no test for a cancelled `Quotes` context producing the documented batch, nor for `authenticate` being called with a request whose context is already done.
- **No test for the fail-open boundary itself.** `TestProtectedEndpointsDenyWithoutSession` enumerates five routes; nothing asserts that every registered `/api` pattern is either guarded or explicitly public.
- **Timing-dependent tests.** `TestServiceBoundedConcurrency` (`providers/service_test.go:529-562`) sleeps 5 ms and asserts `maxInFlight >= 2`, which is load-sensitive; `TestSetupAndActivateAreSerialised` (`auth/service_test.go:355-359`) uses a 100 ms negative window, which can pass vacuously if the second goroutine is slow to start; `TestHTTPClientTimeoutIsTwoSeconds` (`providers/httpclient_test.go:74-95`) sleeps 2100 ms, adding 2.1 s to every run for one assertion.
- **Missing error-path assertions.** No tests for a `POST /api/watchlist` whose body exceeds 4 KiB asserting 413 (it currently asserts 400), for `writeJSON` marshal failure, for `hostGuard` on `127.0.0.2`, or for an upstream `429` from Twelve Data mapping to `429` (the current code returns 502 and nothing notices).
- **Fixtures are duplicated rather than shared.** `testArgon2` in three packages and `testClock` in two, plus three fake stores; a change to `auth.Argon2Params` or the clock contract needs edits in several places.
- **Weak assertions / unrealistic fixtures.** `TestQuotes`' happy-path check compares `map[string]any` elements with `!=` (`api/api_test.go:462-466`), which hides type surprises rather than decoding into `quoteBody`; `TestChart`'s invalid-range loop reads `quotes.seriesCalls` without its mutex (`:622`); `store/users_test.go:46` stores a Unix-seconds value in `LastTOTPStep`, which validates but is ~30× larger than any real 30-second step.
- **No load/shape test for the one-connection pool.** Nothing asserts the number of SQL statements per authenticated request, so the 3-query fan-out documented above would not be caught by a regression.

## Top 5 fixes if only five are done

1. **Invert the auth guard** (`api/middleware.go:226-237`): public allowlist under `/api`, guard everything else per route, and add the reflection test. Removes the one class of bug that no current test can catch.
2. **Shrink `auth.Service.mu`** (`auth/service.go:502-523`): copy the session entry, release the lock, and answer `Authenticate` from the session instead of two SQL reads; move pruning off the request path. This fixes the "one slow query freezes every session" failure and the hot-path query fan-out at the same time.
3. **Meter Argon2 on the public endpoints** (`auth/service.go:212-271`, `:347-373`): a global semaphore plus a global/IP budget, and never hash while holding `setupMu`.
4. **Normalise the provider/transport contract** (`providers/twelvedata.go:143-166`, `providers/httpclient.go:44-70`, `providers/service.go:290-312`): map upstream 429 to `FailureRateLimited`, distinguish cancellation from outage, and add one bounded retry. Right now provider throttling is reported to users as "provider unavailable".
5. **Make the ops surface configurable** (`app/app.go:17-23`, `config/config.go`): export the limits once, add `ALLOWED_HOSTS`, rate-limit/TTL/timeout overrides, and reconcile `shutdownTimeout` with `WriteTimeout`. Without this the app is correct only on the machine it was built for.
