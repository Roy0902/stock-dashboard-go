# stock-dashboard — ATDD run

Artifacts produced by running `spec-to-test-workflow` against `Spec/`. One folder per phase.

## Project

A local, single-user stock dashboard. Node.js + TypeScript, served on `127.0.0.1:3000`,
with a JSON watchlist and two free market-data providers (Finnhub primary, Twelve Data
fallback and charts).

## Features (this iteration)

| Feature | Requirements | Normative spec |
| :-- | :-- | :-- |
| `stock-watchlist` | REQ-stock-watchlist-001 to 052 | `Spec/implementation-spec.md` v1.2 |

## Layout

```
stock-dashboard/
  Spec/implementation-spec.md      normative spec (oracle input)
  spec-to-test-workflow/stock-dashboard/
    00-ingestion/                  Phase 0: inventory, registers, coverage
    01-domain-model/               Phase 1
    02-intent-diagrams/            Phase 2 (versioned, never overwritten)
    03-scenarios/                  Phase 3
    04-checklist/                  Phase 4
    05-task-lists/                 Phase 5
    06-review/                     Phase 6
    07-frozen-plan/                Phase 7 pointer (plan lives in local-env/specs)
    08-red-runs/                   Phase 8
    09-implementation/             Phase 9
    10-verification/               Phase 10
    11-accessibility/              Phase 11
    12-as-built/                   Phase 12
```

## Where the frozen plan will live

The freeze guard resolves `specs/<feature>.md` and `tests/` under its `--root`. To keep
the server code and the acceptance harness separate:

```
local-env/specs/<feature>.md
local-env/specs/<feature>.freeze.json
local-env/tests/specs/<feature>.spec.ts
local-env/tests/api/<feature>.spec.ts
```

as in the existing `find-potential-stock` run.

## Status

| Phase | Artifact | State |
| :-- | :-- | :-- |
| 0 Ingestion | `00-ingestion/` | Complete rev. 3: 52 REQs, 12 AMBs (0 blocking) |
| 1 Domain model | `01-domain-model/domain-model.md` | Approved 2026-11-16 |
| 2 Intent diagrams | `02-intent-diagrams/v1.md` | Approved 2026-11-16 (format check before duplicate) |
| 3 Scenarios | `03-scenarios/` | Approved 2026-11-16 |
| 4 Shared checklist | `04-checklist/shared-checklist.md` | Approved 2026-11-16 |
| 5 Design and tasks | `05-task-lists/` | Approved 2026-11-16 |
| 6 Review | `06-review/decisions.md` | Approved 2026-11-16, 0 blocking |
| 7 Freeze | `local-env/specs/stock-watchlist.md` | **FROZEN 2026-11-16** via `ai:freeze` |
| 8-red-runs | `08-red-runs/` | Red run records (Phase 8) |
| 10-verification | `10-verification/` | Verification and gaps (Phase 10) |

## Extension: user-auth

The authentication extension is a second, independently frozen plan. It adds a single owner
account with mandatory TOTP, recovery codes, sessions and protected routes. It does **not**
modify the `stock-watchlist` plan or its manifests; those stay frozen and unchanged.

| Phase | Artifact | State |
| :-- | :-- | :-- |
| 0 Ingestion | [`00-ingestion/requirement-register-auth.md`](00-ingestion/requirement-register-auth.md) | planned: 26 REQs, 8 non-blocking AMBs |
| 3 Scenarios | [`03-scenarios/auth.md`](03-scenarios/auth.md) | planned: 25 SCNs |
| 4 Checklist | [`04-checklist/auth-checklist.md`](04-checklist/auth-checklist.md) | planned: API, errors, cookies, UI, security |
| 5 Task list | [`05-task-lists/auth-implementation.md`](05-task-lists/auth-implementation.md) | planned: BE-AUTH-001…014, FE-AUTH-001…009 |
| 6 Review | [`06-review/auth-decisions.md`](06-review/auth-decisions.md) | approved 2026-11-18 |
| 7 Freeze | [`07-frozen-plan/user-auth.md`](07-frozen-plan/user-auth.md) | **FROZEN 2026-11-18T03:47:51.993Z** |
| Plan | [`local-env/specs/user-auth.md`](../../local-env/specs/user-auth.md) | sha256 `88e2ff2c…f265`, `tests: null` |
| Manifest | [`local-env/specs/user-auth.freeze.json`](../../local-env/specs/user-auth.freeze.json) | frozen evidence |

Executable evidence for this extension lives in the Go application suites listed in plan §9,
not in this repository's `local-env/tests/` lock. No test result is claimed here.
