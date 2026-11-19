# Test Plan: stock-watchlist

> **Frozen artifact.** Derived only from the specification and the approved intent diagram.
> Code, observations, captures, task lists and engineering design are not expectation
> sources. Never modify this file after freeze, including evidence and status fields.
> Gaps go to a follow-up plan under the verification policy.

## 1. Provenance

| Field | Value |
| :-- | :-- |
| Feature | `stock-watchlist` |
| Spec artifact | `Spec/implementation-spec.md` @ v1.2 `b2d5c797ff565d79` |
| Intent diagram (Phase 2) | `spec-to-test-workflow/stock-dashboard/02-intent-diagrams/v1.md` @ `694adf32398e209c`, user-approved 2026-11-16 |
| Checklist (Phase 4) | `spec-to-test-workflow/stock-dashboard/04-checklist/shared-checklist.md` @ `f403aeb0379421ef` |
| Frontend task list (Phase 5) | `spec-to-test-workflow/stock-dashboard/05-task-lists/frontend.md` @ `810c7437fecae204` (scope only) |
| Backend task list (Phase 5) | `spec-to-test-workflow/stock-dashboard/05-task-lists/backend.md` @ `08d4e81608e0ca81` (scope only) |
| Requirement source | `spec-to-test-workflow/stock-dashboard/00-ingestion/requirement-register.md` @ `9f209cb29f63358d` (rev. 3) |
| Review record | `spec-to-test-workflow/stock-dashboard/06-review/decisions.md` @ 2026-11-16 |
| Frozen by | user |
| Frozen at | 2026-11-16 |
| Implementation design | `spec-to-test-workflow/stock-dashboard/05-task-lists/implementation-design.md` @ `530e71f85262e6dc` (engineering context only) |
| Execution evidence | `spec-to-test-workflow/stock-dashboard/08-red-runs/red-run.md` and `10-verification/report.md` (external; never in this file) |
| Gap register | `spec-to-test-workflow/stock-dashboard/10-verification/gaps.md` |

## 2. Scope

**In scope:**

- `/` dashboard: watchlist add, list, remove; quote rows.
- `/chart/:ticker` daily close chart.
- `GET/POST /api/watchlist`, `DELETE /api/watchlist/:ticker`, `GET /api/quotes`,
  `GET /api/chart/:ticker`, `GET /healthz`.
- Finnhub primary quotes and Twelve Data fallback and chart, both free tier.
- Local JSON persistence of the watchlist only.

**Out of scope (explicit):**

- Accounts, roles or authentication. No role scenarios exist.
- Databases, Docker, cloud or VPS deployment.
- Trading, portfolios, alerts, notifications, intraday data.
- Any provider feature that requires a paid plan.

## 3. Requirement Register

| ID | Requirement | Type | Source | Level | Testability |
| :-- | :-- | :-- | :-- | :-- | :-- |
| REQ-stock-watchlist-001 | The watchlist starts empty and the UI shows `Your watchlist is empty. Add a ticker to begin.` | Data | spec §4.1, §4.4 | Both | testable |
| REQ-stock-watchlist-002 | A ticker is stored and compared in uppercase, so `aapl` is stored as `AAPL`. | Rule | spec §4.1, §5 | Both | testable |
| REQ-stock-watchlist-003 | A ticker matching `^[A-Z][A-Z0-9]{0,9}$` after normalisation is accepted. | Rule | spec §4.1 | API | testable |
| REQ-stock-watchlist-004 | A ticker not matching the format is rejected with `400 INVALID_TICKER` and copy `Enter a valid ticker, e.g. AAPL.` | Rule | spec §4.1, §4.4, §5 | API | testable |
| REQ-stock-watchlist-005 | Adding a ticker that already exists returns `409 DUPLICATE_TICKER` and copy `AAPL is already in your watchlist.` | Rule | spec §4.1, §4.4, §5 | Both | testable |
| REQ-stock-watchlist-006 | A successful add returns `201` with the full ticker list. | Contract | spec §5 | API | testable |
| REQ-stock-watchlist-007 | Adding a symbol the provider does not know returns `404 UNKNOWN_SYMBOL` and copy `Symbol not found.` | Flow | spec §4.4, §5 | API | testable |
| REQ-stock-watchlist-008 | Removing an existing ticker returns `200` with the remaining ticker list. | Contract | spec §5 | API | testable |
| REQ-stock-watchlist-009 | Removing a ticker that is not in the watchlist returns `404 NOT_IN_WATCHLIST`. | Rule | spec §5 | API | testable |
| REQ-stock-watchlist-010 | The watchlist is written to `$DATA_DIR/watchlist.json` and survives a server restart. | Data | spec §4.1, §8 | API | testable |
| REQ-stock-watchlist-011 | `GET /api/watchlist` returns `200 { "tickers": [...] }`. | Contract | spec §5 | API | testable |
| REQ-stock-watchlist-012 | The UI shows one row per watchlist ticker and an `Add` control. | Rule | spec §4.1, §4.2 | UI | testable |
| REQ-stock-watchlist-013 | The watchlist has no maximum size. | Rule | spec §4.1 | API | testable |

### Quotes (QU)

| ID | Requirement | Type | Source | Level | Testability |
| :-- | :-- | :-- | :-- | :-- | :-- |
| REQ-stock-watchlist-014 | Each watchlist row shows ticker, price, change, change percent, provider, timestamp and a direction word (`up`/`down`/`unchanged`). | Data | spec §4.2 | UI | testable |
| REQ-stock-watchlist-015 | `change` is `price - previousClose`, rounded to 2 decimals. | Rule | spec §4.2 | Both | testable |
| REQ-stock-watchlist-016 | `changePercent` is `change / previousClose * 100`, rounded to 2 decimals. | Rule | spec §4.2 | Both | testable |
| REQ-stock-watchlist-017 | Positive values carry a `+` prefix and are shown in `#16a34a`. | Rule | spec §4.2 | UI | testable |
| REQ-stock-watchlist-018 | Negative values carry a `-` sign and are shown in `#dc2626`. | Rule | spec §4.2 | UI | testable |
| REQ-stock-watchlist-019 | A zero change is shown in `#6b7280`. | Boundary | spec §4.2 | UI | testable |
| REQ-stock-watchlist-020 | Finnhub is used for a quote when it succeeds, and the row's provider reads `Finnhub`. | Rule | spec §4.2, §6.1 | Both | testable |
| REQ-stock-watchlist-021 | When Finnhub fails for a ticker, Twelve Data is used and the row's provider reads `Twelve Data`. | Flow | spec §4.2, §6.2 | Both | testable |
| REQ-stock-watchlist-022 | When both providers fail for a ticker, the row shows `Price unavailable` and provider `none`. | Flow | spec §4.2, §4.4 | Both | testable |
| REQ-stock-watchlist-023 | A quote older than 15 minutes shows its timestamp with the suffix ` (stale)`. | Boundary | spec §4.2 | Both | testable |
| REQ-stock-watchlist-024 | Quotes are cached for 60 seconds and repeated refreshes inside that window do not call a provider. | Rule | spec §4.2, §7 | API | testable |
| REQ-stock-watchlist-025 | The UI refreshes quotes every 60 seconds while the page is open. | Flow | spec §4.2 | UI | testable |
| REQ-stock-watchlist-026 | The UI shows `Last updated HH:MM:SS` in local time. | Data | spec §4.2 | UI | testable |
| REQ-stock-watchlist-027 | `GET /api/quotes` omits tickers with no quote and does not fail the whole request. | Contract | spec §5 | API | testable |
| REQ-stock-watchlist-028 | A provider timeout of 2 seconds counts as a provider failure. | Rule | spec §5, §6.2 | API | testable |
| REQ-stock-watchlist-029 | The UI text states up/down/unchanged for each change, so colour is not the only carrier. | Rule | spec §9 | UI | testable-with-assumption (`AMB-stock-watchlist-004`) |

### Chart (CH)

| ID | Requirement | Type | Source | Level | Testability |
| :-- | :-- | :-- | :-- | :-- | :-- |
| REQ-stock-watchlist-030 | Each ticker row has a `View chart` control that opens `/chart/<TICKER>`. | Flow | spec §4.3 | Both | testable |
| REQ-stock-watchlist-031 | The chart page shows a daily close-price line for the last 30 trading days. | Data | spec §4.3, §6.2 | UI | testable-with-assumption (`AMB-stock-watchlist-003`) |
| REQ-stock-watchlist-032 | The chart page shows `Source: Twelve Data`. | Data | spec §4.3 | UI | testable |
| REQ-stock-watchlist-033 | A range selector offers `1M`, `3M`, `1Y`; `1M` is the default. | Rule | spec §4.3, §6.2 | UI | testable |
| REQ-stock-watchlist-034 | `range` accepts `1M`, `3M`, `1Y` and any other value returns `400 INVALID_RANGE`. | Contract | spec §5 | API | testable |
| REQ-stock-watchlist-035 | When chart data is unavailable the page shows `Chart unavailable` and the dashboard stays usable. | Flow | spec §4.3, §4.4 | Both | testable |
| REQ-stock-watchlist-036 | The chart page has a `Back` link to the dashboard. | Flow | spec §4.3 | UI | testable |
| REQ-stock-watchlist-037 | The chart has an accessible text alternative naming the ticker and the range. | Rule | spec §9 | UI | testable |
| REQ-stock-watchlist-038 | Chart points are plotted oldest to newest. | Data | spec §6.2 | Both | testable |
| REQ-stock-watchlist-039 | `GET /api/chart/:ticker` returns the ticker, range, source and points. | Contract | spec §5 | API | testable |

### Providers, limits and errors (PR)

| ID | Requirement | Type | Source | Level | Testability |
| :-- | :-- | :-- | :-- | :-- | :-- |
| REQ-stock-watchlist-040 | A missing API key leaves the server running and the UI shows `Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.` | Flow | spec §3 | Both | testable |
| REQ-stock-watchlist-041 | Quote and chart endpoints return `503 MISSING_API_KEY` when no key is configured. | Contract | spec §3, §5 | API | testable |
| REQ-stock-watchlist-042 | Upstream non-2xx, unknown shape or timeout maps to `502` with copy `Data provider is unavailable. Try again shortly.` | Rule | spec §4.4, §5 | API | testable |
| REQ-stock-watchlist-043 | The app enforces 55 calls/minute to Finnhub and 7 calls/minute to Twelve Data. | Rule | spec §7 | API | testable |
| REQ-stock-watchlist-044 | Exceeding a local limit returns `429 RATE_LIMITED` and copy `Too many requests. Try again in N seconds.` | Flow | spec §7 | API | testable-with-assumption (`AMB-stock-watchlist-005`) |
| REQ-stock-watchlist-045 | A Finnhub response with `c = 0` and `t = 0` means the symbol is unknown. | Rule | spec §6.1 | API | testable |
| REQ-stock-watchlist-046 | An unknown provider response shape is a provider failure and never a crash. | Rule | spec §6.2 | API | testable |
| REQ-stock-watchlist-047 | Raw provider messages, traces and API keys are never shown in the UI or logs. | Rule | spec §3, §4.4 | Both | testable |

### Platform and testability (PT)

| ID | Requirement | Type | Source | Level | Testability |
| :-- | :-- | :-- | :-- | :-- | :-- |
| REQ-stock-watchlist-048 | The server listens on `127.0.0.1` only, default port `3000`. | Contract | spec §3 | API | testable |
| REQ-stock-watchlist-049 | The only persisted data is `$DATA_DIR/watchlist.json`; quotes and charts stay in memory. | Data | spec §8 | API | testable |
| REQ-stock-watchlist-050 | Provider base URLs come from `FINNHUB_BASE_URL` and `TWELVEDATA_BASE_URL`. | Contract | spec §10 | API | testable |
| REQ-stock-watchlist-051 | `GET /healthz` returns `200 { "status": "ok" }` and contacts no provider. | Contract | spec §10 | API | testable |
| REQ-stock-watchlist-052 | When `previousClose` is zero, price and change are `0` and no percentage is shown; when `previousClose` is missing or negative, price is shown as supplied, change as supplied or `0`, and no percentage is shown. | Boundary | spec §4.2, §5 | Both | testable |

## 4. Scenarios (Oracle)

**Every expected value is concrete, observable and sourced from the spec or the approved
intent diagram.** Data is supplied by the local fake provider named in the Data Plan.

| ID | REQ | Tag | Title | Gherkin | Expected Observable |
| :-- | :-- | :-- | :-- | :-- | :-- |
| `SCN-stock-watchlist-001` | `REQ-stock-watchlist-001` | `@smoke` | First use shows the empty-watchlist message | Given the watchlist is empty When the user opens the dashboard Then the dashboard shows "Your watchlist is empty. Add a ticker to begin." And the watchlist has 0 entries | Then the dashboard shows "Your watchlist is empty. Add a ticker to begin." And the watchlist has 0 entries |
| `SCN-stock-watchlist-002` | `REQ-stock-watchlist-002` | `@smoke` | A lowercase ticker is canonicalised to uppercase | Given the provider recognises "AAPL" When the user adds "aapl" Then the watchlist contains "AAPL" And the watchlist contains 0 entries named "aapl" | Then the watchlist contains "AAPL" And the watchlist contains 0 entries named "aapl" |
| `SCN-stock-watchlist-003` | `REQ-stock-watchlist-003` | `@reg` | Ticker canonical form boundary | Given the provider recognises one ticker per example When the user adds a ticker of "<form>" Then the verdict is "<verdict>" | Then the verdict is "<verdict>" |
| `SCN-stock-watchlist-004` | `REQ-stock-watchlist-004` | `@smoke` | Malformed tickers are refused | Given the watchlist is empty When the user adds a ticker of "<form>" Then the verdict is "Invalid" And the response status is 400 with code "INVALID_TICKER" And the message is "Enter a valid ticker, e.g. AAPL." And the watchlist has 0 entries | Then the verdict is "Invalid" And the response status is 400 with code "INVALID_TICKER" And the message is "Enter a valid ticker, e.g. AAPL." And the watchlist has 0 entries |
| `SCN-stock-watchlist-005` | `REQ-stock-watchlist-005` | `@smoke` | A duplicate ticker is refused with its copy | Given the watchlist contains "AAPL" When the user adds "aapl" Then the verdict is "Duplicate" And the response status is 409 with code "DUPLICATE_TICKER" And the message is "AAPL is already in your watchlist." And the watchlist contains 1 entry named "AAPL" | Then the verdict is "Duplicate" And the response status is 409 with code "DUPLICATE_TICKER" And the message is "AAPL is already in your watchlist." And the watchlist contains 1 entry named "AAPL" |
| `SCN-stock-watchlist-006` | `REQ-stock-watchlist-006` | `@smoke` | An accepted ticker returns the full list | Given the watchlist contains "MSFT" And the provider recognises "AAPL" When the user adds "AAPL" Then the response status is 201 And the response lists ["MSFT", "AAPL"] in that order And the watchlist contains 2 entries | And the provider recognises "AAPL" Then the response status is 201 And the response lists ["MSFT", "AAPL"] in that order And the watchlist contains 2 entries |
| `SCN-stock-watchlist-007` | `REQ-stock-watchlist-007` | `@smoke` | An unknown symbol is refused | Given the provider does not recognise "ZZZZ" When the user adds "ZZZZ" Then the verdict is "UnknownSymbol" And the response status is 404 with code "UNKNOWN_SYMBOL" And the message is "Symbol not found." And the watchlist has 0 entries | Then the verdict is "UnknownSymbol" And the response status is 404 with code "UNKNOWN_SYMBOL" And the message is "Symbol not found." And the watchlist has 0 entries |
| `SCN-stock-watchlist-008` | `REQ-stock-watchlist-008` | `@smoke` | Removing a listed ticker returns the remainder | Given the watchlist contains "AAPL" and "MSFT" When the user removes "AAPL" Then the response status is 200 And the response lists ["MSFT"] And the watchlist contains 1 entry | Then the response status is 200 And the response lists ["MSFT"] And the watchlist contains 1 entry |
| `SCN-stock-watchlist-009` | `REQ-stock-watchlist-009` | `@reg` | Removing a ticker that is not listed is refused | Given the watchlist contains "MSFT" When the user removes "AAPL" Then the response status is 404 with code "NOT_IN_WATCHLIST" And the watchlist contains 1 entry named "MSFT" | Then the response status is 404 with code "NOT_IN_WATCHLIST" And the watchlist contains 1 entry named "MSFT" |
| `SCN-stock-watchlist-010` | `REQ-stock-watchlist-010` | `@smoke` | Membership survives a restart | Given the watchlist contains "AAPL" and "MSFT" When the application restarts with the same data directory Then the watchlist contains "AAPL" and "MSFT" And $DATA_DIR/watchlist.json contains "AAPL" | Then the watchlist contains "AAPL" and "MSFT" And $DATA_DIR/watchlist.json contains "AAPL" |
| `SCN-stock-watchlist-011` | `REQ-stock-watchlist-011` | `@smoke` | Listing the watchlist | Given the watchlist contains "AAPL" and "MSFT" When the user requests the watchlist Then the response status is 200 And the body is { "tickers": ["AAPL", "MSFT"] } | Then the response status is 200 And the body is { "tickers": ["AAPL", "MSFT"] } |
| `SCN-stock-watchlist-012` | `REQ-stock-watchlist-012` | `@smoke` | The dashboard lists one row per ticker | Given the watchlist contains "AAPL" and "MSFT" When the user opens the dashboard Then the dashboard shows exactly 2 entry rows And the rows are named "AAPL" and "MSFT" And an "Add" control is present | Then the dashboard shows exactly 2 entry rows And the rows are named "AAPL" and "MSFT" And an "Add" control is present |
| `SCN-stock-watchlist-013` | `REQ-stock-watchlist-013` | `@reg` | The watchlist has no maximum size | Given the provider recognises 50 distinct tickers When the user adds all 50 tickers Then the watchlist contains 50 entries And no add is refused with "WATCHLIST_FULL" | Then the watchlist contains 50 entries And no add is refused with "WATCHLIST_FULL" |
| `SCN-stock-watchlist-014` | `REQ-stock-watchlist-014` | `@smoke` | A quote row carries every required field | Given the watchlist contains "AAPL" And Finnhub returns price 190.12, previous close 188.88, timestamp "2026-11-16T14:30:00Z" When the user requests quotes Then the row shows ticker "AAPL", price "190.12", change "+1.24", change percent "+0.66%", And the row shows provider "Finnhub" and timestamp "2026-11-16T14:30:00Z" | And Finnhub returns price 190.12, previous close 188.88, timestamp "2026-11-16T14:30:00Z" Then the row shows ticker "AAPL", price "190.12", change "+1.24", change percent "+0.66%", And the row shows provider "Finnhub" and timestamp "2026-11-16T14:30:00Z" |
| `SCN-stock-watchlist-015` | `REQ-stock-watchlist-015` | `@smoke` | Price change is derived and rounded to 2 decimals | Given Finnhub returns price 190.125 and previous close 188.00 When the user requests quotes Then change is "2.13" | Then change is "2.13" |
| `SCN-stock-watchlist-016` | `REQ-stock-watchlist-016` | `@smoke` | Change percent is derived from change and previous close | Given Finnhub returns price <price> and previous close <previousClose> When the user requests quotes Then change percent is "<percent>" | Then change percent is "<percent>" |
| `SCN-stock-watchlist-017` | `REQ-stock-watchlist-017` | `@reg` | A positive change is marked up and prefixed | Given Finnhub returns price 190.12 and previous close 188.88 When the user opens the dashboard Then the change text is "+1.24" And the change colour is "#16a34a" | Then the change text is "+1.24" And the change colour is "#16a34a" |
| `SCN-stock-watchlist-018` | `REQ-stock-watchlist-018` | `@reg` | A negative change is marked down | Given Finnhub returns price 190.00 and previous close 200.00 When the user opens the dashboard Then the change text is "-10.00" And the change colour is "#dc2626" | Then the change text is "-10.00" And the change colour is "#dc2626" |
| `SCN-stock-watchlist-019` | `REQ-stock-watchlist-019` | `@reg` | A zero change is neutral | Given Finnhub returns price 100.00 and previous close 100.00 When the user opens the dashboard Then the change text is "0.00" And the change colour is "#6b7280" | Then the change text is "0.00" And the change colour is "#6b7280" |
| `SCN-stock-watchlist-020` | `REQ-stock-watchlist-020` | `@smoke` | Finnhub supplies the quote when it succeeds | Given the watchlist contains "AAPL" And Finnhub returns price 190.12 When the user requests quotes Then the provider is "Finnhub" And Twelve Data receives 0 requests | And Finnhub returns price 190.12 Then the provider is "Finnhub" And Twelve Data receives 0 requests |
| `SCN-stock-watchlist-021` | `REQ-stock-watchlist-021` | `@smoke` | Twelve Data supplies the quote when Finnhub fails | Given the watchlist contains "AAPL" And Finnhub returns HTTP 500 And Twelve Data returns price 190.12 When the user requests quotes Then the provider is "Twelve Data" And the row shows price "190.12" | And Finnhub returns HTTP 500 And Twelve Data returns price 190.12 Then the provider is "Twelve Data" And the row shows price "190.12" |
| `SCN-stock-watchlist-022` | `REQ-stock-watchlist-022` | `@smoke` | Both providers failing leaves the row unavailable | Given the watchlist contains "AAPL" And Finnhub returns HTTP 500 And Twelve Data returns HTTP 500 When the user opens the dashboard Then the row shows "Price unavailable" And the provider is "none" And no price value is shown for "AAPL" | And Finnhub returns HTTP 500 And Twelve Data returns HTTP 500 Then the row shows "Price unavailable" And the provider is "none" And no price value is shown for "AAPL" |
| `SCN-stock-watchlist-023` | `REQ-stock-watchlist-023` | `@reg` | A 16-minute-old quote is stale | Given Finnhub returns timestamp "2026-11-16T14:00:00Z" And the current time is "2026-11-16T14:16:00Z" When the user requests quotes Then the timestamp text is "2026-11-16T14:00:00Z (stale)" And stale is true | And the current time is "2026-11-16T14:16:00Z" Then the timestamp text is "2026-11-16T14:00:00Z (stale)" And stale is true |
| `SCN-stock-watchlist-024` | `REQ-stock-watchlist-023` | `@reg` | A 14-minute-old quote is fresh | Given Finnhub returns timestamp "2026-11-16T14:00:00Z" And the current time is "2026-11-16T14:14:00Z" When the user requests quotes Then the timestamp text is "2026-11-16T14:00:00Z" And stale is false | And the current time is "2026-11-16T14:14:00Z" Then the timestamp text is "2026-11-16T14:00:00Z" And stale is false |
| `SCN-stock-watchlist-025` | `REQ-stock-watchlist-024` | `@reg` | A refresh inside 60 seconds serves the cache | Given the watchlist contains "AAPL" And the user requested quotes once When the user requests quotes again 30 seconds later Then Finnhub received 1 request in total And the quote is unchanged | And the user requested quotes once Then Finnhub received 1 request in total And the quote is unchanged |
| `SCN-stock-watchlist-026` | `REQ-stock-watchlist-024` | `@reg` | A refresh after 60 seconds asks the provider again | Given the watchlist contains "AAPL" And the user requested quotes once When the user requests quotes again 61 seconds later Then Finnhub received 2 requests in total | And the user requested quotes once Then Finnhub received 2 requests in total |
| `SCN-stock-watchlist-027` | `REQ-stock-watchlist-025` | `@reg` | The dashboard refreshes on its own every 60 seconds | Given the watchlist contains "AAPL" When the dashboard has been open for 60 seconds Then the dashboard requested quotes 2 times | Then the dashboard requested quotes 2 times |
| `SCN-stock-watchlist-028` | `REQ-stock-watchlist-026` | `@reg` | The last refresh time is shown | Given the local time is "14:30:00" When the dashboard refreshes Then the dashboard shows "Last updated 14:30:00" | Then the dashboard shows "Last updated 14:30:00" |
| `SCN-stock-watchlist-029` | `REQ-stock-watchlist-027` | `@reg` | A ticker with no quote does not fail the batch | Given the watchlist contains "AAPL" and "ZZZZ" And Finnhub returns a quote for "AAPL" and an unknown symbol for "ZZZZ" And Twelve Data returns an unknown symbol for "ZZZZ" When the user requests quotes Then the response status is 200 And the quotes list contains "AAPL" And the quotes list omits "ZZZZ" And the dashboard row "ZZZZ" shows "Price unavailable" | And Finnhub returns a quote for "AAPL" and an unknown symbol for "ZZZZ" And Twelve Data returns an unknown symbol for "ZZZZ" Then the response status is 200 And the quotes list contains "AAPL" And the quotes list omits "ZZZZ" And the dashboard row "ZZZZ" shows "Price unavailable" |
| `SCN-stock-watchlist-030` | `REQ-stock-watchlist-028` | `@reg` | A provider slower than 2 seconds counts as failed | Given the watchlist contains "AAPL" And Finnhub delays for 2.1 seconds And Twelve Data returns price 190.12 When the user requests quotes Then the provider is "Twelve Data" | And Finnhub delays for 2.1 seconds And Twelve Data returns price 190.12 Then the provider is "Twelve Data" |
| `SCN-stock-watchlist-031` | `REQ-stock-watchlist-029` | `@reg` | The change text states direction without colour | Given Finnhub returns price 190.12 and previous close 188.88 When the user opens the dashboard Then the row states "up" And when the price is below the previous close the row states "down" And when the price equals the previous close the row states "unchanged" | Then the row states "up" And when the price is below the previous close the row states "down" And when the price equals the previous close the row states "unchanged" |
| `SCN-stock-watchlist-032` | `REQ-stock-watchlist-052` | `@reg` | A zero previous close shows zero price and no percentage | Given Finnhub returns price 0.00 and previous close 0.00 When the user requests quotes Then the price is "0.00" And the change is "0.00" And the change percent is not shown | Then the price is "0.00" And the change is "0.00" And the change percent is not shown |
| `SCN-stock-watchlist-033` | `REQ-stock-watchlist-052` | `@reg` | A missing previous close shows the price without a percentage | Given Finnhub returns price 50.00 and no previous close When the user requests quotes Then the price is "50.00" And the change percent is not shown | Then the price is "50.00" And the change percent is not shown |
| `SCN-stock-watchlist-034` | `REQ-stock-watchlist-030` | `@smoke` | Opening a chart from a quote row | Given Twelve Data returns 30 daily points for "AAPL" When the user opens the chart for "AAPL" from its row Then the location is "/chart/AAPL" And the page shows the ticker "AAPL" | Then the location is "/chart/AAPL" And the page shows the ticker "AAPL" |
| `SCN-stock-watchlist-035` | `REQ-stock-watchlist-031` | `@reg` | The default chart shows 30 daily closes | Given Twelve Data returns 30 daily points for "AAPL" When the user opens the chart for "AAPL" with no range Then the chart plots 30 points And every point is a daily close price | Then the chart plots 30 points And every point is a daily close price |
| `SCN-stock-watchlist-036` | `REQ-stock-watchlist-032` | `@reg` | The chart names its source | Given Twelve Data returns 30 daily points for "AAPL" When the user opens the chart for "AAPL" Then the page shows "Source: Twelve Data" | Then the page shows "Source: Twelve Data" |
| `SCN-stock-watchlist-037` | `REQ-stock-watchlist-033` | `@reg` | The range selector defaults to 1M | Given Twelve Data returns points for "AAPL" When the user opens the chart for "AAPL" Then the range selector offers "1M", "3M" and "1Y" And "1M" is selected | Then the range selector offers "1M", "3M" and "1Y" And "1M" is selected |
| `SCN-stock-watchlist-038` | `REQ-stock-watchlist-034` | `@reg` | The chart API accepts only the three ranges | Given Twelve Data returns points for "AAPL" When the user requests the chart for "AAPL" with range "<range>" Then the response is <outcome> | Then the response is <outcome> |
| `SCN-stock-watchlist-039` | `REQ-stock-watchlist-035` | `@smoke` | An unavailable chart leaves the dashboard usable | Given Twelve Data returns HTTP 500 for "AAPL" When the user opens the chart for "AAPL" Then the page shows "Chart unavailable" And the dashboard row for "AAPL" still shows its quote | Then the page shows "Chart unavailable" And the dashboard row for "AAPL" still shows its quote |
| `SCN-stock-watchlist-040` | `REQ-stock-watchlist-036` | `@reg` | The chart page links back | Given Twelve Data returns points for "AAPL" When the user opens the chart for "AAPL" Then a "Back" link points to "/" | Then a "Back" link points to "/" |
| `SCN-stock-watchlist-041` | `REQ-stock-watchlist-037` | `@reg` | The chart has a text alternative | Given Twelve Data returns points for "AAPL" and the range "3M" When the user opens the chart for "AAPL" with range "3M" Then the chart has a text alternative naming "AAPL" and "3M" | Then the chart has a text alternative naming "AAPL" and "3M" |
| `SCN-stock-watchlist-042` | `REQ-stock-watchlist-038` | `@reg` | Points are ordered oldest to newest | Given Twelve Data returns points dated 2026-11-14, 2026-11-13, 2026-11-12 When the user requests the chart for "AAPL" Then the points are ordered 2026-11-12, 2026-11-13, 2026-11-14 | Then the points are ordered 2026-11-12, 2026-11-13, 2026-11-14 |
| `SCN-stock-watchlist-043` | `REQ-stock-watchlist-039` | `@smoke` | The chart response shape | Given Twelve Data returns 2 points for "AAPL" When the user requests the chart for "AAPL" with range "1M" Then the response is 200 And the body is { "ticker": "AAPL", "range": "1M", "source": "Twelve Data", "points": [ { "date": "2026-11-13", "close": 190.1 }, { "date": "2026-11-14", "close": 191.2 } ] } | Then the response is 200 And the body is { "ticker": "AAPL", "range": "1M", "source": "Twelve Data", "points": [ { "date": "2026-11-13", "close": 190.1 }, { "date": "2026-11-14", "close": 191.2 } ] } |
| `SCN-stock-watchlist-044` | `REQ-stock-watchlist-039` | `@reg` | A ticker outside the watchlist still has a chart | Given "MSFT" is not in the watchlist And Twelve Data returns points for "MSFT" When the user opens the chart for "MSFT" directly Then the page shows the chart for "MSFT" | And Twelve Data returns points for "MSFT" Then the page shows the chart for "MSFT" |
| `SCN-stock-watchlist-045` | `REQ-stock-watchlist-040` | `@smoke` | Missing keys leave the app running with a clear message | Given neither FINNHUB_API_KEY nor TWELVEDATA_API_KEY is set When the application starts and the user opens the dashboard Then the server is listening And the dashboard shows "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY." | Then the server is listening And the dashboard shows "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY." |
| `SCN-stock-watchlist-046` | `REQ-stock-watchlist-041` | `@reg` | Quote and chart endpoints refuse without a key | Given no API key is configured When the user requests quotes Then the response is 503 with code "MISSING_API_KEY" And when the user requests the chart for "AAPL" the response is 503 with code "MISSING_API_KEY" | Then the response is 503 with code "MISSING_API_KEY" And when the user requests the chart for "AAPL" the response is 503 with code "MISSING_API_KEY" |
| `SCN-stock-watchlist-047` | `REQ-stock-watchlist-042` | `@reg` | Upstream failures map to one honest message | Given Finnhub behaves as "<behaviour>" When the user requests quotes Then the response is <status> with code "<code>" And the message is "Data provider is unavailable. Try again shortly." | Then the response is <status> with code "<code>" And the message is "Data provider is unavailable. Try again shortly." |
| `SCN-stock-watchlist-048` | `REQ-stock-watchlist-043` | `@reg` | The Finnhub budget is 55 calls per minute | Given the watchlist contains 60 tickers When the user requests quotes for all of them in one minute Then at most 55 Finnhub requests are made | Then at most 55 Finnhub requests are made |
| `SCN-stock-watchlist-049` | `REQ-stock-watchlist-043` | `@reg` | The Twelve Data budget is 7 calls per minute | Given the watchlist contains 10 tickers And every Finnhub request fails When the user requests quotes for all of them in one minute Then at most 7 Twelve Data requests are made | And every Finnhub request fails Then at most 7 Twelve Data requests are made |
| `SCN-stock-watchlist-050` | `REQ-stock-watchlist-044` | `@reg` | Exhausting a budget refuses the request | Given the Finnhub budget is exhausted When the user requests quotes Then the response is 429 with code "RATE_LIMITED" And the message matches "Too many requests. Try again in N seconds." | Then the response is 429 with code "RATE_LIMITED" And the message matches "Too many requests. Try again in N seconds." |
| `SCN-stock-watchlist-051` | `REQ-stock-watchlist-045` | `@reg` | A zero-filled Finnhub response means unknown | Given Finnhub returns c = 0 and t = 0 for "ZZZZ" When the user adds "ZZZZ" Then the verdict is "UnknownSymbol" And the message is "Symbol not found." | Then the verdict is "UnknownSymbol" And the message is "Symbol not found." |
| `SCN-stock-watchlist-052` | `REQ-stock-watchlist-046` | `@reg` | An unknown response shape is a provider failure, not a crash | Given Finnhub returns { "unexpected": true } And Twelve Data returns HTTP 500 When the user requests quotes Then the row shows "Price unavailable" And the server is still listening | And Twelve Data returns HTTP 500 Then the row shows "Price unavailable" And the server is still listening |
| `SCN-stock-watchlist-053` | `REQ-stock-watchlist-047` | `@reg` | Provider detail never leaks | Given Finnhub returns HTTP 500 with body "token=SECRET-XYZ rejected" When the user opens the dashboard Then the page contains no "SECRET-XYZ" And the page contains no stack trace And the server log of that request contains no "SECRET-XYZ" | Then the page contains no "SECRET-XYZ" And the page contains no stack trace And the server log of that request contains no "SECRET-XYZ" |
| `SCN-stock-watchlist-054` | `REQ-stock-watchlist-048` | `@api` | The server binds the loopback interface only | Given the server is running with no PORT override When the server has started Then it is listening on 127.0.0.1:3000 And it is not reachable on a non-loopback address | Then it is listening on 127.0.0.1:3000 And it is not reachable on a non-loopback address |
| `SCN-stock-watchlist-055` | `REQ-stock-watchlist-049` | `@reg` | Only the watchlist is persisted | Given the watchlist contains "AAPL" And the user requested quotes and a chart When the user inspects the data directory Then the directory contains "watchlist.json" And no file in it contains a price | And the user requested quotes and a chart Then the directory contains "watchlist.json" And no file in it contains a price |
| `SCN-stock-watchlist-056` | `REQ-stock-watchlist-050` | `@reg` | Provider base URLs are configurable | Given FINNHUB_BASE_URL and TWELVEDATA_BASE_URL point at the local fixture server When the user requests quotes Then the local fixture server received the requests | Then the local fixture server received the requests |
| `SCN-stock-watchlist-057` | `REQ-stock-watchlist-051` | `@smoke` | Health check touches no provider | Given both providers are unconfigured When the user requests "/healthz" Then the response is 200 with { "status": "ok" } And neither provider received a request | Then the response is 200 with { "status": "ok" } And neither provider received a request |

## 5. State Transitions (from Mermaid)

### 5.1 Watch entry (D1)

| From | To | Guard / Condition | Scenario ID | Covered |
| :-- | :-- | :-- | :-- | :-- |
| Absent | Listed | valid, unseen, provider-recognised | `SCN-stock-watchlist-006` | Yes |
| Absent | Absent | invalid form | `SCN-stock-watchlist-004` | Yes |
| Absent | Absent | provider reports unknown symbol | `SCN-stock-watchlist-007` | Yes |
| Listed | Listed | duplicate add refused | `SCN-stock-watchlist-005` | Yes |
| Listed | Removed | user removes ticker | `SCN-stock-watchlist-008` | Yes |
| Removed | Removed | remove of a non-listed ticker | `SCN-stock-watchlist-009` | Yes |

No cap transition exists: the watchlist has no maximum size.

### 5.2 Quote (D2)

| From | To | Guard / Condition | Scenario ID | Covered |
| :-- | :-- | :-- | :-- | :-- |
| Absent | Fresh | primary ok, previousClose > 0 | `SCN-stock-watchlist-020` | Yes |
| Absent | Fresh | primary failed, secondary ok | `SCN-stock-watchlist-021` | Yes |
| Absent | Fresh (zero) | previousClose = 0 | `SCN-stock-watchlist-032` | Yes |
| Absent | Fresh (no baseline) | previousClose missing or negative | `SCN-stock-watchlist-033` | Yes |
| Absent | Unavailable | both providers failed | `SCN-stock-watchlist-022` | Yes |
| Fresh | Fresh | refresh inside 60 s serves cache | `SCN-stock-watchlist-025` | Yes |
| Fresh | Fresh | refresh after 60 s replaces observation | `SCN-stock-watchlist-026` | Yes |
| Fresh | Stale | age > 15 minutes | `SCN-stock-watchlist-023` | Yes |
| Stale | Fresh | newer usable observation | `SCN-stock-watchlist-024` | Yes |
| Unavailable | Fresh | later refresh succeeds | `SCN-stock-watchlist-021` | Yes |
| any | Absent | ticker removed | `SCN-stock-watchlist-008` | Yes |

### 5.3 Price series (D5)

| From | To | Guard / Condition | Scenario ID | Covered |
| :-- | :-- | :-- | :-- | :-- |
| Absent | Available | secondary returned points for the range | `SCN-stock-watchlist-035` | Yes |
| Absent | Available | no range chosen, so `1M` used | `SCN-stock-watchlist-037` | Yes |
| Absent | Unavailable | provider failed or returned no points | `SCN-stock-watchlist-039` | Yes |
| Absent | Unavailable | range not one of 1M / 3M / 1Y | `SCN-stock-watchlist-038` | Yes |

## 6. API Contract Coverage

All endpoints are new; baselines are deferred. Error envelope: `{ "error": "<copy>", "code": "<CODE>" }`.

| Endpoint | Method | Auth | REQ | Expected Status | Expected Shape / Errors | Baseline |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| `/api/watchlist` | GET | none (loopback) | 011 | 200 | `{ tickers: string[] }` | deferred |
| `/api/watchlist` | POST | none | 003-008 | 201 | `{ tickers: string[] }`; 400 `INVALID_TICKER`, 409 `DUPLICATE_TICKER`, 404 `UNKNOWN_SYMBOL` | deferred |
| `/api/watchlist/:ticker` | DELETE | none | 008, 009 | 200 | `{ tickers: string[] }`; 404 `NOT_IN_WATCHLIST` | deferred |
| `/api/quotes` | GET | none | 014-029, 043, 044, 052 | 200 | `{ quotes: Quote[] }`; 429 `RATE_LIMITED`, 502 `PROVIDER_UNAVAILABLE`, 503 `MISSING_API_KEY` | deferred |
| `/api/chart/:ticker` | GET | none | 030-039 | 200 | `{ ticker, range, source, points[] }`; 400 `INVALID_RANGE`, 502 `CHART_UNAVAILABLE`, 503 `MISSING_API_KEY` | deferred |
| `/healthz` | GET | none | 051 | 200 | `{ status: "ok" }` | deferred |

`Quote` = `{ ticker, price, change, changePercent: number|null, direction: "up"|"down"|"unchanged", provider: "Finnhub"|"Twelve Data"|"none", timestamp, stale: boolean }`.

## 7. Data Plan

| Role | Storage State | Seed Required | Reset Strategy |
| :-- | :-- | :-- | :-- |
| single local user | none (no auth) | fake provider fixtures | fresh `DATA_DIR` per Playwright worker |
| quote fixtures | n/a | `local-env/fixtures/provider.mjs` scripted responses | fixture server restart per run |
| staleness fixtures | n/a | `FIXED_NOW` plus offsets in the provider fixture | per test file |

Deterministic sources:

- `FINNHUB_BASE_URL` and `TWELVEDATA_BASE_URL` point at the fake provider.
- `DATA_DIR` is a temp directory; only `watchlist.json` is written.
- No test reads a live provider. Live values are never oracles.

## 8. Coverage Summary

| Metric | Target | Planned |
| :-- | :-- | :-- |
| Requirements covered | 100% of testable | 52 / 52 |
| Positive scenarios | >= 1 per REQ | 52 |
| Negative / boundary | Applicable | 24 requirements covered |
| Guarded transitions | 100% | 21 / 21 |
| Invariant violations | >= 1 per invariant | 18 / 18 (INV-01 to INV-18) |
| Endpoints | >= 1 API test each | 6 / 6 |
| E2E @smoke journeys | Critical paths | 22 |
| Scenarios tagged unspecified | 0 | 0 |

## 9. Assumptions and Ambiguities

All entries are non-blocking. No blocking ambiguity is open.

| ID | Assumption / Question | Impact | Blocking | Decision |
| :-- | :-- | :-- | :-- | :-- |
| `AMB-stock-watchlist-001` | price and percent show 2 decimals | Low | No | Approved 2026-11-16 |
| `AMB-stock-watchlist-002` | trading days = provider bars for the requested size | Low | No | Approved 2026-11-16 |
| `AMB-stock-watchlist-003` | cache TTL and UI refresh are separate 60 s timers | Low | No | Approved 2026-11-16 |
| `AMB-stock-watchlist-004` | chart is custom SVG, no library | Low | No | Engineering, Phase 5 design |
| `AMB-stock-watchlist-005` | visible up / down / unchanged text | Low | No | Approved 2026-11-16 |
| `AMB-stock-watchlist-006` | local 429 is distinct from 502 | Med | No | Approved 2026-11-16 |
| `AMB-stock-watchlist-007` | omitted quote renders `Price unavailable` | Low | No | Approved 2026-11-16 |
| `AMB-stock-watchlist-008` | chart stays in scope while Twelve Data free tier allows it | Med | No | Verified 2026-11-16 |
| `AMB-stock-watchlist-009` | unknown symbol on chart is `CHART_UNAVAILABLE` | Low | No | Approved 2026-11-16 |
| `AMB-stock-watchlist-010` | per-ticker isolation for the quote batch | Low | No | Approved 2026-11-16 |
| `AMB-stock-watchlist-011` | watchlist order is insertion order | Low | No | Closed by user 2026-11-16 |
| `AMB-stock-watchlist-012` | chart route works for an unlisted ticker | Low | No | Closed by user 2026-11-16 |

## 10. Execution Evidence Location

_Chosen before freeze. Phase 8 and later append here; nothing is written into this plan._

- Red run record: `spec-to-test-workflow/stock-dashboard/08-red-runs/red-run.md`
- Verification record: `spec-to-test-workflow/stock-dashboard/10-verification/report.md`
- Gap register: `spec-to-test-workflow/stock-dashboard/10-verification/gaps.md`
- Required run identity: test/SCN, exact command, working directory, plan and test hashes,
  timestamp, and the captured log. Reviewed, never mechanically validated by the guard.
- Existing frozen plans are not migrated or rewritten.

## 11. Freeze Sign-off

- [x] Every requirement is testable or explicitly blocked. (0 blocked)
- [x] Every expected value is concrete and sourced from the spec or the approved intent diagram.
- [x] No expected value came from code, a capture or a running app.
- [x] No blocking ambiguity remains unresolved.
- [x] Coverage targets met or justified.
- [x] Intent diagram version recorded and approved (v1, 2026-11-16).
- [x] Implementation design and native quality strategy reviewed as engineering context.
- [x] Separate evidence and gap-record locations selected; no later plan edits required.
- [x] Freeze stamp recorded. No edits after this point.
