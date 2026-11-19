# Implementation Design: stock-watchlist

Implementation context only. Expected behaviour comes from the approved requirements and
intent diagram v1, then the frozen acceptance plan. This document never creates a rule.

## 1. Sources and decisions

- Requirements/scenarios: `00-ingestion/requirement-register.md` (`9f209cb29f63358d`),
  `03-scenarios/` (57 SCNs)
- Domain vocabulary and intent diagram: `01-domain-model/domain-model.md`
  (`931f233b964f4305`), `02-intent-diagrams/v1.md` (`694adf32398e209c`, approved 2026-11-16)
- Shared checklist: `04-checklist/shared-checklist.md` (`f403aeb0379421ef`)
- Applicable project instructions: none yet; `AGENTS.md` is created from
  `project-conventions` in this phase (proposed, Phase 6 review)
- Status: **proposed**; reviewer: user (Phase 6)
- Product ambiguities: AMB-004 (chart is custom SVG, engineering decision); AMB-008
  verified; AMB-011, AMB-012 closed by user 2026-11-16. No open product ambiguity.

## 2. Files and patterns studied

| File / scope | Evidence | Reuse or departure, with reason |
| :-- | :-- | :-- |
| `stock-dashboard/` | `Spec/` plus workflow artifacts only | Greenfield. No pattern to copy. |
| `find-potential-stock/local-env/playwright.config.ts` | Playwright config shape with projects and base URL | Reuse the shape for `local-env/playwright.config.ts`. |
| `find-potential-stock/local-env/tests/utils/*.ts` | Test helper style (`db.ts`, `api-failure.ts`) | Reuse the helper style for `fixtures/provider.ts` and `app.ts`; no database helper needed. |
| `find-potential-stock/local-env/package.json` | `ai:*` scripts, Playwright and `vasu-playwright-utils` wiring | Reuse the script wiring exactly. |
| `spec-to-test-workflow/scripts/ai/freeze-guard.mjs` | Guard CLI with `freeze`, `lock-tests`, `verify` | Copy unchanged into `scripts/ai/`. |
| `spec-to-test-workflow/.github/skills/*/SKILL.md` | Phase skills | Copy unchanged into the project. |
| Node 24.22 | Native TypeScript type stripping | Unit tests run `.test.ts` with `node --test`, no `tsx` dependency. |
| Node `node:http` | Built-in server | No web framework dependency; the API is six routes. |

## 3. Proposed structure and ownership

```
stock-dashboard/
  AGENTS.md                        persistent instructions (proposed)
  package.json                     scripts + dev dependencies
  tsconfig.json                    ES2022, NodeNext, strict
  eslint.config.js                 typescript-eslint flat config
  .prettierrc.json
  src/
    index.ts                       process entry: reads env, builds app, listens
    app.ts                         composition root: wires providers, store, routes
    http/
      routes.ts                    URL to handler map; JSON vs HTML dispatch
      errors.ts                    DomainError to status/code/copy mapping
      render.ts                    server-rendered HTML for "/" and "/chart/:ticker"
    domain/
      ticker.ts                    Ticker value object: normalise, validate
      watchlist.ts                 membership rules (uniqueness, no cap)
      quote.ts                     Quote derivation, staleness, direction
      priceSeries.ts               range size, order, unavailable
      errors.ts                    DomainError with stable code, no transport detail
    providers/
      types.ts                     QuoteSource / SeriesSource ports
      finnhub.ts                   primary quote adapter and mapping
      twelvedata.ts                fallback quote + series adapter
      rateBudget.ts                per-provider sliding-window limiter
      fallback.ts                  primary then secondary orchestration
    store/
      watchlistStore.ts            JSON read/write under DATA_DIR, atomic replace
    ui/
      dashboard.html.ts            dashboard markup
      chart.html.ts                chart markup
      client.ts                    browser code compiled for the 60 s refresh
  tests-unit/
    ticker.test.ts
    quote.test.ts
    rateBudget.test.ts
    watchlistStore.test.ts
    errors.test.ts
  local-env/
    package.json  playwright.config.ts  tsconfig.json
    fixtures/provider.ts           fake Finnhub + Twelve Data on one port
    fixtures/app.ts                starts the built app on an ephemeral port
    tests/specs/stock-watchlist.spec.ts
    tests/api/stock-watchlist.spec.ts
    specs/                         frozen plans and .freeze.json
    scripts/ai/freeze-guard.mjs
  .github/skills/                  copied phase skills
```

Dependency direction, strictly one way:

```
src/index.ts -> src/app.ts -> { http/*, providers/*, store/*, ui/* }
http/*, providers/*, store/*, ui/* -> domain/* and other src/* modules
domain/* -> domain/* only   (no http, no fs, no fetch, no DOM)
```

| Path / module | Existing / new | Responsibility and owner | Dependencies / allowed direction | Rationale |
| :-- | :-- | :-- | :-- | :-- |
| `src/domain/ticker.ts` | new | `Ticker` canonicalisation and the `^[A-Z][A-Z0-9]{0,9}$` rule (REQ-002, -003, -004) | none | One rule, one file; both the store and the provider path use it. |
| `src/domain/watchlist.ts` | new | `addTicker`, `removeTicker` pure transitions with uniqueness and no cap (REQ-005, -006, -008, -009, -013) | `domain/ticker` | Membership rules never touch storage or HTTP. |
| `src/domain/quote.ts` | new | `deriveQuote` (change, percent, direction, stale, zero baseline) (REQ-014 to -019, -023, -024, -029, -052) | `domain/errors` | The arithmetic and staleness guards are the invariants; they must be unit-testable without a server. |
| `src/domain/priceSeries.ts` | new | range to point count, oldest-to-newest ordering, unavailable result (REQ-031 to -035, -038) | `domain/errors` | Ordering is a domain rule, not a rendering rule. |
| `src/domain/errors.ts` | new | `DomainError` with a stable code; the only error vocabulary the layers share | none | Keeps transport mapping out of the rules. |
| `src/providers/types.ts` | new | ports `QuoteSource`, `SeriesSource`, `QuoteResult`, `ProviderFailure` | `domain/quote` | Adapters stay swappable and the fallback is testable. |
| `src/providers/finnhub.ts` | new | HTTP call, 2 s timeout, response mapping, `c=0/t=0` unknown (REQ-020, -028, -045) | `providers/types`, `domain/errors` | One adapter, one upstream shape. |
| `src/providers/twelvedata.ts` | new | quote and `time_series` mapping, malformed body guard (REQ-021, -032, -039, -046) | `providers/types`, `domain/*` | Owns the only chart source. |
| `src/providers/rateBudget.ts` | new | sliding window, 55 and 7 per minute, refusal (REQ-043, -044) | `domain/errors` | Shared by both adapters; single place for the budget rule. |
| `src/providers/fallback.ts` | new | primary then secondary, per-ticker isolation, unavailable result (REQ-021, -022, -027) | `providers/types, rateBudget` | Ordering and fallback are provider policy, not transport. |
| `src/store/watchlistStore.ts` | new | `$DATA_DIR/watchlist.json` load/save, atomic replace, only persisted data (REQ-010, -049) | `domain/watchlist` | Persistence boundary kept apart from membership rules. |
| `src/http/routes.ts` | new | six routes, request parsing, dispatch to domain/providers (REQ-006, -011, -039 to -041) | `domain/*`, `providers/*`, `store/*`, `http/errors`, `http/render` | Transport only; no business rule. |
| `src/http/errors.ts` | new | `DomainError` to status, code, copy (REQ-004 to -009, -041 to -044) | `domain/errors` | The single mapping table; tests assert it once. |
| `src/http/render.ts` | new | HTML for dashboard and chart, escaping, no secrets (REQ-047) | `http/render`, `domain/quote` read-only | Server-side rendering keeps the UI a pure consumer of the API contract. |
| `src/ui/client.ts` | new | 60 s refresh, `Last updated`, no business rule (REQ-025, -026) | fetched JSON only | Only browser code; no duplicate arithmetic. |
| `src/app.ts` | new | composition root: env, wiring, healthz (REQ-040, -048, -050, -051) | all of `src/*` | One place where configuration meets construction. |
| `src/index.ts` | new | bind `127.0.0.1:PORT` (REQ-048) | `app.ts` | Process concerns separated from wiring. |

Ownership summary:

- **Validation:** format in `domain/ticker.ts`; request shape in `http/routes.ts`.
- **Invariants:** membership in `domain/watchlist.ts`; arithmetic and staleness in
  `domain/quote.ts`; ordering in `domain/priceSeries.ts`.
- **Errors:** raised as `DomainError` in the domain, mapped once in `http/errors.ts`.
- **Lifecycle state:** quote fresh/stale/unavailable computed in `domain/quote.ts`; the
  cache itself lives in `providers/fallback.ts`.
- **Side effects:** HTTP calls in adapters, disk writes in the store, the refresh timer in
  `ui/client.ts`. None inside `domain/`.

## 4. Naming examples

| Domain term | Path / type / function | Convention and meaning |
| :-- | :-- | :-- |
| Ticker | `Ticker`, `toTicker(raw): Ticker`, `isCanonicalTicker(value)` | Value object, uppercase canonical form; verbs describe the transition. |
| Watchlist | `Watchlist`, `addTicker(watchlist, ticker)`, `removeTicker(watchlist, ticker)` | Pure functions returning a new list; no mutation. |
| Verdict | `TickerVerdict` union `"accepted" \| "duplicate" \| "invalid" \| "unknownSymbol"` | Mirrors the domain glossary, not HTTP. |
| Quote | `Quote`, `deriveQuote(observation, now)` | Derivation takes a clock, so staleness is deterministic. |
| Provider attribution | `ProviderAttribution`, `"Finnhub" \| "Twelve Data" \| "none"` | The exact strings the UI contract names. |
| Rate budget | `RateBudget`, `tryAcquire(): boolean`, `retryAfterSeconds(): number` | One method per question the caller asks. |
| Failure | `DomainError`, `code`, `copy`, `retryAfterSeconds?` | Codes match the checklist's 9 names. |
| Range | `PriceRange`, `pointsFor(range)`, `orderOldestFirst(points)` | Range size is domain knowledge. |

## 5. Supporting-test strategy

Supporting tests live in `tests-unit/` and run with `node --test`, outside the protected
acceptance paths. They never replace the frozen acceptance coverage.

| Responsibility / boundary | Level | Runner and location | Data/reset/isolation | Related REQ / SCN |
| :-- | :-- | :-- | :-- | :-- |
| Ticker canonicalisation and format table | unit | `node --test tests-unit/ticker.test.ts` | table driven, no I/O | REQ-002 to -004 / SCN-002 to -004 |
| Quote derivation, rounding, direction, stale, zero baseline | unit | `node --test tests-unit/quote.test.ts` | fixed `now` values | REQ-015 to -019, -023, -052 / SCN-015 to -019, -023, -024, -032, -033 |
| Rate budget window | unit | `node --test tests-unit/rateBudget.test.ts` | injected clock, no timers | REQ-043, -044 / SCN-048 to -050 |
| Store atomic write and round-trip | unit | `node --test tests-unit/watchlistStore.test.ts` | `mkdtemp` per test | REQ-010, -049 / SCN-010, -055 |
| Domain error to status mapping | unit | `node --test tests-unit/errors.test.ts` | pure table | REQ-004 to -009, -041 to -044 |
| Adapter mapping against a fake upstream | integration | Playwright API tests with `fixtures/provider.ts` | fixture server per test file | REQ-020, -021, -028, -045, -046 / SCN-020, -021, -030, -051, -052 |
| UI states and refresh | acceptance | `local-env/tests/specs` | fixture app per worker | REQ-001, -012, -014 to -019, -025, -026, -029, -036, -037, -041 / SCN-001, -012, -014 to -019, -027, -028, -031, -036, -037 |

## 6. Quality baseline and required gates

No check can run yet: the project has no toolchain. Phase 4 approved the setup. Each row
records the intended command and the required final result.

| Check | Exact command | Working directory / scope | Baseline result and evidence | Required final result / disposition |
| :-- | :-- | :-- | :-- | :-- |
| Format | `npm run format:check` (`prettier --check .`) | `stock-dashboard/` | Not installed | Zero unformatted files |
| Lint | `npm run lint` (`eslint .`) | `stock-dashboard/` | Not installed | Zero errors; no blanket disables |
| Type check | `npm run typecheck` (`tsc --noEmit`) | `stock-dashboard/` | Not installed | Zero errors |
| Build | `npm run build` (`tsc`) | `src/` to `dist/` | Not installed | Build succeeds |
| Unit tests | `npm test` (`node --test tests-unit/`) | `stock-dashboard/` | Not installed | All pass |
| Acceptance | `npm run test` (`playwright test`) | `local-env/` | Playwright 1.65.1 download-only | Red before Phase 9, green at Phase 10 |
| Freeze guard | `npm run ai:freeze -- stock-watchlist` | `local-env/` | Guard available, not wired | Succeeds at Phase 7 |
| Test lock | `npm run ai:lock-tests -- stock-watchlist` | `local-env/` | same | Succeeds at Phase 8 |
| Guard verify | `npm run ai:verify` | `local-env/` | same | Succeeds at Phase 10 and in CI |

## 7. Review and deviations

- Design review: Phase 6, user — responsibilities, dependency direction, names, test
  strategy, quality readiness.
- Baseline/setup decisions: approved 2026-11-16 (Prettier, ESLint, TypeScript, Playwright,
  guard wiring, `local-env/` harness).
- Material deviations: none yet. Any later deviation is recorded here and re-reviewed.
- Deviation D-1 (2026-11-16, toolchain): unit tests use a TypeScript `enum` (`tests-unit/catalogue.ts`,
  the user-requested enumeration). Node's default type stripping rejects enums, so the runner is
  `node --experimental-transform-types --test tests-unit/`, not the plain `node --test` from §5.
  Still no `tsx` dependency. Re-review requested with Phase 9.
- Deviation D-2 (2026-11-16, harness): the acceptance fixture starts the real app through
  `src/index.ts` for each test (`local-env/tests/utils/app.ts`). The design had planned a shared
  `fixtures/app.ts`; per-test apps give isolated DATA_DIR and ports without shared state.
- Deviation D-3 (2026-11-17, UI): `src/http/render.ts` is not used. Pages are rendered by
  `src/ui/dashboard.html.ts` and `src/ui/chart.html.ts`. Styles (`src/ui/styles.ts`) and the browser
  script (`src/ui/client.ts`) are exported TypeScript strings, not `.css` and compiled `.ts`. The
  app runs from source with type stripping, so there is no build step for the browser script.
- Deviation D-4 (2026-11-17, add flow): a new ticker is verified in the order format, duplicate,
  provider. Finnhub `c=0,t=0` refuses at once (SCN-051, GAP-003). If both providers fail without
  an unknown answer, the add returns `502 PROVIDER_UNAVAILABLE`. With no keys it returns
  `503 MISSING_API_KEY`. The spec lists only 400, 409 and 404 for POST, so these follow §4.4/§5.
- Deviation D-5 (2026-11-17, rate limits): a request returns `429 RATE_LIMITED` only when every
  ticker it needed was refused by a local budget and none was served. Otherwise it returns the
  served tickers (REQ-027). Copy is `Too many requests. Try again in N seconds.`
- Deviation D-6 (2026-11-17, copy): the spec gives no copy for `INVALID_RANGE` or
  `NOT_IN_WATCHLIST`, and no code for unexpected errors. Those strings are engineering copy:
  `Range must be 1M, 3M or 1Y.`, `<TICKER> is not in your watchlist.`, and `INTERNAL_ERROR` with
  `Something went wrong. Try again shortly.` They need review.
- Deviation D-7 (2026-11-17, quality scope): Prettier and ESLint cover `src/`, `tests-unit/`,
  `local-env/fixtures/`, and the unprotected `local-env/tests/utils` and `tests/fixtures`. The
  protected acceptance files (`tests/specs/`, `tests/api/`) are excluded so the lock is not altered
  by formatting.
- Decision D-10 (2026-11-17, confirmed by user 2026-11-17): a failed fresh fetch for a ticker
  drops that ticker's cached observation. Without this, the browser's refresh right after a
  failed page view read the cache and showed the old quote again (follow-up 02, scenario 003). It
  is a consequence of DR-03 (page views bypass the cache) and DR-01 (last-known quote only while
  fresh). It is not a new product rule, but it is a behaviour the user should confirm.
- Decision D-9 (2026-11-17, user): the cache serves refreshes, not page views. A dashboard page
  view fetches current quotes (`quotesFor(..., { fresh: true })`). `/api/quotes` refreshes keep the
  60-second cache (REQ-024, SCN-025, SCN-026). This resolves SCN-031, which now passes unchanged.
- Decision D-8 (2026-11-17, user): the dashboard has no remove control. Watchlist removal is
  available only through `DELETE /api/watchlist/:ticker`, which stays in the API contract (REQ-008,
  REQ-009). No UI scenario depends on a remove control, so none is added.
- Out of scope: accounts, databases, trading, alerts, intraday data, Docker, CI hosting.
