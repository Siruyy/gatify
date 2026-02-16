# Gatify Full Audit

> **Date:** February 16, 2026
> **Branch:** `dev`
> **Auditor:** GitHub Copilot

Severity indicators: **[Critical]**, **[High]**, **[Medium]**, **[Low]**

---

## 1. Security

### [Critical] Admin token comparison is not constant-time

In `cmd/gatify/main.go` (line ~252), the token comparison `token != expectedToken` uses plain string comparison, which is vulnerable to timing attacks. Use `crypto/subtle.ConstantTimeCompare()` instead.

### [Critical] WebSocket `CheckOrigin` accepts all origins

In `internal/api/stats_stream.go` (lines 107–109), `CheckOrigin` returns `true` for all requests. This allows any website to open a WebSocket to your gateway and stream live events if someone has the token in the URL. At minimum, validate against a configurable allowed-origins list.

### [High] Admin token passed as query parameter for WebSocket

The stats stream handler (`internal/api/stats_stream.go`) and `web/src/pages/DashboardPage.tsx` (lines 49–53) pass the admin token as a query param (`?token=...`). Query params are logged in server access logs, browser history, and proxy logs. Consider using a short-lived session token or passing the token in the WebSocket subprotocol header instead.

### [High] No request body size limit

The rules API handler (`internal/api/rules_api.go`, line ~320) decodes JSON from `r.Body` without a size limit. An attacker could POST a multi-GB body and exhaust memory. Wrap with `http.MaxBytesReader` (e.g., 1MB).

### [Medium] No CORS middleware

The gateway exposes `/api/*` endpoints but has no CORS configuration on the HTTP server. The frontend connects from a different origin during development. You rely on `credentials: 'include'` in `web/src/lib/api.ts` (line 26) but there's no server-side CORS handler to allow it.

### [Medium] Redis has no password in dev compose

In `docker-compose.yml` (lines 6–13), Redis runs without authentication. While acceptable for local dev, the `.env.example` comment mentions production should use AUTH, but there's no `requirepass` configuration wired up.

---

## 2. Architecture & Design

### [High] Config package is empty

`internal/config/config.go` is just a TODO comment. All configuration is done via `getEnv()` calls scattered in `main.go`. This creates several problems:

- No validation of configuration at startup (e.g., `ADMIN_API_TOKEN=change-me` would be accepted)
- No structured config, making it hard to test or extend
- Duplicate `getEnv`/`getEnvInt`/`getEnvInt64` helpers that belong in a config package

**Recommendation:** Implement a `Config` struct, load from env, validate, and pass through dependency injection.

### [High] Rules engine is not connected to the proxy

The rules API creates/manages rules in an `InMemoryRepository`, but the proxy only uses a single global rate limit from `limiter.New()`. The entire rules engine (pattern matching, per-route limits, `IdentifyBy` header support) is built but **never wired into the request path**. This means:

- Creating rules via the API has no effect on actual traffic
- Per-route rate limiting doesn't work yet
- The `IdentifyBy: header` feature is unused

This is the biggest functional gap in the project.

### [Medium] Analytics logger is never instantiated

The `analytics.Logger` (batch writer to TimescaleDB) exists but is never created in `cmd/gatify/main.go`. Only the `QueryService` (read-only) is wired up. There's no code path that writes events to the database, so the analytics dashboard will always show empty data unless populated externally.

### [Medium] No graceful degradation for Redis failures

If Redis goes down mid-operation, every request returns a 500 error. Consider a circuit-breaker or fallback mode that allows traffic through (open policy) when the limiter is unavailable, rather than blocking all traffic.

### [Low] `IdleTimeout` not set on HTTP server

In `cmd/gatify/main.go` (lines 148–152), the server sets `ReadTimeout` and `WriteTimeout` but not `IdleTimeout`. For production, setting `IdleTimeout` (e.g., 120s) prevents idle connections from accumulating.

### [Low] No structured logging

The project uses `log.Printf` everywhere. Consider `log/slog` (stdlib since Go 1.21) for structured, leveled logging. This would also make the planned `LOG_LEVEL` env var (in `.env.example`) functional.

---

## 3. Code Quality

### [Medium] Duplicate `writeJSON` function

There are two different `writeJSON` implementations: one in `internal/proxy/proxy.go` (using `json.NewEncoder`) and one in `internal/api/rules_api.go` (using `json.Marshal`). They have different behavior (one adds a newline, one doesn't). Consolidate into a shared utility.

### [Medium] Missing `go.sum` in file listing

The `go.sum` file should be committed and is referenced in the Dockerfile `COPY go.mod go.sum ./`. Make sure it's tracked.

### [Low] Server binding address not configurable

The server always binds to `:3000` (`cmd/gatify/main.go`, line 149). This should be configurable via an env var (e.g., `PORT` or `LISTEN_ADDR`).

### [Low] `gatify` service ports not exposed in dev compose

The dev `docker-compose.yml` doesn't expose ports for the `gatify` service. Only `docker-compose.prod.yml` maps `3000:3000`. This is fine for dev if you run the binary locally, but should be documented more clearly.

---

## 4. Testing

### [Medium] No frontend tests

The `web/` directory has zero test files. No unit tests for hooks, components, or utility functions. Consider adding:

- Vitest + React Testing Library for component tests
- Tests for `apiRequest()`, `escapeCsv()`, and `validateForm()`

### [Medium] Analytics flush metrics may miscount

In `internal/analytics/analytics.go` (line ~232), `l.eventsLogged += int64(len(events))` counts all events in the batch as logged, even though individual inserts inside the loop may have failed (logged with `continue`). The count should only increment for successfully inserted events.

### [Low] E2E tests depend on timing assumptions

The E2E tests (`tests/e2e/e2e_test.go`, lines 113–170) assume a default rate limit of 100 and send 150 requests expecting some to be blocked. If the window or limit config changes, these tests silently become invalid. Consider reading rate-limit headers to make assertions more robust.

### [Low] No integration test for the rules API + rules engine round-trip

The management API integration test exists but there's no test validating that creating a rule actually affects the matcher's behavior (because it doesn't currently, per the architecture issue above).

---

## 5. DevOps & CI

### [Medium] CI uses `latest` for golangci-lint

In `.github/workflows/ci.yml` (line 29), `version: latest` for golangci-lint can cause flaky CI when a new version introduces breaking lint rules. Pin to a specific version.

### [Medium] No frontend CI job

The CI pipeline lints, tests, and builds Go code, but there's no job for the frontend. Add steps for `npm ci`, `npm run lint`, `npm run build` (and eventually `npm test`).

### [Medium] Dependabot missing npm ecosystem

`.github/dependabot.yml` tracks `gomod` and `github-actions` but not `npm` for the `web/` directory. Add:

```yaml
- package-ecosystem: "npm"
  directory: "/web"
  schedule:
    interval: "weekly"
```

### [Low] Docker image uses unpinned `alpine:3.20`

The `Dockerfile` (line 14) uses `alpine:3.20` without a digest pin. The prod compose file pins TimescaleDB with a SHA, but the base image for the gateway itself isn't pinned. Either pin with a SHA or accept the tradeoff for auto-patching.

### [Low] No `.goreleaser.yml` in repo

The release workflow uses goreleaser, but there is no `.goreleaser.yml` config file in the workspace. It might use defaults, but having an explicit config helps with reproducibility and changelog generation.

---

## 6. Frontend-Specific

### [Medium] No error boundaries

The React app has no `ErrorBoundary` component. If any component throws during render, the entire app crashes to a white screen. Wrap routes in an error boundary.

### [Medium] No auth/token input UI

The dashboard uses `getRuntimeAdminToken()` but there's no UI for the user to input their admin token. If no token getter is configured, all API calls will silently fail with 401/403. Add a token prompt or settings panel.

### [Low] Vite config missing proxy for dev

The `web/vite.config.ts` doesn't configure a dev server proxy to the Go backend. During local development, the frontend needs to make cross-origin requests to `localhost:3000`, which will fail without CORS. Adding a vite proxy (`/api -> http://localhost:3000`) would fix this.

### [Low] No loading skeletons or empty states

Pages show "Loading..." text but no skeleton UI. The Rules page shows nothing when the rules list is empty (should show an empty state encouraging the user to create a rule).

---

## 7. Priority Action Items (Suggested Order)

| # | Action | Severity |
|---|--------|----------|
| 1 | Wire the rules engine into the proxy | High |
| 2 | Instantiate the analytics logger in `main.go` | Medium |
| 3 | Implement the config package (centralize + validate env vars) | High |
| 4 | Fix constant-time token comparison | Critical |
| 5 | Add `http.MaxBytesReader` to the rules API | High |
| 6 | Add CORS middleware and restrict WebSocket origins | Medium/Critical |
| 7 | Add frontend CI (lint + build + eventual tests) | Medium |
| 8 | Add npm to Dependabot | Medium |
| 9 | Add structured logging via `log/slog` | Low |
| 10 | Add a token input UI to the web dashboard | Medium |

---

## Summary

The project's foundations are solid — clean package structure, good interface design (especially the `Storage`/`SlidingWindowStore` abstraction), proper Lua scripting for atomicity in Redis, well-structured CI, and good test coverage on the backend. The main gaps are in **wiring things together** (rules engine, analytics writer) and **security hardening** for production readiness.
