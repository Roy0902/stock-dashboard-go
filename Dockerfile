# syntax=docker/dockerfile:1

ARG NODE_VERSION=24
ARG GO_VERSION=1.27
ARG ALPINE_VERSION=3.23

# ---------- client ----------
FROM node:${NODE_VERSION}-alpine AS web

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY web/ ./
# Vite writes to ../server/internal/webui/dist; make the target directory first.
RUN mkdir -p /src/server/internal/webui/dist && npm run build

# ---------- server ----------
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
COPY --from=web /src/server/internal/webui/dist ./internal/webui/dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dashboard ./cmd/dashboard \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck \
 && mkdir -p /out/data \
 && chown -R 65532:65532 /out/data

# ---------- runtime ----------
FROM alpine:${ALPINE_VERSION}

LABEL org.opencontainers.image.title="stock-dashboard" \
      org.opencontainers.image.description="Local stock watchlist with TOTP accounts and SQLite storage" \
      org.opencontainers.image.licenses="UNLICENSED"

# CA bundle for HTTPS provider calls; a real non-root identity for exec/debug.
RUN apk add --no-cache ca-certificates \
 && addgroup -g 65532 -S nonroot \
 && adduser -u 65532 -S -D -H -G nonroot nonroot

WORKDIR /app
COPY --from=build /out/dashboard /app/dashboard
COPY --from=build /out/healthcheck /app/healthcheck
COPY --from=build --chown=65532:65532 /out/data /data

ENV PORT=3000 \
    DATA_DIR=/data \
    BIND_HOST=0.0.0.0

EXPOSE 3000
VOLUME ["/data"]

USER 65532:65532

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/healthcheck"]

ENTRYPOINT ["/app/dashboard"]
