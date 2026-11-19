# Gap Register: stock-watchlist

Gaps are recorded outside the frozen plan. The frozen plan is never edited. A gap becomes a
follow-up plan with its own intent review, scenarios, and freeze.

| ID | Scenario(s) | Observation | Conflicting sources | Status |
| :-- | :-- | :-- | :-- | :-- |
| GAP-001 | SCN-047 | Given "Finnhub behaves as ..." alone; expected `502 PROVIDER_UNAVAILABLE`. Under fallback, Twelve Data answers and the request succeeds (SCN-021). Under Price-unavailable, the request is 200 (SCN-022, SCN-029, REQ-027). | Frozen plan SCN-021, SCN-022, SCN-029 vs SCN-047; spec §4.2, §5, REQ-027 | **Blocked by user, 2026-11-16.** Test kept as written and locked; it cannot pass to green as written. Follow-up plan required. |
| GAP-002 | SCN-050 | Exhausted Finnhub budget expected to return a whole-request `429`. Fallback and per-ticker isolation (REQ-027, AMB-010) imply degradation, not refusal. | Frozen plan SCN-050 vs SCN-021, SCN-029; spec §7 | **Blocked by user, 2026-11-16.** Test kept as written and locked. Follow-up plan required. |
| GAP-003 | SCN-051 | Given only Finnhub returns `c=0,t=0`. The plan does not say whether add then consults Twelve Data. | Frozen plan SCN-051 vs SCN-007 (both providers unknown) | **Resolved by user, 2026-11-16:** refuse on Finnhub alone (spec §6.1). Test matches. |
| GAP-004 | (process) | Unit-test enum is in `Spec/unit-test-catalogue.md` and `tests-unit/catalogue.ts`. Unit methods are not yet written. | User instruction, 2026-11-16 | Open, next task |

**Closure, 2026-11-17:** GAP-001 and GAP-002 are closed by the follow-up plan
`local-env/specs/stock-watchlist-followup-01.md` (frozen, tests locked, `11-followup/`). Original
SCN-047 and SCN-050 stay red in the original plan. They are superseded by FU-001 and FU-002, not
edited. The original SCN-047 and SCN-050 still fail in the full run, by design.

Tests were locked after these dispositions (`ai:lock-tests`, 2026-11-16 13:59 UTC). The locked
files are `local-env/tests/specs/stock-watchlist.spec.ts` and
`local-env/tests/api/stock-watchlist.spec.ts`. Blocked scenarios stay red and are reported as
blocked, never as passing.
