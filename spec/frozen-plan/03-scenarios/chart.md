# Phase 3 — Scenarios: Chart

Feature: `stock-watchlist`
Intent diagram: `../02-intent-diagrams/v1.md` (approved 2026-11-16), D5

```gherkin
Feature: Daily price chart
  As the local user
  I want a daily close-price chart for a ticker
  So that I can see its recent trend

  Background:
    Given the watchlist contains "AAPL"
    And "AAPL" is recognised by Twelve Data

  @REQ-stock-watchlist-030 @SCN-stock-watchlist-034 @smoke
  Scenario: Opening a chart from a quote row
    Given Twelve Data returns 30 daily points for "AAPL"
    When the user opens the chart for "AAPL" from its row
    Then the location is "/chart/AAPL"
    And the page shows the ticker "AAPL"

  @REQ-stock-watchlist-031 @SCN-stock-watchlist-035 @reg
  Scenario: The default chart shows 30 daily closes
    Given Twelve Data returns 30 daily points for "AAPL"
    When the user opens the chart for "AAPL" with no range
    Then the chart plots 30 points
    And every point is a daily close price

  @REQ-stock-watchlist-032 @SCN-stock-watchlist-036 @reg
  Scenario: The chart names its source
    Given Twelve Data returns 30 daily points for "AAPL"
    When the user opens the chart for "AAPL"
    Then the page shows "Source: Twelve Data"

  @REQ-stock-watchlist-033 @SCN-stock-watchlist-037 @reg
  Scenario: The range selector defaults to 1M
    Given Twelve Data returns points for "AAPL"
    When the user opens the chart for "AAPL"
    Then the range selector offers "1M", "3M" and "1Y"
    And "1M" is selected

  @REQ-stock-watchlist-034 @SCN-stock-watchlist-038 @reg
  Scenario Outline: The chart API accepts only the three ranges
    Given Twelve Data returns points for "AAPL"
    When the user requests the chart for "AAPL" with range "<range>"
    Then the response is <outcome>

    Examples:
      | range | outcome                                  |
      | 1M    | 200 with source "Twelve Data"            |
      | 3M    | 200 with source "Twelve Data"            |
      | 1Y    | 200 with source "Twelve Data"            |
      | 2M    | 400 with code "INVALID_RANGE"            |
      | ""    | 400 with code "INVALID_RANGE"            |
      | 1m    | 400 with code "INVALID_RANGE"            |

  @REQ-stock-watchlist-035 @SCN-stock-watchlist-039 @smoke
  Scenario: An unavailable chart leaves the dashboard usable
    Given Twelve Data returns HTTP 500 for "AAPL"
    When the user opens the chart for "AAPL"
    Then the page shows "Chart unavailable"
    And the dashboard row for "AAPL" still shows its quote

  @REQ-stock-watchlist-036 @SCN-stock-watchlist-040 @reg
  Scenario: The chart page links back
    Given Twelve Data returns points for "AAPL"
    When the user opens the chart for "AAPL"
    Then a "Back" link points to "/"

  @REQ-stock-watchlist-037 @SCN-stock-watchlist-041 @reg
  Scenario: The chart has a text alternative
    Given Twelve Data returns points for "AAPL" and the range "3M"
    When the user opens the chart for "AAPL" with range "3M"
    Then the chart has a text alternative naming "AAPL" and "3M"

  @REQ-stock-watchlist-038 @SCN-stock-watchlist-042 @reg
  Scenario: Points are ordered oldest to newest
    Given Twelve Data returns points dated 2026-11-14, 2026-11-13, 2026-11-12
    When the user requests the chart for "AAPL"
    Then the points are ordered 2026-11-12, 2026-11-13, 2026-11-14

  @REQ-stock-watchlist-039 @SCN-stock-watchlist-043 @smoke
  Scenario: The chart response shape
    Given Twelve Data returns 2 points for "AAPL"
    When the user requests the chart for "AAPL" with range "1M"
    Then the response is 200
    And the body is { "ticker": "AAPL", "range": "1M", "source": "Twelve Data", "points": [ { "date": "2026-11-13", "close": 190.1 }, { "date": "2026-11-14", "close": 191.2 } ] }

  @REQ-stock-watchlist-039 @SCN-stock-watchlist-044 @reg
  Scenario: A ticker outside the watchlist still has a chart
    Given "MSFT" is not in the watchlist
    And Twelve Data returns points for "MSFT"
    When the user opens the chart for "MSFT" directly
    Then the page shows the chart for "MSFT"
```

## Notes

- The chart never uses Finnhub: the free plan marks candles Premium, so the only chart source
  is Twelve Data (`REQ-032`, `AMB-stock-watchlist-008`).
- `SCN-044` states that the chart route does not require the ticker to be listed. The spec is
  silent on this; it is recorded as `AMB-stock-watchlist-012` for review.
