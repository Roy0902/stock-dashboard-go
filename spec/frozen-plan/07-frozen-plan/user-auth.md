# Phase 7 — Frozen Plan Pointer: user-auth

Feature: `user-auth`
Frozen at: `2026-11-18T03:47:51.993Z`
Plan: `local-env/specs/user-auth.md`
Manifest: `local-env/specs/user-auth.freeze.json`
Plan sha256: `88e2ff2c1249d815049280fad3331dce3b178e2aebf69cef767b6c48f350f265`
`tests`: `null`

This folder holds provenance only. The plan is not copied here, so there is one authority and
one checksum.

## Evidence

The plan is not covered by this repository's `tests/` lock. Executable evidence lives in the
Go application and its client, as listed in plan §9:

```
stock-dashboard-go/server/internal/auth/*_test.go
stock-dashboard-go/server/internal/api/*_test.go
stock-dashboard-go/web/src/features/auth/*.test.tsx
stock-dashboard-go/e2e/tests/auth.spec.ts
stock-dashboard-go/e2e/fixtures/app.ts
```

`tests: null` is deliberate: it records that no `local-env/tests/` lock applies to this
extension. It does not mean the suites are optional.

## Rules

- The plan is never edited. A gap becomes a follow-up plan with its own freeze.
- The original `stock-watchlist` plan and manifests are unchanged by this extension.
- Do not re-freeze `user-auth` to absorb an implementation deviation. Record the deviation in
  `06-review/auth-decisions.md` or open a follow-up.
- Verification runs against the suites above; the freeze checksum covers the plan, not the
  test files.

## What the plan contains

- 26 requirements, 25 scenarios, 8 API endpoints, 7 state transitions, 10 security checks.
- Every scenario ID is `SCN-user-auth-###`; tags are `@REQ-user-auth-NNN`,
  `@SCN-user-auth-NNN`, and `@smoke`/`@reg`.
- Requirements are `REQ-user-auth-001`–`REQ-user-auth-026`.
- Companion artifacts: `00-ingestion/requirement-register-auth.md`,
  `03-scenarios/auth.md`, `04-checklist/auth-checklist.md`,
  `05-task-lists/auth-implementation.md`, `06-review/auth-decisions.md`.
