# Authentication UI — commercial reference study

Date: 2026-11-18. Reference pages inspected with a headless Chromium session:
Vercel `/login`, Linear `/login`, GitHub `/login`, Stripe Dashboard `/login`,
Google Accounts, Notion `/login`.

Screenshots are local-only under `.dev/reference/` and are not part of the product.

## Patterns observed

| Pattern | Vercel | Linear | GitHub | Stripe | Google | Notion |
| :-- | :-- | :-- | :-- | :-- | :-- | :-- |
| Lone brand mark, no marketing nav | left | centered | centered | left | inside card | centered |
| Centered single-column form | yes | yes | yes | yes | card | yes |
| Card or plain background | plain | plain | plain | card + gradient | card | plain |
| Heading names the product | yes | yes | yes | yes | yes | yes |
| Sentence-case field labels | — | — | yes | yes | floating | yes |
| Full-width primary button | yes | yes | yes | yes | no | yes |
| Secondary sign-in methods | 5 | 4 | 3 | 3 | — | 6 |
| Account prompt under the form | yes | yes | yes | yes | left | yes |
| Quiet legal/security footer | yes | — | yes | yes | yes | yes |
| Two-step flow | email → method | email → method | password → 2FA | email → password | email → password | email → method |

## What this app adopted

- **No site header on auth pages.** A lone centered brand mark sits above the card,
  matching Linear, GitHub and Notion. The marketing header and tagline stay on the landing
  page.
- **Narrow single-column card** (26 rem, 34 rem for authenticator setup) with generous
  whitespace and a very subtle accent glow.
- **Product heading + one-line lede**, then labels and a full-width primary action.
- **Sentence-case labels**, larger heading, and normal input heights; no all-caps
  micro-labels.
- **Account prompt below the form**: `New here?` followed by **Set up your account**, always
  visible on the sign-in page. This mirrors "New to X? Create account" and keeps the landing
  page to a single **Sign in** action. The link always opens the Create account flow.
- **2FA is a second step** with its own heading, lede, recovery switch and a "Back to sign
  in" control. It uses the same pattern as password → authenticator flows.
- **Quiet footer** under the card: `Local-only · Password + authenticator`.

## Deliberate differences

- No SSO, passkey, Google/Apple or SAML buttons: the product is a local tool with its own
  password + TOTP accounts. Extra providers would be non-functional.
- No "Forgot password?" link: there is no email delivery. Recovery codes cover a lost
  authenticator, and the recovery step is reachable from the 2FA screen.
- No Terms/Privacy footer: no hosted service, no telemetry, no legal pages. The local-only
  note replaces it.
- No invitation gate: registration is open to anyone who can reach the dashboard, matching
  the product owner's request. The host loopback and Docker port mapping are the boundary.

## Result

The login screen now uses the same information hierarchy as the commercial references:
brand → heading → form → primary action → account prompt → quiet footer, with no
marketing chrome. Setup keeps a wider card for the QR code and recovery code list.
