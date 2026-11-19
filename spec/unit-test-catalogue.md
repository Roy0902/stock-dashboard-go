# Unit Test Catalogue: stock-watchlist

Status: **enumerated before development** (2026-11-16). Methods are authored next; none is
claimed as passing yet.

This catalogue is the spec-derived enumeration requested for the supporting unit tests. It is
kept next to the normative spec because the frozen acceptance plan pins the exact sha256 of
`Spec/implementation-spec.md`; editing that file would invalidate the plan's provenance. The
normative spec remains unchanged.

- Normative spec: `Spec/implementation-spec.md` v1.2, sha256 `b2d5c797ff565d79`
- Domain model: `spec-to-test-workflow/stock-dashboard/01-domain-model/domain-model.md`
- Design: `spec-to-test-workflow/stock-dashboard/05-task-lists/implementation-design.md` §5
- Frozen acceptance plan (unchanged): `local-env/specs/stock-watchlist.md`
- Runner: `node --experimental-transform-types --test tests-unit/` from `stock-dashboard/`
- Enum source: `tests-unit/catalogue.ts` (generated from `scripts/unit-catalogue.mjs`)

These tests support the domain and adapters. They never replace the frozen acceptance coverage
and they do not create product behaviour that the spec does not state.

## 1. Enum

```ts
export enum UnitTestId {
  TICKER_NORMALISES_LOWERCASE = 'UT-stock-watchlist-001',
  TICKER_ACCEPTS_SINGLE_LETTER = 'UT-stock-watchlist-002',
  TICKER_ACCEPTS_FOUR_CHARS = 'UT-stock-watchlist-003',
  TICKER_ACCEPTS_TEN_CHARS = 'UT-stock-watchlist-004',
  TICKER_REJECTS_EMPTY = 'UT-stock-watchlist-005',
  TICKER_REJECTS_LEADING_DIGIT = 'UT-stock-watchlist-006',
  TICKER_REJECTS_HYPHEN = 'UT-stock-watchlist-007',
  TICKER_REJECTS_DOT = 'UT-stock-watchlist-008',
  TICKER_REJECTS_SPACE = 'UT-stock-watchlist-009',
  TICKER_REJECTS_ELEVEN_CHARS = 'UT-stock-watchlist-010',
  TICKER_REJECTS_NON_ASCII = 'UT-stock-watchlist-011',
  WATCHLIST_ADDS_UNSEEN = 'UT-stock-watchlist-012',
  WATCHLIST_REJECTS_DUPLICATE = 'UT-stock-watchlist-013',
  WATCHLIST_HAS_NO_CAP = 'UT-stock-watchlist-014',
  WATCHLIST_PRESERVES_INSERTION_ORDER = 'UT-stock-watchlist-015',
  WATCHLIST_REMOVES_LISTED = 'UT-stock-watchlist-016',
  WATCHLIST_REJECTS_UNLISTED_REMOVE = 'UT-stock-watchlist-017',
  WATCHLIST_CANONICALISES_BEFORE_MEMBERSHIP = 'UT-stock-watchlist-018',
  QUOTE_DERIVES_CHANGE = 'UT-stock-watchlist-019',
  QUOTE_ROUNDS_CHANGE = 'UT-stock-watchlist-020',
  QUOTE_DERIVES_PERCENT = 'UT-stock-watchlist-021',
  QUOTE_DERIVES_NEGATIVE_PERCENT = 'UT-stock-watchlist-022',
  QUOTE_DERIVES_ZERO_PERCENT = 'UT-stock-watchlist-023',
  QUOTE_DIRECTION_UP = 'UT-stock-watchlist-024',
  QUOTE_DIRECTION_DOWN = 'UT-stock-watchlist-025',
  QUOTE_DIRECTION_UNCHANGED = 'UT-stock-watchlist-026',
  QUOTE_STALE_AFTER_15_MINUTES = 'UT-stock-watchlist-027',
  QUOTE_FRESH_AT_15_MINUTES = 'UT-stock-watchlist-028',
  QUOTE_FRESH_BEFORE_15_MINUTES = 'UT-stock-watchlist-029',
  QUOTE_ZERO_PREVIOUS_CLOSE = 'UT-stock-watchlist-030',
  QUOTE_MISSING_PREVIOUS_CLOSE = 'UT-stock-watchlist-031',
  QUOTE_NEGATIVE_PREVIOUS_CLOSE = 'UT-stock-watchlist-032',
  SERIES_POINTS_FOR_1M = 'UT-stock-watchlist-033',
  SERIES_POINTS_FOR_3M = 'UT-stock-watchlist-034',
  SERIES_POINTS_FOR_1Y = 'UT-stock-watchlist-035',
  SERIES_DEFAULTS_TO_1M = 'UT-stock-watchlist-036',
  SERIES_REJECTS_INVALID_RANGE = 'UT-stock-watchlist-037',
  SERIES_ORDERS_OLDEST_FIRST = 'UT-stock-watchlist-038',
  ERROR_HAS_STABLE_CODE = 'UT-stock-watchlist-039',
  ERROR_MAPS_INVALID_TICKER = 'UT-stock-watchlist-040',
  ERROR_MAPS_DUPLICATE_TICKER = 'UT-stock-watchlist-041',
  ERROR_MAPS_UNKNOWN_SYMBOL = 'UT-stock-watchlist-042',
  ERROR_MAPS_NOT_IN_WATCHLIST = 'UT-stock-watchlist-043',
  ERROR_MAPS_MISSING_API_KEY = 'UT-stock-watchlist-044',
  ERROR_MAPS_PROVIDER_UNAVAILABLE = 'UT-stock-watchlist-045',
  ERROR_MAPS_RATE_LIMITED = 'UT-stock-watchlist-046',
  ERROR_MAPS_INVALID_RANGE = 'UT-stock-watchlist-047',
  ERROR_MAPS_CHART_UNAVAILABLE = 'UT-stock-watchlist-048',
  ERROR_ENVELOPE_EXACT = 'UT-stock-watchlist-049',
  ERROR_NEVER_CONTAINS_PROVIDER_DETAIL = 'UT-stock-watchlist-050',
  BUDGET_FINNHUB_ALLOWS_55 = 'UT-stock-watchlist-051',
  BUDGET_TWELVE_DATA_ALLOWS_7 = 'UT-stock-watchlist-052',
  BUDGET_WINDOW_SLIDES_AFTER_60S = 'UT-stock-watchlist-053',
  BUDGET_RETRY_AFTER_SECONDS = 'UT-stock-watchlist-054',
  BUDGET_IS_PER_PROVIDER = 'UT-stock-watchlist-055',
  STORE_LOADS_EMPTY_WHEN_FILE_MISSING = 'UT-stock-watchlist-056',
  STORE_ROUND_TRIPS_TICKERS = 'UT-stock-watchlist-057',
  STORE_PRESERVES_ORDER = 'UT-stock-watchlist-058',
  STORE_WRITES_ONLY_WATCHLIST_JSON = 'UT-stock-watchlist-059',
  STORE_REPLACES_ATOMICALLY = 'UT-stock-watchlist-060',
  STORE_SURVIVES_NEW_INSTANCE = 'UT-stock-watchlist-061',
}
```

## 2. Test methods

| ID | Enum member | File | Unit under test | Spec source | Scenario | Expected outcome |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| `UT-stock-watchlist-001` | `TICKER_NORMALISES_LOWERCASE` | `ticker.test.ts` | `toTicker` | §4.1, REQ-002 | `SCN-002` | 'aapl' -> 'AAPL' |
| `UT-stock-watchlist-002` | `TICKER_ACCEPTS_SINGLE_LETTER` | `ticker.test.ts` | `toTicker` | §4.1, REQ-003 | `SCN-003` | 'A' -> 'A' |
| `UT-stock-watchlist-003` | `TICKER_ACCEPTS_FOUR_CHARS` | `ticker.test.ts` | `toTicker` | §4.1, REQ-003 | `SCN-003` | 'AAPL' -> 'AAPL' |
| `UT-stock-watchlist-004` | `TICKER_ACCEPTS_TEN_CHARS` | `ticker.test.ts` | `toTicker` | §4.1, REQ-003 | `SCN-003` | 'A123456789' -> 'A123456789' |
| `UT-stock-watchlist-005` | `TICKER_REJECTS_EMPTY` | `ticker.test.ts` | `toTicker` | §4.1, REQ-004 | `SCN-004` | '' -> invalid |
| `UT-stock-watchlist-006` | `TICKER_REJECTS_LEADING_DIGIT` | `ticker.test.ts` | `toTicker` | §4.1, REQ-004 | `SCN-004` | '1AAPL' -> invalid |
| `UT-stock-watchlist-007` | `TICKER_REJECTS_HYPHEN` | `ticker.test.ts` | `toTicker` | §4.1, REQ-004 | `SCN-004` | 'AA-PL' -> invalid |
| `UT-stock-watchlist-008` | `TICKER_REJECTS_DOT` | `ticker.test.ts` | `toTicker` | §4.1, REQ-004 | `SCN-004` | 'A.B' -> invalid |
| `UT-stock-watchlist-009` | `TICKER_REJECTS_SPACE` | `ticker.test.ts` | `toTicker` | §4.1, REQ-004 | `SCN-004` | 'AA PL' -> invalid |
| `UT-stock-watchlist-010` | `TICKER_REJECTS_ELEVEN_CHARS` | `ticker.test.ts` | `toTicker` | §4.1, REQ-004 | `SCN-004` | 'A1234567890' -> invalid |
| `UT-stock-watchlist-011` | `TICKER_REJECTS_NON_ASCII` | `ticker.test.ts` | `toTicker` | §4.1, REQ-004 | `SCN-004` | 'Ä' -> invalid |
| `UT-stock-watchlist-012` | `WATCHLIST_ADDS_UNSEEN` | `watchlist.test.ts` | `addTicker` | §4.1, REQ-006 | `SCN-006` | [] + 'AAPL' -> ['AAPL'] |
| `UT-stock-watchlist-013` | `WATCHLIST_REJECTS_DUPLICATE` | `watchlist.test.ts` | `addTicker` | §4.1, REQ-005 | `SCN-005` | ['AAPL'] + 'aapl' -> duplicate |
| `UT-stock-watchlist-014` | `WATCHLIST_HAS_NO_CAP` | `watchlist.test.ts` | `addTicker` | §4.1, REQ-013 | `SCN-013` | 1,000 valid adds all accepted |
| `UT-stock-watchlist-015` | `WATCHLIST_PRESERVES_INSERTION_ORDER` | `watchlist.test.ts` | `addTicker` | §4.1, REQ-006 | `SCN-006` | MSFT then AAPL -> ['MSFT','AAPL'] |
| `UT-stock-watchlist-016` | `WATCHLIST_REMOVES_LISTED` | `watchlist.test.ts` | `removeTicker` | §4.1, REQ-008 | `SCN-008` | remove AAPL -> ['MSFT'] |
| `UT-stock-watchlist-017` | `WATCHLIST_REJECTS_UNLISTED_REMOVE` | `watchlist.test.ts` | `removeTicker` | §4.1, REQ-009 | `SCN-009` | remove absent -> notInWatchlist |
| `UT-stock-watchlist-018` | `WATCHLIST_CANONICALISES_BEFORE_MEMBERSHIP` | `watchlist.test.ts` | `addTicker` | §4.1, REQ-002/005 | `SCN-002, SCN-005` | 'aapl' then 'AAPL' -> duplicate |
| `UT-stock-watchlist-019` | `QUOTE_DERIVES_CHANGE` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-015 | `SCN-015` | 190.12 - 188.88 -> 1.24 |
| `UT-stock-watchlist-020` | `QUOTE_ROUNDS_CHANGE` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-015 | `SCN-015` | 190.125 - 188.00 -> 2.13 |
| `UT-stock-watchlist-021` | `QUOTE_DERIVES_PERCENT` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-016 | `SCN-016` | 1.24 / 188.88 * 100 -> 0.66 |
| `UT-stock-watchlist-022` | `QUOTE_DERIVES_NEGATIVE_PERCENT` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-016 | `SCN-016` | -10 / 200 * 100 -> -5.00 |
| `UT-stock-watchlist-023` | `QUOTE_DERIVES_ZERO_PERCENT` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-016 | `SCN-016` | 0 / 100 * 100 -> 0.00 |
| `UT-stock-watchlist-024` | `QUOTE_DIRECTION_UP` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-014/029 | `SCN-031` | change > 0 -> up |
| `UT-stock-watchlist-025` | `QUOTE_DIRECTION_DOWN` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-014/029 | `SCN-031` | change < 0 -> down |
| `UT-stock-watchlist-026` | `QUOTE_DIRECTION_UNCHANGED` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-014/029 | `SCN-031` | change = 0 -> unchanged |
| `UT-stock-watchlist-027` | `QUOTE_STALE_AFTER_15_MINUTES` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-023 | `SCN-023` | age 16 min -> stale true |
| `UT-stock-watchlist-028` | `QUOTE_FRESH_AT_15_MINUTES` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-023 | `SCN-024` | age exactly 15 min -> stale false |
| `UT-stock-watchlist-029` | `QUOTE_FRESH_BEFORE_15_MINUTES` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-023 | `SCN-024` | age 14 min -> stale false |
| `UT-stock-watchlist-030` | `QUOTE_ZERO_PREVIOUS_CLOSE` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-052 | `SCN-032` | previousClose 0 -> price 0, change 0, percent null |
| `UT-stock-watchlist-031` | `QUOTE_MISSING_PREVIOUS_CLOSE` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-052 | `SCN-033` | no previousClose -> supplied price, change 0, percent null |
| `UT-stock-watchlist-032` | `QUOTE_NEGATIVE_PREVIOUS_CLOSE` | `quote.test.ts` | `deriveQuote` | §4.2, REQ-052 | `SCN-033` | negative previousClose -> no percent |
| `UT-stock-watchlist-033` | `SERIES_POINTS_FOR_1M` | `priceSeries.test.ts` | `pointsFor` | §4.3, REQ-031/033 | `SCN-035` | '1M' -> 30 |
| `UT-stock-watchlist-034` | `SERIES_POINTS_FOR_3M` | `priceSeries.test.ts` | `pointsFor` | §4.3, REQ-031 | `SCN-038` | '3M' -> 90 |
| `UT-stock-watchlist-035` | `SERIES_POINTS_FOR_1Y` | `priceSeries.test.ts` | `pointsFor` | §4.3, REQ-031 | `SCN-038` | '1Y' -> 365 |
| `UT-stock-watchlist-036` | `SERIES_DEFAULTS_TO_1M` | `priceSeries.test.ts` | `resolveRange` | §4.3, REQ-033 | `SCN-037` | undefined -> 1M |
| `UT-stock-watchlist-037` | `SERIES_REJECTS_INVALID_RANGE` | `priceSeries.test.ts` | `resolveRange` | §4.3, REQ-034 | `SCN-038` | '2M', '', '1m' -> INVALID_RANGE |
| `UT-stock-watchlist-038` | `SERIES_ORDERS_OLDEST_FIRST` | `priceSeries.test.ts` | `orderOldestFirst` | §6.2, REQ-038 | `SCN-042` | newest-first input -> oldest-first |
| `UT-stock-watchlist-039` | `ERROR_HAS_STABLE_CODE` | `errors.test.ts` | `DomainError` | §5, REQ-004 | `SCN-004` | code and copy are carried |
| `UT-stock-watchlist-040` | `ERROR_MAPS_INVALID_TICKER` | `errors.test.ts` | `toHttpError` | §5, REQ-004 | `SCN-004` | 400 INVALID_TICKER |
| `UT-stock-watchlist-041` | `ERROR_MAPS_DUPLICATE_TICKER` | `errors.test.ts` | `toHttpError` | §5, REQ-005 | `SCN-005` | 409 DUPLICATE_TICKER |
| `UT-stock-watchlist-042` | `ERROR_MAPS_UNKNOWN_SYMBOL` | `errors.test.ts` | `toHttpError` | §5, REQ-007 | `SCN-007` | 404 UNKNOWN_SYMBOL |
| `UT-stock-watchlist-043` | `ERROR_MAPS_NOT_IN_WATCHLIST` | `errors.test.ts` | `toHttpError` | §5, REQ-009 | `SCN-009` | 404 NOT_IN_WATCHLIST |
| `UT-stock-watchlist-044` | `ERROR_MAPS_MISSING_API_KEY` | `errors.test.ts` | `toHttpError` | §5, REQ-041 | `SCN-046` | 503 MISSING_API_KEY |
| `UT-stock-watchlist-045` | `ERROR_MAPS_PROVIDER_UNAVAILABLE` | `errors.test.ts` | `toHttpError` | §5, REQ-042 | `SCN-047` | 502 PROVIDER_UNAVAILABLE |
| `UT-stock-watchlist-046` | `ERROR_MAPS_RATE_LIMITED` | `errors.test.ts` | `toHttpError` | §5, REQ-044 | `SCN-050` | 429 RATE_LIMITED with retry seconds |
| `UT-stock-watchlist-047` | `ERROR_MAPS_INVALID_RANGE` | `errors.test.ts` | `toHttpError` | §5, REQ-034 | `SCN-038` | 400 INVALID_RANGE |
| `UT-stock-watchlist-048` | `ERROR_MAPS_CHART_UNAVAILABLE` | `errors.test.ts` | `toHttpError` | §5, REQ-035 | `SCN-039` | 502 CHART_UNAVAILABLE |
| `UT-stock-watchlist-049` | `ERROR_ENVELOPE_EXACT` | `errors.test.ts` | `toHttpError` | §5 | `SCN-047` | { error, code } only |
| `UT-stock-watchlist-050` | `ERROR_NEVER_CONTAINS_PROVIDER_DETAIL` | `errors.test.ts` | `toHttpError` | §3, REQ-047 | `SCN-053` | fixed copy, no upstream message |
| `UT-stock-watchlist-051` | `BUDGET_FINNHUB_ALLOWS_55` | `rateBudget.test.ts` | `RateBudget` | §7, REQ-043 | `SCN-048` | 55 allowed, 56th refused |
| `UT-stock-watchlist-052` | `BUDGET_TWELVE_DATA_ALLOWS_7` | `rateBudget.test.ts` | `RateBudget` | §7, REQ-043 | `SCN-049` | 7 allowed, 8th refused |
| `UT-stock-watchlist-053` | `BUDGET_WINDOW_SLIDES_AFTER_60S` | `rateBudget.test.ts` | `RateBudget` | §7, REQ-043 | `SCN-048/049` | slot frees after 60 s |
| `UT-stock-watchlist-054` | `BUDGET_RETRY_AFTER_SECONDS` | `rateBudget.test.ts` | `RateBudget` | §7, REQ-044 | `SCN-050` | positive retry seconds |
| `UT-stock-watchlist-055` | `BUDGET_IS_PER_PROVIDER` | `rateBudget.test.ts` | `RateBudget` | §7, REQ-043 | `SCN-048/049` | two instances do not share slots |
| `UT-stock-watchlist-056` | `STORE_LOADS_EMPTY_WHEN_FILE_MISSING` | `watchlistStore.test.ts` | `loadWatchlist` | §4.1, REQ-010 | `SCN-001` | missing file -> [] |
| `UT-stock-watchlist-057` | `STORE_ROUND_TRIPS_TICKERS` | `watchlistStore.test.ts` | `saveWatchlist` | §4.1, REQ-010 | `SCN-010` | save then load -> same tickers |
| `UT-stock-watchlist-058` | `STORE_PRESERVES_ORDER` | `watchlistStore.test.ts` | `saveWatchlist` | §4.1, REQ-006 | `SCN-006` | order survives round-trip |
| `UT-stock-watchlist-059` | `STORE_WRITES_ONLY_WATCHLIST_JSON` | `watchlistStore.test.ts` | `saveWatchlist` | §8, REQ-049 | `SCN-055` | only watchlist.json exists |
| `UT-stock-watchlist-060` | `STORE_REPLACES_ATOMICALLY` | `watchlistStore.test.ts` | `saveWatchlist` | design BE-011 | `SCN-010` | no temp file remains after save |
| `UT-stock-watchlist-061` | `STORE_SURVIVES_NEW_INSTANCE` | `watchlistStore.test.ts` | `loadWatchlist` | §4.1, REQ-010 | `SCN-010` | a new instance reads the saved file |

## 3. File map

| Unit-test file | Enum range | Responsibility |
| :-- | :-- | :-- |
| `tests-unit/ticker.test.ts` | `UT-001`–`UT-011` | Canonical form and the format boundary table. |
| `tests-unit/watchlist.test.ts` | `UT-012`–`UT-018` | Membership, uniqueness, no cap, insertion order. |
| `tests-unit/quote.test.ts` | `UT-019`–`UT-032` | Derivation, rounding, direction, staleness, zero/no baseline. |
| `tests-unit/priceSeries.test.ts` | `UT-033`–`UT-038` | Range sizes, default, validation, oldest-first ordering. |
| `tests-unit/errors.test.ts` | `UT-039`–`UT-050` | Stable codes and the single HTTP mapping table. |
| `tests-unit/rateBudget.test.ts` | `UT-051`–`UT-055` | Sliding window, limits, retry seconds, per-provider isolation. |
| `tests-unit/watchlistStore.test.ts` | `UT-056`–`UT-061` | JSON round-trip, atomic replace, only-watchlist persistence. |

## 4. Rules

- A unit test asserts only what the spec or the approved design states.
- A gap found while authoring a method becomes a recorded gap; it is never filled by observing
  the implementation.
- The acceptance plan is never edited. A product gap found by these tests follows the
  post-freeze gap protocol.
- Unit tests may be added or updated during implementation; they are outside the protected
  acceptance paths. They cannot satisfy missing acceptance coverage.
