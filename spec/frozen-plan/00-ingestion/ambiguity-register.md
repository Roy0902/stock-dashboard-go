# Phase 0 — Ambiguity Register

Feature: `stock-watchlist`

Only values with `Default assumption` enter scenario drafting. `Blocking = Yes` stops
the affected requirement from entering a frozen plan until the user decides.

Rev. 2 (2026-11-16): `AMB-...-001` (watchlist cap) is resolved as "no cap" per user
decision and is removed. IDs after 001 shift down by 1 and are never reused.

| ID | Question | Conflicting sources | Default assumption | Impact | Blocking | Decision |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| AMB-stock-watchlist-001 | What number format is shown for price and percent? | Spec fixes 2 decimals for `change`/`changePercent` only; price format unstated. | Price and percent both 2 decimals; percent shown with `%`. | Low | No | Approved 2026-11-16. |
| AMB-stock-watchlist-002 | How is a "trading day" counted for 30/90/365 points when the market is closed or a symbol is new? | Spec §4.3 says "last 30 trading days"; provider returns only available bars. | Request `outputsize` 30/90/365 and plot what the provider returns; fewer points is valid. | Low | No | Approved 2026-11-16. |
| AMB-stock-watchlist-003 | Are the 60-second cache and the 60-second UI refresh the same timer? | Spec §4.2 and §7 both state 60 seconds. | Separate: the cache TTL is 60 s per ticker; the UI polls every 60 s. | Low | No | Approved 2026-11-16. |
| AMB-stock-watchlist-004 | Is the chart a custom-drawn SVG or a chart library? | Spec §4.3 fixes behavior, not implementation. | Custom SVG line, no chart dependency. | Low | No | Engineering decision, Phase 5. |
| AMB-stock-watchlist-005 | Which exact up/down text accompanies the change? | Spec §9 requires it; copy unstated. | `up` / `down` / `unchanged` as visible text in the row. | Low | No | Approved 2026-11-16. |
| AMB-stock-watchlist-006 | Is `429 RATE_LIMITED` a distinct code or does it reuse `502 PROVIDER_UNAVAILABLE`? | Spec §5 lists `502` for provider failures; §7 adds a local 429. | `429 RATE_LIMITED` is distinct and takes precedence over the provider mapping. | Med | No | Approved 2026-11-16. |
| AMB-stock-watchlist-007 | What happens to rows whose quote was omitted by `GET /api/quotes`? | Spec §5 omits the ticker; §4.2 says the row shows `Price unavailable`. | Omitted ticker renders `Price unavailable` with provider `none`. | Low | No | Approved 2026-11-16. |
| AMB-stock-watchlist-008 | Does Twelve Data's free plan really allow historical series and at what delay? | Verified from provider docs 2026-11-16; plan terms can change. | Chart stays in scope; if the plan changes, the chart is dropped per the user's "free only" rule. | Med | No | Verified free (800/day, 8/min, 2-year US history). User: re-verify at Phase 4. |
| AMB-stock-watchlist-009 | What does "unknown symbol" mean for the chart path? | Spec §5 maps `404 UNKNOWN_SYMBOL` to adds; chart page behavior unstated. | Chart for an unknown symbol returns `502 CHART_UNAVAILABLE` and the page shows `Chart unavailable`. | Low | No | Approved 2026-11-16. |
| AMB-stock-watchlist-010 | Is a partial batch failure (one ticker out of N) a whole-request error? | Spec §5 says `GET /api/quotes` does not fail the whole request. | Per-ticker isolation; response `200` with the omitted tickers absent. | Low | No | Approved 2026-11-16. |
| AMB-stock-watchlist-011 | In what order does `GET /api/watchlist` return tickers? | Spec §5 fixes the shape, not the order. | Insertion order. | Low | No | Closed 2026-11-16: insertion order confirmed by user. |
| AMB-stock-watchlist-012 | Can the chart route show a ticker that is not in the watchlist? | Spec §4.3 does not restrict the route. | Yes; the chart route is independent of membership. | Low | No | Closed 2026-11-16: confirmed by user. |

## Resolved during ingestion

| Question | Resolution | Source |
| :-- | :-- | :-- |
| Is there a watchlist size cap? | No cap. | User decision 2026-11-16 |
| Which providers are used? | Finnhub primary quotes, Twelve Data fallback quotes and charts. | User decision 2026-11-16 |
| Is the chart in scope? | Yes; Twelve Data Basic (free) includes historical time series. | Provider docs 2026-11-16 |
| Is deployment local only? | Yes; no VPS, cloud or Docker. | User decision 2026-11-16 |
| Is storage a database? | No; local JSON file only. | User decision 2026-11-16 |
