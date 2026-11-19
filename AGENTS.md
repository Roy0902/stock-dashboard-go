# Project instructions

Behaviour comes from the frozen plan in `spec/frozen-plan/` and
`spec/implementation-spec.md`. `docs/contracts.md` restates the
externally observable contract; when in doubt, the spec wins.

## Boundaries

- `server/internal/domain/` is pure Go: no `net/http`, `os`, `context` cancellation as a
  business rule, or provider imports. Rules must be testable with a fixed clock.
- `server/internal/auth/` owns password hashing, TOTP, recovery codes, sessions, challenges
  and login rate limits. It may use `domain` and defines the `UserStore` port that
  `internal/store` implements; it never imports `api` or `store`.
- `server/internal/providers/` owns upstream adapters, budgets, fallback order and the cache.
  Provider messages, URLs and keys never leave this layer.
- `server/internal/store/` owns every database write (`stock-dashboard.db`). Legacy
  `watchlist.json`/`users.json` are imported once and never written again.
- `server/internal/api/` owns HTTP shapes, status codes, cookies, headers and the only error
  mapping.
- `server/internal/webui/` only serves embedded assets.
- `web/src/lib/` is formatting only. Components never recompute change, percent, direction
  or staleness; the API is the single source of those values.
- Authentication state is never stored in browser storage: the session lives in an
  `HttpOnly` cookie and auth responses never expose tokens or secret hashes.

## Style

- Go: `gofmt`, `go vet`, small interfaces, explicit errors wrapping (`%w`), table-driven
  tests, no panics on request paths. Use `slog` with structured fields; never log query
  strings, keys or upstream bodies.
- TypeScript: `strict`, no `any`, no non-null assertions without a proven invariant,
  discriminated unions for API results, named exports, one component per file.
- CSS: tokens in `src/styles/tokens.css`; component styles in co-located CSS files using
  BEM-ish intent names. Never rely on colour alone for meaning.
- Copy: use the exact strings in `docs/contracts.md`. Do not invent user-visible copy.

## Required checks

Never weaken a lint rule, test or accessibility invariant to make a gate pass. Report
failures instead.

```powershell
cd server; gofmt -l .; go vet ./...; go test ./... -count=1
cd web; npm run lint; npm run typecheck; npm run format:check; npm run test; npm run build
```

The race detector needs cgo; this host has `CGO_ENABLED=0`, so run `go test -race` on a
toolchain with a C compiler rather than skipping it silently.

## Secrets and data

- `FINNHUB_API_KEY` and `TWELVEDATA_API_KEY` come from the environment.
- `DATA_DIR/stock-dashboard.db` (SQLite) is the only persisted state. Legacy
  `watchlist.json` and `users.json` are migration inputs, renamed `.migrated` after import.
- Bind `BIND_HOST` (`127.0.0.1` by default); the container image sets `0.0.0.0` and must be
  published on the host loopback only. No telemetry.
- Never log or return passwords, TOTP secrets, verification codes, recovery codes, session
  tokens or password hashes. Use constant-time comparisons for secrets.
- `AUTH_COOKIE_SECURE=true` is required when the app is served over HTTPS.
