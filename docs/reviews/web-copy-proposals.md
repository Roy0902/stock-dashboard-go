# Proposed copy

Strings introduced by review fixes that are not yet in `docs/contracts.md`. Kept minimal on
purpose; the existing contract strings are reused wherever they already say the right thing.

All strings now live in the single locale module `web/src/locale/en.ts`. Components read
`copy` directly; do not add per-feature copy modules.

## Web review fixes (2026-11-18)

| Proposed string | Where | Why it is needed |
| :-- | :-- | :-- |
| `Show password` | `features/auth/AuthPasswordField.tsx` reveal button (`aria-pressed="false"`) | Password reveal control had no label. |
| `Hide password` | same control when pressed (`aria-pressed="true"`) | Same. |
| `Try again` | `features/auth/AuthUnavailable.tsx` | Retry action for a failed session bootstrap (network / 5xx). The message stays the contract `INTERNAL_ERROR` copy. |
| `Reload` | `AppBoundary.tsx` | Recovery action after a render exception; the message stays the contract `INTERNAL_ERROR` copy. |
| `Code expires in N minutes.` / `Code expires in N seconds.` (`N = 1` singular) | `locale/en.ts` `auth.codeExpiry`, rendered on the two-factor step | Surfaces the `expiresInSeconds` value the login challenge already returns; no countdown. |
| `Loading watchlist` | `features/watchlist/WatchlistSkeleton.tsx` (`role="status"`) | Announced loading state; the grid used to be silent. |
| `Loading price` | `features/watchlist/QuoteRow.tsx` pending state | A just-added ticker has no quote yet; this is not the terminal `Price unavailable`. |
| `No data for this range` | `features/chart/ChartView.tsx` | A successful 200 with zero points is not the same as the `Chart unavailable` failure. |
| `Adding…` | `message`/`features/watchlist/AddTickerForm.tsx` pending button label | Pending label change for the add control. |

Reused existing contract copy instead of inventing it:

- Password rule hint: `Use at least 12 characters.` (`WEAK_PASSWORD`) before submit.
- Bootstrap failure and boundary message: `Something went wrong. Try again shortly.`
  (`INTERNAL_ERROR`).
- Verification helper text: `Enter the 6-digit code from your authenticator app.` (existing lede).

## Approval

None of the strings above are in `docs/contracts.md` yet. They need product approval before
they become part of the contract; until then they are implementation copy and are listed here
so the gap is explicit.
