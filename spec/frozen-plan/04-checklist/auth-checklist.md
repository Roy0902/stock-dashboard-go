# Phase 4 — Shared Checklist: user-auth (extension)

Feature: `user-auth`
Sources: `Spec/implementation-spec.md` v1.3 §11 (and §12 for the unchanged timestamp rule),
`local-env/specs/user-auth.md` §3, §6, §7, §8, `03-scenarios/auth.md` (25 SCNs)
Status: planned — implementation and tests are not written.

This checklist is the single source for the auth contract. It extends
`04-checklist/shared-checklist.md`; it does not replace it. The `stock-watchlist` checklist
stays authoritative for watchlist, quotes and chart behaviour.

## 1. Scope boundary and supersedes note

- **Supersedes:** the first clause of spec §2.2, "User accounts, authentication, or
  multi-user support", is superseded **only** for a single owner account. Spec §11.1 is the
  authority.
- **Not superseded:** multiple users, roles, sharing, invitations, public registration after
  setup, and password reset by email remain out of scope.
- **Unchanged:** §1–§10 keep their force. In particular §12's display-only rule
  (`dd-MMM-yyyy` in UTC, ` (stale)` suffix, ISO-8601 kept in `<time datetime>`) is untouched
  by authentication.
- **Single tenant:** the watchlist, quotes and charts belong to the owner account. There is no
  per-user partitioning.
- **Persistence conflict to resolve by the implementation owner:** `stock-dashboard-go/AGENTS.md`
  and `docs/contracts.md` currently say `watchlist.json` is the only persisted file. Spec
  §11.2 and plan §2 add `$DATA_DIR/users.json`. The frozen plan governs this extension; those
  two implementation documents need an owner-approved update, which is outside this artifact
  set.

## 2. API matrix

All errors use the §5 envelope `{ "error": "<copy>", "code": "<CODE>" }`. `user` is
`{ "username": "owner", "totpEnabled": true }`.

| Method | Path | Auth | Request | Success | Errors | Owner SCN |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| GET | `/api/auth/status` | public | — | `200 { setupRequired, authenticated, user? }` | none | 001, 005 |
| POST | `/api/auth/setup` | public, no owner | `{ username, password }` | `201 { username, secret, otpauthUrl, recoveryCodes }` | `400 INVALID_USERNAME`, `400 WEAK_PASSWORD`, `409 SETUP_COMPLETE` | 002–005, 008 |
| POST | `/api/auth/2fa/activate` | `sd_enroll` cookie | `{ code }` | `200 { user }` + session | `401 INVALID_CODE`, `401 ENROLLMENT_EXPIRED` | 006, 007 |
| POST | `/api/auth/login` | public | `{ username, password }` | `200 { status: "totp_required", expiresInSeconds }` | `400 INVALID_REQUEST`, `401 INVALID_CREDENTIALS`, `429 RATE_LIMITED` | 009, 011, 016 |
| POST | `/api/auth/login/totp` | `sd_challenge` cookie | `{ code }` | `200 { user }` + session | `401 INVALID_CODE`, `401 CHALLENGE_EXPIRED`, `429 RATE_LIMITED` | 010, 012, 013, 015, 017 |
| POST | `/api/auth/login/recovery` | `sd_challenge` cookie | `{ code }` | `200 { user }` + session | `401 INVALID_RECOVERY_CODE`, `401 CHALLENGE_EXPIRED`, `429 RATE_LIMITED` | 014 |
| POST | `/api/auth/logout` | session optional | — | `204` | none | 020 |
| GET | `/api/auth/me` | session | — | `200 { user }` | `401 UNAUTHENTICATED` | 018 |

Protected data surfaces: `/api/watchlist` (including `POST` and `DELETE`), `/api/quotes`,
`/api/chart/*`. Public surfaces: `/healthz`, static client assets, `/api/auth/status`,
`/api/auth/setup` while no owner exists, `/api/auth/login`.

## 3. Error codes and copy

Copy is exact. `N` is replaced by the integer seconds in `Retry-After`; the header is
required with `429`.

| Code | Status | Copy | Source | Owner SCN |
| :-- | :-- | :-- | :-- | :-- |
| `INVALID_USERNAME` | 400 | `Use 3-32 lowercase letters, numbers, dots, dashes or underscores.` | spec §11.7 | 003 |
| `WEAK_PASSWORD` | 400 | `Use at least 12 characters.` | spec §11.7 | 004 |
| `SETUP_COMPLETE` | 409 | `Setup is already complete.` | spec §11.7 | 008 |
| `INVALID_CODE` | 401 | `Invalid verification code.` | spec §11.7 | 007, 012, 013 |
| `ENROLLMENT_EXPIRED` | 401 | *(unstated in spec §11.7)* | spec §11.6 | none frozen |
| `INVALID_REQUEST` | 400 | `Enter your username and password.` | spec §11.7 (missing login fields) | none explicit |
| `INVALID_CREDENTIALS` | 401 | `Invalid username or password.` | spec §11.7 | 011 |
| `INVALID_RECOVERY_CODE` | 401 | `Invalid recovery code.` | spec §11.7 | none explicit |
| `CHALLENGE_EXPIRED` | 401 | `Your sign-in attempt expired. Sign in again.` | spec §11.7 | 015, 017 |
| `RATE_LIMITED` | 429 | `Too many requests. Try again in N seconds.` | spec §11.7 | 016 |
| `UNAUTHENTICATED` | 401 | `Authentication required.` | spec §11.7 | 018 |
| `CROSS_ORIGIN_REQUEST` | 403 | *(unstated in plan §6 and spec §11.7)* | plan §4 SCN-024 | 024 |

Rules:

- A wrong username and a wrong password return the same status, code and copy, and comparable
  timing. `INVALID_CREDENTIALS` never says which field failed.
- `INVALID_RECOVERY_CODE` and `INVALID_CODE` are distinct codes with distinct copy; do not
  collapse them.
- The two unstated copies (`ENROLLMENT_EXPIRED`, `CROSS_ORIGIN_REQUEST`) must not acquire
  product copy by invention. Tests assert status and code only.
- Existing non-auth codes (`MISSING_API_KEY`, `PROVIDER_UNAVAILABLE`, `RATE_LIMITED` for
  provider budgets, `INTERNAL_ERROR`, etc.) keep their existing meaning. The auth limiter
  reuses the `RATE_LIMITED` code and copy.

## 4. Cookie contract

| Cookie | Purpose | Flags | Path | Lifetime | Value handling |
| :-- | :-- | :-- | :-- | :-- | :-- |
| `sd_session` | Authenticated session | `HttpOnly; SameSite=Strict`; `Secure` iff `AUTH_COOKIE_SECURE=true` | `/` | 12 h absolute; 30 min idle, extended on each authenticated request | 32 random bytes, base64url; server stores only the SHA-256 digest |
| `sd_challenge` | Pending password step | `HttpOnly; SameSite=Strict`; `Secure` when configured | `/api/auth` | 5 min | Opaque challenge reference; not a session |
| `sd_enroll` | Pending setup enrolment | `HttpOnly; SameSite=Strict`; `Secure` when configured | `/api/auth` | 15 min | Opaque enrolment reference; scopes the TOTP-secret response |

Rules:

- No auth token, challenge id or enrollment id is ever placed in a URL, response body or log.
- Logout clears `sd_session` and revokes the server-side record. Clearing the cookie alone is
  not sufficient.
- A restart drops all sessions and pending challenges/enrolments; the browser may still hold
  a cookie that no longer resolves, which must produce `401 UNAUTHENTICATED`.
- `AUTH_COOKIE_SECURE` defaults to false for loopback HTTP and must be true behind HTTPS.

## 5. UI element and copy contract

All controls have accessible names. The error region is `role="alert"`. Password fields use
`autocomplete="current-password"` or `new-password`. The QR image's text alternative names
the account.

| Element | Role / name | Copy / value | Owner SCN |
| :-- | :-- | :-- | :-- |
| Setup heading | heading | `Set up your account` | 001 |
| Sign-in heading | heading | `Sign in` | 019 |
| Username | textbox `Username` | — | 002–004, 009, 011 |
| Password | textbox `Password` | — | 002–004, 009, 011 |
| Confirm password | textbox `Confirm password` | setup only | 002 |
| Primary action | button | `Create account` / `Sign in` / `Continue` | 001, 006, 009, 010 |
| TOTP heading | heading | `Two-factor authentication` | 009 |
| TOTP input | textbox `Verification code` | six digits, `one-time-code` | 006, 010, 012, 013, 015 |
| Recovery switch | button | `Use a recovery code` | 014 |
| Recovery input | textbox `Recovery code` | — | 014 |
| QR | image `Authenticator QR code` | rendered from `otpauthUrl` | 002 |
| Setup key | text | `Or enter this setup key manually:` then the secret | 002 |
| Recovery codes | list | ten codes with `Copy` | 002, 005 |
| Sign out | button | `Sign out` | 020 |
| Session expiry | alert | `Your session expired. Sign in again.` | 021 |

Spec §11.7 also lists labels that the frozen plan §7 does not assign to an element:
`Back to sign in`, `Set up authenticator`, `Scan this QR code with your authenticator app.`,
`Save these recovery codes.`, `Copied`. They are reserved copy: an implementation may use
them, but no frozen scenario or locator depends on them until a follow-up plan assigns them.
The plan's `Continue` action is the only frozen primary action beyond `Create account` and
`Sign in`; the plan does not define a separate "codes saved" confirmation control.

## 6. Security invariants

| Check | Required result | Source |
| :-- | :-- | :-- |
| Password storage | Argon2id PHC string: memory 64 MiB, iterations 3, parallelism 4, salt 16 bytes, key 32 bytes | spec §11.2 |
| Unknown username | Dummy Argon2id verification; same `INVALID_CREDENTIALS` copy and comparable timing | spec §11.8 |
| Session token | 32 random bytes, base64url; SHA-256 digest stored; token only in `sd_session` | spec §11.5 |
| Cookie flags | `HttpOnly; SameSite=Strict; Path=/`; `Secure` when configured | spec §11.5, plan §8 |
| TOTP | SHA-1, six digits, 30-second period, ±1 step; accepted step recorded | spec §11.4 |
| TOTP replay | A step already used for a login is rejected | spec §11.4 |
| Recovery codes | SHA-256 hashes, single use, constant-time compare, removed on use | spec §11.4, plan §8 |
| Challenge attempts | At most five code attempts, then destroyed | spec §11.4 |
| Password rate limit | 5 failures per username per 15 min, then 60 s refusal with `Retry-After` | spec §11.4 |
| CSRF | Mutating request with mismatched `Origin` rejected; `SameSite=Strict` cookies | spec §11.8 |
| Secret exposure | TOTP secret only in the setup response or with the matching `sd_enroll` cookie | spec §11.8 |
| Response bodies | Never include password, TOTP secret/hash, recovery hash, session token or cookie value | spec §11.8 |
| Logging | Never log password, TOTP code, recovery code, secret, token, cookie or hash | spec §11.8 |
| Binding | Loopback only; `AUTH_COOKIE_SECURE` opt-in for TLS | spec §11.8 |
| Setup state | No owner persisted and no session before activation | spec §11.2, §11.3 |

## 7. Persistence and state rules

| Item | Rule | Source |
| :-- | :-- | :-- |
| `$DATA_DIR/users.json` | The only new persisted file. Created only when activation succeeds. | spec §11.2 |
| Write method | Write a temporary file in the same directory, then atomic replace; no temporary file remains after activation | plan §4 SCN-025 |
| File mode | `0600` requested at create/replace | spec §11.2, plan §8 |
| Owner contents | Exactly one owner: username, Argon2id PHC hash, TOTP secret, TOTP-enabled flag, recovery-code hashes, last accepted TOTP step | spec §11.2, §11.4 |
| JSON shape | Not fixed by spec or plan; storage is an implementation detail and must not leak into API shapes | derived, recorded here |
| Pending setup | In memory only; 15-minute window; gone on expiry or restart | spec §11.3, plan §5 |
| Sessions | In memory only; restart requires sign-in | spec §11.5 |
| `watchlist.json` | Unchanged; still the watchlist store, not auth state | spec §4.1, §8 |
| Other files | None. No database, no audit log, no session file. | plan §2 |

## 8. Rejected alternatives

| Alternative | Decision | Reason |
| :-- | :-- | :-- |
| Email password reset | Rejected | No mail delivery in a loopback app; recovery codes are the only reset path. |
| SMS second factor or reset | Rejected | Requires a paid external service and a phone number; out of scope in plan §2. |
| Social login (OAuth) | Rejected | Adds an external identity provider and redirect surface; single local owner needs none. |
| Passkeys / WebAuthn | Rejected | Not in the frozen plan; TOTP is the approved second factor. |
| Per-user watchlists | Rejected | Single owner; multi-user tenancy is out of scope in spec §11.1. |
| Public signup after setup | Rejected | Setup is one-way; `409 SETUP_COMPLETE` and no registration endpoint. |

## 9. Coverage notes

- API matrix: 7/8 endpoints have a frozen scenario. `GET /api/auth/me` and the
  `INVALID_RECOVERY_CODE` / `ENROLLMENT_EXPIRED` branches are spec obligations without a
  dedicated SCN; they are still implemented and unit-tested.
- UI contract: every plan §7 element is in section 5; the five spec-only labels are marked
  reserved.
- The plan defines 25 scenarios for 26 requirements. All 26 REQ IDs appear in scenario tags:
  SCN-009 carries REQ-009 and REQ-011; SCN-010 carries REQ-013 and REQ-018. REQ-024 and
  REQ-025 are covered by SCN-025 and SCN-023.
