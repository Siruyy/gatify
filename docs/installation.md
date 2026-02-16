# Installation Guide

## Prerequisites

- Go 1.22+
- Docker + Docker Compose
- Make

Optional:

- k6 (for load tests)
- golangci-lint (installed by `make deps`)

## Local setup

1. Clone repository
2. Copy env file from `.env.example` to `.env`
3. Install dependencies with `make deps`
4. Start infra services using `make dev`
5. Apply database migrations:
   - `make migrate-up DATABASE_URL="postgres://gatify:gatify_dev_password@localhost:5432/gatify?sslmode=disable"`
6. Run gateway with `go run ./cmd/gatify`

## Verify installation

Check health endpoint:

- `GET http://localhost:3000/health`

Expected response:

```json
{"status":"ok","service":"gatify"}
```

Check proxy path:

- `GET http://localhost:3000/proxy/get`

If `test-backend` is running, this should return proxied JSON from httpbin.

## Run tests

- Unit + race + coverage: `make test`
- Lint: `make lint`
- Full checks: `make check`
- E2E: `make test-e2e` (requires services + running gateway)

## Migration helpers

- Show migration version:
  - `make migrate-version DATABASE_URL="postgres://gatify:gatify_dev_password@localhost:5432/gatify?sslmode=disable"`
- Roll back one step:
  - `make migrate-steps DATABASE_URL="postgres://gatify:gatify_dev_password@localhost:5432/gatify?sslmode=disable" STEPS=-1`

## Troubleshooting

- If Redis is unavailable, ensure `REDIS_ADDR` matches running instance.
- If proxy calls fail, verify `BACKEND_URL` is reachable from gateway process.
- If rules API returns `403`, set `ADMIN_API_TOKEN` and pass it in request headers.
