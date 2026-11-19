# AI-assisted workflow

This is the short account requested by the submission brief: tools, key steps, approximate time,
and how the result was checked. It is deliberately specific about what the AI did and where a
human decision was required. The project was delivered in **two main AI sessions** with **25
delegated agent runs**, not one-shot generation.

## Tools

| Tool | Role |
| :-- | :-- |
| OpenCode with DeepSeek V4.1 Flash | two main sessions: build, then hardening and submission |
| OpenCode subagents | **25 runs**: 16 during the build, 9 during the review/hardening pass |
| Go 1.27 toolchain | `gofmt`, `go vet`, `go test`, `go build` |
| Node 24 / npm | Vite build, Vitest, ESLint, Prettier |
| Playwright | browser journeys against a real server and fake providers |
| Docker Desktop | multi-stage Linux image and live container smoke test |
| SQLite (`modernc.org/sqlite`) | sole persisted state |
| Archify | function-flow visualisation requested during the build |

No agent had network access to the real providers during tests. A fake provider process served
Finnhub and Twelve Data shapes deterministically, so expected values never came from live data.

## Time

| Measure | Value |
| :-- | :-- |
| Calendar span, first request to last agent run | 2026-11-16 11:22 UTC → 2026-11-19 03:06 UTC |
| Elapsed | **63.7 hours (2.7 days)** |
| Go/React build session | 2026-11-18 01:26 → 2026-11-19 03:06 UTC (**25.7 hours**) |
| Delegated agent runs | **25** (16 build + 9 hardening) |
| Total sessions recorded | **27** (2 main + 25 children) |
| Recorded token use | **≈891M** input + cache + output, plus reasoning reported separately |

The 63.7-hour figure includes idle time between sessions and the earlier specification work. The
Go/React implementation and its hardening occupy the final ~26 hours. All figures come from
OpenCode's local `session_v2` records and are reproducible from that database.

## Method

1. **Frozen expectations first.** A specification, domain model, scenarios and checklist were
   frozen before implementation. Acceptance tests were written red before the code existed.
2. **Baseline the repository.** Recorded `gofmt`, `vet`, Go tests, web typecheck/lint/tests and the
   Playwright suite before changing anything. This made regressions attributable.
3. **Build in delegated slices.** The build session split Go backend, React frontend, auth backend,
   auth UI and multi-user backend into separate agents, then ran a **two-level review**: code smell
   first, method/edge-case verification second.
4. **Extend, do not rewrite.** The authentication and multi-account work extended the frozen plan
   append-only, with new scenarios, contracts and decisions rather than replacing earlier ones.
5. **Harden with independent reviews.** Three read-only reviews ran in parallel: Go code smells,
   React/UX quality, and security. They wrote reports only; no source changes.
6. **Implement in disjoint scopes.** Backend hardening, auth/shell/accessibility and
   dashboard/chart/design ran as separate agents with non-overlapping file ownership. One docs
   agent handled portable scripts, quickstart and quality gates.
7. **Final adversarial review.** A fourth reviewer checked whether the claimed fixes were real,
   whether tests had been weakened, and whether the implementation introduced new smells. It found
   two real defects introduced by the fixes: a busy password-verification semaphore was charged as
   a wrong password, and a lock-order comment was false. Both were fixed with regression tests.
8. **Mechanical verification.** Every layer was re-run independently of the agents' reports:
   `scripts/verify.ps1`, the Playwright suite, a Docker build and a live container smoke test.

## Where the AI needed direction

Product and engineering decisions were made by the human, not inferred from code:

- **Multi-account authentication** with open registration and per-account watchlists, rather than a
  single owner or a shared watchlist.
- **Always-visible sign-in fields and sign-up option**, with no system-status copy such as
  "no account exists".
- **Password policy** of a 12-character minimum and no user-facing maximum.
- **Password hashing**: Argon2id was replaced with **PBKDF2-HMAC-SHA256** (600,000 iterations) after
  a discussion of memory cost and denial-of-service exposure; legacy Argon2 hashes still verify and
  rehash on login.
- **SQLite** as the only persisted state, replacing JSON.
- **Timestamp display** as `dd-MMM-yyyy` in UTC.
- **Docker**: Alpine builder, distroless runtime, non-root, loopback-only publication.
- **Commercial auth-page conventions** studied from Vercel, Linear, GitHub, Stripe, Google and Notion.

Review findings were triaged: some accepted, some deferred with a recorded reason, and some wrong.
Agent claims were verified. One agent reported a green web suite while a test was failing; the
failure was reproduced and fixed. Two agents edited the same report concurrently and one update was
lost; the lost content was restored by hand.

## How the result was checked

| Check | Evidence |
| :-- | :-- |
| Formatting, vet, build | `gofmt -l .` empty; `go vet ./...`; `go build ./...` |
| Go tests | `go test ./... -count=1` — 8 packages pass |
| Web unit/component tests | 131 pass across 19 files |
| Browser acceptance | 33 Playwright journeys, one worker |
| Contract and security | route-table deny test, concurrency tests, secret-leak tests, body-limit tests |
| Container | image builds; health check `healthy`; `/healthz` 200; `/api/watchlist` 401; non-root |
| Review dispositions | `docs/verification.md` records every accepted/deferred finding |

## Token use (recorded)

| Scope | Sessions | Fresh input | Cache-read input | Cache-write | Output |
| :-- | --: | --: | --: | --: | --: |
| Build session + 16 subagents | 17 | 5,334,866 | 663,648,768 | 0 | 814,053 |
| Hardening session + 9 subagents | 10 | 5,532,543 | 211,227,129 | 3,848,455 | 572,214 |
| **Total** | **27** | **10,867,409** | **874,875,897** | **3,848,455** | **1,386,267** |

Reasoning tokens were reported separately (1,398,759 total). Cache reads dominate because the
repository and the frozen plan were repeatedly re-read under a long context; they are not
equivalent to fresh generation cost. Recorded cost was `$0.00` under the provider's plan accounting.

## What I would do next

- Run `go test -race ./...` on a cgo-enabled toolchain; it is the highest-value missing gate.
- Decide whether TOTP seeds should be encrypted at rest, which needs a key-management choice.
- Add the UI removal control and route-level code splitting if the product scope grows.
