# Test Plan: user-auth (extension)

> Extension plan for the authentication slice. It does not modify or supersede
> `local-env/specs/stock-watchlist.md`; that plan and its freeze remain untouched.
> This plan is frozen independently before implementation.

## 1. Provenance

| Field | Value |
| :-- | :-- |
| Feature | `user-auth` |
| Spec artifact | `Spec/implementation-spec.md` v1.3 §11 (2026-11-18, append-only extension) |
| Original plan | `local-env/specs/stock-watchlist.md` @ `7ed3f26e…` (unchanged) |
| Requirement register | `spec-to-test-workflow/stock-dashboard/00-ingestion/requirement-register-auth.md` |
| Scenarios | `spec-to-test-workflow/stock-dashboard/03-scenarios/auth.md` |
| Checklist | `spec-to-test-workflow/stock-dashboard/04-checklist/auth-checklist.md` |
| Task list | `spec-to-test-workflow/stock-dashboard/05-task-lists/auth-implementation.md` |
| Review | `spec-to-test-workflow/stock-dashboard/06-review/auth-decisions.md` |
| Executable evidence | `stock-dashboard-go/server/internal/auth`, `web/src/features/auth`, `e2e/tests/auth.spec.ts` |

This plan targets a **single owner account** for the existing local, single-user product.
Multi-user tenancy, roles, invitations and password reset by email are out of scope.

## 2. Scope

In scope:

- First-run account setup: username, password, mandatory TOTP enrolment, recovery codes.
- Password login plus TOTP second factor, or a recovery code.
- Session cookie, idle and absolute expiry, logout, and session-expiry handling.
- Protection of `/`, `/chart/:ticker`, `/api/watchlist*`, `/api/quotes`, `/api/chart/*`.
- Brute-force limiting, generic errors, CSRF origin checks, no secret leakage.
- Persistence of the owner account in `$DATA_DIR/users.json` (only new persisted file).

Out of scope:

- Multiple users, roles, sharing, invitations, public registration after setup.
- Email/SMS delivery, password reset links, social or passkey login.
- Remember-me checkboxes, device management, audit UI.
- HTTPS termination (the local app stays loopback HTTP; `AUTH_COOKIE_SECURE` supports TLS).

## 3. Requirement Register

| ID | Requirement | Type | Level |
| :-- | :-- | :-- | :-- |
| REQ-user-auth-001 | First start with no owner reports `setupRequired: true`; `/` redirects to `/setup`. | Flow | Both |
| REQ-user-auth-002 | The setup username is 3–32 characters, lowercase, matching `^[a-z0-9][a-z0-9._-]{2,31}$`. | Rule | API |
| REQ-user-auth-003 | The setup password is 12–128 characters. | Rule | API |
| REQ-user-auth-004 | Setup returns a TOTP secret, an `otpauth://` URI and ten recovery codes. | Contract | API |
| REQ-user-auth-005 | Setup is pending only; no owner is persisted and no session exists until activation. | Rule | API |
| REQ-user-auth-006 | Setup after an owner exists returns `409 SETUP_COMPLETE`. | Rule | API |
| REQ-user-auth-007 | Activation with a valid TOTP code persists the owner and creates a session. | Flow | Both |
| REQ-user-auth-008 | Activation with an invalid or replayed code returns `401 INVALID_CODE`. | Rule | API |
| REQ-user-auth-009 | Login requires username and password; missing fields return `400 INVALID_REQUEST`. | Rule | API |
| REQ-user-auth-010 | Wrong username and wrong password produce the same `401 INVALID_CREDENTIALS` copy. | Rule | API |
| REQ-user-auth-011 | A correct password returns `totp_required` and a five-minute challenge, not a session. | Contract | Both |
| REQ-user-auth-012 | An expired challenge returns `401 CHALLENGE_EXPIRED`. | Rule | API |
| REQ-user-auth-013 | TOTP accepts six digits within ±1 thirty-second step. | Rule | API |
| REQ-user-auth-014 | A TOTP time step already used for login cannot be replayed. | Rule | API |
| REQ-user-auth-015 | A recovery code completes login once and is then removed. | Flow | Both |
| REQ-user-auth-016 | Five failed password attempts for one username in 15 minutes returns `429` for 60 seconds. | Rule | API |
| REQ-user-auth-017 | A challenge allows at most five code attempts, then it is destroyed. | Rule | API |
| REQ-user-auth-018 | The session cookie is `HttpOnly`, `SameSite=Strict`, `Path=/`, 12-hour absolute expiry and 30-minute idle expiry. | Rule | API |
| REQ-user-auth-019 | Logout revokes the current session. | Flow | Both |
| REQ-user-auth-020 | Protected API requests without a session return `401 UNAUTHENTICATED` with `Authentication required.` | Rule | Both |
| REQ-user-auth-021 | Protected pages redirect unauthenticated visitors to `/login`; setup-required visitors go to `/setup`. | Flow | UI |
| REQ-user-auth-022 | Passwords are stored as Argon2id hashes; TOTP secrets and recovery hashes never appear in logs or error bodies. | Rule | API |
| REQ-user-auth-023 | Mutating requests with a mismatched `Origin` header are rejected. | Rule | API |
| REQ-user-auth-024 | `$DATA_DIR/users.json` is written atomically with mode `0600`. | Data | API |
| REQ-user-auth-025 | `/healthz` and static assets remain public; data endpoints require a session. | Contract | Both |
| REQ-user-auth-026 | The SPA shows `Your session expired. Sign in again.` when a session ends mid-use. | Flow | UI |

## 4. Scenarios

Tags use `SCN-user-auth-NNN`, matching the guard grammar.

### SCN-user-auth-001 @smoke — First run requires setup

```gherkin
Scenario: First run requires setup
  Given no owner account exists
  When the user opens "/"
  Then the page shows "Set up your account"
  And the API status reports setupRequired true
```

### SCN-user-auth-002 @reg — Setup returns authenticator material

```gherkin
Scenario: Setup returns authenticator material
  When the user submits username "owner" and a 12-character password
  Then the response is 201
  And the body contains an "otpauth://totp/" URI
  And the body contains 10 recovery codes
  And the page shows a QR code and the setup key
```

### SCN-user-auth-003 @reg — Setup validates the username

```gherkin
Scenario Outline: Setup validates the username
  When the user submits username "<username>"
  Then the response is 400 with code "INVALID_USERNAME"
  And the message is "Use 3-32 lowercase letters, numbers, dots, dashes or underscores."

  Examples:
    | username |
    | ab       |
    | _owner   |
    | Owner    |
    | owner!   |
```

### SCN-user-auth-004 @reg — Setup validates the password

```gherkin
Scenario Outline: Setup validates the password
  When the user submits a password of <length> characters
  Then the response is 400 with code "WEAK_PASSWORD"
  And the message is "Use at least 12 characters."

  Examples:
    | length |
    | 0      |
    | 11     |
```

### SCN-user-auth-005 @reg — Setup stays pending until activation

```gherkin
Scenario: Setup stays pending until activation
  When the user submits valid setup details
  Then no session cookie is issued
  And the status still reports setupRequired true
  And users.json does not exist
```

### SCN-user-auth-006 @reg — A valid code activates the owner

```gherkin
Scenario: A valid code activates the owner
  Given a pending setup for "owner"
  When the user submits the current authenticator code
  Then the response is 200
  And a session cookie is set
  And users.json contains one owner with TOTP enabled
```

### SCN-user-auth-007 @reg — Invalid activation code is refused

```gherkin
Scenario: Invalid activation code is refused
  Given a pending setup for "owner"
  When the user submits the code "000000" (invalid for the secret)
  Then the response is 401 with code "INVALID_CODE"
  And the message is "Invalid verification code."
```

### SCN-user-auth-008 @reg — Setup is one-way

```gherkin
Scenario: Setup is one-way
  Given an activated owner exists
  When a second setup request is submitted
  Then the response is 409 with code "SETUP_COMPLETE"
  And the message is "Setup is already complete."
```

### SCN-user-auth-009 @smoke — Password login requires a second factor

```gherkin
Scenario: Password login requires a second factor
  Given an activated owner exists
  When the user signs in with the correct password
  Then the response is 200 with status "totp_required"
  And no session cookie is set
  And the page shows "Two-factor authentication"
```

### SCN-user-auth-010 @smoke — Correct password and code create a session

```gherkin
Scenario: Correct password and code create a session
  Given a successful password step
  When the user submits the current authenticator code
  Then the session cookie is set
  And the dashboard shows the watchlist
```

### SCN-user-auth-011 @reg — Wrong credentials are indistinguishable

```gherkin
Scenario: Wrong credentials are indistinguishable
  Given an activated owner exists
  When the user submits a wrong password or unknown username
  Then the response is 401 with code "INVALID_CREDENTIALS"
  And the message is "Invalid username or password."
```

### SCN-user-auth-012 @reg — Wrong TOTP code is refused

```gherkin
Scenario: Wrong TOTP code is refused
  Given a successful password step
  When the user submits an invalid code
  Then the response is 401 with code "INVALID_CODE"
  And the message is "Invalid verification code."
```

### SCN-user-auth-013 @reg — TOTP codes cannot be replayed

```gherkin
Scenario: TOTP codes cannot be replayed
  Given a session was created with the current code
  When a second login submits the same code
  Then the response is 401 with code "INVALID_CODE"
```

### SCN-user-auth-014 @reg — A recovery code works once

```gherkin
Scenario: A recovery code works once
  Given a successful password step
  When the user submits recovery code "AAAA-BBBB-CCCC-DDDD"
  Then the session cookie is set
  And the same recovery code is refused on the next attempt
```

### SCN-user-auth-015 @reg — An expired challenge is refused

```gherkin
Scenario: An expired challenge is refused
  Given a password step completed 5 minutes ago
  When the user submits a valid code
  Then the response is 401 with code "CHALLENGE_EXPIRED"
  And the message is "Your sign-in attempt expired. Sign in again."
```

### SCN-user-auth-016 @reg — Password attempts are rate limited

```gherkin
Scenario: Password attempts are rate limited
  Given five failed password attempts for "owner" within 15 minutes
  When a sixth attempt is made
  Then the response is 429 with code "RATE_LIMITED"
  And the response carries a Retry-After header
```

### SCN-user-auth-017 @reg — A challenge allows five code attempts

```gherkin
Scenario: A challenge allows five code attempts
  Given a successful password step
  When five invalid codes are submitted
  Then the challenge is destroyed
  And the sixth attempt returns 429 or CHALLENGE_EXPIRED
```

### SCN-user-auth-018 @smoke — Protected endpoints require a session

```gherkin
Scenario: Protected endpoints require a session
  Given no session cookie
  When the user requests "/api/quotes"
  Then the response is 401 with code "UNAUTHENTICATED"
  And the message is "Authentication required."
```

### SCN-user-auth-019 @reg — Setup-required and unauthenticated routes

```gherkin
Scenario: Setup-required and unauthenticated routes
  Given no session
  When the user opens "/chart/AAPL"
  Then the page shows the sign-in screen
  And a link to "/login" is not usable before setup when setup is required
```

### SCN-user-auth-020 @reg — Logout revokes the session

```gherkin
Scenario: Logout revokes the session
  Given an authenticated session
  When the user signs out
  Then the session cookie is cleared
  And the next protected request returns 401
```

### SCN-user-auth-021 @reg — Session expiry is announced

```gherkin
Scenario: Session expiry is announced
  Given an authenticated page
  When the session is removed server-side
  Then the next request shows "Your session expired. Sign in again."
```

### SCN-user-auth-022 @reg — No secret leakage

```gherkin
Scenario: No secret leakage
  Given a failed login and a failed TOTP attempt
  Then no response contains the password, TOTP secret, session token or recovery code hashes
  And no server log line contains them
```

### SCN-user-auth-023 @reg — Health and assets stay public

```gherkin
Scenario: Health and assets stay public
  Given no session
  When the user requests "/healthz" and the client assets
  Then both return 200
```

### SCN-user-auth-024 @reg — Cross-origin mutations are refused

```gherkin
Scenario: Cross-origin mutations are refused
  Given a request with Origin "https://evil.example"
  When the user posts to "/api/auth/login"
  Then the response is 403 with code "CROSS_ORIGIN_REQUEST"
```

### SCN-user-auth-025 @reg — The account file is private and atomic

```gherkin
Scenario: The account file is private and atomic
  Given an activated owner
  When the data directory is inspected
  Then users.json exists with mode 0600
  And no temporary file remains after activation
```

## 5. State transitions

| From | To | Guard |
| :-- | :-- | :-- |
| No owner | Pending setup | Valid username and password |
| Pending setup | Owner active | Valid TOTP code |
| Pending setup | Gone | 15-minute expiry or restart |
| Owner active | Password verified | Correct credentials |
| Password verified | Session active | Valid TOTP or unused recovery code |
| Password verified | Gone | Five failed codes or 5-minute expiry |
| Session active | Signed out | Logout or 12-hour/30-minute expiry |

## 6. API matrix

| Method | Path | Auth | Success | Errors |
| :-- | :-- | :-- | :-- | :-- |
| GET | `/api/auth/status` | public | 200 status | none |
| POST | `/api/auth/setup` | public, no owner | 201 setup material | 400, 409 |
| POST | `/api/auth/2fa/activate` | enroll cookie | 200 user + session | 401 |
| POST | `/api/auth/login` | public | 200 totp_required | 400, 401, 429 |
| POST | `/api/auth/login/totp` | challenge cookie | 200 user + session | 401, 429 |
| POST | `/api/auth/login/recovery` | challenge cookie | 200 user + session | 401, 429 |
| POST | `/api/auth/logout` | session optional | 204 | none |
| GET | `/api/auth/me` | session | 200 user | 401 |

## 7. UI contract

| Element | Role / name | Copy |
| :-- | :-- | :-- |
| Setup heading | heading | `Set up your account` |
| Sign-in heading | heading | `Sign in` |
| Username | textbox `Username` | |
| Password | textbox `Password` | |
| Confirm password | textbox `Confirm password` | setup only |
| Primary action | button | `Create account` / `Sign in` / `Continue` |
| TOTP heading | heading | `Two-factor authentication` |
| TOTP input | textbox `Verification code` | six digits, `one-time-code` |
| Recovery switch | button | `Use a recovery code` |
| Recovery input | textbox `Recovery code` | |
| QR | image `Authenticator QR code` | from `otpauthUrl` |
| Setup key | text | `Or enter this setup key manually:` then the secret |
| Recovery codes | list | ten codes with `Copy` |
| Sign out | button | `Sign out` |
| Session expiry | alert | `Your session expired. Sign in again.` |

All controls have accessible names; the error region is `role="alert"`; the QR text
alternative names the account. Password fields use `autocomplete="current-password"` or
`new-password`.

## 8. Security checks

| Check | Required result |
| :-- | :-- |
| Password storage | Argon2id PHC string only |
| Session token storage | SHA-256 digest only, 32-byte random token |
| Cookie flags | `HttpOnly; SameSite=Strict; Path=/`; `Secure` when configured |
| TOTP replay | Rejected by stored last-used step |
| Recovery codes | SHA-256 hashes, single use, constant-time compare |
| Enumeration | Same copy and comparable timing for unknown users |
| Rate limit | 5 password failures / 15 min / username; Retry-After |
| CSRF | Mismatched Origin rejected; SameSite=Strict cookies |
| Logging | No password, code, secret, token or hash |
| File mode | `users.json` 0600, atomic replace |

## 9. Test mapping

| Surface | Location |
| :-- | :-- |
| Auth primitives | `server/internal/auth/*_test.go` |
| API flow | `server/internal/auth` and `server/internal/api` tests |
| UI flow | `web/src/features/auth/*.test.tsx` |
| End-to-end | `e2e/tests/auth.spec.ts` and the authenticated fixture |
| Existing product regression | existing Go, Vitest and Playwright suites |

The Go application is not covered by this repository's `tests/` lock; the plan is frozen
with `tests: null` and evidenced by the suites above.
