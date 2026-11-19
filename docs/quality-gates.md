# Quality gates

The gates below are the ones this project actually runs. Every command exists in the repo
(`server/` Go tooling, `web/package.json`, `e2e/package.json`, `scripts/*.ps1`, Dockerfile).
A change is mergeable only when every gate passes; never weaken a lint rule, test or
accessibility invariant to make one pass.

Baseline measured on this host (Windows, Go 1.27.4, Node 24.22.0, npm 11.19.1,
2026-11-18): Go gates pass, web `typecheck`/`lint`/`format:check`/`test`/`build` pass
(20 files, 150 tests), e2e `typecheck` passes and the Playwright suite passes (33 tests,
one worker, ~60 s). Test counts grow; the pass criterion is "everything green", not a number.

## Go (`server/`)

| Gate | Command | Expected result |
| :-- | :-- | :-- |
| Format | `gofmt -l .` | no output (prints any file that needs `gofmt -w`) |
| Build | `go build ./...` | exit code 0, no output |
| Vet | `go vet ./...` | exit code 0, no output |
| Tests | `go test ./... -count=1` | every package `ok` or `[no test files]` |
| Race | `go test -race ./...` | every package `ok`; needs cgo |
| Coverage (optional) | `go test ./... -cover` | per-package percentages printed |

```powershell
cd server
gofmt -l .
go build ./...
go vet ./...
go test ./... -count=1
go test -race ./...
```

The race detector needs a C compiler and cgo. This host runs with `CGO_ENABLED=0`, so the
command fails with `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`. Run that
gate in an environment where cgo works before release; do not silently drop it. The
container build sets `CGO_ENABLED=0` on purpose (pure-Go SQLite driver), which is unrelated
to running the race detector locally.

## Web (`web/`)

Install once from the lock file, then run the gates:

| Gate | Command | Expected result |
| :-- | :-- | :-- |
| Install | `npm ci` | `node_modules` matches `package-lock.json` |
| Types | `npm run typecheck` | `tsc --noEmit`, exit code 0 |
| Lint | `npm run lint` | `eslint .`, 0 errors and 0 warnings |
| Format check | `npm run format:check` | `prettier --check .`, "All matched files use Prettier code style!" |
| Unit tests | `npm run test` | `vitest run`, all files and tests pass |
| Build | `npm run build` | Vite build succeeds and writes `server/internal/webui/dist` |
| Format (fix) | `npm run format` | rewrites files with Prettier; then re-run `format:check` |

```powershell
cd web
npm ci
npm run typecheck
npm run lint
npm run format:check
npm run test
npm run build
```

## Acceptance (`e2e/`)

Playwright runs against a freshly built binary with an isolated `DATA_DIR` and a fake
provider server, so no API keys or network access are needed. The `pretest` script
(`e2e/scripts/build-app.mjs`) builds the client with `web/node_modules` and the server into
`e2e/.tmp`; install `web/` dependencies first.

| Gate | Command | Expected result |
| :-- | :-- | :-- |
| Install | `npm ci` | Playwright test runner installed |
| Browser | `npm run install:browsers` | Chromium downloaded once |
| Types | `npm run typecheck` | exit code 0 |
| Suite | `npm test` | `33 passed`, 1 worker, exit code 0 |
| Headed run (debug) | `npm run test:headed` | same suite with a visible browser |
| Report | `npm run report` | opens the HTML report from the last run |

```powershell
cd e2e
npm ci
npm run install:browsers
npm run typecheck
npm test
```

If the Go toolchain is not on `PATH`, set `GO_BINARY` to the `go` executable before
`npm test`.

## Docker

| Gate | Command | Expected result |
| :-- | :-- | :-- |
| Build | `docker build -t stock-dashboard:local .` | exit code 0; Alpine runtime layer built |
| Compose syntax | `docker compose config --quiet` | exit code 0 |
| Run | `docker compose up -d --build` | container starts on `127.0.0.1:3000` |
| Health | `docker inspect --format '{{.State.Health.Status}}' stock-dashboard` | `healthy` |
| Healthcheck binary | `docker exec stock-dashboard /app/healthcheck` | exit code 0 |
| Landing | `Invoke-WebRequest http://127.0.0.1:3000/healthz` | `200` with `"status":"ok"` |
| Unauthenticated API | `Invoke-WebRequest http://127.0.0.1:3000/api/watchlist` | `401` |
| Clean up | `docker compose down` | container removed, volume kept |

```powershell
docker build -t stock-dashboard:local .
docker compose config --quiet
docker compose up -d --build
docker inspect --format '{{.State.Health.Status}}' stock-dashboard
docker compose down
```

The image must not contain API keys, must run as `65532:65532` on a read-only root, and
must be published on host `127.0.0.1` only. `docs/docker.md` records the last verified run.

## One-shot runner

`scripts/verify.ps1` runs the Go and web gates in order and stops at the first failure; it
resolves the repository root from its own location, so it can be called from anywhere.

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1 -WithE2E
```

Expected output ends with `All gates passed.` There is no root `package.json`, so
`npm run verify` does not exist; use the PowerShell script.

## Definition of done

- [ ] `gofmt -l .` prints nothing and `go vet ./...` is clean in `server/`.
- [ ] `go test ./... -count=1` passes; `go test -race ./...` passes wherever cgo is available (recorded, not skipped silently).
- [ ] `npm run typecheck`, `npm run lint`, `npm run format:check`, `npm run test` and `npm run build` pass in `web/`.
- [ ] `npm test` passes in `e2e/` and `npm run typecheck` passes there too.
- [ ] Docker image builds and reports `healthy`; host port still published on `127.0.0.1` only.
- [ ] No new environment variable is read without an entry in `.env.example` and the README table.
- [ ] Behaviour changes are reflected in `docs/contracts.md` (or `docs/contracts-auth.md`) and, when a decision changed, in `docs/decisions.md`.
- [ ] No secrets, keys, passwords, TOTP secrets, recovery codes or session tokens appear in logs, responses, docs or test fixtures.
- [ ] `scripts/*.ps1`, `start.cmd`, `stop.cmd` and `status.cmd` still work from a directory other than the repository root.
- [ ] New user-visible copy comes from `docs/contracts.md`, not from the implementation.
