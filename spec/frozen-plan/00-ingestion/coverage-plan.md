# Phase 0 — Coverage Plan

Feature: `stock-watchlist`

## Counts

| Metric | Count |
| :-- | :-- |
| Requirements total | 52 |
| Testable | 49 |
| Testable with assumption | 3 |
| Blocked | 0 |
| Ambiguities | 10 (1 approved, 9 open-but-non-blocking; AMB-008 re-checked in Phase 4) |
| Blocking ambiguities | 0 |

## Transitions and provider paths to cover

| From | To | Guard / condition | Planned scenario |
| :-- | :-- | :-- | :-- |
| Empty watchlist | Populated | valid new ticker | positive add |
| Populated | Populated | duplicate ticker | duplicate error |
| Populated | Populated | invalid format | validation error |
| Populated | Populated | unknown symbol | unknown error |
| Populated | Populated | remove existing | remove success |
| Populated | Populated | remove missing | not-in-watchlist error |
| Populated | Large | many tickers added | no-cap boundary (REQ-013) |
| Finnhub ok | Quote | `c > 0` | primary path |
| Finnhub fail | Twelve Data ok | provider error / timeout | fallback path |
| Finnhub fail | Twelve Data fail | both error | unavailable path |
| Cached | Cached | TTL < 60 s | no upstream call |
| Cached | Fresh | TTL >= 60 s | upstream call |
| Quote fresh | Quote stale | age > 15 min | stale suffix |
| Chart available | Chart shown | points present | chart path |
| Chart unavailable | Error state | provider error | chart unavailable |
| Local limit ok | Limited | > 55 / > 7 per minute | `429 RATE_LIMITED` |

## Endpoints to cover

| Endpoint | Planned API tests |
| :-- | :-- |
| `GET /api/watchlist` | empty, populated |
| `POST /api/watchlist` | valid, uppercase, duplicate, invalid, unknown |
| `DELETE /api/watchlist/:ticker` | existing, missing |
| `GET /api/quotes` | primary, fallback, both fail, cache, stale, missing key |
| `GET /api/chart/:ticker` | 1M default, 3M, 1Y, invalid range, unavailable |
| `GET /healthz` | ok |

## Roles and auth

Single local user, no authentication. `127.0.0.1` binding is the only access control;
covered by REQ-048. No role scenarios.

## Error exits to cover

1. Invalid ticker format.
2. Duplicate ticker.
3. Unknown symbol.
4. Not in watchlist.
5. Missing API key.
6. Provider unavailable or timeout.
7. Local rate limit.
8. Invalid chart range.
9. Chart unavailable.

## Data needs

| Need | Approach |
| :-- | :-- |
| Deterministic provider responses | Local fake provider via `FINNHUB_BASE_URL` / `TWELVEDATA_BASE_URL`. |
| Repeatable empty state | Fresh `DATA_DIR` per test run. |
| Stale quote | Fake provider timestamp older than 15 minutes. |
| Upstream failures | Fake provider returns 5xx, malformed body, or delayed response. |
| Rate-limit boundary | Fake provider plus a burst request in one test. |

## Coverage quotas for Phase 7

- Every REQ: at least 1 positive scenario; negative or boundary where the table above applies.
- Every transition in the table: covered.
- Every endpoint: at least 1 API test.
- Every error exit: at least 1 scenario.
- Every invariant from Phase 1: at least 1 violation scenario.
