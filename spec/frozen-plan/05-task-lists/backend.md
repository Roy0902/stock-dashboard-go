# Phase 5 — Backend Task List

Feature: `stock-watchlist`
Design: `implementation-design.md` (proposed, Phase 6 review)
Checklist: `../04-checklist/shared-checklist.md` sections 1, 2, 3, 6

Prepared independently of the frontend task list. The API contract matrix is authoritative.

## 1. Files Studied

| File | Why it matters |
| :-- | :-- |
| `Spec/implementation-spec.md` v1.2 | Status codes, copy, provider mapping, limits |
| `00-ingestion/requirement-register.md` | 52 REQs and their levels |
| `01-domain-model/domain-model.md` | Invariants INV-01 to INV-18 |
| `02-intent-diagrams/v1.md` | D1, D2, D3 guard labels |
| `04-checklist/shared-checklist.md` §3 | The contract to implement |
| `find-potential-stock/local-env/package.json` | `ai:*` script wiring to copy |
| `find-potential-stock/local-env/playwright.config.ts` | Harness config shape |
| `find-potential-stock/local-env/tests/utils/*.ts` | Helper style for fixtures |
| `spec-to-test-workflow/scripts/ai/freeze-guard.mjs` | Guard to copy unchanged |

## 2. Endpoint Baseline Status

| Endpoint | Exists? | Baseline | Notes |
| :-- | :-- | :-- | :-- |
| `GET /api/watchlist` | No | deferred | Contract from checklist §3 |
| `POST /api/watchlist` | No | deferred | Contract from checklist §3 |
| `DELETE /api/watchlist/:ticker` | No | deferred | Contract from checklist §3 |
| `GET /api/quotes` | No | deferred | Contract from checklist §3 |
| `GET /api/chart/:ticker` | No | deferred | Contract from checklist §3 |
| `GET /healthz` | No | deferred | Contract from checklist §3 |

No baseline capture: the project is greenfield and all endpoints are new.

## 3. Task List

| ID | Checklist Ref | Action | Target File | Existing / New | Pattern To Follow | Change Detail | Scenario(s) |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| BE-001 | §7 toolchain; design §6 | Add | `package.json`, `tsconfig.json`, `eslint.config.js`, `.prettierrc.json` | new | `find-potential-stock/local-env/package.json` | Dev deps `typescript`, `eslint`, `typescript-eslint`, `prettier`, `@types/node`; scripts `build`, `typecheck`, `lint`, `format:check`, `test` (`node --test tests-unit/`) | all (build/test prerequisite) |
| BE-002 | §3 code list | Add | `src/domain/errors.ts` | new | design §3 | `DomainError` with the 9 stable codes and optional `retryAfterSeconds`; no HTTP types | 004–009, 041–044 |
| BE-003 | §3 `POST/DELETE` | Add | `src/domain/ticker.ts` | new | design §3 | `toTicker`, `isCanonicalTicker`, `TickerVerdict` | 002–004 |
| BE-004 | §3 `POST/DELETE` | Add | `src/domain/watchlist.ts` | new | design §3 | `addTicker`, `removeTicker` pure transitions; uniqueness; no cap | 005, 006, 008, 009, 013 |
| BE-005 | §3 `GET /api/quotes` | Add | `src/domain/quote.ts` | new | design §3 | `deriveQuote(observation, now)`: change, percent, direction, stale, zero/missing baseline | 014–019, 023, 024, 029, 052 |
| BE-006 | §3 `GET /api/chart/:ticker` | Add | `src/domain/priceSeries.ts` | new | design §3 | `pointsFor(range)`, `orderOldestFirst`, `unavailableSeries` | 031–035, 038 |
| BE-007 | §3 quote providers | Add | `src/providers/finnhub.ts` | new | design §3 | 2 s timeout, `c`-based mapping, `c=0 && t=0` unknown, malformed body failure | 020, 028, 045, 046 |
| BE-008 | §3 fallback + chart | Add | `src/providers/twelvedata.ts` | new | design §3 | quote mapping, `time_series` mapping, `status: error` guard | 021, 032, 039, 046 |
| BE-009 | §3 budgets | Add | `src/providers/rateBudget.ts` | new | design §3 | Sliding window 55 and 7 per minute; `tryAcquire`, `retryAfterSeconds` | 043, 044 |
| BE-010 | §3 fallback policy | Add | `src/providers/fallback.ts` | new | design §3 | Primary then secondary per ticker; 60 s cache; isolation; `none` result | 021, 022, 024, 027 |
| BE-011 | §3 persistence | Add | `src/store/watchlistStore.ts` | new | design §3 | Load or `[]`; atomic write to `$DATA_DIR/watchlist.json`; no other file | 010, 049 |
| BE-012 | §3 routes | Add | `src/http/errors.ts` | new | design §3 | `DomainError` to status/code/`{ error, code }`; 502/429/503 mapping | 004–009, 041–044 |
| BE-013 | §3 routes | Add | `src/http/routes.ts` | new | design §3 | Six routes; JSON parse guard; error envelope; JSON or HTML dispatch | 001, 006, 011, 039–041, 054 |
| BE-014 | §1 and §3 wiring | Add | `src/app.ts` | new | design §3 | Env read, composition, `/healthz`, provider base URLs, missing-key state | 040, 041, 045, 050, 051 |
| BE-015 | §1 and §3 entry | Add | `src/index.ts` | new | design §3 | Bind `127.0.0.1:${PORT ?? 3000}` | 048 |
| BE-016 | §3 UI server side | Add | `src/ui/dashboard.html.ts`, `src/ui/chart.html.ts`, `src/ui/styles.css` | new | design §3 (frontend contract) | Server-rendered markup matching the UI contract; the backend owns serving, the frontend task owns the markup | 001, 012, 014–019, 027, 028, 031, 034–037, 039–041 |
| BE-017 | §7 guard | Add | `local-env/package.json`, `local-env/playwright.config.ts`, `local-env/tsconfig.json`, `local-env/scripts/ai/freeze-guard.mjs`, `.github/skills/` | new | `find-potential-stock/local-env/*` | `ai:freeze`, `ai:lock-tests`, `ai:verify` with `--root .`; `test`, `test:api`, `test:smoke`; base URL `http://127.0.0.1:3100` | all |
| BE-018 | §7 fixtures | Add | `local-env/fixtures/provider.ts`, `local-env/fixtures/app.ts` | new | `local-env/tests/utils/api-failure.ts` style | One fake provider port serving both provider shapes with scripted responses; app launcher with `DATA_DIR` per worker | 014–057 |

## 4. Case Matrix

| Operation | Case | Expected handling (from spec) | Scenario | Status |
| :-- | :-- | :-- | :-- | :-- |
| `POST /api/watchlist` | empty or malformed `ticker` | 400 `INVALID_TICKER`, copy from spec §4.4 | SCN-004 | Mapped |
| `POST /api/watchlist` | lowercase valid | stored uppercase, 201 | SCN-002, SCN-003 | Mapped |
| `POST /api/watchlist` | duplicate | 409 `DUPLICATE_TICKER` | SCN-005 | Mapped |
| `POST /api/watchlist` | unknown symbol | 404 `UNKNOWN_SYMBOL` | SCN-007, SCN-051 | Mapped |
| `POST /api/watchlist` | 50 tickers | all accepted, no cap | SCN-013 | Mapped |
| `POST /api/watchlist` | body is not JSON | 400 `INVALID_TICKER` | none explicit | Candidate |
| `DELETE /api/watchlist/:ticker` | listed | 200 with remainder | SCN-008 | Mapped |
| `DELETE /api/watchlist/:ticker` | not listed | 404 `NOT_IN_WATCHLIST` | SCN-009 | Mapped |
| `GET /api/quotes` | primary ok | provider `Finnhub` | SCN-020 | Mapped |
| `GET /api/quotes` | primary fails, secondary ok | provider `Twelve Data` | SCN-021, SCN-030 | Mapped |
| `GET /api/quotes` | both fail | row `Price unavailable`, omitted from JSON | SCN-022, SCN-027, SCN-052 | Mapped |
| `GET /api/quotes` | cache inside 60 s | no upstream call | SCN-025, SCN-026 | Mapped |
| `GET /api/quotes` | stale observation | timestamp suffix, `stale: true` | SCN-023, SCN-024 | Mapped |
| `GET /api/quotes` | previousClose 0 | price 0, change 0, percent null | SCN-032 | Mapped |
| `GET /api/quotes` | previousClose missing | percent null | SCN-033 | Mapped |
| `GET /api/quotes` | budget exhausted | 429 `RATE_LIMITED` | SCN-050 | Mapped |
| `GET /api/quotes` | no key | 503 `MISSING_API_KEY` | SCN-046 | Mapped |
| `GET /api/quotes` | primary 5xx | 502 only when both fail | SCN-047 | Mapped |
| `GET /api/chart/:ticker` | 1M default | 30 points, source Twelve Data | SCN-035, SCN-043 | Mapped |
| `GET /api/chart/:ticker` | invalid range | 400 `INVALID_RANGE` | SCN-038 | Mapped |
| `GET /api/chart/:ticker` | series unavailable | 502 `CHART_UNAVAILABLE` | SCN-039 | Mapped |
| `GET /api/chart/:ticker` | points out of order | ordered oldest first | SCN-042 | Mapped |
| `GET /healthz` | no key configured | 200 `{status:"ok"}`, no provider call | SCN-057 | Mapped |
| Any route | provider body leaks | no secret in response or log | SCN-053 | Mapped |

## 5. Seed and Data Needs

- `DATA_DIR`: `mkdtemp` per test file; `watchlist.json` written only by the store.
- Fake provider fixtures: per-ticker price, previous close, timestamp, plus failure modes
  (`500`, delay, malformed body, `status: error`, `c=0/t=0`).
- Clock: the app reads `Date.now`; staleness fixtures set timestamps relative to a fixed
  `now` injected through env `FIXED_NOW` (test-only) so SCN-023 and SCN-024 are deterministic.
- No database, no accounts, no seed users.

## 6. Conflicts for Phase 6

- None. Every backend surface in checklist §2 and §3 has a task.

## 7. Coverage Against Checklist

- Checklist backend items covered: 6/6 endpoints, 9/9 error codes, 3/3 budgets.
- Scenarios with a backend task: 52/57 (the 5 UI-only rows are frontend tasks).

## 8. Implementation Design and Validation

| Task | Design responsibility / dependency rule | Naming example | Supporting test / location | Native validation command |
| :-- | :-- | :-- | :-- | :-- |
| BE-002 | `domain/errors.ts`, no dependencies | `DomainError`, `DomainCode` | `tests-unit/errors.test.ts` | `npm run typecheck` in `stock-dashboard/` |
| BE-003 | `domain/ticker.ts`, no dependencies | `toTicker("aapl")` | `tests-unit/ticker.test.ts` | `npm run lint` + `node --test tests-unit/ticker.test.ts` |
| BE-004 | `domain/watchlist.ts`, depends on ticker only | `addTicker(list, "AAPL")` | `tests-unit/watchlist.test.ts` | same |
| BE-005 | `domain/quote.ts`, depends on errors only | `deriveQuote(obs, now)` | `tests-unit/quote.test.ts` | same |
| BE-006 | `domain/priceSeries.ts` | `pointsFor("1M")` | `tests-unit/priceSeries.test.ts` | same |
| BE-007 | `providers/finnhub.ts`, must not import http | `mapFinnhubQuote(json)` | Playwright API via fixture | `npm run test:api` in `local-env/` |
| BE-008 | `providers/twelvedata.ts` | `mapTwelveDataSeries(json)` | Playwright API via fixture | same |
| BE-009 | `providers/rateBudget.ts`, injected clock | `new RateBudget(55, now)` | `tests-unit/rateBudget.test.ts` | `node --test tests-unit/rateBudget.test.ts` |
| BE-010 | `providers/fallback.ts` | `fetchQuoteWithFallback(ticker)` | Playwright API via fixture | `npm run test:api` |
| BE-011 | `store/watchlistStore.ts`, depends on fs only | `saveWatchlist(dir, tickers)` | `tests-unit/watchlistStore.test.ts` | `node --test tests-unit/watchlistStore.test.ts` |
| BE-012 | `http/errors.ts`, depends on domain errors | `toHttpError(err)` | `tests-unit/errors.test.ts` | `node --test tests-unit/errors.test.ts` |
| BE-013 | `http/routes.ts`, transport only | `handleRequest(req, res, deps)` | Playwright API | `npm run test:api` |
| BE-014 | `app.ts`, composition root | `createApp(env)` | Playwright app launcher | `npm run test` |
| BE-015 | `index.ts` | `listenOnLoopback(port)` | Playwright API | `npm run test:api` |
| BE-016 | `ui/*.ts`, markup only | `renderDashboard(model)` | Playwright UI | `npm run test:ui` |
| BE-017 | harness, outside acceptance paths except `specs/` | `ai:freeze`, `ai:verify` | n/a | `npm run ai:verify` |
| BE-018 | harness fixtures | `startFakeProvider()` | Playwright | `npm run test:api` |
