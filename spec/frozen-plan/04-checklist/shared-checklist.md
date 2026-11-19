# Phase 4 — Shared Implementation Checklist

Feature: `stock-watchlist`
Sources: `Spec/implementation-spec.md` v1.2, `02-intent-diagrams/v1.md` (approved),
`03-scenarios/` (57 SCNs), `00-ingestion/requirement-register.md` (rev. 3)

This checklist is the single source for shared contracts. It is not the task list and not the
implementation design; those come in Phase 5.

## 1. Architecture Context

| Item | Observation | Kind |
| :-- | :-- | :-- |
| Repository state | `stock-dashboard/` contains only `Spec/` and the workflow artifacts. No `package.json`, no source, no tests. | Evidence |
| Stack runtime | Node.js `v24.22.0`, npm `11.19.1` on Windows. | Evidence |
| Playwright | Not installed in this project. `npx playwright --version` resolves `1.65.1` by download. | Evidence |
| Playwright helpers | `vasu-playwright-utils@^1.25.0` is used by the sibling `find-potential-stock/local-env` project; not installed here. | Evidence |
| Guard scripts | `spec-to-test-workflow/scripts/ai/freeze-guard.mjs` exists in the workspace. | Evidence |
| Deployment target | Local Windows host, `127.0.0.1`. The submission allows Linux/Docker; this project stays local per the user decision. | Decision |
| Proposed server | TypeScript, a single Node HTTP process, static UI assets served by the same process. | Recommendation |
| Proposed UI | Hand-written TypeScript and CSS, no framework, no chart library (`AMB-...-004`). | Recommendation |
| Test harness location | `local-env/` mirroring the sibling run: `specs/`, `tests/`, `playwright.config.ts`, fake provider fixture. | Recommendation |
| Persistent instructions | No `AGENTS.md` or equivalent exists yet. Phase 4 must create one from `project-conventions` template. | Setup gap |

Patterns available to copy: the sibling run's `local-env/tests/utils/*`, `playwright.config.ts`
shape, and `ai:*` script wiring. No existing stock-dashboard code to follow.

## 2. Scenario-to-Scope Map

Surfaces: `backend` = server, storage, providers, rate limiter; `frontend` = rendered UI.

| Scenario range | User-visible behaviour | REQ | Surface | Level |
| :-- | :-- | :-- | :-- | :-- |
| `SCN-001` | Empty-watchlist message | 001 | both | UI |
| `SCN-002` | Lowercase ticker becomes uppercase | 002 | backend | API |
| `SCN-003` | Canonical-form boundary table | 003 | backend | API |
| `SCN-004` | Malformed ticker refusals | 004 | both | API |
| `SCN-005` | Duplicate refusal | 005 | both | Both |
| `SCN-006` | Add returns full list | 006 | backend | API |
| `SCN-007` | Unknown symbol refusal | 007 | backend | API |
| `SCN-008` | Remove returns remainder | 008 | backend | API |
| `SCN-009` | Remove non-listed refusal | 009 | backend | API |
| `SCN-010` | Membership survives restart | 010 | backend | API |
| `SCN-011` | List response shape | 011 | backend | API |
| `SCN-012` | One row per ticker, Add control | 012 | both | UI |
| `SCN-013` | No maximum size | 013 | backend | API |
| `SCN-014` | Quote row fields | 014 | both | UI |
| `SCN-015` | Change rounding | 015 | backend | Both |
| `SCN-016` | Change-percent derivation table | 016 | backend | Both |
| `SCN-017` | Positive markup | 017 | both | UI |
| `SCN-018` | Negative markup | 018 | both | UI |
| `SCN-019` | Zero change neutral | 019 | both | UI |
| `SCN-020` | Primary provider path | 020 | backend | Both |
| `SCN-021` | Secondary fallback path | 021 | backend | Both |
| `SCN-022` | Both providers fail | 022 | both | Both |
| `SCN-023` | 16-minute quote is stale | 023 | backend | Both |
| `SCN-024` | 14-minute quote is fresh | 023 | backend | Both |
| `SCN-025` | Cache hit inside 60 s | 024 | backend | API |
| `SCN-026` | Cache miss after 60 s | 024 | backend | API |
| `SCN-027` | 60 s UI refresh | 025 | frontend | UI |
| `SCN-028` | Last-updated text | 026 | frontend | UI |
| `SCN-029` | Batch isolation | 027 | both | API |
| `SCN-030` | 2 s timeout is failure | 028 | backend | API |
| `SCN-031` | Direction text without colour | 029 | both | UI |
| `SCN-032` | Zero previous close | 052 | backend | Both |
| `SCN-033` | Missing previous close | 052 | backend | Both |
| `SCN-034` | Chart route from row | 030 | both | Both |
| `SCN-035` | 30 daily closes | 031 | both | UI |
| `SCN-036` | Chart source label | 032 | frontend | UI |
| `SCN-037` | Range selector defaults to 1M | 033 | frontend | UI |
| `SCN-038` | Range validation table | 034 | backend | API |
| `SCN-039` | Chart unavailable | 035 | both | Both |
| `SCN-040` | Back link | 036 | frontend | UI |
| `SCN-041` | Chart text alternative | 037 | frontend | UI |
| `SCN-042` | Point ordering | 038 | backend | Both |
| `SCN-043` | Chart response shape | 039 | backend | API |
| `SCN-044` | Chart for unlisted ticker | 039 | backend | API |
| `SCN-045` | Missing keys message | 040 | both | Both |
| `SCN-046` | 503 without keys | 041 | backend | API |
| `SCN-047` | Upstream failure mapping | 042 | backend | API |
| `SCN-048` | Primary budget 55/min | 043 | backend | API |
| `SCN-049` | Secondary budget 7/min | 043 | backend | API |
| `SCN-050` | 429 when exhausted | 044 | backend | API |
| `SCN-051` | Finnhub zero means unknown | 045 | backend | API |
| `SCN-052` | Unknown shape no crash | 046 | backend | API |
| `SCN-053` | No provider detail leak | 047 | both | Both |
| `SCN-054` | Loopback binding | 048 | backend | API |
| `SCN-055` | Only watchlist persisted | 049 | backend | API |
| `SCN-056` | Base URLs configurable | 050 | backend | API |
| `SCN-057` | Health check | 051 | backend | API |

No scenario lacks a surface.

## 3. API Contract Matrix

All endpoints are new. Baselines are deferred. Error envelope:
`{ "error": "<copy>", "code": "<CODE>" }`.

| Endpoint | Method | Auth | Request Shape | Response Shape | Status Codes | Owner SCN |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| `/api/watchlist` | GET | none (loopback) | — | `{ tickers: string[] }` | 200 | 011 |
| `/api/watchlist` | POST | none | `{ ticker: string }` | `{ tickers: string[] }` | 201, 400, 409, 404 | 003–007 |
| `/api/watchlist/:ticker` | DELETE | none | path param | `{ tickers: string[] }` | 200, 404 | 008, 009 |
| `/api/quotes` | GET | none | — | `{ quotes: Quote[] }` | 200, 429, 502, 503 | 014–033 |
| `/api/chart/:ticker` | GET | none | `range?=1M\|3M\|1Y` | `{ ticker, range, source, points: { date, close }[] }` | 200, 400, 502, 503 | 034–044 |
| `/healthz` | GET | none | — | `{ status: "ok" }` | 200 | 057 |

`Quote` = `{ ticker, price, change, changePercent: number|null, direction: \`up\`|\`down\`|\`unchanged\`, provider: \`Finnhub\`|\`Twelve Data\`|\`none\`, timestamp: ISO-8601 string, stale: boolean }`.

Codes: `INVALID_TICKER`, `DUPLICATE_TICKER`, `UNKNOWN_SYMBOL`, `NOT_IN_WATCHLIST`,
`MISSING_API_KEY`, `PROVIDER_UNAVAILABLE`, `RATE_LIMITED`, `INVALID_RANGE`, `CHART_UNAVAILABLE`.

## 4. UI Element and Copy Contract Matrix

New UI; every element is declared here. Locators use roles, labels and copy only. The test
helper API (`getLocatorByRole`, `getLocatorByLabel`, `getLocatorByPlaceholder`, `getLocatorByText`)
comes from `vasu-playwright-utils`.

| Element | Type | Copy / Name | Locator | Owner SCN |
| :-- | :-- | :-- | :-- | :-- |
| Add form input | textbox | label `Ticker`; placeholder `e.g. AAPL` | `getLocatorByRole('textbox', { name: 'Ticker' })` | 002–007 |
| Add form button | button | `Add ticker` | `getLocatorByRole('button', { name: 'Add ticker' })` | 006 |
| Empty state | alert | `Your watchlist is empty. Add a ticker to begin.` | `getLocatorByText(/Your watchlist is empty/)` | 001 |
| Ticker row | listitem | accessible name `AAPL` | `getLocatorByRole('listitem', { name: 'AAPL' })` | 012, 014 |
| Price cell | text | `190.12` | inside row, `getLocatorByText('190.12')` | 014 |
| Change cell | text | `+1.24` in `#16a34a` | inside row, text and computed colour | 017 |
| Direction word | text | `up` / `down` / `unchanged` | inside row, `getLocatorByText(/^up$/)` | 031 |
| Provider cell | text | `Finnhub` / `Twelve Data` / `none` | inside row | 020–022 |
| Timestamp cell | text | ISO-8601, ` (stale)` suffix | inside row | 023, 024 |
| Unavailable state | text | `Price unavailable` | inside row, `getLocatorByText('Price unavailable')` | 022 |
| Validation message | alert | `Enter a valid ticker, e.g. AAPL.` | `getLocatorByRole('alert')` | 004 |
| Duplicate message | alert | `AAPL is already in your watchlist.` | `getLocatorByRole('alert')` | 005 |
| Unknown message | alert | `Symbol not found.` | `getLocatorByRole('alert')` | 007 |
| Missing-key message | alert | `Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.` | `getLocatorByRole('alert')` | 045 |
| Last updated | status | `Last updated HH:MM:SS` | `getLocatorByRole('status')` | 028 |
| Chart link | link | `View chart` | `getLocatorByRole('link', { name: 'View chart' })` in the row | 034 |
| Range group | group | label `Range` | `getLocatorByRole('group', { name: 'Range' })` | 037 |
| Range buttons | button | `1M`, `3M`, `1Y` | `getLocatorByRole('button', { name: '1M' })` | 037, 038 |
| Chart | image | aria-label `AAPL price chart, 1M range` | `getLocatorByRole('img', { name: /price chart/ })` | 041 |
| Chart source | text | `Source: Twelve Data` | `getLocatorByText('Source: Twelve Data')` | 036 |
| Chart unavailable | alert | `Chart unavailable` | `getLocatorByText('Chart unavailable')` | 039 |
| Back link | link | `Back` | `getLocatorByRole('link', { name: 'Back' })` | 040 |

Banned locators: DOM-path CSS, XPath, styling classes, `nth-child` chains.

## 5. Discovery Notes

No existing screens. `playwright-cli` discovery is N/A because the UI does not exist yet; all
locators above are declared contracts, not observations.

| Screen | Element | Observed | Recorded in |
| :-- | :-- | :-- | :-- |
| `/` | — | not built | n/a |
| `/chart/:ticker` | — | not built | n/a |

## 6. Coverage Inputs for the Test Plan

Task-list references are blank until Phase 5. Any blank after Phase 5 is flagged for Phase 6.

| Scenario range | REQ | Level | Surface | FE task | BE task |
| :-- | :-- | :-- | :-- | :-- | :-- |
| 001 | 001 | UI | both | TBD | TBD |
| 002–013 | 002–013 | API | backend | TBD | TBD |
| 014–031 | 014–029 | Both | both | TBD | TBD |
| 032–033 | 052 | Both | backend | TBD | TBD |
| 034–044 | 030–039 | Both | both | TBD | TBD |
| 045–057 | 040–051 | API | backend | TBD | TBD |

## 7. Native Quality Baseline and Setup Gaps

The project has no toolchain yet. Commands below are the intended ones; none can run before the
harness exists. No result is reported as a pass.

| Check | Intended native command | Scope | Baseline | Disposition |
| :-- | :-- | :-- | :-- | :-- |
| Format | `npx prettier --check .` | `src/`, `tests/` | Not installed | Setup gap: add Prettier and a config in Phase 5 |
| Lint / static analysis | `npx eslint .` | `src/`, `tests/` | Not installed | Setup gap: add ESLint with `@typescript-eslint` in Phase 5 |
| Type check | `npx tsc --noEmit` | `tsconfig.json` | Not installed | Setup gap: add TypeScript and `tsconfig` in Phase 5 |
| Build | `npm run build` | `src/` | Not installed | Setup gap: add the build script in Phase 5 |
| Acceptance tests | `npm run test` (`playwright test`) | `local-env/` | Playwright 1.65.1 available by download only | Setup gap: install `@playwright/test` and `vasu-playwright-utils` |
| Guard: freeze | `npm run ai:freeze -- stock-watchlist` | `local-env/` | Guard script available in the workspace, not wired | Setup gap: copy `scripts/ai` and add `ai:*` scripts |
| Guard: lock | `npm run ai:lock-tests -- stock-watchlist` | `local-env/` | same | same |
| Guard: verify | `npm run ai:verify` | `local-env/` | same | same |

## 8. Re-check: Twelve Data Free Scope (AMB-stock-watchlist-008)

| Question | Verified value | Source | Disposition |
| :-- | :-- | :-- | :-- |
| Does Basic (free) include historical time series? | Yes, `time_series` is on the free plan. | twelvedata.com docs, `/stocks` page | Chart stays in scope. |
| Free quota | 8 credits/minute, 800/day. | twelvedata.com/pricing, support §Trial | Matches REQ-043 budget. |
| US market history depth | 2 years for US markets. | third-party comparison citing Twelve Data pricing | 30/90/365 points fit well inside 2 years. |
| Usage restriction | Free plan is internal use only. | same comparison | Matches "local only" scope. |
| Real-time data on free? | Real-time for all US markets is advertised. | support §Trial | Not required; UI labels the source, not the delay. |

AMB-008 is closable as **Verified** for this scope. If the provider changes terms after this
date, the chart is dropped under the "free only" rule.

## 9. Decisions needed before Phase 5

1. Approve the setup gaps in section 7 (Prettier, ESLint, TypeScript, Playwright install,
   guard wiring) as the project toolchain.
2. Approve `local-env/` as the harness location.
3. Approve the direction word as visible row text (`REQ-014` update in the spec).
4. Approve AMB-011 (insertion order) and AMB-012 (chart for unlisted ticker).
