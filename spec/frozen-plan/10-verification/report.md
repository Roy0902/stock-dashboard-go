# Verification Report: stock-watchlist (Phase 10) and accessibility (Phase 11)

Date: 2026-11-17. Scope: final code in `stock-dashboard/`, locked acceptance files as amended by
AMD-001, and follow-up plan `stock-watchlist-followup-01`.
Status: **gates pass, with two blocked scenarios and recorded baseline dispositions.** Nothing is
reported as a clean pass where a check did not run.

## 1. Native gates (final code)

| Check | Command (working directory `stock-dashboard/`) | Result |
| :-- | :-- | :-- |
| Typecheck | `npm run typecheck` | pass, 0 errors |
| Format | `npm run format:check` | pass (scope per D-7 in the design) |
| Lint | `npm run lint` | pass, 0 errors, no disables in project code |
| Build | `npm run build` | pass |
| Supporting unit tests | `npm test` | **61 of 61 pass** (`UT-stock-watchlist-001..061`) |
| Acceptance | `npx playwright test --reporter=list` (`local-env/`) | **62 pass, 2 fail, 64 total** |
| Integrity, original plan | `npm run ai:verify -- stock-watchlist` (`local-env/`) | OK |
| Integrity, follow-up plan | `npm run ai:verify -- stock-watchlist-followup-01` | OK |
| Integrity, all features | `npm run ai:verify` | OK |

### Acceptance failures and dispositions

| Test | Disposition |
| :-- | :-- |
| SCN-047 (original, `tests/api/stock-watchlist.spec.ts`) | **Blocked, by design.** Superseded by FU-001 (`followup-01-001`), which passes. Original kept unchanged, as the approved process requires. |
| SCN-050 (original) | **Blocked, by design.** Superseded by FU-002 (`followup-01-002`), which passes. Original kept unchanged. |

The two failures are the expected red state for the superseded originals. No locked test was
edited outside AMD-001.

## 2. Test identity for this run

| Item | Value |
| :-- | :-- |
| Original plan hash | `7ed3f26e8c4015c81d96d1e35e4b777363759b41d1f9d315afc023ca2d955773` |
| Follow-up plan hash | `df4e2c61874e464b90442dab0ca3a77a981ea239fd381f62bc0e32fe77a2fd1e` |
| Acceptance file hashes | see `local-env/specs/*.freeze.json` (current `tests` map) |
| Final acceptance log | `AppData\Local\Temp\opencode\green-run-5.log` |

## 3. Requirement results

Every requirement has at least one scenario. Results are per scenario, from the final run.

| Requirement | Scenarios | Result |
| :-- | :-- | :-- |
| REQ-001 to REQ-013 (watchlist) | SCN-001 to SCN-013 | pass (SCN-003 pass after AMD-001) |
| REQ-014 to REQ-029 (quotes) | SCN-014 to SCN-031 | pass (SCN-019 and SCN-027 pass after AMD-001; SCN-031 passes after D-9) |
| REQ-030 to REQ-039 (chart) | SCN-034 to SCN-044 | pass |
| REQ-040 to REQ-051 (providers, platform) | SCN-045 to SCN-057 | pass, except SCN-047 and SCN-050 (blocked; superseded by FU-001 and FU-002, which pass) |
| REQ-052 (zero and missing baseline) | SCN-032, SCN-033 | pass |
| REQ-042 and REQ-044 under the approved rules | FU-001, FU-002 | pass |
| REQ-027 under the approved rules | FU-003 | pass |

SCN-053 passes under AMD-001. Its original assertion did not isolate a cause, and the replacement
is a stack-trace pattern. That is recorded in `10-verification/amendments.md`.

## 4. Red-first record

- Original plan: 61 red test cases before implementation, all failing with `src/index.ts` missing
  (`08-red-runs/red-run.md`).
- Follow-up plan: **no red run.** The three tests pass on the first run, because the approved rules
  were already implemented. Substitute evidence: a mutation check (429 rule disabled, FU-002 fails;
  rule restored, all pass). Recorded as a deviation.

## 5. Maintainability review

Checked against the implementation design (`05-task-lists/implementation-design.md`).

- **Ownership:** each module sits where the design places it. Validation is in `domain/ticker.ts`;
  invariants are in `domain/watchlist.ts`, `domain/quote.ts`, `domain/priceSeries.ts`; the only HTTP
  error mapping is in `http/errors.ts`; persistence is in `store/watchlistStore.ts`.
- **Dependencies:** no `src/domain/` file imports `node:*`, `fetch`, `http`, `providers`, `store`,
  `ui`, or `app`. Checked with a search of `src/domain/`.
- **Naming:** domain terms match the glossary (Ticker, Watchlist, Quote, Price Series, Price Range,
  Rate Budget, Fresh, Stale).
- **Secrets:** API keys are read from the environment only. Provider URLs and errors never reach a
  response or log; the provider errors carry only a provider name and a kind.
- **Concern (not blocking):** `src/http/routes.ts` is the largest module, and it holds the route
  table and the request parsing together. It is still cohesive (transport only, no business rules).
  Splitting it is an option for a later iteration, not a defect now.
- **Concern (not blocking):** the browser script and styles are TypeScript strings, not compiled
  files (D-3). This is deliberate, but it means the script is not type-checked as browser code.

## 6. Quality-bypass review

- No `eslint-disable`, `@ts-ignore`, `@ts-expect-error`, `.skip`, `fixme`, or `.only` in project
  code, tests, or fixtures. Checked by search.
- No test is skipped or disabled. All 64 acceptance tests run.
- Lint and format scope excludes only the protected acceptance files, per D-7. Those files are
  checked by the guard, not by Prettier.

## 7. Accessibility check (Phase 11)

Method: a throwaway browser probe ran against the live app and the fake provider, then was deleted.
It checked each contract locator from `04-checklist/shared-checklist.md` section 4. It used
`getByRole`, `getByText`, and `locator` on the stated roles, not CSS paths.

| Contract locator | Count | Copy or state verified |
| :-- | :-- | :-- |
| textbox `Ticker` | 1 | label and placeholder present |
| button `Add ticker` | 1 | |
| listitem `AAPL`, `MSFT` | 1 each | one row per ticker |
| status (`Last updated`) | 1 | text `Last updated HH:MM:SS` |
| price `190.12` in AAPL row | 1 | |
| direction word in AAPL row | 1 | visible `up` / `down` / `unchanged` text |
| provider `Finnhub` in AAPL row | 1 | |
| link `View chart` in AAPL row | 1 | |
| alert, duplicate copy | 1 | `AAPL is already in your watchlist.` |
| link `Back` (chart page) | 1 | |
| group `Range` | 1 | |
| buttons `1M`, `3M`, `1Y` | 1 each | `aria-pressed` set on the selected range |
| image `price chart` | 1 | aria-label names ticker and range (`AAPL price chart, 3M range`) |
| text `Source: Twelve Data` | 1 | |
| heading `AAPL` (chart page) | 1 | |

Result: **every probed contract locator resolves to exactly one element with the stated copy.** The
dashboard has no alert when the watchlist is loaded, which is correct.

Not probed by this check, but covered by acceptance tests that pass: `Missing API key` alert
(SCN-045), `Chart unavailable` (SCN-039), empty-watchlist message (SCN-001), and the Price
unavailable row (SCN-022).

Accessibility gaps noted, not blocking: the add form and the refresh need JavaScript, so without it
the form does nothing. The server renders the initial quotes, so the page still shows data.

## 8. Open items before Phase 12

None blocking. Two scenarios remain red by design (superseded). Deviations and open copy decisions
are in `05-task-lists/implementation-design.md` (D-3 to D-9), with the review requested there.
