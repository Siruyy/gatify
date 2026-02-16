# API Reference

Base URL (local): `http://localhost:3000`

## Health

### `GET /health`

Returns gateway health status.

## Root

### `GET /`

Returns a short plaintext banner.

## Reverse Proxy

### `ANY /proxy/*`

Forwards requests to `BACKEND_URL` while applying rate limiting.

Response headers on proxied requests:

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`

When limited, returns `429` JSON:

```json
{
  "error": "rate limit exceeded",
  "limit": 100,
  "remaining": 0,
  "reset_at": "2026-01-01T00:00:00Z"
}
```

## Rules Management API

Protected endpoints under `/api/rules`.

Auth options:

- `Authorization: Bearer <ADMIN_API_TOKEN>`
- `X-Admin-Token: <ADMIN_API_TOKEN>`

### `GET /api/rules`

Returns:

```json
{ "data": [/* rules */] }
```

### `POST /api/rules`

Creates a rule.

Required fields:

- `name`
- `pattern`
- `limit` (> 0)
- `window_seconds` (> 0)

Optional fields:

- `methods` (valid HTTP methods)
- `priority`
- `identify_by` (`ip` or `header`, default `ip`)
- `header_name` (required when `identify_by=header`)
- `enabled` (default `true`)

### `GET /api/rules/{id}`

Fetches a single rule.

### `PUT /api/rules/{id}`

Updates a rule (partial merge behavior for omitted optional fields).

### `DELETE /api/rules/{id}`

Deletes a rule and returns `204`.

## Analytics & Statistics API

Protected endpoints under `/api/stats`.

Auth options:

- `Authorization: Bearer <ADMIN_API_TOKEN>`
- `X-Admin-Token: <ADMIN_API_TOKEN>`

Optional query parameters:

- `window` (default `24h`, supports values like `15m`, `1h`, `7d`)
- `limit` (only for top blocked clients, default `10`, max `100`)
- `bucket` (only for timeline, default `5m`, min `1m`, max `24h`)

### `GET /api/stats/overview`

Returns aggregate request stats (total/allowed/blocked/unique clients/block rate).

### `GET /api/stats/top-blocked`

Returns top blocked clients for the selected window.

### `GET /api/stats/rules/{id}`

Returns rule-specific analytics for the selected window.

### `GET /api/stats/timeline`

Returns time-bucketed allowed/blocked/total counts.

## OpenAPI

Machine-readable specification:

- [`openapi.yaml`](./openapi.yaml)
