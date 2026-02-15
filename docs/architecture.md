# Architecture

## Current runtime topology

```text
Client
  |
  v
Gatify (Go HTTP server)
  |- /health
  |- /api/rules* (admin token protected)
  |- /proxy/* -> ReverseProxy -> Upstream backend
        |
        +-> Redis (rate limiter state)

TimescaleDB is present in local stack and used by analytics integration tests.
```

## Core components

- `cmd/gatify/main.go`
  - process bootstrap
  - env configuration
  - route registration
  - graceful shutdown

- `internal/proxy`
  - reverse proxy implementation
  - rate-limiting middleware behavior
  - `X-Forwarded-For` trust toggle

- `internal/limiter`
  - limiter interface and orchestration

- `internal/storage`
  - Redis-backed persistence for limiter state

- `internal/api`
  - rules CRUD handler and repository abstraction

## Request lifecycle (`/proxy/*`)

1. Gateway receives request.
2. Client key is derived (remote addr or `X-Forwarded-For` when trusted).
3. Limiter evaluates allowance using Redis state.
4. Response includes rate-limit headers.
5. If allowed: request is proxied upstream.
6. If blocked: `429` JSON response is returned.

## Security notes

- Rules API is disabled unless `ADMIN_API_TOKEN` is configured.
- `TRUST_PROXY=true` should be used only behind trusted ingress/reverse proxy.
