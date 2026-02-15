# Deployment Guide

This guide documents a production-oriented Docker Compose deployment flow.

## Files used

- `docker-compose.prod.yml`
- `.env` (copy from `.env.example` and customize)

## 1) Prepare environment variables

Create deployment env file:

- Copy `.env.example` to `.env`

Set at minimum:

- `ADMIN_API_TOKEN` (long random secret, 32+ chars)
- `POSTGRES_PASSWORD`
- `DATABASE_URL` (keep password in sync with `POSTGRES_PASSWORD`)
- `BACKEND_URL` (your real upstream service)

## 2) Start the stack

Run:

- `docker compose -f docker-compose.prod.yml up -d --build`

Verify:

- `docker compose -f docker-compose.prod.yml ps`
- `curl http://localhost:3000/health`

## 3) Configure TLS/SSL

Terminate TLS in front of Gatify with a reverse proxy or load balancer (for example Nginx, Caddy, Traefik, Cloudflare Tunnel, or managed LB).

Recommended:

- Serve HTTPS only
- Redirect HTTP to HTTPS
- Keep Gatify on private network where possible

## 4) Security baseline for deployment

- Keep `ADMIN_API_TOKEN` in a secret manager for production.
- Do not expose Redis or TimescaleDB directly to the internet.
- Set `TRUST_PROXY=true` only when traffic passes through a trusted proxy that sets `X-Forwarded-For`.
- Rotate credentials regularly.

## 5) Backup and recovery

### TimescaleDB backup

Example command (run from host):

- `docker exec -t gatify-timescaledb pg_dump -U gatify gatify > backup.sql`

Restore:

- `cat backup.sql | docker exec -i gatify-timescaledb psql -U gatify gatify`

### Redis backup

Redis append-only data is persisted in `redis-data` volume.

For host-level backup, snapshot Docker volumes regularly.

## 6) Monitoring and logs

- Tail gateway logs: `docker compose -f docker-compose.prod.yml logs -f gatify`
- Tail full stack logs: `docker compose -f docker-compose.prod.yml logs -f`
- Check container health: `docker compose -f docker-compose.prod.yml ps`

## 7) Upgrade strategy

1. Pull latest source.
2. Rebuild and restart stack:
   - `docker compose -f docker-compose.prod.yml up -d --build`
3. Verify health endpoint and key proxy paths.

## 8) Rollback strategy

If a deployment fails:

1. Revert to previous known-good commit/tag.
2. Rebuild and restart using that revision.
3. Restore database backup if schema/data was changed.
