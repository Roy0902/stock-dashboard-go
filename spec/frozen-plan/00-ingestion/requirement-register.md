# Phase 0 — Requirement Register

Feature: `stock-watchlist`
Source: `Spec/implementation-spec.md` v1.2 (2026-11-16)

**Rules applied**

- One rule per row. Compound spec sentences are split.
- Expected values are quoted from the spec only. No value here comes from code, the
  providers' live responses or the running app.
- `testable-with-assumption` means the spec value exists but depends on an `AMB-*` default.
- Requirements with no concrete expectation are `blocked` and do not enter the frozen plan.
- Rev. 2 (2026-11-16): watchlist cap removed per user decision. REQ-...-005 and -006
  (`WATCHLIST_FULL`) are deleted; IDs after 006 shift down by 2. IDs are never reused.
- Rev. 3 (2026-11-16): add REQ-...-052 for zero/missing previous close.

## Watchlist (WE)

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

## Quotes (QU)

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

## Chart (CH)

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

## Providers, limits and errors (PR)

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

## Platform and testability (PT)

| ID | Requirement | Type | Source | Level | Testability |
| :-- | :-- | :-- | :-- | :-- | :-- |
| REQ-stock-watchlist-048 | The server listens on `127.0.0.1` only, default port `3000`. | Contract | spec §3 | API | testable |
| REQ-stock-watchlist-049 | The only persisted data is `$DATA_DIR/watchlist.json`; quotes and charts stay in memory. | Data | spec §8 | API | testable |
| REQ-stock-watchlist-050 | Provider base URLs come from `FINNHUB_BASE_URL` and `TWELVEDATA_BASE_URL`. | Contract | spec §10 | API | testable |
| REQ-stock-watchlist-051 | `GET /healthz` returns `200 { "status": "ok" }` and contacts no provider. | Contract | spec §10 | API | testable |
| REQ-stock-watchlist-052 | When `previousClose` is zero, price and change are `0` and no percentage is shown; when `previousClose` is missing or negative, price is shown as supplied, change as supplied or `0`, and no percentage is shown. | Boundary | spec §4.2, §5 | Both | testable |

## Verdict summary

| Metric | Count |
| :-- | :-- |
| Requirements | 52 |
| Testable | 49 |
| Testable with assumption | 3 |
| Blocked | 0 |
