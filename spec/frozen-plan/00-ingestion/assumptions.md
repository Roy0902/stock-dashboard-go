# Phase 0 — Assumptions and Decisions

Feature: `stock-watchlist`
Source: `Spec/implementation-spec.md` v1.1, provider documentation, and user decisions.

## User decisions

| # | Decision | Effect |
| :-- | :-- | :-- |
| 1 | Project: own idea — a stock dashboard. | Scope in `Spec/implementation-spec.md`. |
| 2 | UI as well as API. | Both UI and API acceptance levels. |
| 3 | Free data only. Any feature that needs a paid plan is dropped. | No paid candles; chart uses Twelve Data free tier. |
| 4 | Fallback provider prepared. | Finnhub → Twelve Data for quotes. |
| 5 | Local only; no VPS, cloud or Docker. | Server binds `127.0.0.1`; JSON file storage. |
| 6 | Keys supplied by the user through environment variables. | No secrets in the repo. |
| 7 | No watchlist cap (2026-11-16, rev. 2). | REQ-005/-006 `WATCHLIST_FULL` removed. |
| 8 | Two decimals, cache/refresh timers, 429 distinct, chart error code approved 2026-11-16. | AMB-001, -003, -005, -006, -007, -009, -010 approved. |
| 9 | Twelve Data free-plan scope to be re-checked in Phase 4 (2026-11-16). | AMB-008 stays open until Phase 4. |
| 10 | Zero previous close shows price and change `0` with no percentage (2026-11-16, rev. 3). | REQ-052, INV-10. |
| 11 | Direction word `up`/`down`/`unchanged` is visible row text (2026-11-16). | REQ-014, REQ-029. |
| 12 | Toolchain approved: Prettier, ESLint, TypeScript, Playwright, guard wiring, `local-env/` harness (2026-11-16). | Implementation design §6, backend tasks BE-001/BE-017. |
| 13 | List order is insertion order; chart route works for unlisted tickers (2026-11-16). | AMB-011, AMB-012 closed; SCN-006, SCN-044. |

## Provider facts used (verified 2026-11-16)

| Fact | Source | Consequence |
| :-- | :-- | :-- |
| Finnhub free key, 60 calls/min, US quotes real-time. | finnhub.io pricing / docs | Primary quote source. |
| Finnhub stock candles (OHLCV) are Premium. | finnhub.io/docs/api/stock-candles | Chart cannot use Finnhub. |
| Twelve Data Basic: 8 credits/min, 800/day, historical time series, 2-year US history. | twelvedata.com pricing, support article 5335783 | Fallback quotes and chart source. |
| Twelve Data free plan is internal use. | Massive vs Twelve Data comparison | Acceptable for a local personal tool. |

## Assumptions

| ID | Assumption | Where used |
| :-- | :-- | :-- |
| AMB-stock-watchlist-001 | Price and percent show 2 decimals. | REQ-014 |
| AMB-stock-watchlist-002 | "Trading days" means whatever bars the provider returns for the requested size. | REQ-031 |
| AMB-stock-watchlist-003 | Cache TTL and UI refresh are separate 60-second timers. | REQ-024, REQ-025 |
| AMB-stock-watchlist-004 | Chart is custom SVG, no library. | Phase 5 |
| AMB-stock-watchlist-005 | Visible `up` / `down` / `unchanged` text accompanies the change. | REQ-029 |
| AMB-stock-watchlist-006 | Local rate limit returns `429 RATE_LIMITED`, distinct from `502`. | REQ-044 |
| AMB-stock-watchlist-007 | Omitted quote renders `Price unavailable`. | REQ-027 |
| AMB-stock-watchlist-008 | Chart stays in scope while Twelve Data free tier allows it. | REQ-031, Phase 4 re-check |
| AMB-stock-watchlist-009 | Unknown symbol on the chart path maps to `CHART_UNAVAILABLE`. | REQ-035 |
| AMB-stock-watchlist-010 | Per-ticker isolation for the quotes batch. | REQ-027 |

## Testing posture

- Acceptance tests run against a local fake provider, not the live APIs, so expected values
  are deterministic and no quota is consumed.
- Live-provider behavior is a manual smoke check only; it is never an oracle.
- Existing tools: Node.js, TypeScript, Playwright, `vasu-playwright-utils`, `playwright-cli`
  (Phase 4 confirms the exact versions and paths).
