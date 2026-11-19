# Architecture

## Shape

A two-tier monorepo: a Go HTTP service (`server/`) that owns all behaviour and persistence,
and a React single-page client (`web/`) that renders the API. There is no separate Node
runtime in production; the built client is embedded in the Go binary.

```
stock-dashboard-go/
  server/
    cmd/dashboard/main.go        process entry: env, signals, listen 127.0.0.1
    internal/config/             environment parsing and validation
    internal/domain/             pure rules: ticker, watchlist, quote, price series, errors
    internal/auth/               password hashing, TOTP, sessions, challenges, rate limits
    internal/providers/          Finnhub + Twelve Data adapters, budgets, fallback/cache
    internal/store/              SQLite persistence (accounts, watchlists, recovery hashes)
    internal/api/                HTTP handlers, routing, error mapping, middleware
    internal/webui/              embedded client build (dist/) and static handler
  web/
    src/api/                     typed HTTP client and wire types
    src/features/auth/           setup, login, TOTP, recovery, session gate
    src/features/watchlist/      add form, rows, empty state
    src/features/quotes/         query hook, polling, quote presentation
    src/features/chart/          range selector, accessible SVG chart
    src/ui/                      shared presentational primitives
    src/lib/                     formatting helpers with no business rules
  e2e/                           Playwright acceptance smoke (fake providers)
  docs/                          contracts, decisions, architecture
```

## Dependency rule

```
cmd -> app -> {api, auth, store, providers, webui}
api -> {auth, domain, providers, store, webui}
store -> auth (the persisted account record)
auth -> domain, and defines the UserStore port that store implements
providers -> domain
store -> domain
webui -> (embedded assets only)
domain -> domain + standard library
```

Forbidden: `domain` importing `net/http`, `os`, `database`, `auth`, `providers` or `api`;
`api` performing provider arithmetic or password/TOTP logic; `auth` importing `api` or
`store`; `web` recomputing derived quote fields; provider keys, passwords, TOTP secrets,
recovery codes or session tokens reaching a log line or response body.

The frontend has one direction of dependency:

```
pages -> features -> api / ui / lib
```

`lib/` contains formatting only. No component owns business arithmetic.

## Behaviour placement

| Concern | Owner |
| :-- | :-- |
| Ticker canonicalisation and format | `domain.Ticker` |
| Membership, duplicates, insertion order | `domain.Watchlist` |
| Change, percent, direction, staleness | `domain.DeriveQuote` |
| Range sizes and point ordering | `domain.PriceSeries` |
| Stable error codes and copy | `domain.Error` |
| Password hashing, TOTP, recovery codes | `auth` |
| Sessions, 2FA challenges, rate limits | `auth` |
| Upstream HTTP and response mapping | `providers` |
| 60 s cache and primary/fallback policy | `providers.Service` |
| Sliding-window budgets | `providers.Budget` |
| Transactional persistence (watchlists, accounts) | `store` |
| HTTP shape, status, cookies, headers | `api` |
| Polling, layout, accessibility, auth screens | `web` |

## Decisions

1. **React SPA over server-rendered HTML.** The workflow allows either; this build uses
   React 19 + Vite with a Go API. The API contract is unchanged, so the acceptance suite can
   exercise HTTP directly.
2. **Custom SVG chart.** No charting dependency; the chart is a presentational React
   component with `role="img"` and an accessible summary. This keeps the bundle small and
   the accessible name under our control.
3. **TanStack Query for fetching.** Polling, cache invalidation after adds, and error
   surfaces are explicit rather than hand-rolled effects.
4. **No CSS framework.** A token-based stylesheet keeps the review surface small and stops
   utility churn in components. Class names describe intent (`quote-change--up`), not style.
5. **Embedded assets with a dev proxy.** `go run ./cmd/dashboard` serves the built SPA from
   `internal/webui/dist`; `npm run dev` proxies `/api` to Go for a fast edit loop.
6. **Strict key policy.** Both keys are required for data endpoints, matching the spec's
   missing-key message, which names both variables. The process still starts and `/healthz`
   stays green.
7. **`INTERNAL_ERROR` for unexpected failures.** The spec defines only domain errors; a
   generic copy prevents accidental leakage of internals.
8. **Multi-user accounts, mandatory TOTP.** Any number of accounts can register from the
   sign-in page. Every account has its own password, authenticator, recovery codes and
   watchlist; there is no public user directory.
9. **PBKDF2 and server-side sessions.** New passwords use PBKDF2-HMAC-SHA256 with 600,000
   iterations (CPU-hard, low memory); legacy Argon2id hashes still verify and are rehashed
   after a successful login. Sessions are random 256-bit tokens stored hashed in memory and
   sent only as `HttpOnly`, `SameSite=Strict` cookies. Restarting the process requires
   signing in again.
10. **SQLite is the only persisted state.** `$DATA_DIR/stock-dashboard.db` holds the
    per-user watchlists, accounts and recovery hashes in transactional tables. Legacy JSON
    files are imported once and renamed `.migrated`. Sessions remain in memory.
11. **CSRF and brute-force defence at the edge.** Same-site cookies plus an Origin check on
    unsafe methods, JSON-only mutations and per-username login throttling.
12. **Copy and paths are centralized.** Every user-visible string lives in
    `web/src/locale/en.ts` behind `copy`; adding a locale means adding a catalog, not editing
    components. HTTP endpoints and the router basename derive from Vite's `BASE_URL`
    (`web/src/api/endpoints.ts`), so a sub-path deployment only changes configuration.

## Quality gates

Commands, expected results and the definition of done live in `docs/quality-gates.md`.
Summary:

| Gate | Command | Scope |
| :-- | :-- | :-- |
| Go format | `gofmt -l .` (must print nothing) | `server/` |
| Go vet | `go vet ./...` | `server/` |
| Go tests | `go test ./... -count=1` (`-race` where cgo is available) | `server/` |
| Web types | `npm run typecheck` | `web/` |
| Web lint | `npm run lint` | `web/` |
| Web format | `npm run format:check` | `web/` |
| Web tests | `npm run test` | `web/` |
| Web build | `npm run build` | `web/` |
| Acceptance smoke | `npm run test` | `e2e/` |
