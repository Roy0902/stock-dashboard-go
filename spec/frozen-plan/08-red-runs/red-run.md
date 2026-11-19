# Execution Evidence: stock-watchlist

Mutable execution record outside the frozen plan. Preserve chronological entries;
append corrections/results rather than overwriting red history. This is reviewed
evidence, not a checksum-protected artifact or mechanically verified proof.
Never include secrets or raw sensitive payloads.

## Identity

- Frozen plan: `local-env/specs/stock-watchlist.md`, sha256 `7ed3f26e8c4015c81d96d1e35e4b777363759b41d1f9d315afc023ca2d955773`
- Feature manifest: `local-env/specs/stock-watchlist.freeze.json` (`tests: null`, not yet locked)
- Acceptance files (red run 1):
  - `local-env/tests/specs/stock-watchlist.spec.ts` sha256 prefix `e834f57b4becaa2a` (22 UI tests)
  - `local-env/tests/api/stock-watchlist.spec.ts` sha256 prefix `ccd164c40f133135` (35 API tests across 3 describe blocks; SCN-016 expands to 3 examples, so 39 test cases)
- Harness: `local-env/fixtures/provider.mjs` `b90735b0f2acf600`, `local-env/tests/utils/app.ts` `bf963264ecb38b43`, `local-env/tests/fixtures/setup.ts` `996c506b65281041`
- Reviewed design: `05-task-lists/implementation-design.md` (`530e71f85262e6dc`)
- Existing historical evidence: none

## Red acceptance runs (Phase 8)

Command: `npx playwright test --reporter=list` in `stock-dashboard/local-env/`.
Full log: `C:\Users\pigcr\AppData\Local\Temp\opencode\red-run-1.log`.

Result: **61 test cases, 61 failed, 0 passed.** Every failure is the same feature-missing error:

```
Error: Cannot find module '...\stock-dashboard\src\index.ts'
Error: application exited before becoming ready
```

Every acceptance test starts the real app via `apps.start()`. Because `src/` does not exist yet,
the process exits and the harness reports it. No failure is caused by a locator, auth, route,
timing, or fixture defect. No route mocks are used.

| Run / timestamp | REQ / SCN / test identity | Command and working directory | Artifact identity | Failure and feature-missing reason | Log / reviewer |
| :-- | :-- | :-- | :-- | :-- | :-- |
| red-1 / 2026-11-16 | SCN-001 ... SCN-057, all 57 IDs (UI and API files) | `npx playwright test --reporter=list`, `stock-dashboard/local-env/` | see Identity above | feature missing: `src/index.ts` absent | red-run-1.log / user review pending |

Per-scenario mapping (SCN to file and test title) is the test title suffix
`[REQ-stock-watchlist-NNN][SCN-stock-watchlist-NNN]`. A static check confirmed each of the 57 IDs
appears in at least one protected file.

## Supporting unit tests (red, before development)

Command: `node --experimental-transform-types --test "tests-unit/*.test.ts"` in `stock-dashboard/`.
Result: all 7 files fail to load with `ERR_MODULE_NOT_FOUND` for their `src/` module. The
`UnitTestId` enum import resolves in each file, so the failure is feature-missing only.
61 catalogue IDs (`UT-stock-watchlist-001` to `061`) are authored; none is passing.

## Subsequent verification

Phase 9 implementation runs, 2026-11-17. Working directory `stock-dashboard/`, then `local-env/`.

| Run / timestamp | Check / scope | Command | Result | Evidence / disposition |
| :-- | :-- | :-- | :-- | :-- |
| 2026-11-17 | Typecheck | `npm run typecheck` | pass | 0 errors |
| 2026-11-17 | Format | `npm run format:check` | pass | scoped per D-7 |
| 2026-11-17 | Lint | `npm run lint` | pass | 0 errors, no disables |
| 2026-11-17 | Build | `npm run build` | pass | `dist/` |
| 2026-11-17 | Supporting unit tests | `npm test` | pass 61/61 | `UT-stock-watchlist-001..061` |
| 2026-11-17 | Acceptance (run 2) | `npx playwright test --reporter=list` in `local-env/` | 54 pass, 7 fail | `green-run-2.log` (AppData temp) |
| 2026-11-17 | Integrity | `npm run ai:verify -- stock-watchlist` | OK | locked hashes unchanged |

Acceptance failures, by disposition:

- **Blocked by user (expected red):** SCN-047 (`GAP-001`), SCN-050 (`GAP-002`).
- **Test defect in a locked file (not fixed; needs a reviewed follow-up):**
  - SCN-003 (`api`): the four outline examples share one app. `aapl` after `AAPL` gets `409`.
    Each example needs a fresh state.
  - SCN-019 (`specs`): `getByText('0.00')` matches three elements (`100.00`, `0.00`, `0.00%`).
    The locator needs an exact match.
  - SCN-031 (`specs`): the test changes the provider price and reloads inside 60 seconds. REQ-024
    serves the cache, so the row stays `up`. The expectation conflicts with REQ-024.
  - SCN-027 (`specs`): `page.clock.fastForward('60s')` is not a valid Playwright argument.
    It should be `60_000`.
  - SCN-053 (`specs`): `not.toContainText('at ')` is an over-broad substring check. I could not
    pin the matched fragment from the output. The received text repeats the dashboard twice,
    which is also unexplained. In this run the `no "SECRET-XYZ"` step passed, the stack-trace
    step failed, and the log step did not run. Requires review before any conclusion.

Locked files were not edited. Each defect above needs a reviewed follow-up.

Later runs, 2026-11-17, after D-9 and AMD-001 (`10-verification/amendments.md`):

| Run | Result | Notes |
| :-- | :-- | :-- |
| green-run-3 | 55 pass, 6 fail | SCN-031 now passes (D-9) |
| green-run-4 | 59 pass, 2 fail | Failing: SCN-047 and SCN-050, both blocked (`GAP-001`, `GAP-002`) |

`ai:verify -- stock-watchlist` reports OK after re-locking. Plan hash unchanged.

### Follow-up plan `stock-watchlist-followup-01` (2026-11-17)

- Plan frozen (`planSha256` `df4e2c61874e464b90442dab0ca3a77a981ea239fd381f62bc0e32fe77a2fd1e`).
  The first freeze was discarded before any lock, because its text named the superseded IDs. The
  guard counts those IDs. The approved scenario content did not change.
- Tests locked: `tests/api/stock-watchlist-followup-01.spec.ts` (3 tests).
- **Deviation (red-first not met):** the three follow-up tests passed on the first run. They
  describe behaviour the current build already has, so no feature-missing red was possible.
- Substitute evidence: mutation check. With the 429 rule disabled (`allRefused = false`),
  `Nothing priced after budgets are exhausted` failed. With the rule restored, all 3 pass.
- Full run 5: **62 pass, 2 fail.** The 2 failures are the original SCN-047 and SCN-050, still
  red by design.

### Follow-up plan `stock-watchlist-followup-02` (2026-11-17)

- Plan frozen (`planSha256` `692b27f01d35f5f26a88b3fb5f1cbc936b35648fa4e86ec7843a157ee960e428`).
- Tests written and run red before implementation: 2 fail, 5 pass.
  - 02-003 fails: the build showed the 15.5-minute-old quote (feature missing: stale last-known rule).
  - 02-006 fails: the build showed "Chart unavailable" (feature missing: invalid-range rule).
  - The other five passed, as the build already had those behaviours.
- Implementation: stale last-known rule (`isStale` in `domain/quote.ts`, applied in the fallback),
  and the 1M chart for an invalid range. One more change was needed after the first green run,
  recorded as D-10: a failed fresh fetch drops the cached observation. Without it, 02-003 failed.
- Tests locked: `tests/specs/stock-watchlist-followup-02.spec.ts` and
  `tests/api/stock-watchlist-followup-02.spec.ts`.
- Full run 6: **69 pass, 2 fail, 71 total.** The 2 failures are the original SCN-047 and SCN-050,
  still red by design.
- `ai:verify` reports OK for all three plans.

## Subsequent verification (earlier note)

No verification runs yet (Phase 10).

## Known plan questions found during red authoring (not resolved, not edited)

These came up while writing expectations literally from the frozen plan. They are not
authoring defects. They are recorded in `10-verification/gaps.md` for a user decision before lock.

1. **SCN-047 contradicts SCN-021 and SCN-022.** The plan says a single Finnhub failure with
   the stated Given returns `502 PROVIDER_UNAVAILABLE` from `/api/quotes`. Twelve Data's
   fallback (SCN-021) and the `Price unavailable` row (SCN-022/029) say the request succeeds.
   Spec §5 also says quote timeouts/non-2xx map to 502.
2. **SCN-050 vs fallback.** The plan says an exhausted Finnhub budget returns `429` for the
   request. The fallback rules and REQ-027 imply per-ticker degradation, not a whole-request
   refusal. The test is written literally.
3. **SCN-051 vs add-flow fallback.** The plan's Given covers only Finnhub (`c=0,t=0`). It does
   not say whether the add flow then consults Twelve Data. The test is written literally.

## Reviews, gaps, and completion

- Maintainability: not yet reviewed (no implementation).
- Shared helpers/configuration: fixtures are new; no disabled suites, no weakened assertions.
- Accessibility: not yet run (Phase 11).
- Requirement results: red only; no REQ is green yet.
- Gaps and blocked work: see `10-verification/gaps.md`. The frozen plan is not edited.
- As-built comparison: not yet (Phase 12).
- Final status: **Phase 8 red, tests locked 2026-11-16 13:59 UTC.** Dispositions: SCN-047 and
  SCN-050 blocked (`GAP-001`, `GAP-002`), kept literal and red; SCN-051 resolved (`GAP-003`).
  No scenario is green. Phase 9 implementation has not started.
