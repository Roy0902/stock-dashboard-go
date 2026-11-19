# Test Plan: stock-watchlist-followup-01

> **Frozen artifact.** Supersedes the original quote-failure scenarios 047 and 050 in
> `specs/stock-watchlist.md`. The original plan stays unchanged and those two scenarios stay blocked
> there. Expectations come only from the approved decisions in
> `spec-to-test-workflow/stock-dashboard/11-followup/gap-decisions.md` (user, 2026-11-17) and the
> approved draft `11-followup/followup-plan.md`.

## 1. Provenance

| Field | Value |
| :-- | :-- |
| Feature | `stock-watchlist-followup-01` |
| Supersedes | original scenarios 047 (GAP-001) and 050 (GAP-002) in `specs/stock-watchlist.md` |
| Decisions | `11-followup/gap-decisions.md`, both option A, user 2026-11-17 |
| Draft approved | `11-followup/followup-plan.md`, user 2026-11-17 |
| Id change from draft | `f001..f003` became `followup-01-001..003`. The guard requires three numeric digits. |
| Gap register | `10-verification/gaps.md` (GAP-001, GAP-002 closed by this plan) |

## 2. Rules

- R1: each ticker is priced by Finnhub, then by Twelve Data. An unpriced ticker is omitted.
- R2: `GET /api/quotes` returns 200 when at least one ticker is priced.
- R3: a 429 `RATE_LIMITED` is returned only when every ticker needing an upstream call was refused
  by a local budget and none was priced.
- R4: the 429 copy is `Too many requests. Try again in N seconds.` (N is a positive integer).

## 3. Scenarios

| ID | REQ | Tag | Title | Expected observable |
| :-- | :-- | :-- | :-- | :-- |
| `SCN-stock-watchlist-followup-01-001` | `REQ-stock-watchlist-042` | `@reg` | A failed Finnhub ticker is served by Twelve Data or omitted (three failure kinds) | Given "AAPL" and Finnhub fails as HTTP 503, a malformed body, or a 2.1 second delay, and Twelve Data returns HTTP 500 for "AAPL". Then the response is 200 and the quotes list omits "AAPL". |
| `SCN-stock-watchlist-followup-01-002` | `REQ-stock-watchlist-044` | `@reg` | Nothing priced after budgets are exhausted refuses the request | Given 56 tickers and every upstream request fails. Then the response is 429 with code "RATE_LIMITED" and the message matches "Too many requests. Try again in N seconds." |
| `SCN-stock-watchlist-followup-01-003` | `REQ-stock-watchlist-027` | `@reg` | Tickers beyond the Finnhub budget are omitted while the rest are served | Given 60 tickers, Finnhub prices every ticker, and Twelve Data returns HTTP 500 for every ticker. Then the response is 200 and the quotes list contains 55 tickers. |

## 4. Setup

- Each scenario starts a fresh application process with a fresh `DATA_DIR`.
- The fake provider is reset before each test.
- No live provider is contacted.

## 5. Freeze sign-off

- [x] Decisions recorded and approved by the user (2026-11-17).
- [x] Each scenario has a concrete observable.
- [x] No blocking ambiguity.
- [x] Original plan unchanged; the superseded scenarios remain recorded as blocked there.
