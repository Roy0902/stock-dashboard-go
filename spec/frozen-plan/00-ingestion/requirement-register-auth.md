# Phase 0 — Requirement Register: user-auth (extension)

Feature: `user-auth`
Source: `Spec/implementation-spec.md` v1.3 (2026-11-18) §11, extended by the frozen plan
`local-env/specs/user-auth.md` §3
Status: planned — no executable evidence exists yet.

**Rules applied**

- This is an extension register. It does not replace or renumber
  `00-ingestion/requirement-register.md` for `stock-watchlist`.
- The 26 statements below are the frozen plan §3 statements, copied verbatim. IDs are never
  reused.
- Citations name the spec subsection that makes the requirement normative. Spec §11 is
  append-only; §1–§10 remain in force.
- `testable` means the expectation is concrete and can be asserted against the Go API, the
  React client or the account file. Timing-dependent rows use an injected clock.
- No value here comes from `stock-dashboard-go`, its tests or a running process.

## Requirements

| ID | Requirement | Type | Source | Level | Testability |
| :-- | :-- | :-- | :-- | :-- | :-- |
| REQ-user-auth-001 | First start with no owner reports `setupRequired: true`; `/` redirects to `/setup`. | Flow | spec §11.3; plan §4 SCN-001 | Both | testable |
| REQ-user-auth-002 | The setup username is 3–32 characters, lowercase, matching `^[a-z0-9][a-z0-9._-]{2,31}$`. | Rule | spec §11.2, §11.3; plan §4 SCN-003 | API | testable |
| REQ-user-auth-003 | The setup password is 12–128 characters. | Rule | spec §11.2, §11.3; plan §4 SCN-004 | API | testable |
| REQ-user-auth-004 | Setup returns a TOTP secret, an `otpauth://` URI and ten recovery codes. | Contract | spec §11.3, §11.6; plan §4 SCN-002 | API | testable |
| REQ-user-auth-005 | Setup is pending only; no owner is persisted and no session exists until activation. | Rule | spec §11.2, §11.3; plan §4 SCN-005 | API | testable |
| REQ-user-auth-006 | Setup after an owner exists returns `409 SETUP_COMPLETE`. | Rule | spec §11.3, §11.6; plan §4 SCN-008 | API | testable |
| REQ-user-auth-007 | Activation with a valid TOTP code persists the owner and creates a session. | Flow | spec §11.3, §11.6; plan §4 SCN-006 | Both | testable |
| REQ-user-auth-008 | Activation with an invalid or replayed code returns `401 INVALID_CODE`. | Rule | spec §11.3, §11.6, §11.7; plan §4 SCN-007 | API | testable |
| REQ-user-auth-009 | Login requires username and password; missing fields return `400 INVALID_REQUEST`. | Rule | spec §11.4, §11.6, §11.7; plan §4 SCN-009 | API | testable |
| REQ-user-auth-010 | Wrong username and wrong password produce the same `401 INVALID_CREDENTIALS` copy. | Rule | spec §11.4, §11.7, §11.8; plan §4 SCN-011 | API | testable |
| REQ-user-auth-011 | A correct password returns `totp_required` and a five-minute challenge, not a session. | Contract | spec §11.4, §11.6; plan §4 SCN-009 | Both | testable |
| REQ-user-auth-012 | An expired challenge returns `401 CHALLENGE_EXPIRED`. | Rule | spec §11.4, §11.6, §11.7; plan §4 SCN-015 | API | testable |
| REQ-user-auth-013 | TOTP accepts six digits within ±1 thirty-second step. | Rule | spec §11.4; plan §4 SCN-010, SCN-012 | API | testable |
| REQ-user-auth-014 | A TOTP time step already used for login cannot be replayed. | Rule | spec §11.4; plan §4 SCN-013 | API | testable |
| REQ-user-auth-015 | A recovery code completes login once and is then removed. | Flow | spec §11.4, §11.6; plan §4 SCN-014 | Both | testable |
| REQ-user-auth-016 | Five failed password attempts for one username in 15 minutes returns `429` for 60 seconds. | Rule | spec §11.4; plan §4 SCN-016 | API | testable |
| REQ-user-auth-017 | A challenge allows at most five code attempts, then it is destroyed. | Rule | spec §11.4; plan §4 SCN-017 | API | testable |
| REQ-user-auth-018 | The session cookie is `HttpOnly`, `SameSite=Strict`, `Path=/`, 12-hour absolute expiry and 30-minute idle expiry. | Rule | spec §11.5; plan §4 SCN-010, SCN-020 | API | testable |
| REQ-user-auth-019 | Logout revokes the current session. | Flow | spec §11.5, §11.6; plan §4 SCN-020 | Both | testable |
| REQ-user-auth-020 | Protected API requests without a session return `401 UNAUTHENTICATED` with `Authentication required.` | Rule | spec §11.5, §11.6, §11.7; plan §4 SCN-018 | Both | testable |
| REQ-user-auth-021 | Protected pages redirect unauthenticated visitors to `/login`; setup-required visitors go to `/setup`. | Flow | spec §11.3, §11.5; plan §4 SCN-019 | UI | testable |
| REQ-user-auth-022 | Passwords are stored as Argon2id hashes; TOTP secrets and recovery hashes never appear in logs or error bodies. | Rule | spec §11.2, §11.8; plan §4 SCN-022 | API | testable |
| REQ-user-auth-023 | Mutating requests with a mismatched `Origin` header are rejected. | Rule | spec §11.8; plan §4 SCN-024 | API | testable |
| REQ-user-auth-024 | `$DATA_DIR/users.json` is written atomically with mode `0600`. | Data | spec §11.2; plan §4 SCN-025 | API | testable |
| REQ-user-auth-025 | `/healthz` and static assets remain public; data endpoints require a session. | Contract | spec §11.6, §11.8; plan §4 SCN-023 | Both | testable |
| REQ-user-auth-026 | The SPA shows `Your session expired. Sign in again.` when a session ends mid-use. | Flow | spec §11.5, §11.7; plan §4 SCN-021 | UI | testable |

## Domain glossary additions

| Term | Meaning in this feature | Source |
| :-- | :-- | :-- |
| Owner | The exactly one local account holding the username, Argon2id password hash, TOTP secret and recovery-code hashes. | spec §11.1, §11.2 |
| Setup | First-run enrolment. It is pending and unpersisted until the first valid TOTP code is verified. | spec §11.2, §11.3 |
| Challenge | The five-minute state created by a correct password, held by `sd_challenge`, naming the second-factor step. It is not a session. | spec §11.4, §11.5 |
| Session | A server-side, in-memory record referenced by the `sd_session` cookie, with 12-hour absolute and 30-minute idle expiry. | spec §11.5 |
| Recovery code | A single-use fallback code whose SHA-256 hash is stored; a successful use removes it. | spec §11.4, §11.8 |
| TOTP step | The 30-second counter window used to validate a six-digit code. An accepted step is recorded so the same step cannot be replayed. | spec §11.4 |

## Ambiguities

No ambiguity blocks the extension. Each row records the resolution used by the register,
scenarios and checklist.

| ID | Question | Resolution | Impact | Blocking |
| :-- | :-- | :-- | :-- | :-- |
| AMB-user-auth-001 | SCN-017 says the sixth attempt returns `429` **or** `CHALLENGE_EXPIRED`. | The challenge is destroyed after five failed codes. The test accepts either refusal only if the challenge is unusable afterwards; the API may answer `429` or `401 CHALLENGE_EXPIRED`. | Low | No |
| AMB-user-auth-002 | Spec §11.6 names `401 ENROLLMENT_EXPIRED`, but §11.7 defines no copy for it. | Implement the code and status; assert status and code only. No user-visible copy is invented here. | Low | No |
| AMB-user-auth-003 | Plan SCN-024 fixes `403 CROSS_ORIGIN_REQUEST`, but neither source defines its copy. | Implement status and code from the frozen scenario; assert those only. The body keeps the §5 envelope with an implementation copy that is not a product string. | Low | No |
| AMB-user-auth-004 | Recovery-code wire format is not defined. | Treat the value returned by setup as opaque. SCN-014's `AAAA-BBBB-CCCC-DDDD` is a seeded literal, not a format contract. | Low | No |
| AMB-user-auth-005 | SCN-019 says an unauthenticated `/chart/AAPL` shows the sign-in screen, while REQ-021 sends setup-required visitors to `/setup`. | Apply §11.3 precedence: with no owner, protected pages go to `/setup`; with an owner, to `/login`. SCN-019 is exercised in the no-owner state, so the setup screen is the observed one. | Med | No |
| AMB-user-auth-006 | Does a pending setup survive a restart? | No. Pending setup is in-memory; only an activated owner is written to `users.json`. The state table in plan §5 lists restart as leaving pending setup. | Low | No |
| AMB-user-auth-007 | Does the five-failure username limiter also count TOTP and recovery failures? | REQ-016 fixes the password limiter. REQ-017 caps code attempts per challenge at five. Both may answer `429`; neither reveals whether the username exists. | Low | No |
| AMB-user-auth-008 | Can `0600` be asserted on Windows? | Production code requests `0600` on create and replace. On Windows Go synthesises permission bits, so the acceptance assertion is platform-dependent and must be reported as such rather than silently skipped. | Med | No |

## Verdict summary

| Metric | Count |
| :-- | :-- |
| Requirements | 26 |
| Testable | 26 |
| Testable with assumption | 0 |
| Blocked | 0 |
| Scenarios defined by the frozen plan | 25 |
| Blocking ambiguities | 0 |
