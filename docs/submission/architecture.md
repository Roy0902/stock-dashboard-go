# Architecture and demonstration diagrams

Companion to [`../architecture.md`](../architecture.md). These are the diagrams a reviewer can
follow without reading the code.

Key decisions visible in the diagrams:

- **PBKDF2-HMAC-SHA256 (600,000 iterations) for new passwords**, chosen over Argon2id after
  weighing memory cost and denial-of-service exposure; legacy Argon2 hashes verify and rehash.
- **Mandatory TOTP and single-use recovery codes**, with each account owning a separate watchlist.
- **SQLite is the only persisted state**; sessions and provider caches stay in memory.
- **The client never recomputes derived quote values**; change, percent, direction and staleness
  come from the API.
- **The container is distroless and non-root**, listens on `0.0.0.0` internally and is published
  only on host `127.0.0.1`.

## Components

```mermaid
flowchart TB
  subgraph Browser
    SPA[React 19 SPA<br/>Vite + TanStack Query + Router]
  end
  subgraph Server[Go binary]
    API[api<br/>routing, middleware, status mapping]
    DOM[domain<br/>ticker, watchlist, quote, series]
    AUTH[auth<br/>PBKDF2, TOTP, sessions, rate limits]
    PROV[providers<br/>Finnhub then Twelve Data]
    STORE[store<br/>SQLite, migrations]
    UI[webui<br/>embedded SPA assets]
  end
  FIN[(Finnhub)]
  TD[(Twelve Data)]
  DB[(stock-dashboard.db)]

  SPA -->|/api/* JSON + HttpOnly cookie| API
  API --> DOM
  API --> AUTH
  API --> PROV
  API --> STORE
  API --> UI
  PROV --> FIN
  PROV --> TD
  STORE --> DB
```

Dependency direction is one-way: `api → {auth, domain, providers, store}`;
`auth → domain` and defines the `UserStore` port; `store → {auth, domain}`;
`providers → domain`; `domain → standard library only`.

## Quote request, cache and fallback

```mermaid
sequenceDiagram
  autonumber
  participant B as Browser
  participant A as Go API
  participant P as providers.Service
  participant F as Finnhub
  participant T as Twelve Data

  B->>A: GET /api/quotes (session cookie)
  A->>A: authenticate session (no store call on the hot path)
  loop each watchlist ticker
    A->>P: Quote(ticker)
    alt fresh cached observation (< 60 s)
      P-->>A: cached observation
    else cache miss
      P->>P: local budget check (55/min, 7/min)
      P->>F: GET /quote
      alt Finnhub succeeds
        F-->>P: price, previous close, timestamp
      else Finnhub fails, times out, or says unknown
        P->>T: GET /quote
        alt Twelve Data succeeds
          T-->>P: price, previous close, timestamp
        else both fail
          P-->>A: last known observation if fresh, otherwise no quote
        end
      end
    end
  end
  A-->>B: 200 { quotes[ ] } or 429 when nothing could be served
```

Derived change, percent, direction and staleness are computed once in `domain.DeriveQuote`. The
client renders those values; it never recomputes them.

## Setup and sign-in

```mermaid
sequenceDiagram
  autonumber
  participant U as User
  participant S as React SPA
  participant A as Go API
  participant T as Authenticator

  U->>S: open /setup (first run redirects here)
  S->>A: POST /api/auth/setup {username, password}
  A->>A: validate, hash password, create pending enrollment
  A-->>S: secret, otpauth URL, 10 recovery codes
  U->>T: scan QR
  T-->>U: 6-digit code
  U->>S: enter code, confirm recovery codes saved
  S->>A: POST /api/auth/2fa/activate
  A->>A: verify TOTP window and unused step
  A-->>S: session cookie (HttpOnly, SameSite=Strict)

  U->>S: later, sign in
  S->>A: POST /api/auth/login
  A-->>S: challenge token (5 min)
  U->>S: enter TOTP (auto-submits at 6 digits)
  S->>A: POST /api/auth/2fa/verify
  A-->>S: session cookie
```

Unsafe API calls require a same-origin `Origin`; every `/api` route is guarded by a public
allowlist, so a newly added route cannot ship unauthenticated by accident.

## Deployment

```mermaid
flowchart LR
  R[Reviewer browser] -->|127.0.0.1:3000| HOST[Host loopback]
  HOST --> C[Container<br/>distroless, non-root 65532]
  C --> BIN[dashboard static binary]
  C --> VOL[(named volume /data)]
  BIN --> ENV[Runtime env<br/>API keys, ALLOWED_HOSTS]
```

The image is multi-stage: Node builds the SPA, Go builds one static binary with the SPA embedded,
and the distroless runtime contains only the binary, the healthcheck and an empty `/data` volume.
The container listens on `0.0.0.0` internally and is published only on host `127.0.0.1`.

## Persistence

```mermaid
erDiagram
  USERS ||--o{ RECOVERY_CODES : has
  USERS ||--o{ WATCHLIST : owns
  USERS {
    text id PK
    text username UK
    text password_hash
    text totp_secret
    int totp_enabled
    int last_totp_step
  }
  RECOVERY_CODES {
    text user_id FK
    text code_hash
  }
  WATCHLIST {
    text user_id FK
    text ticker
    int position
  }
```

Only `stock-dashboard.db` persists. Sessions and quote/chart caches are in memory, so a restart
requires signing in again.
