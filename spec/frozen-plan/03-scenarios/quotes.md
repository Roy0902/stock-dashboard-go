# Phase 3 — Scenarios: Quotes

Feature: `stock-watchlist`
Intent diagram: `../02-intent-diagrams/v1.md` (approved 2026-11-16), D2 and D3

```gherkin
Feature: Latest quotes
  As the local user
  I want to see the latest price for every ticker I follow
  So that I can judge the market at a glance

  Background:
    Given the provider fixtures respond deterministically
    And the quote cache is empty

  @REQ-stock-watchlist-014 @SCN-stock-watchlist-014 @smoke
  Scenario: A quote row carries every required field
    Given the watchlist contains "AAPL"
    And Finnhub returns price 190.12, previous close 188.88, timestamp "2026-11-16T14:30:00Z"
    When the user requests quotes
    Then the row shows ticker "AAPL", price "190.12", change "+1.24", change percent "+0.66%",
    And the row shows provider "Finnhub" and timestamp "2026-11-16T14:30:00Z"

  @REQ-stock-watchlist-015 @SCN-stock-watchlist-015 @smoke
  Scenario: Price change is derived and rounded to 2 decimals
    Given Finnhub returns price 190.125 and previous close 188.00
    When the user requests quotes
    Then change is "2.13"

  @REQ-stock-watchlist-016 @SCN-stock-watchlist-016 @smoke
  Scenario Outline: Change percent is derived from change and previous close
    Given Finnhub returns price <price> and previous close <previousClose>
    When the user requests quotes
    Then change percent is "<percent>"

    Examples:
      | price  | previousClose | percent |
      | 190.12 | 188.88        | "0.66"  |
      | 190.00 | 200.00        | "-5.00" |
      | 100.00 | 100.00        | "0.00"  |

  @REQ-stock-watchlist-017 @SCN-stock-watchlist-017 @reg
  Scenario: A positive change is marked up and prefixed
    Given Finnhub returns price 190.12 and previous close 188.88
    When the user opens the dashboard
    Then the change text is "+1.24"
    And the change colour is "#16a34a"

  @REQ-stock-watchlist-018 @SCN-stock-watchlist-018 @reg
  Scenario: A negative change is marked down
    Given Finnhub returns price 190.00 and previous close 200.00
    When the user opens the dashboard
    Then the change text is "-10.00"
    And the change colour is "#dc2626"

  @REQ-stock-watchlist-019 @SCN-stock-watchlist-019 @reg
  Scenario: A zero change is neutral
    Given Finnhub returns price 100.00 and previous close 100.00
    When the user opens the dashboard
    Then the change text is "0.00"
    And the change colour is "#6b7280"

  @REQ-stock-watchlist-020 @SCN-stock-watchlist-020 @smoke
  Scenario: Finnhub supplies the quote when it succeeds
    Given the watchlist contains "AAPL"
    And Finnhub returns price 190.12
    When the user requests quotes
    Then the provider is "Finnhub"
    And Twelve Data receives 0 requests

  @REQ-stock-watchlist-021 @SCN-stock-watchlist-021 @smoke
  Scenario: Twelve Data supplies the quote when Finnhub fails
    Given the watchlist contains "AAPL"
    And Finnhub returns HTTP 500
    And Twelve Data returns price 190.12
    When the user requests quotes
    Then the provider is "Twelve Data"
    And the row shows price "190.12"

  @REQ-stock-watchlist-022 @SCN-stock-watchlist-022 @smoke
  Scenario: Both providers failing leaves the row unavailable
    Given the watchlist contains "AAPL"
    And Finnhub returns HTTP 500
    And Twelve Data returns HTTP 500
    When the user opens the dashboard
    Then the row shows "Price unavailable"
    And the provider is "none"
    And no price value is shown for "AAPL"

  @REQ-stock-watchlist-023 @SCN-stock-watchlist-023 @reg
  Scenario: A 16-minute-old quote is stale
    Given Finnhub returns timestamp "2026-11-16T14:00:00Z"
    And the current time is "2026-11-16T14:16:00Z"
    When the user requests quotes
    Then the timestamp text is "2026-11-16T14:00:00Z (stale)"
    And stale is true

  @REQ-stock-watchlist-023 @SCN-stock-watchlist-024 @reg
  Scenario: A 14-minute-old quote is fresh
    Given Finnhub returns timestamp "2026-11-16T14:00:00Z"
    And the current time is "2026-11-16T14:14:00Z"
    When the user requests quotes
    Then the timestamp text is "2026-11-16T14:00:00Z"
    And stale is false

  @REQ-stock-watchlist-024 @SCN-stock-watchlist-025 @reg
  Scenario: A refresh inside 60 seconds serves the cache
    Given the watchlist contains "AAPL"
    And the user requested quotes once
    When the user requests quotes again 30 seconds later
    Then Finnhub received 1 request in total
    And the quote is unchanged

  @REQ-stock-watchlist-024 @SCN-stock-watchlist-026 @reg
  Scenario: A refresh after 60 seconds asks the provider again
    Given the watchlist contains "AAPL"
    And the user requested quotes once
    When the user requests quotes again 61 seconds later
    Then Finnhub received 2 requests in total

  @REQ-stock-watchlist-025 @SCN-stock-watchlist-027 @reg
  Scenario: The dashboard refreshes on its own every 60 seconds
    Given the watchlist contains "AAPL"
    When the dashboard has been open for 60 seconds
    Then the dashboard requested quotes 2 times

  @REQ-stock-watchlist-026 @SCN-stock-watchlist-028 @reg
  Scenario: The last refresh time is shown
    Given the local time is "14:30:00"
    When the dashboard refreshes
    Then the dashboard shows "Last updated 14:30:00"

  @REQ-stock-watchlist-027 @SCN-stock-watchlist-029 @reg
  Scenario: A ticker with no quote does not fail the batch
    Given the watchlist contains "AAPL" and "ZZZZ"
    And Finnhub returns a quote for "AAPL" and an unknown symbol for "ZZZZ"
    And Twelve Data returns an unknown symbol for "ZZZZ"
    When the user requests quotes
    Then the response status is 200
    And the quotes list contains "AAPL"
    And the quotes list omits "ZZZZ"
    And the dashboard row "ZZZZ" shows "Price unavailable"

  @REQ-stock-watchlist-028 @SCN-stock-watchlist-030 @reg
  Scenario: A provider slower than 2 seconds counts as failed
    Given the watchlist contains "AAPL"
    And Finnhub delays for 2.1 seconds
    And Twelve Data returns price 190.12
    When the user requests quotes
    Then the provider is "Twelve Data"

  @REQ-stock-watchlist-029 @SCN-stock-watchlist-031 @reg
  Scenario: The change text states direction without colour
    Given Finnhub returns price 190.12 and previous close 188.88
    When the user opens the dashboard
    Then the row states "up"
    And when the price is below the previous close the row states "down"
    And when the price equals the previous close the row states "unchanged"

  @REQ-stock-watchlist-052 @SCN-stock-watchlist-032 @reg
  Scenario: A zero previous close shows zero price and no percentage
    Given Finnhub returns price 0.00 and previous close 0.00
    When the user requests quotes
    Then the price is "0.00"
    And the change is "0.00"
    And the change percent is not shown

  @REQ-stock-watchlist-052 @SCN-stock-watchlist-033 @reg
  Scenario: A missing previous close shows the price without a percentage
    Given Finnhub returns price 50.00 and no previous close
    When the user requests quotes
    Then the price is "50.00"
    And the change percent is not shown
```

## Notes

- `SCN-030` and `SCN-021` share the fallback path but differ in the failure kind (timeout vs
  server error); both are kept because the timeout guard is its own requirement.
- No scenario asserts a live provider value. Every value comes from the fixture named in the
  `Given`.
