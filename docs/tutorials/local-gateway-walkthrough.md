# Tutorial: Local Gateway Walkthrough

This tutorial validates the full local flow:

1. Start infra
2. Run gateway
3. Create a rate-limit rule
4. Send proxied traffic and observe behavior

## 1) Prepare environment

- Copy env file: `cp .env.example .env`
- Set a secure `ADMIN_API_TOKEN` value in `.env`

## 2) Start dependencies

- Start Docker services: `make dev`

## 3) Run gateway

- Run: `go run ./cmd/gatify`

Health check:

- `curl http://localhost:3000/health`

## 4) Create a rule

Example body:

```json
{
  "name": "httpbin-get-limit",
  "pattern": "/get",
  "methods": ["GET"],
  "priority": 10,
  "limit": 5,
  "window_seconds": 60,
  "identify_by": "ip",
  "enabled": true
}
```

Send with your admin token to `POST /api/rules`.

## 5) Exercise proxy

Make repeated requests to `/proxy/get`.

Observe headers:

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`

After threshold, expect `429` with JSON error.

## 6) Cleanup

- Stop gateway (`Ctrl+C`)
- Stop services: `docker-compose down`
