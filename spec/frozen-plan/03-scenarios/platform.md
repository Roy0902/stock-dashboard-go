# Phase 3 — Scenarios: Providers, errors, platform

Feature: `stock-watchlist`
Intent diagram: `../02-intent-diagrams/v1.md` (approved 2026-11-16), D3

```gherkin
Feature: Provider boundaries and local platform
  As the operator of the dashboard
  I want predictable behaviour when a provider is missing or failing
  So that the dashboard degrades honestly and stays local

  Background:
    Given the watchlist contains "AAPL"

  @REQ-stock-watchlist-040 @SCN-stock-watchlist-045 @smoke
  Scenario: Missing keys leave the app running with a clear message
    Given neither FINNHUB_API_KEY nor TWELVEDATA_API_KEY is set
    When the application starts and the user opens the dashboard
    Then the server is listening
    And the dashboard shows "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY."

  @REQ-stock-watchlist-041 @SCN-stock-watchlist-046 @reg
  Scenario: Quote and chart endpoints refuse without a key
    Given no API key is configured
    When the user requests quotes
    Then the response is 503 with code "MISSING_API_KEY"
    And when the user requests the chart for "AAPL" the response is 503 with code "MISSING_API_KEY"

  @REQ-stock-watchlist-042 @SCN-stock-watchlist-047 @reg
  Scenario Outline: Upstream failures map to one honest message
    Given Finnhub behaves as "<behaviour>"
    When the user requests quotes
    Then the response is <status> with code "<code>"
    And the message is "Data provider is unavailable. Try again shortly."

    Examples:
      | behaviour             | status | code                  |
      | HTTP 503              | 502    | PROVIDER_UNAVAILABLE  |
      | a malformed body      | 502    | PROVIDER_UNAVAILABLE  |
      | a 2.1 second delay    | 502    | PROVIDER_UNAVAILABLE  |

  @REQ-stock-watchlist-043 @SCN-stock-watchlist-048 @reg
  Scenario: The Finnhub budget is 55 calls per minute
    Given the watchlist contains 60 tickers
    When the user requests quotes for all of them in one minute
    Then at most 55 Finnhub requests are made

  @REQ-stock-watchlist-043 @SCN-stock-watchlist-049 @reg
  Scenario: The Twelve Data budget is 7 calls per minute
    Given the watchlist contains 10 tickers
    And every Finnhub request fails
    When the user requests quotes for all of them in one minute
    Then at most 7 Twelve Data requests are made

  @REQ-stock-watchlist-044 @SCN-stock-watchlist-050 @reg
  Scenario: Exhausting a budget refuses the request
    Given the Finnhub budget is exhausted
    When the user requests quotes
    Then the response is 429 with code "RATE_LIMITED"
    And the message matches "Too many requests. Try again in N seconds."

  @REQ-stock-watchlist-045 @SCN-stock-watchlist-051 @reg
  Scenario: A zero-filled Finnhub response means unknown
    Given Finnhub returns c = 0 and t = 0 for "ZZZZ"
    When the user adds "ZZZZ"
    Then the verdict is "UnknownSymbol"
    And the message is "Symbol not found."

  @REQ-stock-watchlist-046 @SCN-stock-watchlist-052 @reg
  Scenario: An unknown response shape is a provider failure, not a crash
    Given Finnhub returns { "unexpected": true }
    And Twelve Data returns HTTP 500
    When the user requests quotes
    Then the row shows "Price unavailable"
    And the server is still listening

  @REQ-stock-watchlist-047 @SCN-stock-watchlist-053 @reg
  Scenario: Provider detail never leaks
    Given Finnhub returns HTTP 500 with body "token=SECRET-XYZ rejected"
    When the user opens the dashboard
    Then the page contains no "SECRET-XYZ"
    And the page contains no stack trace
    And the server log of that request contains no "SECRET-XYZ"

  @REQ-stock-watchlist-048 @SCN-stock-watchlist-054 @api
  Scenario: The server binds the loopback interface only
    Given the server is running with no PORT override
    When the server has started
    Then it is listening on 127.0.0.1:3000
    And it is not reachable on a non-loopback address

  @REQ-stock-watchlist-049 @SCN-stock-watchlist-055 @reg
  Scenario: Only the watchlist is persisted
    Given the watchlist contains "AAPL"
    And the user requested quotes and a chart
    When the user inspects the data directory
    Then the directory contains "watchlist.json"
    And no file in it contains a price

  @REQ-stock-watchlist-050 @SCN-stock-watchlist-056 @reg
  Scenario: Provider base URLs are configurable
    Given FINNHUB_BASE_URL and TWELVEDATA_BASE_URL point at the local fixture server
    When the user requests quotes
    Then the local fixture server received the requests

  @REQ-stock-watchlist-051 @SCN-stock-watchlist-057 @smoke
  Scenario: Health check touches no provider
    Given both providers are unconfigured
    When the user requests "/healthz"
    Then the response is 200 with { "status": "ok" }
    And neither provider received a request
```

## Notes

- `SCN-048` and `SCN-049` state a maximum calls per minute rather than an exact count,
  because the spec fixes the budget, not the batching strategy.
- `SCN-053` checks the page and the log separately; logs are an operator surface.
