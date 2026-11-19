# Phase 5 — Task List: user-auth (extension)

Feature: `user-auth`
Checklist: `../04-checklist/auth-checklist.md`
Scenarios: `../03-scenarios/auth.md` (25 SCNs)
Stack: Go HTTP service + React SPA in `stock-dashboard-go/`
Status: planned — no task is implemented and no test result is claimed.

This is a single, combined task list. The backend section and the frontend section are kept
separate because they have different owners and gates; the dependency order is at the end.

## 1. Baselines and target paths

| Surface | Existing today | Target |
| :-- | :-- | :-- |
| Auth primitives | no `server/internal/auth/` package | `server/internal/auth/` (new) |
| HTTP | `handlers.go`, `router.go`, `middleware.go`, `errors.go`, `respond.go` | extend in place; add `auth_handlers.go` |
| Composition | `server/internal/app/app.go` | extend in place |
| Config | `server/internal/config/config.go` | extend in place |
| React features | `web/src/features/watchlist`, `quotes`, `chart` | add `web/src/features/auth` |
| Pages | `DashboardPage`, `ChartPage` | add `SetupPage`, `LoginPage` |
| Acceptance | `e2e/tests/{api,chart,dashboard}.spec.ts` | add `e2e/tests/auth.spec.ts` |

No auth baseline exists. Existing `go test`, Vitest and Playwright suites are regression
evidence only; they do not cover auth until the tasks below land.

## 2. Backend tasks

| ID | Action | Target path | Existing / New | Change detail | Scenarios |
| :-- | :-- | :-- | :-- | :-- | :-- |
| BE-AUTH-001 | Extend | `server/internal/config/config.go`, `config_test.go` | existing | Parse `AUTH_COOKIE_SECURE` as a boolean, default false; keep `DATA_DIR`, `PORT`, provider keys unchanged. Expose one config value used by cookie construction. | 018 |
| BE-AUTH-002 | Add | `server/internal/auth/password.go`, `password_test.go` | new | Argon2id PHC encode/verify: memory 64 MiB, iterations 3, parallelism 4, salt 16 bytes, key 32 bytes. Add a dummy verify for unknown usernames. No plaintext retention. | 011, 022 |
| BE-AUTH-003 | Add | `server/internal/auth/totp.go`, `totp_test.go` | new | Generate a TOTP secret, base32 encode it, build `otpauth://totp/...`, validate SHA-1/6/30 with ±1 step, and return the matched step for replay recording. | 002, 006, 007, 010, 012, 013 |
| BE-AUTH-004 | Add | `server/internal/auth/recovery.go`, `recovery_test.go` | new | Generate ten codes, normalise input, SHA-256 hash, constant-time compare, mark one used and persist the removal. | 002, 014 |
| BE-AUTH-005 | Add | `server/internal/auth/store.go`, `store_test.go` | new | Load `$DATA_DIR/users.json` (missing file = no owner), validate one owner, write via same-directory temp file + atomic replace, request mode `0600`, clean the temp file on error. | 005, 006, 008, 024, 025 |
| BE-AUTH-006 | Add | `server/internal/auth/service.go`, `service_test.go` | new | Pending setup (15 min), one-way activation, password step (5 min challenge), TOTP/recovery verification, five-attempt challenge cap, generic credential failure, username limiter hook. No HTTP types. | 001, 005–017 |
| BE-AUTH-007 | Add | `server/internal/auth/session.go`, `session_test.go` | new | 32-byte base64url token, SHA-256 digest lookup, in-memory store, 12 h absolute and 30 min idle expiry, touch on authenticated use, revoke on logout. | 018, 019, 020, 021, 026 |
| BE-AUTH-008 | Add | `server/internal/auth/ratelimit.go`, `ratelimit_test.go` | new | Per-username sliding window: five failures in 15 min, 60 s refusal, `Retry-After` seconds; injected clock. | 016 |
| BE-AUTH-009 | Add | `server/internal/api/auth_handlers.go`, `auth_handlers_test.go` | new | Eight handlers with exact request/response shapes, cookie set/clear, `Origin` guard call, error envelope. No business rules in handlers. | 001–026 |
| BE-AUTH-010 | Extend | `server/internal/api/middleware.go`, `middleware_test.go` | existing | `requireSession` for protected API routes; `requireOrigin` for mutating requests; skip origin enforcement only for safe methods. | 018, 020, 023, 025 |
| BE-AUTH-011 | Extend | `server/internal/api/router.go`, `router_test.go` | existing | Register the eight auth routes, keep `/healthz` and assets public, gate `/api/watchlist*`, `/api/quotes`, `/api/chart/*`. Order public auth routes before the session middleware. | 001, 018, 023, 025 |
| BE-AUTH-012 | Extend | `server/internal/api/errors.go`, `errors_test.go` | existing | Add `INVALID_USERNAME`, `WEAK_PASSWORD`, `SETUP_COMPLETE`, `INVALID_CODE`, `ENROLLMENT_EXPIRED`, `INVALID_REQUEST`, `INVALID_CREDENTIALS`, `INVALID_RECOVERY_CODE`, `CHALLENGE_EXPIRED`, `UNAUTHENTICATED`, `CROSS_ORIGIN_REQUEST` mappings. Preserve existing codes. | 003, 004, 007, 008, 011, 012, 015, 016, 018, 024 |
| BE-AUTH-013 | Extend | `server/internal/app/app.go`, `app_test.go` | existing | Construct the auth service from config and `DATA_DIR`; inject the clock; serve `/setup` and `/login` shells publicly; keep the SPA fallback for `/` and `/chart/*`; do not log secrets. | 001, 021, 025 |
| BE-AUTH-014 | Add | `server/internal/api/auth_flow_test.go` | new | `httptest` flow: fresh temp `DATA_DIR`, setup → activate → login → TOTP → protected request → logout; fixed clock; cookie jar assertions. | 001–021, 023, 024 |

## 3. Frontend tasks

| ID | Action | Target path | Existing / New | Change detail | Scenarios |
| :-- | :-- | :-- | :-- | :-- | :-- |
| FE-AUTH-001 | Add | `web/src/features/auth/api.ts` | new | Typed client for the eight endpoints; `credentials: "include"`; discriminated error union carrying `code` and `error`; no copy invented. | 001–021, 023, 024 |
| FE-AUTH-002 | Add | `web/src/features/auth/AuthContext.tsx` | new | Bootstrap `GET /api/auth/status`; expose `setupRequired`, `authenticated`, `user`; on any protected `401 UNAUTHENTICATED`, clear state and surface `Your session expired. Sign in again.` | 001, 018, 021 |
| FE-AUTH-003 | Add | `web/src/features/auth/SetupFlow.tsx` | new | Setup form (`Username`, `Password`, `Confirm password`), submit, render `Authenticator QR code` from `otpauthUrl`, setup key, ten recovery codes with `Copy`, `Continue` to activation code, activation submit. | 001–008 |
| FE-AUTH-004 | Add | `web/src/features/auth/LoginForm.tsx` | new | Password step, `Two-factor authentication` step, `Verification code` input (`one-time-code`), `Use a recovery code` switch, `Recovery code` input, inline `role="alert"` error. | 009–017 |
| FE-AUTH-005 | Add | `web/src/features/auth/ProtectedRoute.tsx` | new | Redirect to `/setup` when `setupRequired`, to `/login` otherwise. No post-login deep-link contract is defined, so none is asserted. | 019, 021 |
| FE-AUTH-006 | Add | `web/src/pages/SetupPage.tsx`, `web/src/pages/LoginPage.tsx` | new | Page shells and routes `/setup`, `/login`; headings `Set up your account` and `Sign in`; mount the feature components. | 001, 009, 019 |
| FE-AUTH-007 | Extend | `web/src/pages/DashboardPage.tsx`, `web/src/pages/ChartPage.tsx` | existing | Wrap with `ProtectedRoute`; add `Sign out` calling `POST /api/auth/logout` then `/login`; do not alter quote or chart rendering. | 018, 020, 021 |
| FE-AUTH-008 | Add | `web/src/features/auth/auth.test.tsx` | new | Vitest + Testing Library: setup validation copy, recovery-code switch, session-expiry alert, redirect precedence, no secret in DOM. | 001–021 |
| FE-AUTH-009 | Add | `e2e/tests/auth.spec.ts`; extend `e2e/fixtures/app.ts` | new / existing | Authenticated fixture with temp `DATA_DIR` and fixed clock; API and UI paths for all 25 SCNs; cookie assertions via Playwright context. | 001–025 |

## 4. Dependency order

1. `BE-AUTH-001` (config) unlocks every cookie/expiry task.
2. `BE-AUTH-002`, `BE-AUTH-003`, `BE-AUTH-004` are independent primitives; run in parallel.
3. `BE-AUTH-005` needs 002–004 for the stored shape; `BE-AUTH-006` needs 002–005.
4. `BE-AUTH-007` and `BE-AUTH-008` can run in parallel with 006 once 005 is stable.
5. `BE-AUTH-009` needs 006–008; `BE-AUTH-010`, `BE-AUTH-011`, `BE-AUTH-012` follow 009.
6. `BE-AUTH-013` needs 009–012; `BE-AUTH-014` closes the backend after 013.
7. `FE-AUTH-001` can start once `BE-AUTH-009` freezes the wire shapes. `FE-AUTH-002`–`FE-AUTH-005`
   follow 001 in parallel; `FE-AUTH-006` and `FE-AUTH-007` follow 002–005.
8. `FE-AUTH-009` is last and needs `BE-AUTH-013` plus `FE-AUTH-006`/`FE-AUTH-007`.

No task may edit `local-env/specs/user-auth.md`, the frozen `stock-watchlist` plan, or any
existing workflow artifact.

## 5. Validation and gate commands

Run from the stated directory. A gate is not "passed" until it exits zero on this change set.

| Gate | Command | Working directory |
| :-- | :-- | :-- |
| Go format | `gofmt -l .` (must print nothing) | `stock-dashboard-go/server` |
| Go vet | `go vet ./...` | `stock-dashboard-go/server` |
| Go tests | `go test ./... -count=1` | `stock-dashboard-go/server` |
| Go race (when cgo exists) | `go test -race ./... -count=1` | `stock-dashboard-go/server` |
| Web lint | `npm run lint` | `stock-dashboard-go/web` |
| Web type check | `npm run typecheck` | `stock-dashboard-go/web` |
| Web format | `npm run format:check` | `stock-dashboard-go/web` |
| Web unit tests | `npm run test` | `stock-dashboard-go/web` |
| Web build | `npm run build` | `stock-dashboard-go/web` |
| Acceptance | `npm test` | `stock-dashboard-go/e2e` |

Notes:

- This host has `CGO_ENABLED=0`; the race gate must be run on a toolchain with a C compiler
  or reported as not run. It is never silently skipped.
- Tests use a fresh `t.TempDir()` / Playwright temp directory for `DATA_DIR`. Never point a
  test at the developer's real `./data`.
- Auth tests must not depend on real time. Inject the clock for TOTP, challenge, session and
  rate-limit assertions.
- No task in this list claims the tests have run. Phase 8 records the red run; Phase 10
  records the verification.

## 6. Coverage notes

- Backend tasks cover all 8 endpoints, all new error codes and both persisted/unpersisted
  state boundaries.
- Frontend tasks cover every plan §7 element; the five spec-only labels are not assigned and
  are not asserted.
- `REQ-user-auth-009` (missing login fields) has no dedicated frozen scenario; `BE-AUTH-009`
  and `FE-AUTH-004` still implement the `INVALID_REQUEST` branch from spec §11.6.
- `ENROLLMENT_EXPIRED` and `CROSS_ORIGIN_REQUEST` are implemented from spec/plan codes but
  have no copy contract; tests assert status and code only.
