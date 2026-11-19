# Follow-up Plan: stock-watchlist, failure rules (GAP-001, GAP-002)

Status: **draft for user approval. Not yet frozen.**
Feature: `stock-watchlist-followup-01`
Supersedes: SCN-047 and SCN-050 in `local-env/specs/stock-watchlist.md`, which stay blocked and unchanged.
Decisions: user, 2026-11-17, both option A (`gap-decisions.md`).

## 1. Rules

- **R1 (per ticker, SCN-021, SCN-022, SCN-029):** each ticker is priced by Finnhub, then by Twelve
  Data. A ticker neither provider prices is omitted from `GET /api/quotes`.
- **R2 (partial success, REQ-027):** `GET /api/quotes` returns 200 whenever at least one ticker is
  priced, with the priced tickers only.
- **R3 (budget refusal, REQ-044):** a ticker whose call is refused by a local budget is treated as
  unpriced for that request. The request returns 429 `RATE_LIMITED` only when every ticker needing
  an upstream call was refused and none was priced.
- **R4 (copy):** the 429 message is `Too many requests. Try again in N seconds.`, where N is a
  positive integer. The spec defines this copy in §4.4. The `Data provider is unavailable` copy is
  not used on `GET /api/quotes`. The UI shows `Price unavailable` for omitted tickers.

## 2. Scenarios

Tags follow the frozen convention. Each scenario supersedes the listed original scenario.

### FU-001 Supersedes SCN-047 (REQ-042)

```gherkin
@REQ-stock-watchlist-042 @SCN-stock-watchlist-047 @reg
Scenario Outline: A failed Finnhub ticker is served by Twelve Data or omitted
  Given the watchlist contains "AAPL"
  And Finnhub behaves as "<behaviour>"
  And Twelve Data returns HTTP 500 for "AAPL"
  When the user requests quotes
  Then the response is 200
  And the quotes list omits "AAPL"

  Examples:
    | behaviour        |
    | HTTP 503         |
    | a malformed body |
    | a 2.1 second delay |
```

Tag note: the tag stays `SCN-stock-watchlist-047` only as a reference. A new id is preferred
(`SCN-stock-watchlist-f001`) so the guard does not confuse it with the blocked original. Confirm
at approval.

### FU-002 Supersedes SCN-050 (REQ-044)

```gherkin
@REQ-stock-watchlist-044 @SCN-stock-watchlist-050 @reg
Scenario: Exhausting both budgets with nothing priced refuses the request
  Given the watchlist contains 56 tickers
  And every upstream request fails
  When the user requests quotes
  Then the response is 429 with code "RATE_LIMITED"
  And the message matches "Too many requests. Try again in N seconds."
```

Setup note: 55 Finnhub calls and then 7 Twelve Data calls are spent on the first 55 tickers and
the next 7, all failing. The remaining tickers are refused. Nothing is priced, so R3 gives 429.

### FU-003 New: partial budget refusal still returns priced tickers (REQ-027, REQ-043)

```gherkin
@REQ-stock-watchlist-027 @SCN-stock-watchlist-f003 @reg
Scenario: Tickers beyond the Finnhub budget are omitted while the rest are served
  Given the watchlist contains 60 tickers
  And Finnhub returns a quote for every ticker
  And Twelve Data returns HTTP 500 for every ticker
  When the user requests quotes
  Then the response is 200
  And the quotes list contains 55 tickers
```

## 3. Gaps closed by this plan

| Gap | Status after approval |
| :-- | :-- |
| GAP-001 | Closed by R1/R2 and FU-001. |
| GAP-002 | Closed by R3 and FU-002. FU-003 adds the partial case. |

## 4. Open confirmations for the user

1. Scenario ids: new `SCN-stock-watchlist-f00N` ids, or reuse of the original ids (the guard will
   reject a duplicate tag on lock).
2. FU-002 setup: the 56-ticker watchlist with all calls failing is the only deterministic way to
   reach "nothing priced". Confirm.
3. Whether the `Data provider is unavailable` copy should remain on any quotes response. Under R4
   it does not, so it appears only on the add flow (D-4) and the chart flow.

## 5. Next steps after approval

1. Freeze `local-env/specs/stock-watchlist-followup-01.md` with `ai:freeze`.
2. Author the follow-up tests and run them red.
3. Lock them, then update the implementation only as far as the new rules require.
4. Re-run the full gates, then return to Phase 10 for the original plan.
