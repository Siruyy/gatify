# Deployment Troubleshooting

Use this guide when `docker-compose.prod.yml` deployments fail or behave unexpectedly.

## Gateway does not start

Check logs:

- `docker compose -f docker-compose.prod.yml logs gatify --tail=200`

Common causes:

- `ADMIN_API_TOKEN` missing in `.env`
- invalid `BACKEND_URL`
- Redis not reachable via `REDIS_ADDR`

## `/health` is down

1. Confirm container state:
   - `docker compose -f docker-compose.prod.yml ps`
2. Check gateway logs.
3. Verify port mapping (`3000:3000`) is free on host.

## Proxy requests fail (`502` / bad gateway)

Likely upstream issue.

Check:

- `BACKEND_URL` value
- network reachability from gateway container
- upstream service health

Quick network test from gateway container:

- `docker exec -it gatify-gateway wget -qO- "$BACKEND_URL"`

## Rules API returns `401` or `403`

- Ensure `ADMIN_API_TOKEN` is set in deployment environment.
- Send either `Authorization: Bearer <token>` or `X-Admin-Token: <token>`.
- If token is empty/misconfigured, `/api/rules` remains inaccessible.

## Redis connection errors

Check:

- `REDIS_ADDR` points to reachable host:port
- Redis container is healthy
- If using authenticated Redis outside the stack, ensure gateway supports matching credentials in runtime config

## TimescaleDB auth failures

Ensure these match:

- `POSTGRES_PASSWORD`
- password section of `DATABASE_URL`

Mismatch is a common source of integration and compose failures.

## Changes are not reflected after update

Rebuild containers:

- `docker compose -f docker-compose.prod.yml up -d --build`

If needed, remove old containers first:

- `docker compose -f docker-compose.prod.yml down`
- `docker compose -f docker-compose.prod.yml up -d --build`

## Volume/data concerns

Inspect volumes:

- `docker volume ls | grep gatify`

Do not delete data volumes in production unless you have verified backups.
