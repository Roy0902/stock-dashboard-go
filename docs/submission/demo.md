# Five-minute demo script

A reviewer can follow this without reading code. Docker path is fastest; the local path is in the
[README](../README.md).

## Before you start

- Docker Desktop or Docker Engine with Compose v2.
- Optional but recommended: free keys from <https://finnhub.io/register> and
  <https://twelvedata.com/register>.
- An authenticator app for the TOTP step.
- Port `3000` free.

Without keys the app still starts and shows the missing-key message, so the UI can be reviewed
before signing up for anything.

## 1. Start from a clean checkout (1 min)

```bash
git clone https://github.com/Roy0902/stock-dashboard-go.git
cd stock-dashboard-go
cp .env.example .env
# edit .env: FINNHUB_API_KEY=... and TWELVEDATA_API_KEY=...
docker compose up -d --build
docker inspect --format '{{.State.Health.Status}}' stock-dashboard
```

Expected: `healthy`. Open <http://127.0.0.1:3000>; first run redirects to `/setup`.

To reset between demos: `docker compose down -v` (removes the SQLite volume).

## 2. Create the first account (1 min)

1. Enter a username and a password of at least 12 characters.
2. Scan the QR with the authenticator app, or use the shown setup key.
3. Save the ten recovery codes; tick the confirmation box.
4. Enter the six-digit code. It submits automatically.

Expected: you land on the dashboard with the empty-state message
`Your watchlist is empty. Add a ticker to begin.`

Talking point: passwords are PBKDF2, the session is an `HttpOnly` cookie, and the TOTP step is
replay-protected.

## 3. Add tickers and read a quote (1 min)

Add `AAPL`, then `MSFT` (lowercase also works and is canonicalised).

Expected per row: ticker, price, change with sign, percent, direction word (`up`/`down`/`unchanged`),
provider (`Finnhub`), timestamp and `View chart`. A newly added row shows a loading placeholder
until its first quote arrives, not the terminal `Price unavailable`.

## 4. Show provider fallback (1 min)

```bash
docker compose down
# remove FINNHUB_API_KEY from .env, keep TWELVEDATA_API_KEY
docker compose up -d
```

Reload the dashboard. Expected: provider becomes `Twelve Data`; prices still render. Remove both
keys and reload to see the missing-key alert while the server stays healthy at `/healthz`.

Talking point: the fallback is per ticker, budgets are local (55/min and 7/min), and a failed
ticker never fails the whole batch.

## 5. Chart and accessibility (1 min)

Open `View chart` on `AAPL`; switch between `1M`, `3M`, `1Y`.

Expected: an SVG line chart, `Source: Twelve Data`, a `Back` link, and range buttons with
`aria-pressed` on the selected range. Inspect the image role: its accessible name is
`AAPL price chart, 1M range` and its description states the range and price bounds. An invalid
range in the URL shows the `1M` chart rather than a dead end.

## 6. Verify the claims (1 min)

```bash
# Local gates (Go + web)
powershell -ExecutionPolicy Bypass -File scripts/verify.ps1

# Browser acceptance
cd e2e && npm test
```

Expected: `All gates passed.` and `33 passed`. The Go suite is 8 packages; the web suite is 131
tests across 19 files.

Also useful:

```bash
docker exec stock-dashboard /app/healthcheck      # exit 0
curl -i http://127.0.0.1:3000/api/watchlist        # 401 without a session
docker image inspect stock-dashboard:local --format '{{.Size}} {{.Config.User}}'
```

## What to ask me about

- How the frozen plan and acceptance tests were produced before the code.
- Why authorisation is a public allowlist rather than a protected-path list.
- How the auth mutex, Argon2/PBKDF2 work factor and rate limits were hardened after review.
- Which review findings were rejected or deferred, and why.
- What I would change before running this outside a single-user machine.
