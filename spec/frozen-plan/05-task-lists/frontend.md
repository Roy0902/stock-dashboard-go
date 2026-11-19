# Phase 5 — Frontend Task List

Feature: `stock-watchlist`
Design: `implementation-design.md` (proposed, Phase 6 review)
Checklist: `../04-checklist/shared-checklist.md` sections 1, 2, 3 (read only), 4 and 6

Prepared independently of the backend task list. The UI contract in checklist §4 is
authoritative; copy and locators may not drift from it.

## 1. Files Studied

| File | Why it matters |
| :-- | :-- |
| `04-checklist/shared-checklist.md` §4 | Every role, label and copy string |
| `04-checklist/shared-checklist.md` §2 | Which scenarios are frontend-visible |
| `03-scenarios/watchlist.md`, `quotes.md`, `chart.md` | UI-level Then steps |
| `02-intent-diagrams/v1.md` D2, D3, D5 | States and guards the UI must render |
| `01-domain-model/domain-model.md` §1 | Vocabulary used in markup text |
| `find-potential-stock/local-env/tests/specs` | Existing UI-test style (none specific to this project) |

No existing frontend code. All files below are new; patterns are the server-rendered
markup and browser module proposed in design §3.

## 2. Task List

| ID | Checklist Ref | Action | Target File | Existing / New | Pattern To Follow | Change Detail | Scenario(s) |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| FE-001 | §4 add form + row + alerts | Add | `src/ui/dashboard.html.ts` | new | design §3 | `<main>` with `textbox` labelled `Ticker` (placeholder `e.g. AAPL`), `button` `Add ticker`, `alert` slot for one message, `<ul>` of `listitem` rows named by ticker, `Back`-free dashboard shell, `status` line `Last updated HH:MM:SS`, empty-state alert copy | 001, 004, 005, 007, 012, 014, 022, 028, 040, 045 |
| FE-002 | §4 change, direction, provider, timestamp | Add | `src/ui/dashboard.html.ts` | new | design §3 | Per-row cells: price, change with `+`/`-` and class `up`/`down`/`flat`, direction word `up`/`down`/`unchanged`, provider text, timestamp with ` (stale)` suffix, `Price unavailable` state, `View chart` link | 014, 017–019, 020–024, 031, 032, 033 |
| FE-003 | §4 chart page | Add | `src/ui/chart.html.ts` | new | design §3 | Ticker heading, `group` labelled `Range` with buttons `1M`, `3M`, `1Y`, `img` with aria-label `<ticker> price chart, <range> range`, `Source: Twelve Data`, `Chart unavailable` alert, `Back` link to `/` | 034–037, 039–041 |
| FE-004 | §4 refresh + last updated | Add | `src/ui/client.ts` | new | design §3 | Fetch `/api/quotes` on load and every 60 s; replace row values only; never recompute change or percent (server is the only arithmetic); stop on `document.hidden`; show `Last updated HH:MM:SS` from the local clock at render time | 025, 026, 028 |
| FE-005 | §4 styling tokens | Add | `src/ui/styles.css` | new | design §3 | `.up { color: #16a34a }`, `.down { color: #dc2626 }`, `.flat { color: #6b7280 }`; visible focus rings; no colour-only meaning because the direction word is text | 017–019, 031 |
| FE-006 | §4 add-form states | Add | `src/ui/client.ts`, `src/ui/dashboard.html.ts` | new | design §3 | Disable `Add ticker` while a request is in flight; re-enable on completion; render the returned error copy in the single alert slot; clear the alert on the next successful add | 004, 005, 007 |

## 3. Case Matrix

| Behaviour | Case | Handling in UI | Scenario / AMB | Status |
| :-- | :-- | :-- | :-- | :-- |
| Dashboard load | empty watchlist | single alert `Your watchlist is empty. Add a ticker to begin.`, no rows | SCN-001 | Mapped |
| Dashboard load | rows present | one `listitem` per ticker, named by ticker | SCN-012, SCN-014 | Mapped |
| Add ticker | success | row appears, alert cleared | SCN-006 | Mapped |
| Add ticker | 400 invalid | alert `Enter a valid ticker, e.g. AAPL.` | SCN-004 | Mapped |
| Add ticker | 409 duplicate | alert `AAPL is already in your watchlist.` | SCN-005 | Mapped |
| Add ticker | 404 unknown | alert `Symbol not found.` | SCN-007 | Mapped |
| Add ticker | double submit | button disabled until the request settles | SCN-006 | Mapped (FE-006) |
| Quotes | loading | rows keep previous values, no spinner copy invented | none stated | Candidate (no copy in spec) |
| Quotes | positive / negative / zero | colour class plus `+`/`-` and direction word | SCN-017–019, SCN-031 | Mapped |
| Quotes | stale | timestamp suffixed ` (stale)` | SCN-023, SCN-024 | Mapped |
| Quotes | both providers fail | row text `Price unavailable`, provider `none` | SCN-022 | Mapped |
| Quotes | zero previous close | price `0.00`, change `0.00`, no percent | SCN-032, SCN-033 | Mapped |
| Quotes | refresh | poll every 60 s, update `Last updated` | SCN-027, SCN-028 | Mapped |
| Quotes | missing keys | alert `Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.` | SCN-045 | Mapped |
| Chart | load | chart renders with aria-label | SCN-035, SCN-041 | Mapped |
| Chart | range change | buttons switch range, aria-label updates | SCN-037, SCN-041 | Mapped |
| Chart | unavailable | `Chart unavailable`, dashboard unaffected | SCN-039 | Mapped |
| Chart | navigation during pending | back link always enabled | SCN-040 | Mapped |
| Any | provider error detail | never rendered; only the fixed copy | SCN-053 | Mapped |
| Any | role changes view | no roles in this product | n/a | Mapped (single local user) |

No candidate case invents copy. The loading state has no spec copy, so it renders nothing new.

## 4. Conflicts for Phase 6

- None. The API contract in checklist §3 supports every UI behaviour listed above,
  including `changePercent: null` for SCN-032/SCN-033 and `provider: "none"` for SCN-022.

## 5. Coverage Against Checklist

- Checklist frontend items covered: 21/21 UI contract rows (6 tasks).
- Scenarios with a frontend task: 24/57; the remaining 33 are API-only.

## 6. Implementation Design and Validation

| Task | Design responsibility / dependency rule | Naming example | Supporting test / location | Native validation command |
| :-- | :-- | :-- | :-- | :-- |
| FE-001 | `ui/dashboard.html.ts`, markup only, no rules | `renderDashboard(model)` | Playwright UI `local-env/tests/specs` | `npm run test:ui` in `local-env/` |
| FE-002 | same module, presentation of server values | `renderQuoteRow(quote)` | Playwright UI | same |
| FE-003 | `ui/chart.html.ts` | `renderChartPage(series)` | Playwright UI | same |
| FE-004 | `ui/client.ts`, no arithmetic | `startAutoRefresh(ms)` | Playwright UI with clock | same |
| FE-005 | `ui/styles.css`, tokens only | n/a | Playwright UI colour assertions | same |
| FE-006 | `ui/client.ts`, form state only | `setSubmitting(bool)` | Playwright UI | same |

Native gates that must also pass: `npm run format:check`, `npm run lint`, `npm run typecheck`
in `stock-dashboard/`.
