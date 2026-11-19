# Phase 3 — Scenarios: Watchlist

Feature: `stock-watchlist`
Intent diagram: `../02-intent-diagrams/v1.md` (approved 2026-11-16)
Vocabulary: `../01-domain-model/domain-model.md` §1

Tags map each scenario to its requirement; the guard also reads `[SCN-...]` tags in tests.

```gherkin
Feature: Watchlist membership
  As the local user
  I want to curate a list of tickers
  So that I can follow the quotes I care about

  Background:
    Given the dashboard has no listed tickers

  @REQ-stock-watchlist-001 @SCN-stock-watchlist-001 @smoke
  Scenario: First use shows the empty-watchlist message
    Given the watchlist is empty
    When the user opens the dashboard
    Then the dashboard shows "Your watchlist is empty. Add a ticker to begin."
    And the watchlist has 0 entries

  @REQ-stock-watchlist-002 @SCN-stock-watchlist-002 @smoke
  Scenario: A lowercase ticker is canonicalised to uppercase
    Given the provider recognises "AAPL"
    When the user adds "aapl"
    Then the watchlist contains "AAPL"
    And the watchlist contains 0 entries named "aapl"

  @REQ-stock-watchlist-003 @SCN-stock-watchlist-003 @reg
  Scenario Outline: Ticker canonical form boundary
    Given the provider recognises one ticker per example
    When the user adds a ticker of "<form>"
    Then the verdict is "<verdict>"

    Examples:
      | form        | verdict  |
      | A           | Accepted |
      | AAPL        | Accepted |
      | A123456789  | Accepted |
      | aapl        | Accepted |

  @REQ-stock-watchlist-004 @SCN-stock-watchlist-004 @smoke
  Scenario Outline: Malformed tickers are refused
    Given the watchlist is empty
    When the user adds a ticker of "<form>"
    Then the verdict is "Invalid"
    And the response status is 400 with code "INVALID_TICKER"
    And the message is "Enter a valid ticker, e.g. AAPL."
    And the watchlist has 0 entries

    Examples:
      | form        |
      | ""          |
      | "1AAPL"     |
      | "AA-PL"     |
      | "A.B"       |
      | "AA PL"     |
      | "A1234567890" |
      | "Ä"         |

  @REQ-stock-watchlist-005 @SCN-stock-watchlist-005 @smoke
  Scenario: A duplicate ticker is refused with its copy
    Given the watchlist contains "AAPL"
    When the user adds "aapl"
    Then the verdict is "Duplicate"
    And the response status is 409 with code "DUPLICATE_TICKER"
    And the message is "AAPL is already in your watchlist."
    And the watchlist contains 1 entry named "AAPL"

  @REQ-stock-watchlist-006 @SCN-stock-watchlist-006 @smoke
  Scenario: An accepted ticker returns the full list
    Given the watchlist contains "MSFT"
    And the provider recognises "AAPL"
    When the user adds "AAPL"
    Then the response status is 201
    And the response lists ["MSFT", "AAPL"] in that order
    And the watchlist contains 2 entries

  @REQ-stock-watchlist-007 @SCN-stock-watchlist-007 @smoke
  Scenario: An unknown symbol is refused
    Given the provider does not recognise "ZZZZ"
    When the user adds "ZZZZ"
    Then the verdict is "UnknownSymbol"
    And the response status is 404 with code "UNKNOWN_SYMBOL"
    And the message is "Symbol not found."
    And the watchlist has 0 entries

  @REQ-stock-watchlist-008 @SCN-stock-watchlist-008 @smoke
  Scenario: Removing a listed ticker returns the remainder
    Given the watchlist contains "AAPL" and "MSFT"
    When the user removes "AAPL"
    Then the response status is 200
    And the response lists ["MSFT"]
    And the watchlist contains 1 entry

  @REQ-stock-watchlist-009 @SCN-stock-watchlist-009 @reg
  Scenario: Removing a ticker that is not listed is refused
    Given the watchlist contains "MSFT"
    When the user removes "AAPL"
    Then the response status is 404 with code "NOT_IN_WATCHLIST"
    And the watchlist contains 1 entry named "MSFT"

  @REQ-stock-watchlist-010 @SCN-stock-watchlist-010 @smoke
  Scenario: Membership survives a restart
    Given the watchlist contains "AAPL" and "MSFT"
    When the application restarts with the same data directory
    Then the watchlist contains "AAPL" and "MSFT"
    And $DATA_DIR/watchlist.json contains "AAPL"

  @REQ-stock-watchlist-011 @SCN-stock-watchlist-011 @smoke
  Scenario: Listing the watchlist
    Given the watchlist contains "AAPL" and "MSFT"
    When the user requests the watchlist
    Then the response status is 200
    And the body is { "tickers": ["AAPL", "MSFT"] }

  @REQ-stock-watchlist-012 @SCN-stock-watchlist-012 @smoke
  Scenario: The dashboard lists one row per ticker
    Given the watchlist contains "AAPL" and "MSFT"
    When the user opens the dashboard
    Then the dashboard shows exactly 2 entry rows
    And the rows are named "AAPL" and "MSFT"
    And an "Add" control is present

  @REQ-stock-watchlist-013 @SCN-stock-watchlist-013 @reg
  Scenario: The watchlist has no maximum size
    Given the provider recognises 50 distinct tickers
    When the user adds all 50 tickers
    Then the watchlist contains 50 entries
    And no add is refused with "WATCHLIST_FULL"
```

## Notes

- `SCN-004` covers the equivalence classes and the boundary of the canonical form in one
  table; `A1234567890` is 11 characters (max + 1).
- `SCN-006` fixes the returned order as insertion order. The spec does not state an order;
  this is recorded as `AMB-stock-watchlist-011` before freeze if the review wants a different
  rule.
