# Phase 1 — Domain Model

Feature: `stock-watchlist`
Date: 2026-11-16

## 0. Sources and Traceability

- Spec: `Spec/implementation-spec.md` v1.1 (`cce969cfe59e2241`)
- Registers: `00-ingestion/requirement-register.md` (`57c9832134b731b7`), `ambiguity-register.md` (`b495b9b5a8ea466f`)
- Requirements covered: REQ-stock-watchlist-001 … 051 (all)
- Diagrams: none yet. Phase 2 will produce them; any unlabeled guard found then becomes a new `AMB`.
- Blocking ambiguities: none.

## 1. Ubiquitous Language

- **Watchlist:** The user's personally curated set of tickers. One watchlist per installation. It has no size limit (REQ-013).
- **Ticker:** A market symbol identifying a listed instrument. Canonical form is uppercase, 1–10 alphanumeric characters, first character a letter (REQ-002, REQ-003).
- **Watch Entry:** One listed ticker in the watchlist. It is either present or removed; there is no other state.
- **Quote:** The latest price observation for a ticker: price, previous close, change, change percent, observation time, and the provider that supplied it (REQ-014).
- **Provider:** An external market-data service. `Primary` is Finnhub; `Secondary` is Twelve Data (REQ-020, REQ-021, spec §4.2).
- **Provider Attribution:** The provider that supplied the quote currently shown. Its value is `Primary`, `Secondary` or `none` (REQ-021, REQ-022).
- **Fresh / Stale:** A quote is fresh when its observation time is at most 15 minutes old and stale otherwise (REQ-023).
- **Price Change:** `price - previousClose`, to 2 decimals (REQ-015).
- **Change Percent:** `priceChange / previousClose * 100`, to 2 decimals (REQ-016).
- **Price Series:** An ordered daily series of close prices for one ticker and one range covering the last 30, 90 or 365 trading days (REQ-031, REQ-038).
- **Price Range:** One of `1M`, `3M`, `1Y`; `1M` is the default (REQ-033).
- **Verdict:** The outcome of offering a ticker to the watchlist: `Accepted`, `Duplicate`, `Invalid`, `UnknownSymbol` (REQ-003 to REQ-007).
- **Rate Budget:** The maximum upstream calls per minute: 55 for Primary, 7 for Secondary (REQ-043).
- **Observation Time:** The provider's timestamp for a quote, not the time the app received it (REQ-014, REQ-023).

## 2. Aggregates and Entities

### Aggregate Root: `Watchlist`

What it represents: the curated set of tickers and the sole authority over its own membership.

- **Properties / State:** an unordered set of `WatchEntry`; no maximum size.
- **Value Objects:** `Ticker`, `TickerVerdict`, `WatchEntry`.
- **Guard:** `Watchlist` alone decides whether a ticker can be added, and it rejects duplicates and invalid forms before anything else happens (REQ-003, REQ-005, REQ-013).
- **Lifecycle:** created empty on first use (REQ-001); membership changes only through `AddTicker` and `RemoveTicker` (REQ-006, REQ-008).

### Entity: `WatchEntry`

- **Properties:** `Ticker` (canonical uppercase), present or removed.
- **Invariant:** a ticker appears at most once in the watchlist (REQ-005).

### Entity: `Quote`

- **Properties:** `Ticker`, `price`, `previousClose`, `priceChange`, `changePercent`, `observationTime`, `providerAttribution`, `stale`.
- **Value Objects:** `Price`, `PriceChange`, `ChangePercent`, `ObservationTime`, `ProviderAttribution`.
- **Derivation rules:** change and change percent are always derived from price and previous close, never supplied independently (REQ-015, REQ-016).
- **Invariant:** a quote exists only when at least one provider supplied a usable observation (REQ-020 to REQ-022).

### Entity: `PriceSeries`

- **Properties:** `Ticker`, `PriceRange`, `points[]` (date, close), `source`.
- **Invariant:** points are stored oldest to newest (REQ-038).
- **Rule:** the source is always Secondary, because the chart cannot use Primary (REQ-032, spec §4.3).

### Entity: `RateBudget`

- **Properties:** `provider`, `maxCallsPerMinute`, `callsInWindow`.
- **Invariant:** a call is permitted only while `callsInWindow < maxCallsPerMinute` (REQ-043, REQ-044).

### Entity: `ConnectionSettings`

- **Properties:** which provider credentials are present; no credential values are part of the domain.
- **Rule:** quote and price-series demand is unsatisfiable while the required credentials are absent (REQ-040, REQ-041).

## 3. Business Rules (Invariants)

| # | Invariant | REQ |
| :-- | :-- | :-- |
| INV-01 | A ticker is canonicalised before any comparison, so `aapl` and `AAPL` are the same entry. | REQ-002 |
| INV-02 | A ticker that is not 1–10 alphanumerics starting with a letter is never accepted. | REQ-003, REQ-004 |
| INV-03 | A watchlist never contains the same ticker twice. | REQ-005 |
| INV-04 | A watchlist has no maximum size. | REQ-013 |
| INV-05 | A ticker the provider does not recognise never becomes a watch entry. | REQ-007, REQ-045 |
| INV-06 | Membership changes persist across restarts; nothing else is persisted. | REQ-010, REQ-049 |
| INV-07 | A quote is attributed to Primary when Primary supplied it and to Secondary only when Primary failed for that ticker. | REQ-020, REQ-021 |
| INV-08 | A ticker with no usable observation from either provider is reported unavailable, never with an invented price. | REQ-022, REQ-042, REQ-046 |
| INV-09 | Price change and change percent are always derived, never independent, and always rounded to 2 decimals. | REQ-015, REQ-016 |
| INV-10 | A previous close of zero yields price and change `0` with no percentage; a missing or negative previous close yields the supplied price with no percentage. A percentage is never invented. | REQ-052 |
| INV-11 | A quote is stale once its observation time is more than 15 minutes old, permanently for that observation. | REQ-023 |
| INV-12 | No ticker demands an upstream observation more than once per 60 seconds. | REQ-024 |
| INV-13 | Upstream demand never exceeds the rate budget; excess demand is refused, not queued indefinitely. | REQ-043, REQ-044 |
| INV-14 | A price series always comes from Secondary, and `1M` is used when no range is chosen. | REQ-032, REQ-033 |
| INV-15 | Price-series points are ordered oldest to newest. | REQ-038 |
| INV-16 | Provider error detail never leaves the domain; the user sees only approved copy. | REQ-047 |
| INV-17 | No market data is persisted; only watchlist membership is. | REQ-049 |
| INV-18 | All outbound demand goes only to the two named providers. | Spec §8 |

## 4. Domain Events

| Event | Meaning | REQ |
| :-- | :-- | :-- |
| `WatchlistCreated` | The user used the dashboard for the first time. | REQ-001 |
| `TickerAccepted` | A valid, unseen, provider-recognised ticker entered the watchlist. | REQ-003, REQ-006 |
| `TickerRejected` | A ticker was refused as duplicate, invalid or unknown. | REQ-004, REQ-005, REQ-007 |
| `TickerRemoved` | A ticker left the watchlist. | REQ-008, REQ-009 |
| `QuoteAccepted` | At least one provider supplied a usable observation for a ticker. | REQ-020, REQ-021 |
| `QuoteUnavailable` | No provider supplied a usable observation for a ticker. | REQ-022 |
| `QuoteBecameStale` | An accepted observation passed the 15-minute freshness limit. | REQ-023 |
| `RateBudgetExhausted` | A provider's per-minute allowance was reached and demand was refused. | REQ-043, REQ-044 |
| `ChartSeriesAccepted` | A price series for a ticker and range was obtained from Secondary. | REQ-032, REQ-039 |
| `ChartSeriesUnavailable` | No price series could be obtained. | REQ-035 |

## 5. State Transitions

### Quote lifecycle

| From | To | Guard | REQ |
| :-- | :-- | :-- | :-- |
| Absent | Fresh | Primary supplied a usable observation | REQ-020 |
| Absent | Fresh | Primary failed and Secondary supplied a usable observation | REQ-021 |
| Absent | Unavailable | both providers failed | REQ-022 |
| Fresh | Fresh | a refresh inside 60 s serves the cached observation | REQ-024 |
| Fresh | Fresh | a refresh after 60 s replaces the observation | REQ-024 |
| Absent | Fresh (zero baseline) | previous close is zero, so price and change are `0` and no percentage is shown | REQ-052 |
| Absent | Fresh (no baseline) | previous close is missing or negative, so no percentage is shown | REQ-052 |
| Fresh | Stale | observation age exceeds 15 minutes | REQ-023 |
| Stale | Fresh | a newer usable observation arrives | REQ-023, REQ-024 |
| Unavailable | Fresh | a later refresh succeeds | REQ-021 |

### Watch entry lifecycle

| From | To | Guard | REQ |
| :-- | :-- | :-- | :-- |
| Absent | Listed | valid, unseen, provider-recognised ticker | REQ-003, REQ-006 |
| Absent | Absent | invalid form | REQ-004 |
| Absent | Absent | provider does not recognise the ticker | REQ-007 |
| Listed | Listed | duplicate offer is refused, membership unchanged | REQ-005 |
| Listed | Removed | user removes the ticker | REQ-008 |
| Removed | Removed | removal of a ticker that is not listed is refused | REQ-009 |

### Price series lifecycle

| From | To | Guard | REQ |
| :-- | :-- | :-- | :-- |
| Absent | Available | Secondary returned at least one point for the requested range | REQ-031, REQ-039 |
| Absent | Available | no range chosen, so `1M` was used | REQ-033 |
| Absent | Unavailable | Secondary failed, returned no points, or the range is not one of `1M`/`3M`/`1Y` | REQ-034, REQ-035 |

## 6. Invariant-to-requirement gaps

Each invariant has at least one scenario obligation in `00-ingestion/coverage-plan.md`. No new
ambiguity was needed for Phase 1: the only unstated values are the exact up/down copy
(`AMB-stock-watchlist-005`) and the number format (`AMB-stock-watchlist-001`), both already
registered and approved.
