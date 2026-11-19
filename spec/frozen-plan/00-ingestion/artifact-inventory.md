# Phase 0 — Artifact Inventory

Feature: `stock-watchlist`
Ingested: 2026-11-16
Rev. 6: Phase 6 review record added; plan frozen at `local-env/specs/stock-watchlist.md`.

## Artifacts

| Artifact | Version / hash | Notes |
| :-- | :-- | :-- |
| `Spec/implementation-spec.md` | v1.2 `b2d5c797ff565d79` | Normative oracle |
| `../../../work_submission.md` | v1, unversioned | Task brief only |
| `00-ingestion/requirement-register.md` | `9f209cb29f63358d` | 52 REQs |
| `00-ingestion/ambiguity-register.md` | `a0a9004a5897d486` | 12 AMBs, 0 blocking |
| `00-ingestion/assumptions.md` | `c26e9ca72c315f86` | 13 user decisions |
| `00-ingestion/coverage-plan.md` | `35cba2f5ab4c7817` | |
| `00-ingestion/traceability-matrix.md` | `f347d1eb1151b7eb` | 52 REQ to 57 SCN |
| `01-domain-model/domain-model.md` | `931f233b964f4305` | Approved |
| `02-intent-diagrams/v1.md` | `694adf32398e209c` | Approved 2026-11-16 |
| `03-scenarios/*.md` | watchlist `8bdb967a74055204`, quotes `f5be6777c72b063f`, chart `740360903404fc1e`, platform `08c8016a7e96213e` | Approved |
| `04-checklist/shared-checklist.md` | `f403aeb0379421ef` | Approved |
| `05-task-lists/implementation-design.md` | `530e71f85262e6dc` | Approved |
| `05-task-lists/backend.md` | `08d4e81608e0ca81` | BE-001..018 |
| `05-task-lists/frontend.md` | `810c7437fecae204` | FE-001..006 |
| `06-review/decisions.md` | 2026-11-16 | Approved, 0 blocking |
| `local-env/specs/stock-watchlist.md` | sha256 `7ed3f26e8c4015c81d96d1e35e4b777363759b41d1f9d315afc023ca2d955773` | **FROZEN 2026-11-16** |
| `local-env/specs/stock-watchlist.freeze.json` | `tests: null` | Plan checksum only |
| `AGENTS.md` | project conventions | Proposed, Phase 6 review covered |
| Finnhub / Twelve Data docs | live, 2026-11-16 | Discovery only |

## Traceability check

- Requirements: 52, all sourced.
- Scenarios: 57, all tagged to a REQ, all present in the frozen plan.
- Tests locked: 0 of 57 (Phase 8, expected).
- Blocking ambiguities: 0.
- `@unspecified`: 0.
- Plan integrity: `npm run ai:verify -- stock-watchlist` reports `stock-watchlist: OK`
  with the note "tests not locked yet; 57 scenario(s) without tests".
