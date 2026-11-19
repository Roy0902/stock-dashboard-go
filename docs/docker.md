# Docker image

## Build

```powershell
docker build -t stock-dashboard:local .
```

Multi-stage:

1. `node:24-alpine` runs `npm ci` and builds the Vite client into the Go embed directory.
2. `golang:1.27-alpine` downloads modules and builds the static server and the tiny
   `/app/healthcheck`, with `CGO_ENABLED=0` for the pure-Go SQLite driver.
3. `alpine:3.23` is the runtime: `apk add --no-cache ca-certificates`, a dedicated
   non-root user, the two static binaries and `/data`. BusyBox and `apk` stay available for
   in-container debugging; no source, tests, `node_modules`, tzdata or docs are added.

The build context is filtered by `.dockerignore`: `.env`, `data/`, `node_modules`, `e2e/`,
`.dev/`, Go/Web tests, prior build output and docs never enter the build; only the two
binaries, CA bundle and runtime files reach the final layer.

## Run

```powershell
docker run -d --name stock-dashboard `
  -p 127.0.0.1:3000:3000 `
  -v stock-dashboard-data:/data `
  --read-only --tmpfs /tmp `
  -e FINNHUB_API_KEY=... `
  -e TWELVEDATA_API_KEY=... `
  stock-dashboard:local
```

Compose equivalent:

```powershell
docker compose up -d --build
```

`docker-compose.yml` publishes `127.0.0.1:3000`, mounts `dashboard-data` at `/data`, runs
read-only with a `/tmp` tmpfs, and reads the provider keys from `.env`.

## Why `BIND_HOST=0.0.0.0` is safe here

The process default is still `127.0.0.1`. Inside a container that address is unreachable
from the host, so the image sets `BIND_HOST=0.0.0.0`. The host port mapping is restricted to
`127.0.0.1`, so the dashboard is not exposed to the LAN. Do not change the published host
address to `0.0.0.0`.

## Verified on this machine

| Check | Result |
| :-- | :-- |
| Image | `stock-dashboard:local`, 35.03 MB uncompressed, Alpine 3.23.8, Linux/amd64 |
| User | `65532:65532` (nonroot) |
| Root filesystem | read-only, `/tmp` tmpfs |
| Health | `healthy`; bundled `/app/healthcheck` exits 0 |
| Landing / health | `200` |
| Unauthenticated API | `/api/watchlist` → `401` |
| Registration | `owner` setup + TOTP activation → `201` / `200` |
| Duplicate account | `owner` again → `409 USERNAME_TAKEN` |
| Second account | `second` setup + TOTP → `200`; independent session |
| Persistence | container restart with the same volume → accounts still present; owner login (password + TOTP) succeeds |
| Database | `/data/stock-dashboard.db` copied out: 45,056 bytes after two accounts |
| Compose | `docker compose config --quiet` passes |

The image was exercised against an empty provider configuration; provider calls are covered
by the Go and Playwright suites. API keys were not baked into the image.

### Re-verification after the Go/React hardening pass (2026-11-18)

| Check | Result |
| :-- | :-- |
| Build | `docker build -t stock-dashboard:local .` exit 0 |
| Image size | 32,028,754 bytes (uncompressed), Linux/amd64 |
| User | `65532:65532` |
| Health | image healthcheck `healthy`; `docker exec /app/healthcheck` exit 0 |
| `/healthz` | `200 {"status":"ok"}` |
| `/` | `200` |
| `/api/watchlist` without a session | `401` |
| Image environment | provider keys supplied at runtime only; two test keys were passed to the smoke container, none are baked into the image |

The smoke container was published on `127.0.0.1:3210` only and removed after the check.

### Scratch runtime trial (2026-11-19)

The runtime was also tried from `scratch` with a trimmed base and a raw-socket healthcheck:

| Check | Result |
| :-- | :-- |
| Build | `docker build --no-cache --pull` exit 0 |
| Image size | 21,012,421 bytes (uncompressed), down from 32,047,906 |
| Filesystem | only `/app/dashboard`, `/app/healthcheck`, `/data`, CA bundle, `passwd`, `group`, `nsswitch.conf`; no shell, tzdata, source, tests, `node_modules` or package manager |
| Binary sizes | dashboard 12.16 MB; healthcheck 2.18 MB (was 5.89 MB with `net/http`) |
| User | `65532:65532` |
| Health | `healthy`; `/app/healthcheck` exit 0 |
| Auth flow | setup `201`, TOTP activation `200`, logout, password login `200`, TOTP `200`, status authenticated |
| `/healthz` / `/` | `200` / `200` |
| `/api/watchlist` without a session | `401` |

Superseded for deployment by the Alpine runtime below.

### Alpine runtime re-verification (2026-11-19)

The product owner selected Alpine so the runtime keeps a shell and package manager:

| Check | Result |
| :-- | :-- |
| Base | `alpine:3.23.8` |
| Build | `docker build --no-cache --pull` exit 0 |
| Image size | 35,031,247 bytes (uncompressed) |
| Runtime files | `ca-certificates`, BusyBox, `apk`, non-root `65532:65532`, two static binaries, `/data` |
| User | `65532(nonroot)` from `/etc/passwd`; read-only root and `/tmp` tmpfs |
| Health | `healthy`; `/app/healthcheck` exit 0 |
| Auth flow | setup `201`, TOTP activation `200`, logout, password login `200`, TOTP `200`, status authenticated |
| `/healthz` / `/` | `200` / `200` |
| `/api/watchlist` without a session | `401` |
