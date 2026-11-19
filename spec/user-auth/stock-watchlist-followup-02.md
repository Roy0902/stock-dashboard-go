# Test Plan: stock-watchlist-followup-02

> **Frozen artifact.** Follow-up to `specs/stock-watchlist.md` and `specs/stock-watchlist-followup-01.md`.
> Those plans and their locked tests are unchanged. Expectations come only from the approved
> decisions in `spec-to-test-workflow/stock-dashboard/02-intent-diagrams/v2.md` (DR-01, DR-03, DR-05,
> as amended in rev. 3) and the approved draft `11-followup/followup-plan-02-draft.md` (rev. 3).

## 1. Provenance

| Field | Value |
| :-- | :-- |
| Feature | `stock-watchlist-followup-02` |
| Intent | `02-intent-diagrams/v2.md`, DR-01, DR-03, DR-05 (rev. 3), user-approved 2026-11-17 |
| Draft | `11-followup/followup-plan-02-draft.md` rev. 3, user-approved 2026-11-17 |
| Frozen at | 2026-11-17 |
| Decisions | DR-01 amended: a last-known quote is shown only while fresh (at most 15 minutes old). DR-05 amended: an invalid chart range shows the 1M chart. |

## 2. Rules

- R1: when every provider fails for a ticker, the row shows the last known quote only if it is fresh
  (timestamp at most 15 minutes before now). Otherwise, or with no previous quote, the row shows
  `Price unavailable`.
- R2: a dashboard page view fetches current quotes. A `GET /api/quotes` refresh inside 60 seconds
  serves the cache.
- R3: on the chart page, a `range` not in `1M`, `3M`, `1Y` shows the `1M` chart with `1M` selected.
  The API still returns 400 `INVALID_RANGE`.

## 3. Scenarios

| ID | REQ | Tag | Title | Expected observable |
| :-- | :-- | :-- | :-- | :-- |
| `SCN-stock-watchlist-followup-02-001` | `REQ-stock-watchlist-022` | `@reg` | A fresh previous quote is shown when all providers fail | Given a page view priced "AAPL" at 190.12 with a fresh timestamp, and both providers now fail. When the dashboard is opened again. Then the row shows price "190.12", provider "Finnhub", and no "Price unavailable". |
| `SCN-stock-watchlist-followup-02-002` | `REQ-stock-watchlist-022` | `@reg` | A ticker that has never been priced still shows unavailable | Given a freshly started application that has never priced "MSFT", and both providers fail. Then the row shows "Price unavailable". |
| `SCN-stock-watchlist-followup-02-003` | `REQ-stock-watchlist-022` | `@reg` | A stale previous quote is not shown when all providers fail | Given a page view priced "AAPL" with a timestamp 14 minutes 30 seconds old, 60 seconds pass, and both providers now fail. Then the row shows "Price unavailable" and does not show "190.12". Real clock. |
| `SCN-stock-watchlist-followup-02-004` | `REQ-stock-watchlist-024` | `@reg` | A page view fetches the current price even inside 60 seconds | Given a page view priced "AAPL" at 190.12. When Finnhub now returns 180.00 and the dashboard is opened again. Then the row shows "180.00" and direction "down". |
| `SCN-stock-watchlist-followup-02-005` | `REQ-stock-watchlist-024` | `@reg` | A quote refresh inside 60 seconds after a page view serves the cache | Given a page view that made 1 Finnhub request. When quotes are requested. Then the response is 200 and Finnhub received 1 request in total. |
| `SCN-stock-watchlist-followup-02-006` | `REQ-stock-watchlist-033` | `@reg` | An invalid range on the chart page shows the 1M chart | Given Twelve Data returns points for "AAPL". When the chart for "AAPL" is opened with range "5Y". Then "1M" is selected and the page shows the "AAPL price chart, 1M range" image. |
| `SCN-stock-watchlist-followup-02-007` | `REQ-stock-watchlist-034` | `@reg` | The chart API still rejects an invalid range | When the chart for "AAPL" is requested with range "5Y". Then the response is 400 with code "INVALID_RANGE". |

## 4. Setup

- Each scenario starts a fresh application with a fresh `DATA_DIR`. Provider state is reset per test.
- Scenario 003 uses the real clock (`FIXED_NOW` unset), because it must age a quote past 15 minutes.
  It waits 60 seconds inside the test.
- No live provider is contacted.

## 5. Freeze sign-off

- [x] Decisions recorded and approved by the user (2026-11-17), including the rev. 3 amendments.
- [x] Each scenario has a concrete observable.
- [x] No blocking ambiguity.
- [x] Earlier frozen plans and locked tests unchanged.
