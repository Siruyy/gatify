# Configuration Reference

Gatify currently reads configuration from environment variables in `cmd/gatify/main.go`.

## Runtime variables

| Variable | Default | Required | Description |
| --- | ---: | :---: | --- |
| `REDIS_ADDR` | `localhost:6379` | No | Redis endpoint for limiter state. |
| `BACKEND_URL` | `http://localhost:8080` | No | Upstream target for reverse proxy. Must be valid `http`/`https` URL. |
| `TRUST_PROXY` | `false` | No | If `true`, client key uses first IP from `X-Forwarded-For`. |
| `RATE_LIMIT_REQUESTS` | `100` | No | Allowed requests per window per client key. |
| `RATE_LIMIT_WINDOW_SECONDS` | `60` | No | Window length in seconds. |
| `ADMIN_API_TOKEN` | _(empty)_ | Yes (for `/api/rules`) | Bearer token for rules management API. |

## Optional / test-related

| Variable | Default | Description |
| --- | ---: | --- |
| `DATABASE_URL` | `postgres://gatify:gatify_dev_password@localhost:5432/gatify?sslmode=disable` | Used by analytics integration tests. |
| `POSTGRES_PASSWORD` | `gatify_dev_password` | Used by Docker Compose TimescaleDB service. |

## Rules API authentication

When `ADMIN_API_TOKEN` is empty, `/api/rules` endpoints are disabled and return `403`.

Provide token through either:

- `Authorization: Bearer <token>`
- `X-Admin-Token: <token>`

## Notes

- `TRUST_PROXY=true` should only be enabled behind trusted reverse proxies.
- Invalid numeric values for rate-limit env vars fall back to defaults.
