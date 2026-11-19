# Web Code Review

Scope: `web/` (`src/App.tsx`, `main.tsx`, `routes.ts`, `api/`, `lib/`, `pages/`, `features/`, `ui/`, `styles/`, `test/`) plus `e2e/` for UI-behaviour coverage. Read-only review; no source changed.

Baseline re-verified in `web/` on 2026-11-18:

| Gate | Result |
| :-- | :-- |
| `npm run typecheck` | pass |
| `npm run lint` | pass |
| `npm test` | 12 files / 94 tests pass |
| `npm audit --omit=dev` | 0 vulnerabilities |

Visual bar: `.dev/reference/{linear,github,notion,vercel,stripe,google}.png`, used for hierarchy, form craft and density — not as designs to copy.

## Verdict

**Approve with changes — strong foundation, not yet at the commercial-reference bar.**

The engineering substrate is genuinely good: strict TS with `noUncheckedIndexedAccess`, runtime parsing of every API body instead of `as` casts, server-owned numbers and copy, token-based CSS with dark/light parity, reduced-motion support, and role-based tests that assert the contract strings exactly. That is above the average React codebase.

What holds it back is the layer a senior reviewer feels before they read code: **where focus goes after navigation**, **what a brand-new install sees**, **what happens when one request blips**, **what the light theme looks like**, and **what happens at 900 px wide**. Those are the five themes of this review. Component duplication (Login/Setup), missing form-error wiring, and a table that behaves visually like a table but has no table semantics are the next tier.

Counts: **0 critical, 5 high, 10 medium, 9 low**, plus 10 appendix one-liners.

## What is already good

- **API boundary is defensive, not trusting.** Every response is validated (`api/client.ts:85-192`) and a malformed success body becomes `INTERNAL_ERROR` rather than a render-time crash. `ApiResult<T>` is a discriminated union, `unwrap` is the single throw point, and abort errors are re-thrown so TanStack can ignore them (`client.ts:194-199,216-230`).
- **Behaviour placement respects the written boundaries.** `lib/format.ts` only formats; change/percent/direction/staleness come from the API (`QuoteRow.tsx:37-55`). No component recomputes derived quote arithmetic.
- **Auth is cookie-only and the 401 path is narrow.** Tokens never touch JS; the expiry handler fires only for `401 + UNAUTHENTICATED` (`client.ts:31-35`), so a wrong password is not reported as an expired session — and that is explicitly tested (`api/auth.test.ts:220-242`).
- **Cache hygiene on the two paths that matter.** `queryClient.clear()` on sign-out and on session expiry (`AuthProvider.tsx:75,103`) is the right blunt instrument for a shared cookie jar.
- **Accessible chart done properly.** `role="img"` with an `aria-label` that matches the contract plus a `<desc>` narrating range, low and high (`PriceChart.tsx:188-204`), responsive via `ResizeObserver` with a non-observing fallback (`32-63`), and no chart dependency.
- **Design system discipline.** Tokens cover colour/space/radius/motion; light theme is a full parallel token block (`tokens.css:83-125`); `prefers-reduced-motion` is handled both in tokens and globally (`tokens.css:127-132`, `global.css:134-143`); focus rings and a skip link exist (`global.css:78-107`).
- **Loading / empty / error triads exist** for both the dashboard and the chart, including skeletons, and the empty state is the exact contract copy (`DashboardPage.tsx:14,31-33`).
- **Tests are role-first.** `getByRole('listitem', { name: 'AAPL' })`, `getByRole('group', { name: 'Range' })`, accessible-name checks on the SVG — not `data-testid` soup. E2E covers the hard cases: provider fallback, both providers down, missing keys, session expiry mid-use, recovery-code single use, multi-account isolation (`e2e/tests/*.spec.ts`).

## Findings

### [High] Nothing moves focus on client-side navigation
**Where:** `web/src/App.tsx:17-21` (`<main id="main-content" tabIndex={-1}>` exists, but is never focused), `web/src/features/watchlist/QuoteRow.tsx:59` → `web/src/pages/ChartPage.tsx:27`
**Smell:** The skip link and the focusable `<main>` are in place, but only the auth step transitions move focus (`LoginPage.tsx:46-52`, `SetupPage.tsx:39-45`). `App` has no effect on `location.pathname`.
**Why it matters:** Activating "View chart" unmounts the focused link, so focus falls to `<body>`. Keyboard and screen-reader users lose their position, the new `<h1>` is not announced, and the skip link is the only recovery path. This is the single most noticeable a11y regression versus Linear/GitHub, which announce the new view.
**Concrete fix:** Add a `useEffect` in `App` keyed on `useLocation().pathname` that focuses `mainRef` (or the page `<h1>` with `tabIndex={-1}`) and optionally calls `scrollTo(0,0)`. Skip the first render so initial load keeps normal document order.
**Effort:** S

### [High] First run lands on marketing copy and a sign-in form that cannot succeed
**Where:** `web/src/pages/LandingRoute.tsx:11-17`, `web/src/features/auth/AuthGate.tsx:23-26`, `web/src/features/auth/LoginPage.tsx:151-153`
**Smell:** `LandingRoute` branches only on `loading` and `authenticated`. `setupRequired` falls through to the marketing page. `AuthGate mode="login"` also lets `setupRequired` through, and `LoginPage` then renders the credential form while merely hiding the lede.
**Why it matters:** `docs/decisions.md` D-29 says "First-run still goes to `/setup`". On a fresh install a user sees a hero, clicks **Sign in**, and gets a username/password form that can only return an error; setup is one more click away under "New here?". The empty-database case is the worst possible first impression.
**Concrete fix:** In `LandingRoute` and `AuthGate` (`mode="login"`), `if (phase === 'setupRequired') return <Navigate to={ROUTE_PATHS.setup} replace />;`, or render a single `Create the first account` CTA on the landing page. Keep the `New here?` link for the post-setup case.
**Effort:** S

### [High] One transient failure becomes a persistent error; no boundary if render throws
**Where:** `web/src/main.tsx:10-20` (`retry: false`), `web/src/features/quotes/useQuotes.ts:19-20` (`refetchOnWindowFocus:false`, `refetchOnReconnect:false`), `web/src/App.tsx:22-50` (no error boundary)
**Smell:** Every query is configured to fail instantly and never recover from reconnection; the only recovery is the 60 s poll, and a *render* exception blanks `#root` because React 19's root has no `onUncaughtError`/boundary.
**Why it matters:** A laptop sleeping through a Wi-Fi handover, or a 2 s provider timeout, replaces the whole quote table with `Data provider is unavailable…` and keeps it there for a minute, with no retry affordance and no "retrying" state. A single bad `Intl` result or chart edge case turns the app white. Commercial dashboards always degrade locally.
**Concrete fix:** Query defaults `retry: 2` with exponential backoff, retry only `NETWORK_ERROR`/5xx (not 4xx), `refetchOnReconnect: true`; add an `ErrorBoundary` around `<main>` with the contract `Something went wrong. Try again shortly.` copy and a Reload action.
**Effort:** M

### [High] Up/down colours are hard-coded, bypass the tokens, and fail AA in light mode
**Where:** `web/src/features/watchlist/QuoteRow.css:77-92` (`#16a34a`, `#dc2626`, `#6b7280`), duplicated in `web/src/styles/tokens.css:58-60`
**Smell:** The comment at `QuoteRow.css:77-78` states the literals exist so a component test can read `getComputedStyle` without loading the token sheet. The tokens already exist and are used for the direction dot (`125-131`), so the same meaning now has two sources.
**Why it matters:** `#16a34a` on `--color-surface:#ffffff` is **3.3:1**, and the change text is `--text-sm` (14 px, weight 650) — below the 4.5:1 AA threshold for normal text. In light mode the most important number on the page is the least legible, and a future palette change silently desynchronises the arrow-less direction dot from the number. This is the one place where the test suite is actively driving a visual defect.
**Concrete fix:** Use `var(--color-up/down/neutral)`; add light-mode overrides in `tokens.css` (`--color-up:#15803d` gives ≈4.7:1 on white). Assert the class/`aria` meaning in the unit test and check colour in a visual/contrast test instead of `getComputedStyle` hex.
**Effort:** S

### [High] The 900 px breakpoint does not fit the grid, and `overflow-x:hidden` hides the damage
**Where:** `web/src/features/watchlist/WatchlistList.css:2-3`, `web/src/features/watchlist/QuoteRow.css:3-7,201`, `web/src/styles/global.css:17`, `web/src/features/chart/PriceChart.tsx:27,45,124`
**Smell:** The seven tracks have a 710 px minimum; with 6×16 px gaps, 2×18 px padding and 2 px border a row needs **844 px**. The layout only collapses at `max-width: 900px`, while `.app-main` subtracts `clamp(1rem,3vw,2rem)` per side (~27 px at 901 px) and the scrollbar takes ~15 px — leaving ≈832 px. `body { overflow-x: hidden }` then hides the overflow instead of exposing a scrollbar. Separately, the chart clamps measured width up to `MIN_WIDTH = 280`, so in a 240 px container the `width:100%` SVG is scaled to 0.86 and its 11 px labels render at ≈9.4 px.
**Why it matters:** Around 900–930 px viewports (small laptop window, tablet landscape, 125 % zoom) the rightmost column — the **View chart** link — is clipped with no indication. On a 320–360 px phone the chart axes become sub-legible. Both are invisible in jsdom tests and in the e2e suite (no viewport projects).
**Concrete fix:** Move the row breakpoint to `max-width: 960px` (or use `@container` on `.watchlist`) and delete `overflow-x:hidden` from `body`; lower `MIN_WIDTH` to ~240 and switch to a 3-tick axis plus `insets.left: 48` under 400 px.
**Effort:** S

### [Medium] Field errors are never associated with the fields that failed
**Where:** `web/src/features/auth/LoginPage.tsx:161-165` (single `Alert` above the form), `:173-205` (inputs with no `aria-invalid`/`aria-describedby`), `web/src/features/auth/AuthForms.css:49-51` (styling for `aria-invalid='true'` that nothing ever sets)
**Smell:** Validation and server errors are rendered once, outside the form, then inputs are left untouched. The `[aria-invalid]` rule is dead CSS.
**Why it matters:** A screen-reader user hears "Use at least 12 characters." but cannot tell which field is wrong, and there is no focus move to the invalid field. The same pattern repeats in `SetupPage` (`117-172`) and `AddTickerForm` (`42-68`). The dead rule is a fossil of the intended design.
**Concrete fix:** Per-field error state; `aria-invalid={Boolean(error)}` and `aria-describedby={errorId}` on the input; render the message next to the field; on submit, focus the first invalid input. Keep the single `role="alert"` slot for server errors only.
**Effort:** M

### [Medium] A repeated identical error is never re-announced
**Where:** `web/src/features/auth/LoginPage.tsx:137` (`const message = error ?? notice`), rendered at `:161-165`
**Smell:** The alert node is keyed by message content. Submitting `000000` twice sets the same string, React bails out, the DOM does not change, and the live region stays silent.
**Why it matters:** The most common auth failure — retrying the same wrong TOTP code — produces no feedback at all beyond the cursor, so users press Continue again and assume the app is frozen.
**Concrete fix:** Track an attempt counter and key the alert on it (or clear then set on the next frame); alternatively use a persistent `aria-live="polite"` region with an incrementing nonce appended as visually hidden text.
**Effort:** S

### [Medium] Password fields are write-only: no reveal, no rule, no hint, no `minLength`
**Where:** `web/src/features/auth/SetupPage.tsx:138-172`, `web/src/features/auth/validation.ts:22-25`, `web/src/features/auth/copy.ts:6`
**Smell:** The 12-character rule exists only as a message that appears after a failed submit. There is no helper text, no `minLength`, no reveal toggle, and the confirm field has no paste/autofill affordance.
**Why it matters:** On a phone, typing a 12+ character password blind into two fields with no stated rule is the highest-friction moment in the product. Every reference (GitHub, Stripe, Notion) shows the rule up front and offers reveal. A user who types 11 characters only learns why on submit.
**Concrete fix:** Add `<p className="auth-form__hint">At least 12 characters.</p>` under the password field, `minLength={12}`, a reveal button with `aria-pressed`, and keep submit-time validation as the server mirror.
**Effort:** S

### [Medium] TOTP step leaves obvious UX on the table — including data the API already returns
**Where:** `web/src/features/auth/LoginPage.tsx:213-266`, `web/src/api/client.ts:180-185` (`LoginChallenge.expiresInSeconds`), `web/src/api/types.ts:64-67`
**Smell:** One plain text input, no auto-advance or auto-submit at six digits, `pattern="[0-9]*"` (matches any length, so it constrains nothing), no `aria-describedby` linking the lede, and `Continue` is disabled until six digits with no explanation. `expiresInSeconds` is parsed and then never used anywhere in `src/`.
**Why it matters:** GitHub/Vercel/Stripe all use a segmented or auto-submitting code field, because users paste and mistype. The challenge expiry is invisible, so the only signal is an error *after* the code is submitted — and the user cannot tell whether the code was wrong or too late.
**Concrete fix:** Auto-submit at six digits (or segmented inputs with paste handling), `pattern="[0-9]{6}"`, helper text "The code refreshes every 30 seconds", and a small countdown or "expires in N minutes" derived from `expiresInSeconds`.
**Effort:** M

### [Medium] The watchlist looks like a table but has no table semantics
**Where:** `web/src/features/watchlist/WatchlistList.tsx:13-27` (header is a `div` of `span`s; body is a `ul` of `li`), `web/src/features/watchlist/WatchlistList.css:37-50` (header visually removed under 900 px)
**Smell:** Column labels live in a sibling `div`, are not associated with any cell, and are removed from the visual layout below 900 px with no per-cell replacement. A `listitem` named `AAPL` exposes seven unlabelled numbers/words.
**Why it matters:** Financial tabular data is exactly what tables are for; screen-reader users hear "AAPL, 190.12, +1.24, +0.66%, up, Finnhub, 16-Nov-2026" with no way to know which is which. The visual header gives sighted users context that AT users never get.
**Concrete fix:** Prefer a real `<table>` with `<caption class="sr-only">Watchlist</caption>` and `<th scope="col">`, keeping the grid with `display: contents` on rows or `role="row"/"columnheader"/"cell"`. Minimum viable: give each row cell an `aria-label`/visually hidden label.
**Effort:** M

### [Medium] Server cache is not scoped to the account
**Where:** `web/src/api/queryKeys.ts:3-7` (`['watchlist']`, `['quotes']`, `['chart', …]`), `web/src/features/auth/AuthProvider.tsx:72-84,99-108`
**Smell:** Cache is cleared on sign-out and on a 401, but keys carry no user identity and there is no cross-tab coordination.
**Why it matters:** Sign out of account A in tab 2 and sign in as B: tab 1 still holds A's watchlist and keeps polling. On the next 60 s tick it fetches B's quotes into a component still rendering A's tickers (mostly "Price unavailable"), and A's ticker list stays visible until a focus refetch past `staleTime: 30_000`. On a shared machine that is a small but real disclosure.
**Concrete fix:** Include the username in the key (`['watchlist', user.username]`) or call `queryClient.clear()` inside `acceptUser`; add a `storage`/`BroadcastChannel` signal on login/logout so other tabs clear too.
**Effort:** M

### [Medium] "Server unreachable" is presented as "signed out"
**Where:** `web/src/features/auth/AuthProvider.tsx:46-70`
**Smell:** Both the error branch and the `catch` set `phase='unauthenticated'` regardless of the failure kind — network, 500, malformed body.
**Why it matters:** If the API is down (or the JSON is broken), the user is dropped on the landing/sign-in page as if their session vanished, and may re-enter a password that cannot be checked. It also means the app cannot distinguish "not logged in" from "cannot tell", which is the case an error state exists for.
**Concrete fix:** Add an `AuthPhase` of `'unavailable'` (or reuse a `loadError`) when `result.error.code` is `NETWORK_ERROR`/`INTERNAL_ERROR`; render a retry panel with the generic copy instead of the landing page.
**Effort:** S

### [Medium] Login success has two competing redirects and the deep-link restore is untested
**Where:** `web/src/features/auth/AuthGate.tsx:23-26` (`<Navigate to="/dashboard" replace />` once authenticated) vs `web/src/features/auth/LoginPage.tsx:25-31,105-106,133-134` (imperative `navigate(destination)`)
**Smell:** The gate redirects declaratively the moment `acceptUser` flips the phase, while the handler separately navigates to the remembered `from`. Whichever update lands second wins, and nothing verifies the requested path survives.
**Why it matters:** `docs/decisions.md` D-29 promises "the requested page is restored after sign-in". `/chart/AAPL → /login → sign in` is the one flow where the two mechanisms can disagree; there is no unit test and no e2e test for it, so a regression would be silent.
**Concrete fix:** Pick one owner. Keep the gate for "already authenticated visitors", and have the login handler do the only post-auth navigation (or let the gate read `location.state.from`). Add a test asserting `/chart/AAPL` after sign-in.
**Effort:** S

### [Medium] A newly added ticker renders as a failure before its quote arrives
**Where:** `web/src/features/watchlist/useAddTicker.ts:13-16`, `web/src/pages/DashboardPage.tsx:55`, `web/src/features/watchlist/QuoteRow.tsx:21-33`, `web/src/features/watchlist/WatchlistList.css:32-35`
**Smell:** `onSuccess` writes the new watchlist into cache immediately, then invalidates quotes. The new row therefore renders from a quote map that cannot contain it, showing `Price unavailable` / `none` / `—` in muted grey until the refetch lands. At the same time `aria-busy` drops the whole list to `opacity: 0.72`, so every row flickers on every 60 s poll.
**Why it matters:** `Price unavailable` is a *terminal* state in the contract (provider `none`), so the UI briefly tells the user their add failed after it succeeded. The minute-by-minute dimming makes a calm dashboard feel unstable — the opposite of the landing page's promise.
**Concrete fix:** Pass a `pending` flag per ticker (or read `mutation.isPending` + the just-added ticker) and render a skeleton price cell; replace the global list opacity with a subtle 1–2 px progress bar or none at all.
**Effort:** S

### [Medium] No route-level code splitting; the QR library ships to everyone
**Where:** `web/src/App.tsx:1-11` (all pages imported eagerly), `web/src/features/auth/SetupPage.tsx:3` (`qrcode.react`), `web/package.json:21`
**Smell:** One entry chunk contains the landing page, both auth flows, the dashboard and the chart, plus `qrcode.react` — used only on a screen most users see once.
**Why it matters:** The bundle is the first-paint budget for a local tool opened dozens of times a day. Lazy routes plus a dynamic `qrcode.react` import would cut the initial payload and make the auth screens render sooner. There is also no `manualChunks`, so the cost only grows.
**Concrete fix:** `const SetupPage = lazy(() => import('@/features/auth/SetupPage'))` (named export → `.then(m => ({ default: m.SetupPage }))`), same for `ChartPage`; wrap `<Routes>` in `<Suspense fallback={<AuthSplash />}>`; `await import('qrcode.react')` inside the verify step.
**Effort:** M

### [Low] Login and Setup duplicate ~600 lines of the same form
**Where:** `web/src/features/auth/LoginPage.tsx:167-310` (three near-identical form branches), `web/src/features/auth/SetupPage.tsx:117-259`
**Smell:** Label + input + error + `aria-busy` + pending-boolean + `if (pending) return` + `setError` on every keystroke is copy-pasted four times across two files. `LoginPage` mixes the credentials, TOTP and recovery steps in one 319-line component.
**Why it matters:** Every form fix (the `aria-invalid` work above, reveal toggles, pending labels) must be applied in four places, which is why none of them is consistent today. `SetupPage.tsx` is 263 lines, `LoginPage.tsx` 319, with the same intent.
**Concrete fix:** A `ui/Field` primitive (label, hint, error, `aria-invalid`, `aria-describedby`) plus a small `useAsyncSubmit` hook; extract `CredentialsStep`, `TotpStep`, `RecoveryStep` and `AccountForm`/`AuthenticatorStep`.
**Effort:** M

### [Low] The skeleton has different geometry from the real rows and is never announced
**Where:** `web/src/features/watchlist/WatchlistSkeleton.tsx:8-19`, `web/src/features/watchlist/WatchlistSkeleton.css:9`, `web/src/features/watchlist/WatchlistList.css:2-3`, `web/src/pages/DashboardPage.tsx:33,53`
**Smell:** The skeleton uses `80px 110px 130px minmax(120px,1fr)` while real rows use seven tracks totalling 844 px; `rows={Math.max(tickers.length, 3)}` knows nothing about a 30-ticker list. The block is `aria-hidden` with no `role="status"` anywhere.
**Why it matters:** The layout visibly jumps when data lands, and screen-reader users get a silent gap instead of "Loading watchlist". A skeleton that does not match its content is worse than a spinner.
**Concrete fix:** Reuse `--watchlist-columns` in the skeleton, keep the last known row count in a ref, and add `<p role="status" class="sr-only">Loading watchlist</p>` (or `aria-busy` + `aria-live` on the container).
**Effort:** S

### [Low] Pending buttons only fade; disabled contrast and "nothing is happening" both suffer
**Where:** `web/src/ui/Button.css:24-38`, `web/src/features/watchlist/AddTickerForm.tsx:43,65`, `web/src/features/auth/LoginPage.tsx:208`
**Smell:** `disabled` = `opacity: 0.55` on the primary accent, no spinner, no label change; `aria-busy` sits on the form, not on the control that changed.
**Why it matters:** A provider call can take two upstream attempts × 2 s. The button looks *broken*, the label still says "Add ticker"/"Sign in", and the faded accent text can drop below 4.5:1. Users double-click (blocked) and assume a hang.
**Concrete fix:** `Button` gains `loading?: boolean` → inline spinner + `Adding…`/`Signing in…`, keep `disabled`, set `aria-busy` on the button. Raise disabled opacity or use a muted surface instead of fading text.
**Effort:** S

### [Low] Weight and spacing craft is inconsistent for a hand-built system
**Where:** `web/src/features/watchlist/QuoteRow.css:44,95,112` (`650`, `550`), `web/src/ui/Button.css:10`, `web/src/ui/PageHeader.css:12`, `web/src/features/auth/AuthPanel.css:74` (`680`), versus raw px paddings in `QuoteRow.css:11`, `AddTickerForm.css:35`, `Alert.css:5`
**Smell:** `font-weight: 550/650/680` only exist on variable fonts. `--font-sans` resolves to Segoe UI/Roboto/Helvetica on most of the target machines, which snap to 500/600/700; `680` falls back to 700. Spacing mixes `--space-*` tokens with `14px/18px/10px/44px` one-offs.
**Why it matters:** The typographic hierarchy that looks deliberate in the reference bar becomes subtly inconsistent per OS/browser. Mixed raw px breaks the vertical rhythm the tokens exist to protect.
**Concrete fix:** Constrain to 400/500/600/700 and use tokens for the rest; replace raw paddings with `--space-3/4/5` (add `--space-11: 2.75rem` for the 44 px control height if needed).
**Effort:** S

### [Low] Numbers are formatted two different ways
**Where:** `web/src/lib/format.ts:29-31` (`toFixed(2)`, no grouping, no symbol) vs `:1-6,70-72` (`Intl` currency with `$` and grouping)
**Smell:** The dashboard prints `1234.56`; the chart axis prints `$1,234.56`. Same product, same data family, two conventions.
**Why it matters:** Side-by-side it reads as two different products; four-digit prices also break the column's right-aligned tabular rhythm less than a grouped value would help scanning.
**Concrete fix:** Either add grouping to `formatPrice` (`Intl.NumberFormat` with `useGrouping` and no symbol) or drop the symbol from the axis. Keep one helper and one decimal policy.
**Effort:** S

### [Low] Dead rules and a few duplicated constants
**Where:** `web/src/features/watchlist/QuoteRow.css:3-7` (default `--watchlist-columns` duplicates `WatchlistList.css:2-3`), `web/src/features/auth/AuthForms.css:191-193` (`.auth-account`, unused), `:49-51` (`[aria-invalid]`, never set), `web/src/pages/LandingPage.css:4` (`max-width: 72rem` inside a 70 rem `.app-main`), `web/src/features/chart/PriceChart.tsx:121,199` (`titleId` unused because `aria-label` overrides `<title>`), `web/src/pages/ChartPage.tsx:13-15` (leading `{' '}` before the arrow)
**Smell:** Small leftovers that each suggest a design that was replaced but not removed. The duplicated column template is the dangerous one: two magic-number lists must stay identical or the header and rows drift.
**Why it matters:** Dead CSS is cheap to keep and expensive to trust; the duplicated grid is a latent alignment bug, and the unused `titleId` suggests the a11y wiring was changed without cleanup.
**Concrete fix:** Delete `.auth-account` and the `aria-invalid` rule (or implement it), define the column template once on `.watchlist`, cap `.landing` at `--content-width`, drop `titleId` or drop `aria-label`, remove the stray space.
**Effort:** S

### [Low] Recovery codes can be copied one at a time, but not saved as a set
**Where:** `web/src/features/auth/SetupPage.tsx:219-231`, `web/src/features/auth/RecoveryCodeList.tsx:26-53`
**Smell:** Ten individual Copy buttons, no "Copy all", no download/print, no explicit "shown only once" warning, and the only gate is a checkbox.
**Why it matters:** Recovery codes are the *only* fallback if the authenticator is lost (D-19). Asking a user to click Copy ten times, or transcribe them, is the moment where people give up and lock themselves out. GitHub and 1Password both offer download/print at this step.
**Concrete fix:** Add `Download .txt` (Blob + `URL.createObjectURL`) and `Copy all`, state "These codes are shown once. Store them somewhere safe.", and keep the checkbox as the gate.
**Effort:** S

### [Low] Polling is implemented three times over
**Where:** `web/src/features/quotes/useQuotes.ts:12-31`
**Smell:** `refetchInterval` (hidden-aware) + `refetchIntervalInBackground: true` + `refetchOnWindowFocus:false` + a manual `wasHidden` ref that calls `refetch()` on visibility change. Three mechanisms for one behaviour, two of which fight each other.
**Why it matters:** The `wasHidden` logic is the part most likely to regress (initial value is `!isVisible`, so a background-loaded tab fires an extra refetch on first visible). It also duplicates what `focusManager`/`refetchIntervalInBackground:false` already do, which makes the polling story hard to reason about during review.
**Concrete fix:** `refetchInterval: 60_000, refetchIntervalInBackground: false` and let TanStack handle visibility; if an immediate refresh on return is required, keep only the visibility effect and drop the background flag.
**Effort:** S

### [Low] Landing copy is stale about the product it describes
**Where:** `web/src/pages/LandingPage.tsx:17` (`Loopback-only, one owner…`), `:61-62` (`Runs on 127.0.0.1. Quotes are cached for 60 seconds…`)
**Smell:** "one owner" contradicts D-34 (multiple accounts, separate watchlists) and the visible `Sign out`/username UI. The bind address and cache TTL are implementation facts hard-coded in marketing copy.
**Why it matters:** It is the first sentence a visitor reads about the product, and it is wrong. If `BIND_HOST` is overridden or the Docker port mapping is used, it is also inaccurate about where the app runs.
**Concrete fix:** "Local-only, private accounts, password plus authenticator." Drop the cache TTL from copy; if the address matters, derive it from `location.host` rather than a literal.
**Effort:** S

## UI quality gaps vs senior standard (visual/UX, with page references)

**Landing (`pages/LandingPage.tsx` / `.css`)**
Hierarchy is right (eyebrow → h1 → lede → CTA → three cards) and matches the calm of Linear's marketing pages. Gaps: no product visual or "what the dashboard looks like", so the page describes rather than shows; the hero occupies a 46 rem column inside a 70 rem shell, leaving a large dead right side at ≥1280 px; the three feature cards are engineering-flavoured ("provider", "Loopback-only") where the reference bar uses outcome copy; there is no quiet footer (the auth pages have one — `AuthPanel.tsx:36` — the landing page has only a footnote); and the page does not change for `setupRequired`, so the first-run state looks identical to the returning state.

**Sign-in (`features/auth/LoginPage.tsx`)**
The brand → heading → lede → fields → full-width primary → "New here?" → quiet footer sequence matches `docs/auth-ui.md` and the Linear/GitHub references. Remaining gaps versus that bar: no reveal control on either password field; no inline validation; no lockout countdown after `429` (the copy says "in N seconds" but nothing ticks); the TOTP step does not say *which* account is being verified; `Continue` is dead at 5 digits with no hint (GitHub's 2FA screen shows the expected length); the challenge expiry (`expiresInSeconds`) is never surfaced; there is no way to cancel a half-completed sign-in except "Back to sign in", which loses the password.

**Create account (`features/auth/SetupPage.tsx`)**
Better than most: QR, manual setup key with `user-select: all`, ten codes, explicit confirmation gate. Gaps: the card is a single ~1000 px vertical stack even at 34 rem wide (QR + secret + 10 codes + checkbox + code + button) — a two-column layout at ≥720 px would cut it in half; no download/copy-all for codes (see finding); no stated recovery-code count or "shown once" warning; no password rule until failure; no back/cancel link; the QR is hard-coded `bgColor="#ffffff"` which is correct for scanning but sits as a bright rectangle in the otherwise dark card (`AuthForms.css:95-101`).

**Dashboard (`pages/DashboardPage.tsx`, `features/watchlist/*`)**
The header + add card + alert + rows order is correct, numbers are right-aligned with `font-variant-numeric: tabular-nums`, and the direction is never colour-only. Gaps: the header row disappears below 900 px with no per-row labels, so the mobile layout is a pile of unlabelled values; rows dim globally on every poll (`WatchlistList.css:32-35`) which reads as a loading failure; a new ticker shows `Price unavailable` before its quote (finding); the empty state is an error-styled dashed box rather than a first-run invitation with the add field highlighted; there is no remove action (documented D-13, but the UI offers no explanation either), no sort, no search and no sticky column header for long lists; a 60-ticker list is a single unvirtualised scroll with no grouping.

**Chart (`pages/ChartPage.tsx`, `features/chart/*`)**
The accessible name, `desc`, source line and range group are contract-solid. Versus the reference bar: no hover/touch crosshair or value readout, so the chart is a picture rather than an instrument; only the first and last x labels plus min/max dots, with a fixed five-tick axis regardless of range (`PriceChart.tsx:182`); the skeleton (`clamp(260px,38vw,380px)`) and the rendered chart (300/380 px) have different heights, so the panel jumps; a 200 with zero points renders the error copy `Chart unavailable` (`ChartView.tsx:34-40`) instead of "No data for this range"; there is no "as of" timestamp, and switching ranges replaces the chart with a skeleton rather than keeping the previous line dimmed.

**Shell (`ui/SiteHeader.tsx` / `.css`)**
Sticky translucent header with brand, descriptor, username and Sign out is clean, and hiding it on auth pages matches the reference study. Gaps: the tagline is repeated on every authenticated page and pushed right, competing with the account area; below 680 px it wraps to a second line and the header doubles in height (`SiteHeader.css:101-106`); the username truncates at 14 rem with no title/tooltip and there is no account menu, so "Sign out" is the only account affordance; there is no active-page indicator (only two destinations, but the brand link and "Back" do different things).

**Cross-cutting**
Light mode is media-query only (no toggle despite "dark and light themes" in the README), and it is the theme with the contrast defect. Error copy is server-driven and consistent, with two seams: `Chart unavailable` has no period where every other string does, and "Your session expired. Sign in again." is the only message that appears without a matching field or action context. Touch targets are mostly 44 px, except the range buttons at 36 px (`RangeSelector.css:13`). There are no dialogs, so focus trapping is N/A — but that also means every modal-like pattern (none yet) has no precedent to copy.

## Hardcoded / portability list

| Value | Where | Why it hurts |
| :-- | :-- | :-- |
| `process.env.API_PORT ?? '3000'` | `web/vite.config.ts:24` | Dev-only, but the port is duplicated from the server config with no shared source. |
| `/api/watchlist`, `/api/quotes`, `/api/chart/…`, `/api/auth/…` | `web/src/api/client.ts:258-347` | No base-URL indirection; a reverse proxy under a path prefix requires edits in eight places. |
| `NETWORK_ERROR_MESSAGE` / `GENERIC_ERROR_MESSAGE` | `web/src/api/client.ts:19-20` | Duplicates the contract copy for `PROVIDER_UNAVAILABLE` / `INTERNAL_ERROR`; the server also sends these strings. |
| `QUOTE_POLL_INTERVAL_MS = 60_000` | `web/src/features/quotes/useQuotes.ts:10` | Must stay in sync with the server's 60 s quote cache; no shared constant or API hint. |
| `MIN_PASSWORD_LENGTH = 12`, `USERNAME_PATTERN`, `RECOVERY_CODE_PATTERN` | `web/src/features/auth/validation.ts:10-12` | Mirrors Go rules by hand; a server-side change silently desynchronises client validation. |
| `PROVIDERS`, `DIRECTIONS`, `PRICE_RANGES` | `web/src/api/types.ts:1-8` | Hand-maintained wire enums; no schema/codegen link to `docs/contracts.md`. |
| `en-US`, `USD`, `'$'` | `web/src/lib/format.ts:1-12,70-72` | Locale and currency fixed; formatting cannot be localised without touching the module. |
| `MONTH_ABBREVIATIONS` | `web/src/lib/format.ts:14-27` | English-only date labels next to an `Intl`-based chart date formatter. |
| `127.0.0.1`, "one owner", "cached for 60 seconds" | `web/src/pages/LandingPage.tsx:17,61-62` | User-visible facts hard-coded in copy; one is now false. |
| `#16a34a` / `#dc2626` / `#6b7280` | `web/src/features/watchlist/QuoteRow.css:79-92` | Duplicates `--color-up/down/neutral`; light-mode contrast failure. |
| `#ffffff` QR frame | `web/src/features/auth/SetupPage.tsx:205-207` and `AuthForms.css:99` | Justified for scanability, but duplicated in two layers. |
| `#08090b` / `#f2f5f9` | `web/index.html:5-6` | Duplicates `--color-canvas` in both themes; drifts silently. |
| Chart geometry `720/280/1200/380/300`, `48` | `web/src/features/chart/PriceChart.tsx:26-30,107-115` | Magic numbers for insets/heights/anchors; no tokens, no shared scale with the CSS skeleton. |
| `--watchlist-columns` magic px (710 px of tracks) | `web/src/features/watchlist/WatchlistList.css:2-3` | Duplicated in `QuoteRow.css:3-7`; drives the 900 px overflow bug. |
| Breakpoints `380/400/520/600/680/760/900` | seven CSS files | No breakpoint tokens; the 900 px one is wrong for its own grid. |
| `min-height: 36px` range buttons | `web/src/features/chart/RangeSelector.css:13` | Below the 44 px touch-target convention used everywhere else. |
| Raw paddings `14px 18px`, `10px 14px`, `26px` | `QuoteRow.css:11`, `Alert.css:5`, `ChartView.css:8` | Bypasses the `--space-*` scale. |
| `10` recovery codes | `web/src/api/client.ts:173` | Contract constant duplicated as a length check. |
| `@types/node ^26` (web) vs `^24` (e2e) | `web/package.json:31`, `e2e/package.json` | Two Node type generations in one repo. |

## Test gaps

- **No automated a11y assertions.** `eslint-plugin-jsx-a11y` catches static mistakes but cannot see the missing `aria-invalid`, the unassociated error alert, the non-table watchlist or the absent focus move. There is no `jest-axe`/`axe-core` anywhere in `web/` or `e2e/` (grep for `axe` returns nothing).
- **Focus after SPA navigation is untested.** The only focus assertions are the login/setup heading cases (`LoginPage.test.tsx:136,182-183`, `SetupPage.test.tsx:148,233`). Nothing asserts focus after "View chart" or after session expiry.
- **Deep-link restore is untested.** `LoginPage.tsx:25-31` and `AuthGate.tsx:29-34` implement `from`, but no unit or e2e test signs in from `/chart/AAPL` and asserts the return. The e2e suite stops at "shows Sign in".
- **Light mode and contrast are invisible to CI.** `QuoteRow.test.tsx:43-46` asserts literal hex via `getComputedStyle`, which pins the wrong values rather than testing WCAG. jsdom never applies the `prefers-color-scheme: light` block, so the 3.3:1 failure cannot be caught.
- **Responsive behaviour is untested.** jsdom has no layout and `ResizeObserver` is not stubbed, so `PriceChart` always stays at the 720 px default; Playwright runs with no viewport projects (`e2e/playwright.config.ts`), so the 900 px overflow and the 36 px range buttons are never exercised.
- **Error-path coverage is thin where it matters.** No test for `requestError` precedence in `DashboardPage.tsx:30` (both watchlist and quotes failing), no test for `AuthProvider` bootstrap failure being shown as signed-out, and no test that cache is cleared on 401 beyond the visible alert.
- **Repeated-error announcement is untested** (because it does not work): submitting the same wrong code twice has no assertion on live-region behaviour.
- **Chart edge states are partial.** `PriceChart.test.tsx` covers 30 points and 1 point, but not 0 points, and no test covers `ChartView` rendering "Chart unavailable" for a successful empty series.
- **Test design is shaping production CSS.** The `getComputedStyle` hex assertions are the stated reason for the literal colours in `QuoteRow.css:77-78`. Moving colour assertions to class + a contrast test would remove both the coupling and the bug.
- **Contract/doc drift is untested by construction.** `docs/contracts.md:236` still describes `Sign-up is closed on this installation.` while `AppAuth.test.tsx:98,126,141` asserts that string is *absent* (D-34 supersedes it). Pick one and update the doc.

## Top 5 fixes if only five are done

1. **Move focus on every route change** (`App.tsx`): focus `#main-content` or the page `h1` on `pathname` change, skip the first render. One effect, removes the largest keyboard/SR regression.
2. **Route first-run users to `/setup`** (`LandingRoute.tsx`, `AuthGate.tsx`): redirect on `phase === 'setupRequired'` so nobody meets a sign-in form that cannot work. Matches D-29 and the e2e first-run intent.
3. **Wire form errors to fields and give TOTP real UX** (`LoginPage.tsx`, `SetupPage.tsx`, `AuthForms.css`): `aria-invalid` + `aria-describedby` + focus first error, password hint/reveal, six-digit paste/auto-submit, and surface `expiresInSeconds`.
4. **Fix light-mode contrast by deleting the literal colours** (`QuoteRow.css` → `tokens.css`): use `--color-up/down/neutral` with light-mode overrides; move the test off `getComputedStyle` hex.
5. **Make failure recoverable** (`main.tsx`, `App.tsx`): retry network/5xx twice with backoff, `refetchOnReconnect: true`, and an error boundary with the contract copy — so one blip is a retry, not a minute of error, and one render bug is a panel, not a blank page.

## Appendix — one-liners

- `features/auth/index.ts` is the only barrel in the project and nothing imports it (grep: no matches); inconsistent with the direct-path convention used everywhere else.
- `client.ts:160` silently classifies `authenticated: true` without `user` as "unauthenticated"; it arguably deserves the generic error instead of a silent downgrade.
- `ApiErrorPayload.code` is `string` (`types.ts:41-44`) while the contract enumerates codes; a `ErrorCode` union would make `CHALLENGE_EXPIRED`/`ENROLLMENT_EXPIRED` comparisons typo-proof.
- `formatTimestamp` hand-rolls English months (`format.ts:14-27,53-61`) next to the `Intl` chart formatter; the comment explains the format but not why `Intl` was avoided.
- `LoginPage.tsx:25` casts router state with `as { from?: unknown } | null`; a type guard would keep `verbatimModuleSyntax`-era strictness honest.
- `ChartView.tsx:23` computes a message even when `series` exists, and `:34-40` treats an empty series as an error; a distinct "No data" state would be more honest.
- `AddTickerForm` leaves empty-input validation entirely to the API (`:104-115` in its test) — contract-compatible, but it costs a 2 s provider round trip to say "Enter a valid ticker".
- No `<noscript>` in `index.html:10-14`; with JS disabled the page is blank with only the tab title.
- `AuthSplash.tsx:5` uses `role="status"` but no live text ("Stock dashboard" is the only content), so the loading state is announced as the product name.
- `e2e/playwright.config.ts` has no `projects` (single desktop Chromium), no `reducedMotion` and no mobile viewport; the UI behaviours most likely to regress are the ones it cannot see.
