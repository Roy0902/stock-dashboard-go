# AI-assisted workflow

This is the short account requested by the submission brief: tools, key steps, approximate time,
and how the result was checked. It is deliberately specific about what the AI did and where a
human decision was required.

## Tools

| Tool | Role |
| :-- | :-- |
| OpenCode with DeepSeek V4.1 Flash | primary agent: planning, implementation, verification, orchestration |
| OpenCode subagents | nine delegated runs: three code/security reviews, one docs pass, four implementation passes, one final review |
| Go 1.27 toolchain | `gofmt`, `go vet`, `go test`, `go build` |
| Node 24 / npm | Vite build, Vitest, ESLint, Prettier |
| Playwright | 33 browser journeys against a real server and fake providers |
| Docker Desktop | multi-stage Linux image and live container smoke test |
| SQLite (`modernc.org/sqlite`) | sole persisted state |

No agent had network access to the real providers during tests. A fake provider process served
Finnhub and Twelve Data shapes deterministically, so expected values never came from live data.

## Time

| Measure | Value |
| :-- | :-- |
| Calendar span, first request to last agent run | 2026-11-16 11:22 UTC → 2026-11-19 03:05 UTC |
| Elapsed | **63.7 hours (2.7 days)** |
| Delegated agent runs | 9 |
| Main-session + subagent token use | ≈208.2M recorded (95% cache reads; see below) |

The elapsed figure includes idle time between sessions. Work was done in a handful of focused
sessions, not continuously. Token counts are from OpenCode's local `session_v2` records and are
reproducible from that database.

## Method

1. **Frozen expectations first.** A specification, domain model, scenarios and checklist were
   frozen before implementation. Acceptance tests were written red before the code existed.
2. **Baseline the repository.** Recorded `gofmt`, `vet`, Go tests, web typecheck/lint/tests and the
   Playwright suite before changing anything. This made regressions attributable.
3. **Review before implementation.** Three independent read-only reviews ran in parallel:
   Go code smells, React/UX quality, and security. They wrote reports only; no source changes.
4. **Implement in disjoint scopes.** Backend hardening, auth/shell/accessibility and
   dashboard/chart/design ran as separate agents with non-overlapping file ownership. One docs
   agent handled portable scripts, quickstart and quality gates.
5. **Final adversarial review.** A fourth reviewer checked whether the claimed fixes were real,
   whether tests had been weakened, and whether the implementation introduced new smells. It found
   two real defects introduced by the fixes: a busy password-verification semaphore was charged as
   a wrong password, and a lock-order comment was false. Both were fixed with regression tests.
6. **Mechanical verification.** Every layer was re-run independently of the agents' reports:
   `scripts/verify.ps1`, the Playwright suite, a Docker build and a live container smoke test.

## Where the AI needed direction

- **Product rules** (fallback behaviour, budget refusal, cache semantics, copy) were decided by the
  human, not inferred from code.
- **Review findings** were triaged: some were accepted, some deferred with a recorded reason, and
  some were wrong. The reports are inputs, not specifications.
- **Agent claims were verified.** One agent reported a green web suite while a test was failing; the
  failure was reproduced and fixed. Two agents edited the same report concurrently and one update
  was lost; the lost content was restored by hand.

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

| Scope | Fresh input | Cache-read input | Cache-write | Output |
| :-- | --: | --: | --: | --: |
| Main session | 3,871,967 | 120,325,881 | 3,848,455 | 319,754 |
| 9 subagents | 1,624,028 | 77,980,160 | 0 | 239,693 |
| **Total** | **5,495,995** | **198,306,041** | **3,848,455** | **559,447** |

Reasoning tokens were reported separately (495,286 total). Cache reads dominate because the
repository and the frozen plan were repeatedly re-read under a long context; they are not
equivalent to fresh generation cost.

## What I would do next

- Run `go test -race ./...` on a cgo-enabled toolchain; it is the highest-value missing gate.
- Decide whether TOTP seeds should be encrypted at rest, which needs a key-management choice.
- Add the UI removal control and route-level code splitting if the product scope grows.
