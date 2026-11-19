# Phase 3 — Scenarios: user-auth (extension)

Feature: `user-auth`
Plan: `local-env/specs/user-auth.md` §4 (frozen 2026-11-18T03:47:51.993Z)
Requirements: `../00-ingestion/requirement-register-auth.md`
Status: planned — no executable evidence yet.

The 25 scenarios below are the frozen plan §4 scenarios, with the plan's titles, steps and
copy preserved. Tags add the `REQ` link used by the guard; each scenario keeps its plan tag
(`@smoke` or `@reg`).

```gherkin
Feature: Owner account setup, sign-in and sessions
  As the local owner
  I want the dashboard protected by a password and an authenticator
  So that only I can read and change my watchlist

  @REQ-user-auth-001 @SCN-user-auth-001 @smoke
  Scenario: First run requires setup
    Given no owner account exists
    When the user opens "/"
    Then the page shows "Set up your account"
    And the API status reports setupRequired true

  @REQ-user-auth-004 @SCN-user-auth-002 @reg
  Scenario: Setup returns authenticator material
    When the user submits username "owner" and a 12-character password
    Then the response is 201
    And the body contains an "otpauth://totp/" URI
    And the body contains 10 recovery codes
    And the page shows a QR code and the setup key

  @REQ-user-auth-002 @SCN-user-auth-003 @reg
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

  @REQ-user-auth-003 @SCN-user-auth-004 @reg
  Scenario Outline: Setup validates the password
    When the user submits a password of <length> characters
    Then the response is 400 with code "WEAK_PASSWORD"
    And the message is "Use at least 12 characters."

    Examples:
      | length |
      | 0      |
      | 11     |

  @REQ-user-auth-005 @SCN-user-auth-005 @reg
  Scenario: Setup stays pending until activation
    When the user submits valid setup details
    Then no session cookie is issued
    And the status still reports setupRequired true
    And users.json does not exist

  @REQ-user-auth-007 @SCN-user-auth-006 @reg
  Scenario: A valid code activates the owner
    Given a pending setup for "owner"
    When the user submits the current authenticator code
    Then the response is 200
    And a session cookie is set
    And users.json contains one owner with TOTP enabled

  @REQ-user-auth-008 @SCN-user-auth-007 @reg
  Scenario: Invalid activation code is refused
    Given a pending setup for "owner"
    When the user submits the code "000000" (invalid for the secret)
    Then the response is 401 with code "INVALID_CODE"
    And the message is "Invalid verification code."

  @REQ-user-auth-006 @SCN-user-auth-008 @reg
  Scenario: Setup is one-way
    Given an activated owner exists
    When a second setup request is submitted
    Then the response is 409 with code "SETUP_COMPLETE"
    And the message is "Setup is already complete."

  @REQ-user-auth-009 @REQ-user-auth-011 @SCN-user-auth-009 @smoke
  Scenario: Password login requires a second factor
    Given an activated owner exists
    When the user signs in with the correct password
    Then the response is 200 with status "totp_required"
    And no session cookie is set
    And the page shows "Two-factor authentication"

  @REQ-user-auth-013 @REQ-user-auth-018 @SCN-user-auth-010 @smoke
  Scenario: Correct password and code create a session
    Given a successful password step
    When the user submits the current authenticator code
    Then the session cookie is set
    And the dashboard shows the watchlist

  @REQ-user-auth-010 @SCN-user-auth-011 @reg
  Scenario: Wrong credentials are indistinguishable
    Given an activated owner exists
    When the user submits a wrong password or unknown username
    Then the response is 401 with code "INVALID_CREDENTIALS"
    And the message is "Invalid username or password."

  @REQ-user-auth-013 @SCN-user-auth-012 @reg
  Scenario: Wrong TOTP code is refused
    Given a successful password step
    When the user submits an invalid code
    Then the response is 401 with code "INVALID_CODE"
    And the message is "Invalid verification code."

  @REQ-user-auth-014 @SCN-user-auth-013 @reg
  Scenario: TOTP codes cannot be replayed
    Given a session was created with the current code
    When a second login submits the same code
    Then the response is 401 with code "INVALID_CODE"

  @REQ-user-auth-015 @SCN-user-auth-014 @reg
  Scenario: A recovery code works once
    Given a successful password step
    When the user submits recovery code "AAAA-BBBB-CCCC-DDDD"
    Then the session cookie is set
    And the same recovery code is refused on the next attempt

  @REQ-user-auth-012 @SCN-user-auth-015 @reg
  Scenario: An expired challenge is refused
    Given a password step completed 5 minutes ago
    When the user submits a valid code
    Then the response is 401 with code "CHALLENGE_EXPIRED"
    And the message is "Your sign-in attempt expired. Sign in again."

  @REQ-user-auth-016 @SCN-user-auth-016 @reg
  Scenario: Password attempts are rate limited
    Given five failed password attempts for "owner" within 15 minutes
    When a sixth attempt is made
    Then the response is 429 with code "RATE_LIMITED"
    And the response carries a Retry-After header

  @REQ-user-auth-017 @SCN-user-auth-017 @reg
  Scenario: A challenge allows five code attempts
    Given a successful password step
    When five invalid codes are submitted
    Then the challenge is destroyed
    And the sixth attempt returns 429 or CHALLENGE_EXPIRED

  @REQ-user-auth-020 @SCN-user-auth-018 @smoke
  Scenario: Protected endpoints require a session
    Given no session cookie
    When the user requests "/api/quotes"
    Then the response is 401 with code "UNAUTHENTICATED"
    And the message is "Authentication required."

  @REQ-user-auth-021 @SCN-user-auth-019 @reg
  Scenario: Setup-required and unauthenticated routes
    Given no session
    When the user opens "/chart/AAPL"
    Then the page shows the sign-in screen
    And a link to "/login" is not usable before setup when setup is required

  @REQ-user-auth-019 @SCN-user-auth-020 @reg
  Scenario: Logout revokes the session
    Given an authenticated session
    When the user signs out
    Then the session cookie is cleared
    And the next protected request returns 401

  @REQ-user-auth-026 @SCN-user-auth-021 @reg
  Scenario: Session expiry is announced
    Given an authenticated page
    When the session is removed server-side
    Then the next request shows "Your session expired. Sign in again."

  @REQ-user-auth-022 @SCN-user-auth-022 @reg
  Scenario: No secret leakage
    Given a failed login and a failed TOTP attempt
    Then no response contains the password, TOTP secret, session token or recovery code hashes
    And no server log line contains them

  @REQ-user-auth-025 @SCN-user-auth-023 @reg
  Scenario: Health and assets stay public
    Given no session
    When the user requests "/healthz" and the client assets
    Then both return 200

  @REQ-user-auth-023 @SCN-user-auth-024 @reg
  Scenario: Cross-origin mutations are refused
    Given a request with Origin "https://evil.example"
    When the user posts to "/api/auth/login"
    Then the response is 403 with code "CROSS_ORIGIN_REQUEST"

  @REQ-user-auth-024 @SCN-user-auth-025 @reg
  Scenario: The account file is private and atomic
    Given an activated owner
    When the data directory is inspected
    Then users.json exists with mode 0600
    And no temporary file remains after activation
```

## Notes

- **Timing.** Challenge expiry (5 minutes), session idle (30 minutes), session absolute
  (12 hours), rate-limit windows (15 minutes, 60 seconds) and the 15-minute setup window are
  all driven by an injected clock in Go tests. No scenario may wait on wall-clock time.
- **Fake clock.** Advance the same clock the auth service reads; cookie expiry headers must
  be asserted against that clock, not `time.Now()` in the test.
- **TOTP determinism.** Derive codes from the `secret` returned by setup at a fixed step.
  TOTP is SHA-1, six digits, 30-second period, ±1 step. Pin the clock so SCN-010, SCN-012,
  SCN-013 and SCN-015 are reproducible. For replay, reuse the exact code from the accepted
  step; a new code in a later step must succeed.
- **Fresh `DATA_DIR` per test.** Each test gets its own temporary directory, so `users.json`
  and `watchlist.json` never leak between scenarios. Only the restart case deliberately
  reopens the same directory.
- **SCN-017** accepts either refusal, but the challenge must be unusable afterwards
  (`AMB-user-auth-001`). **SCN-019** runs in the no-owner state, so §11.3 sends the user to
  `/setup`; the "sign-in screen" wording is read as the pre-auth screen (`AMB-user-auth-005`).
- **SCN-025** asks for POSIX `0600`. On Windows Go synthesises permission bits, so the mode
  assertion is platform-dependent and must be reported honestly (`AMB-user-auth-008`).
- Copy strings are asserted exactly as written. `INVALID_RECOVERY_CODE` and
  `ENROLLMENT_EXPIRED` have no copy in spec §11.7; scenarios that need them assert status
  and code only.
- Two scenarios carry a second `REQ` tag. SCN-009 also covers REQ-009 (login requires both
  fields); SCN-010 also covers REQ-018 at the only frozen cookie-setting step. Cookie flags
  and lifetimes are asserted by the Go API tests, not by extra scenario steps.
- The plan has no scenario for pending-setup expiry (15 minutes) or for `ENROLLMENT_EXPIRED`;
  these remain implementation obligations from spec §11.3/§11.5, not frozen scenarios.
