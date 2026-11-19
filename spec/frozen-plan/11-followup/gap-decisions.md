# Follow-up Plan Draft: stock-watchlist, failure rules (GAP-001, GAP-002)

Status: **draft, awaiting user decision.** No scenarios or freeze until a rule is chosen.

Purpose: the frozen plan (`local-env/specs/stock-watchlist.md`, unchanged) contains two scenarios
whose expected response conflicts with its own fallback and per-ticker rules. SCN-047 and SCN-050
are blocked (`10-verification/gaps.md`). This follow-up records the rule the user chooses.

## Decision 1: upstream failure for `GET /api/quotes` (GAP-001, SCN-047)

SCN-047 expects `502 PROVIDER_UNAVAILABLE` when Finnhub fails. SCN-021 says Twelve Data then
answers. SCN-022 and SCN-029 say a failed ticker shows `Price unavailable` in a 200 response.

| Option | Rule | Consequence for the frozen plan |
| :-- | :-- | :-- |
| A (current build) | Per ticker: Finnhub, then Twelve Data. Unpriced tickers are omitted, and the page shows `Price unavailable`. Request is 200. | SCN-047 rewritten: expect 200 with the ticker omitted. Matches SCN-022 and SCN-029. |
| B | 200 for partial success. 502 only when every watched ticker is unpriced by all providers. | SCN-047 rewritten to the all-fail case. Adds a rule for the 502 threshold. |
| C | 502 whenever any upstream call fails, even if fallback succeeds. | Contradicts SCN-021 (fallback must answer). Requires changing SCN-021. Not recommended. |

## Decision 2: local budget exhaustion (GAP-002, SCN-050)

SCN-050 expects `429 RATE_LIMITED` for a whole request after the Finnhub budget is exhausted.
Fallback to Twelve Data (SCN-021) and per-ticker isolation (SCN-029) suggest partial results.

| Option | Rule | Consequence |
| :-- | :-- | :-- |
| A (current build, decision D-5) | 429 only when every ticker needing an upstream call was refused by a budget and none was served. Otherwise 200 with served tickers. | SCN-050 rewritten to the all-refused case, with a `Retry-After`-style N seconds in the copy. |
| B | Per-ticker refusal: refused tickers show a rate-limit row; request is 200. | Needs new row copy, which the spec does not define. |
| C | 429 whenever any budget refuses a call. | Degrades fallback: one exhausted budget blocks the whole page. Not recommended. |

## Other points for the decision record

- D-5 (current build) is an engineering interpretation I chose, not a rule from the spec. It needs
  your confirmation as Decision 2, option A.
- Option A for Decision 1 is what the build already does. Choosing it changes only SCN-047's
  expectation.
- Once chosen, the follow-up gets its own intent diagram review, scenarios, and freeze. It does
  not edit `specs/stock-watchlist.md`, and the locked tests stay as they are. SCN-047 and SCN-050
  remain blocked in the original plan. The follow-up supersedes them only by the process in
  `verification-loop` (post-freeze gaps).
