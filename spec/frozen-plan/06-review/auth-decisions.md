# Phase 6 — Review Record: user-auth (extension)

Feature: `user-auth`
Date: 2026-11-18
Reviewer: user
Status: **Approved by user request 2026-11-18 for the extension**

This record covers the auth extension only. It does not reopen or amend
`06-review/decisions.md` or the frozen `stock-watchlist` plan.

## 1. Artifacts reviewed

| Artifact | State | Decision |
| :-- | :-- | :-- |
| `Spec/implementation-spec.md` v1.3 §11, §12 | append-only extension | Approved for this scope |
| `local-env/specs/user-auth.md` | frozen 2026-11-18T03:47:51.993Z, sha256 `88e2ff2c…f265` | Approved and unchanged |
| `00-ingestion/requirement-register-auth.md` | 26 REQs, 8 non-blocking AMBs | Approved |
| `03-scenarios/auth.md` | 25 SCNs | Approved |
| `04-checklist/auth-checklist.md` | API, error, cookie, UI, security and persistence contract | Approved |
| `05-task-lists/auth-implementation.md` | BE-AUTH-001…014, FE-AUTH-001…009 | Approved |

## 2. Decisions

| # | Decision | Rationale | Source |
| :-- | :-- | :-- | :-- |
| 1 | Exactly one owner account. No roles, tenancy, invitations or sharing. | The product is a local single-user dashboard; the extension protects that one owner. | spec §11.1, plan §1 |
| 2 | No public registration after setup. Setup is one-way and returns `409 SETUP_COMPLETE`. | A reachable signup route would defeat the single-owner model. | spec §11.3 |
| 3 | TOTP is mandatory; a password alone never creates a session. | A local second factor without an external provider or per-user device management. | spec §11.4, plan §3 |
| 4 | Recovery codes are the only fallback. No email or SMS reset. | No mail/SMS infrastructure, no remote identity provider, no account-recovery service. | plan §2, §4 SCN-014 |
| 5 | Sessions live in memory; 12 h absolute and 30 min idle expiry, extended on use. | Restart invalidation is acceptable locally and avoids a session state file. | spec §11.5 |
| 6 | Argon2id PHC: memory 64 MiB, iterations 3, parallelism 4, salt 16 bytes, key 32 bytes. | Fixed work factor from the spec; PHC keeps parameters with the hash. | spec §11.2 |
| 7 | TOTP is SHA-1, six digits, 30-second period, ±1 step, with the accepted step recorded and replayed steps rejected. | Matches authenticator apps; the stored step is the replay defence. | spec §11.4 |
| 8 | Owner state is `$DATA_DIR/users.json`, written atomically with mode `0600`. | One file, one owner, no database; atomic replace avoids a truncated account. | spec §11.2, plan §4 SCN-025 |
| 9 | `SameSite=Strict` cookies plus an `Origin` check on mutating requests. | Two independent CSRF defences for a loopback HTTP app. | spec §11.8, plan §4 SCN-024 |
| 10 | Generic credential copy: `Invalid username or password.` for unknown user and wrong password, with a dummy Argon2id verify. | Prevents account enumeration by copy and timing. | spec §11.4, §11.7, §11.8 |
| 11 | Password limiter: five failures per username per 15 minutes, 60-second refusal with `Retry-After`; a challenge allows five codes. | Bounds online guessing without locking the owner out permanently. | spec §11.4 |
| 12 | `AUTH_COOKIE_SECURE` is opt-in, default false; must be true behind HTTPS. | Loopback HTTP is the supported local deployment. | spec §11.5, §11.8 |
| 13 | No secret reaches a log, response body or error envelope: password, TOTP code/secret, recovery code/hash, session token, cookie value. | The account file and cookies are the only secret stores; logs are not. | spec §11.8 |

## 3. Deviations and risks

| Risk / deviation | Effect | Disposition |
| :-- | :-- | :-- |
| Lost authenticator **and** all recovery codes | There is no self-service recovery. The owner must stop the server, remove or reset `$DATA_DIR/users.json`, and run setup again; the watchlist file is untouched. | Accepted for a local single-owner app. Recorded here; not automated. |
| Restart requires re-login | Sessions, pending challenges and pending setups are memory-only. A restart invalidates every cookie and abandons an unactivated setup. | Accepted; matches spec §11.5 and plan §5. |
| `0600` on Windows | Windows does not enforce POSIX mode bits; Go reports synthesised permissions. The code still requests `0600`, but the acceptance assertion is platform-dependent. | Reported as a test-environment limitation; never silently skipped. |
| Two error codes have no copy | `ENROLLMENT_EXPIRED` (spec §11.6) and `CROSS_ORIGIN_REQUEST` (plan SCN-024) have status and code but no §11.7 copy. | Tests assert status and code only. No product copy invented. A follow-up plan may add copy. |
| Pending-setup expiry untested | Spec §11.3 gives 15 minutes, but no frozen scenario advances past it. | Implementation obligation in `BE-AUTH-006`; no scenario claim. |
| Spec labels not in the UI contract | `Back to sign in`, `Set up authenticator`, `Scan this QR code with your authenticator app.`, `Save these recovery codes.`, `Copied` are in spec §11.7 but unassigned in plan §7. | Reserved copy only; frozen locators do not depend on them. |
| Single-tenant data | The watchlist, quotes and charts belong to the owner. There is no per-user data partitioning. | Consistent with spec §11.1; multi-user remains out of scope. |
| Rate-limit state is memory-only | A restart clears the five-failure window. | Accepted locally; the limiter is not an audit control. |
| Existing implementation docs say one persisted file | `stock-dashboard-go/AGENTS.md` and `docs/contracts.md` still say `watchlist.json` is the only persisted file. | Owned by the implementation repository; this extension records `users.json` as the second file. |

## 4. Scope confirmed

- **In scope:** first-run setup, password + mandatory TOTP, ten single-use recovery codes,
  in-memory sessions, logout, protected pages and APIs, password/code rate limits, CSRF
  origin checks, `users.json` persistence.
- **Out of scope:** multi-user, roles, invitations, public registration after setup, email or
  SMS reset, social login, passkeys, remember-me, device management, audit UI, HTTPS
  termination.
- **Unchanged:** watchlist, quotes, charts, provider budgets and §12 quote timestamp display.

## 5. Gate decision

- Blocking ambiguities: **0**. The eight AMBs in the register are resolved or bounded.
- Copy contract: every frozen scenario string is covered; two unstated copies are explicitly
  excluded from assertions.
- Test evidence: **none claimed**. The plan is frozen with `tests: null`; executable evidence
  will be the Go, Vitest and Playwright suites in plan §9.

**Approved for the extension on 2026-11-18. The plan remains frozen and is never edited.**
