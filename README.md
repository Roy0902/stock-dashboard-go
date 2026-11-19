# Stock Dashboard (Go + React)

> Reviewing this as a work submission? Start with [`SUBMISSION.md`](SUBMISSION.md), then the
> [five-minute demo](docs/submission/demo.md) and the [AI workflow](docs/submission/ai-workflow.md).

A local stock watchlist with per-account watchlists. Go serves the API and persists state in
SQLite; React renders the dashboard and the per-ticker daily chart. Quotes come from Finnhub
with Twelve Data as fallback; charts come from Twelve Data.

This is the Go + React implementation of the requirements frozen in
[`spec/frozen-plan/`](spec/frozen-plan/) and
[`spec/implementation-spec.md`](spec/implementation-spec.md).

## Architecture

1. A React 19 SPA (Vite, TanStack Query, React Router) renders every page; there is no
   server-rendered HTML and no Node runtime in production.
2. One static Go binary serves `/api/*`, `/healthz` and the built client embedded from
   `server/internal/webui/dist`.
3. `server/internal/api` owns HTTP shapes; `internal/domain` owns the pure rules;
   `internal/auth` owns passwords, TOTP and sessions; `internal/store` is the only SQLite
   writer; `internal/providers` is the only caller of the upstream APIs.
4. Quotes use Finnhub first and Twelve Data as fallback, charts use Twelve Data, both behind
   a 60 s cache and per-provider request budgets.
5. `DATA_DIR/stock-dashboard.db` is the only persisted state; sessions live in memory, so a
   restart asks everyone to sign in again.

| Layer | Choice |
| :-- | :-- |
| API | Go 1.24+, standard library HTTP router, SQLite |
| Client | React 19, TypeScript, Vite |
| Data fetching | TanStack Query |
| Routing | React Router |
| Styling | Token-based CSS, dark and light themes |
| Chart | Hand-built accessible SVG |
| Tests | `go test`, Vitest + React Testing Library, Playwright |

## Prerequisites

| Tool | Requirement | Notes |
| :-- | :-- | :-- |
| Go | 1.24.0 or newer | `server/go.mod` declares `go 1.24.0`; `gofmt` and `go vet` ship with the toolchain |
| Node.js | 20 or newer | `engines` in `web/package.json`; install the bundled npm |
| PowerShell | Windows PowerShell 5.1 or PowerShell 7 | runs `scripts/*.ps1` on Windows |
| Docker | Docker Desktop or Engine with Compose v2 | optional, container path only |
| Playwright Chromium | optional | e2e only, installed once with `npm run install:browsers` in `e2e/` |
| Provider keys | optional to start, required for quotes and charts | `FINNHUB_API_KEY`, `TWELVEDATA_API_KEY` |

Ports `3000` (API and production build) and `5173` (Vite dev server) must be free. The scripts
resolve the repository root from their own location (`%~dp0` in `.cmd`, `$PSScriptRoot` in
PowerShell), so they work from any current directory.

Verified baseline on this machine (2026-11-18):

| Tool | Version |
| :-- | :-- |
| Go | 1.27.4 |
| Node.js | 24.22.0 |
| npm | 11.19.1 |
| Playwright Chromium | installed in the user browser cache |

Client and test dependencies are installed from the lock files (`npm ci`). The production
client bundle and server binary are built into `server/internal/webui/dist` and
`server/bin`.

## First run

```powershell
Copy-Item .env.example .env
notepad .env            # add FINNHUB_API_KEY and TWELVEDATA_API_KEY
powershell -ExecutionPolicy Bypass -File scripts/start.ps1
# open http://127.0.0.1:3000
```

`start.ps1` loads `.env`, checks the port, rebuilds the client and server, and runs the
production binary in the foreground. Pass `-Port 3001` to change the port or `-SkipBuild`
for a fast restart. Without keys the app still starts and shows the missing-key message.
On first visit, open `/setup` to create the first account (see [Authentication](#authentication)).

### Background service

```powershell
.\start.cmd        # builds, starts detached, prints the URL
.\status.cmd       # checks the API and dev UI
.\stop.cmd         # stops both
```

Detached runs are tracked in `.dev` (`server.pid`, `server.log`). `scripts/start.ps1
-Detached` is idempotent: it reports an already-running server instead of failing, and
`-Restart` replaces it (`.\start.cmd -Restart`).

### Development loop

```powershell
powershell -ExecutionPolicy Bypass -File scripts/dev.ps1
# app: http://127.0.0.1:5173   api: http://127.0.0.1:3000
```

The dev script builds the API, waits for `/healthz`, starts Vite with the API proxy, and
stops the API when Vite exits. To attach the dev client to an already-running API, use
`scripts/start-ui.ps1` (detached). API logs go to `.dev/server.log`.

### Manual commands

```powershell
cd web; npm ci; npm run build
cd ../server; go build -o bin/dashboard.exe ./cmd/dashboard
$env:FINNHUB_API_KEY = "..."; $env:TWELVEDATA_API_KEY = "..."
./bin/dashboard.exe
```

Running the binary directly uses the process environment, not `.env`; set `DATA_DIR` to keep
the database beside the repository (`./data` is relative to the current directory otherwise).

### Adopting your own module path

The module is `stock-dashboard/server`: neutral, so it builds on any machine without a personal
account in the path. It is not a publishable VCS path. If you fork, mirror or publish this repo,
change it once:

```powershell
node scripts/set-module.mjs github.com/acme/stock-dashboard/server --dry-run
node scripts/set-module.mjs github.com/acme/stock-dashboard/server
cd server; gofmt -l .; go build ./...; go test ./...
```

The script resolves the repository from its own location, so it works from any directory. It
rewrites `server/go.mod`, every Go import, and docs or scripts that name the module path. It
refuses an empty or invalid path and never touches `node_modules`, `dist`, `.dev`, `data` or
the historical reports under `docs/reviews/`.

## Configuration

The Go server reads only these variables. `scripts/*.ps1` load `.env` from the repository
root before starting it, and a real environment variable wins over the same name in `.env`.

| Variable | Default | Purpose |
| :-- | :-- | :-- |
| `FINNHUB_API_KEY` | empty | primary quote provider; required for quotes and charts |
| `TWELVEDATA_API_KEY` | empty | fallback quotes and the only chart source; required with the above |
| `PORT` | `3000` | listen port, 1–65535 |
| `BIND_HOST` | `127.0.0.1` | listen IP; must be an IP address; the image sets `0.0.0.0` |
| `DATA_DIR` | `./data` | directory holding `stock-dashboard.db`; a relative value is resolved against the process working directory |
| `AUTH_COOKIE_SECURE` | `false` | `true` adds `Secure` to auth cookies; required when served over HTTPS |
| `ALLOWED_HOSTS` | `127.0.0.1,localhost,::1` | comma-separated `Host` allowlist; entries are hostnames or IPs, no scheme or port |
| `FINNHUB_RPM` | `55` | Finnhub calls per minute |
| `TWELVEDATA_RPM` | `7` | Twelve Data calls per minute |
| `UPSTREAM_TIMEOUT` | `2s` | per-request upstream timeout (Go duration, e.g. `1500ms`) |
| `QUOTE_CACHE_TTL` | `1m` | quote cache lifetime (Go duration) |
| `FINNHUB_BASE_URL` | `https://finnhub.io` | provider base URL override, for tests |
| `TWELVEDATA_BASE_URL` | `https://api.twelvedata.com` | provider base URL override, for tests |

`API_PORT` is not read by the server: it only tells the Vite dev proxy in
`web/vite.config.ts` where the API listens, and `scripts/dev.ps1` sets it for you.

The server binds `127.0.0.1` by default. Keys are read from the environment and never logged
or returned. `DATA_DIR/stock-dashboard.db` (SQLite) is the only persisted state: per-account
watchlists, accounts and hashed recovery codes. Sessions stay in memory.

## Tests and quality gates

```powershell
cd server; gofmt -l .; go vet ./...; go test ./... -count=1
cd web; npm run lint; npm run typecheck; npm run format:check; npm run test; npm run build
cd e2e; npm test
```

`scripts/verify.ps1` runs the Go and web gates in one go
(`powershell -ExecutionPolicy Bypass -File scripts/verify.ps1`); add `-WithE2E` to include
the Playwright suite. Expected results and the definition of done are in
[`docs/quality-gates.md`](docs/quality-gates.md).

`go test -race` needs cgo. This host runs with `CGO_ENABLED=0` and has no C compiler, so run
the race gate on a toolchain that has one instead of skipping it silently.

## Docker

The image is multi-stage, runs as nonroot on Alpine 3.23, and keeps SQLite on a volume.
A read-only root is supported; BusyBox and `apk` remain available for debugging. The container sets `BIND_HOST=0.0.0.0`; always publish the port on
the host loopback:

```powershell
docker build -t stock-dashboard:local .
docker run -d --name stock-dashboard `
  -p 127.0.0.1:3000:3000 `
  -v stock-dashboard-data:/data `
  --read-only --tmpfs /tmp `
  -e FINNHUB_API_KEY=... -e TWELVEDATA_API_KEY=... `
  stock-dashboard:local
```

Or use Compose (it reads the two keys from `.env`):

```powershell
docker compose up -d --build
```

The image includes `/app/healthcheck`, runs as `65532:65532`, and never contains API keys.
Do not publish `0.0.0.0:3000`; that would expose the dashboard beyond the host. Details and
the verified run record: [`docs/docker.md`](docs/docker.md).

On first start after the SQLite upgrade, an existing `watchlist.json` / `users.json` is
validated, imported into the database and renamed `<name>.json.migrated`. After that, the
server never writes JSON state.

If the database is corrupt or contains an invalid row, startup fails rather than silently
discarding your data. Back up the file while the server is stopped.

## Authentication

Anonymous visitors see the public landing page at `/` with a `Sign in` action. The sign-in
page always offers `New here? Set up your account`. The dashboard lives at `/dashboard`;
opening it or `/chart/<ticker>` without a session redirects to `/login`, and the requested
page is restored after sign-in.

The app supports multiple accounts, each with its own watchlist and mandatory TOTP.

1. Open `/setup` from `New here? Set up your account` on the sign-in page: choose a unique
   username, a password of at least 12 characters, and scan the QR code with an
   authenticator app (or enter the setup key manually). Save the ten recovery codes.
2. Enter the six-digit code to create the account and sign in. The account is stored in
   `$DATA_DIR/stock-dashboard.db` only after this step.
3. Later visits use `/login`: password first, then the authenticator code or a single-use
   recovery code. A new account starts with an empty watchlist; accounts never see each
   other's tickers.
4. Sessions last 12 hours with a 30-minute idle timeout; restarting the server requires
   signing in again. `Sign out` is in the header.

The database holds each account's password hash, TOTP secret, hashed recovery codes and
watchlist. New passwords use PBKDF2-HMAC-SHA256; legacy Argon2id hashes are upgraded on the
next successful login. Passwords, codes and session tokens are never logged. If both the authenticator
and all recovery codes are lost for one account, stop the server and use an SQLite client to
delete that row from `users`; its recovery codes and watchlist cascade. The existing
single-owner data is migrated to the first account on upgrade. Set
`AUTH_COOKIE_SECURE=true` only when serving the app over HTTPS.

## Troubleshooting

| Symptom | Cause and fix |
| :-- | :-- |
| `Port 3000 is already in use by another process (PID …)` | Something else owns the port. Run `.\stop.cmd`, or start with `-Port 3001` (and `.\status.cmd -Port 3001`). |
| `The dashboard is already running` from `start.cmd` | Expected: detached starts are idempotent. Use `.\start.cmd -Restart` to replace the running process. |
| `No server binary at …` with `-SkipBuild` | The binary was never built. Run `scripts/start.ps1` once without `-SkipBuild`. |
| The app shows the missing-key message | `FINNHUB_API_KEY` / `TWELVEDATA_API_KEY` are empty. Scripts read `.env`; a direct `bin/dashboard.exe` run reads only the process environment. |
| `403 CROSS_ORIGIN_REQUEST` in the browser | The `Host` header is not in `ALLOWED_HOSTS` (default `127.0.0.1`, `localhost`, `::1`) or an unsafe `/api` call came from another origin. Open the app via an allowed host, not a LAN IP, and keep the Vite proxy's `changeOrigin: false`. |
| Login code rejected right after account setup | TOTP replay protection consumes the activation step on purpose. Wait for the next 30-second window and enter the new code, or use a recovery code. |
| Locked out of an account | With the server stopped, delete that row from `users` with an SQLite client; recovery codes and watchlist cascade. There is no email reset. |
| Startup fails with a database error | Intentional fail-closed behaviour on a corrupt database or invalid row. Restore a backup of `stock-dashboard.db`; the server never silently discards data. |
| `go test -race` fails with `-race requires cgo` | The host has `CGO_ENABLED=0` and no C compiler. Run the race gate where cgo is available. |
| `e2e` reports `Go toolchain not found` | The build script looks for `GO_BINARY`, then `go` on `PATH`. Set `GO_BINARY` to the full path of `go.exe`. |
| PowerShell refuses to run a script | Launch it as the `.cmd` wrappers do: `powershell -ExecutionPolicy Bypass -File scripts/<name>.ps1`. |
| Container starts but the page is unreachable | Do not override `BIND_HOST` back to `127.0.0.1` inside the container, and publish only `127.0.0.1:3000:3000`. |

## Documentation

- [`docs/contracts.md`](docs/contracts.md) - API and UI contract with exact copy
- [`docs/contracts-auth.md`](docs/contracts-auth.md) - authentication API, cookies and UI flow
- [`docs/quality-gates.md`](docs/quality-gates.md) - exact commands, expected results, definition of done
- [`docs/architecture.md`](docs/architecture.md) - boundaries, decisions, gates
- [`docs/docker.md`](docs/docker.md) - container image build, run and verification
- [`docs/verification.md`](docs/verification.md) - gate results and manual checks
- [`docs/decisions.md`](docs/decisions.md) - engineering decision log
- [`docs/auth-ui.md`](docs/auth-ui.md) - commercial reference study and applied patterns
- [`docs/diagrams/login-function-flow/login-function-flow.html`](docs/diagrams/login-function-flow/login-function-flow.html) - login and password-hash function flow
- [`spec/`](spec/) - frozen plan and implementation specifications

## Acceptance suite

`e2e/` starts a fresh server binary per test with its own `DATA_DIR`, so caches and
watchlists never leak between tests. It uses a deterministic fake Finnhub/Twelve Data
server; no API keys or network access are needed.

```powershell
cd e2e
npm ci
npm run install:browsers   # once
npm test                   # builds client + binary, then runs Playwright
```

`npm test` runs the `pretest` build (`e2e/scripts/build-app.mjs`), which builds the client
with `web/node_modules` and the server into `e2e/.tmp`; install the web dependencies first.
