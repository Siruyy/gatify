# Gatify

[![Documentation](https://img.shields.io/badge/documentation-available-blue)](docs/README.md)

Self-hosted API Gateway with rate limiting, reverse proxying, and management APIs.

> **Early development**: Gatify is under active development and not yet production-ready.

## Demo

- Demo video: _coming soon_ (tracked in docs roadmap)

## What you get

- Sliding-window request limiting backed by Redis
- Reverse proxy mode for upstream backends
- Rules CRUD API (`/api/rules`)
- Analytics/stats API (`/api/stats/*`)
- React + Vite dashboard scaffold in `web/`
- Unit/integration/e2e test suites
- Container-friendly local setup via Docker Compose

## 5-minute quick start

1. Clone and enter the repository.
2. Copy env defaults:
     - `cp .env.example .env`
3. Start infrastructure:
     - `make dev`
4. Run the gateway:
     - `go run ./cmd/gatify`
5. Verify:
     - `curl http://localhost:3000/health`

The gateway serves:

- Health: `GET /health`
- Proxy: `/{proxy path under /proxy/...}`
- Rules management API: `/api/rules` (requires `ADMIN_API_TOKEN`)

## Documentation

- [Documentation index](docs/README.md)
- [Installation guide](docs/installation.md)
- [Configuration reference](docs/configuration.md)
- [API reference](docs/api/README.md)
- [OpenAPI spec](docs/api/openapi.yaml)
- [Architecture notes](docs/architecture.md)
- [Deployment guide](docs/deployment.md)
- [Deployment troubleshooting](docs/deployment-troubleshooting.md)
- [Local walkthrough tutorial](docs/tutorials/local-gateway-walkthrough.md)
- [Production Compose file](docker-compose.prod.yml)
- [FAQ](docs/faq.md)
- [Support](SUPPORT.md)
- [Security policy](SECURITY.md)
- [Contributing guide](CONTRIBUTING.md)

## Developer workflow

- Install deps: `make deps`
- Lint: `make lint`
- Test: `make test`
- Full checks: `make check`
- E2E tests: `make test-e2e`
- Load tests: `make test-load-quick`
- Migrations up: `make migrate-up DATABASE_URL="postgres://gatify:gatify_dev_password@localhost:5432/gatify?sslmode=disable"`
- Migration version: `make migrate-version DATABASE_URL="postgres://gatify:gatify_dev_password@localhost:5432/gatify?sslmode=disable"`
- Frontend install: `make web-install`
- Frontend dev server: `make web-dev`
- Frontend build: `make web-build`
- Frontend lint: `make web-lint`

## Dashboard scaffold (`web/`)

The frontend scaffold includes:

- React 18 + TypeScript + Vite
- React Router for app routing
- TanStack Query for data fetching
- Tailwind CSS for styling
- Recharts for analytics visualizations

Environment variables (frontend):

- `VITE_API_BASE_URL` (default: `http://localhost:3000`)

Runtime auth notes (frontend):

- API auth token is read at runtime from an injected in-memory source (`window.__GATIFY_ADMIN_TOKEN__`) or secure cookies.
- Legacy browser storage lookup is disabled by default and requires explicit opt-in.
- No build-time admin token is embedded in the frontend bundle.

## Project roadmap

Track progress in GitHub:

- [Issues](https://github.com/Siruyy/gatify/issues)
- [Pull Requests](https://github.com/Siruyy/gatify/pulls)

## License

MIT — see [LICENSE](LICENSE).

## Support

- For usage questions and troubleshooting, see [SUPPORT.md](SUPPORT.md).
- For vulnerability reports, see [SECURITY.md](SECURITY.md).
