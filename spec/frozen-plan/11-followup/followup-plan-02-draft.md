# Follow-up Plan 02 (DRAFT, rev. 3): last known quote, page-view cache, page range fallback

Status: **draft for final approval. Not frozen. No tests written.**
Feature: `stock-watchlist-followup-02`
Source decisions: `02-intent-diagrams/v2.md` DR-01, DR-03 and DR-05, as amended in rev. 3.
Frozen plan `specs/stock-watchlist.md` and follow-up 01 are unchanged.

## Revision log

- rev. 1 (2026-11-17): first draft.
- rev. 2 (2026-11-17): user answers. Last known quote shown; invalid range shows the 1M chart;
  restart accepted; scenario 02-002 setup clarified.
- rev. 3 (2026-11-17): user answer. A last known quote is shown only while it is fresh (within
  15 minutes of now). If it is stale, the row shows `Price unavailable`. Intent v2 DR-01 and DR-05
  amended to match.

## 1. Rules

- **R1 (DR-01, amended):** when every provider fails for a ticker, the row shows the last known
  quote only if that quote is fresh (its timestamp is at most 15 minutes old). Price, change,
  percent, provider and timestamp come from that quote. If it is stale, or there is no previous
  quote, the row shows `Price unavailable`.
- **R2 (DR-03):** a dashboard page view fetches current quotes. A `GET /api/quotes` refresh inside
  60 seconds serves the cache.
- **R3 (DR-05, amended):** on the chart page, a `range` value not in `1M`, `3M`, `1Y` shows the `1M`
  chart with the `1M` control selected. The API still returns 400 `INVALID_RANGE`.

The `(stale)` marker is unchanged. It still applies to a quote that a provider returns while its
timestamp is older than 15 minutes (SCN-023). It never applies to a last-known quote, which is
either fresh or shown as unavailable.

## 2. Scenarios

### SCN-stock-watchlist-followup-02-001 (R1, REQ-stock-watchlist-022)

```gherkin
@REQ-stock-watchlist-022 @SCN-stock-watchlist-followup-02-001 @reg
Scenario: A fresh previous quote is shown when all providers fail
  Given the watchlist contains "AAPL"
  And the dashboard was opened once and Finnhub returned price 190.12 with a timestamp 5 minutes ago
  And Finnhub and Twelve Data now return HTTP 500 for "AAPL"
  When the user opens the dashboard again
  Then the row for "AAPL" shows price "190.12"
  And the row for "AAPL" shows provider "Finnhub"
  And the row does not show "Price unavailable"
```

### SCN-stock-watchlist-followup-02-002 (R1, REQ-stock-watchlist-022)

```gherkin
@REQ-stock-watchlist-022 @SCN-stock-watchlist-followup-02-002 @reg
Scenario: A ticker that has never been priced still shows unavailable
  Given the application has just started and has never priced "MSFT"
  And Finnhub and Twelve Data return HTTP 500 for "MSFT"
  When the user opens the dashboard
  Then the row for "MSFT" shows "Price unavailable"
```

### SCN-stock-watchlist-followup-02-003 (R1, REQ-stock-watchlist-022)

```gherkin
@REQ-stock-watchlist-022 @SCN-stock-watchlist-followup-02-003 @reg
Scenario: A stale previous quote is not shown when all providers fail
  Given the watchlist contains "AAPL"
  And the dashboard was opened once and Finnhub returned price 190.12 with a timestamp 14 minutes 30 seconds ago
  And 60 seconds have passed
  And Finnhub and Twelve Data now return HTTP 500 for "AAPL"
  When the user opens the dashboard again
  Then the row for "AAPL" shows "Price unavailable"
  And the row does not show "190.12"
```

Setup note: this scenario uses the real clock, not `FIXED_NOW`, so that the quote ages past 15
minutes. The 60-second wait is needed for that reason.

### SCN-stock-watchlist-followup-02-004 (R2, REQ-stock-watchlist-024)

```gherkin
@REQ-stock-watchlist-024 @SCN-stock-watchlist-followup-02-004 @reg
Scenario: A page view fetches the current price even inside 60 seconds
  Given the watchlist contains "AAPL"
  And the dashboard was opened once and Finnhub returned price 190.12
  When Finnhub now returns price 180.00 for "AAPL"
  And the user opens the dashboard again
  Then the row for "AAPL" shows price "180.00"
  And the row for "AAPL" shows direction "down"
```

### SCN-stock-watchlist-followup-02-005 (R2, REQ-stock-watchlist-024)

```gherkin
@REQ-stock-watchlist-024 @SCN-stock-watchlist-followup-02-005 @reg
Scenario: A quote refresh inside 60 seconds after a page view serves the cache
  Given the watchlist contains "AAPL"
  And the dashboard was opened once and Finnhub received 1 request
  When the user requests quotes
  Then the response is 200
  And Finnhub received 1 request in total
```

### SCN-stock-watchlist-followup-02-006 (R3, REQ-stock-watchlist-033)

```gherkin
@REQ-stock-watchlist-033 @SCN-stock-watchlist-followup-02-006 @reg
Scenario: An invalid range on the chart page shows the 1M chart
  Given the watchlist contains "AAPL"
  And Twelve Data returns points for "AAPL"
  When the user opens the chart for "AAPL" with range "5Y"
  Then the range "1M" is selected
  And the page shows the chart for "AAPL" with a "1M" range
```

### SCN-stock-watchlist-followup-02-007 (R3, REQ-stock-watchlist-034)

```gherkin
@REQ-stock-watchlist-034 @SCN-stock-watchlist-followup-02-007 @reg
Scenario: The chart API still rejects an invalid range
  Given the watchlist contains "AAPL"
  When the user requests the chart for "AAPL" with range "5Y"
  Then the response is 400 with code "INVALID_RANGE"
```

## 3. Coverage after approval

| Decision | Scenarios |
| :-- | :-- |
| DR-01 (amended) | 02-001, 02-002, 02-003 |
| DR-03 | 02-004, 02-005 |
| DR-05 (amended) | 02-006, 02-007 |

No change to the frozen plans or locked tests.

## 4. Approval

Awaiting final user approval. On approval: freeze as `local-env/specs/stock-watchlist-followup-02.md`,
author the tests, run them red where the build does not yet match (expected: 02-001, 02-003,
02-006), then implement and lock.
