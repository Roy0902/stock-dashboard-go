# Phase 7 — Frozen Plan Pointer

Feature: `stock-watchlist`
Frozen at: 2026-11-16
Manifest: `stock-watchlist.freeze.json`

The freeze guard resolves `specs/` and `tests/` under its `--root`. Our `ai:*` scripts use
`--root .` inside `local-env/`, so the frozen artifacts stay there:

```
local-env/specs/stock-watchlist.md              FROZEN plan
local-env/specs/stock-watchlist.freeze.json     plan checksum, tests: null
local-env/tests/specs/stock-watchlist.spec.ts   (Phase 8)
local-env/tests/api/stock-watchlist.spec.ts     (Phase 8)
```

Freeze command and result:

```
cd stock-dashboard/local-env
npm run ai:freeze -- stock-watchlist
# stock-watchlist: frozen (specs/stock-watchlist.md)
```

This folder holds only provenance. The plan is never copied here, so there is one authority
and one checksum.

## What the plan contains

- 52 requirements, 57 scenarios, 21 guard transitions, 6 endpoints, 12 ambiguities.
- Every scenario ID is `SCN-stock-watchlist-###` and every one has a planned test tag.
- Evidence paths: `08-red-runs/red-run.md`, `10-verification/report.md`,
  `10-verification/gaps.md`.

## Rules after this point

- The plan is never edited. A gap becomes a follow-up plan with its own freeze.
- Tests are locked with `npm run ai:lock-tests -- stock-watchlist` only after the red run.
- `npm run ai:verify -- stock-watchlist` must pass before any commit.
