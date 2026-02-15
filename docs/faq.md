# FAQ

## Why does `/api/rules` return `403`?

`/api/rules` is protected and requires `ADMIN_API_TOKEN`.

Set `ADMIN_API_TOKEN` in your environment, then send one of:

- `Authorization: Bearer <token>`
- `X-Admin-Token: <token>`

## Why does the proxy not reach my backend?

Check `BACKEND_URL` and ensure it is reachable from the gateway process.

For local Docker-based flow, `http://test-backend:8080` is used inside Compose.
For host-based flow, use something like `http://localhost:8080`.

## When should I set `TRUST_PROXY=true`?

Only when Gatify is behind a trusted reverse proxy or load balancer that correctly sets `X-Forwarded-For`.

If Gatify is internet-facing directly, keep `TRUST_PROXY=false`.

## Why am I not seeing expected rate limiting behavior?

Verify:

- `RATE_LIMIT_REQUESTS`
- `RATE_LIMIT_WINDOW_SECONDS`
- Client identity source (`TRUST_PROXY` and request origin)

Also inspect response headers:

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`

## How do I run all quality checks before opening a PR?

Run:

- `make check`

This executes formatting, vet, lint, and tests as configured in the Makefile.

## Should I use GitHub Wiki or `docs/`?

For this project, `docs/` is canonical.

Reasons:

- Versioned with code
- Reviewed through pull requests
- Easier to keep accurate over time
