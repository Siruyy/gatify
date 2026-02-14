# End-to-End Tests

This directory contains comprehensive end-to-end integration tests that verify the complete Gatify stack.

## Overview

E2E tests verify:
- ✅ Full request flow (client → gateway → redis → backend)
- ✅ Rate limiting enforcement (allow/block)
- ✅ Concurrent client handling
- ✅ Independent rate limits per client IP
- ✅ Proper HTTP proxying
- ✅ Response headers (X-RateLimit-*)

## Prerequisites

All services must be running before tests:

1. **Redis** (for rate limiting state)
2. **TimescaleDB** (for analytics - optional for basic tests)
3. **Test Backend** (httpbin)
4. **Gatify Gateway** (the service being tested)

## Quick Start

### 1. Start Services

```bash
# Start infrastructure services
docker-compose up -d redis timescaledb test-backend

# Start Gatify gateway
go run ./cmd/gatify
```

### 2. Run E2E Tests

```bash
# Run all E2E tests
make test-e2e

# Or run directly with go
go test -tags=e2e -v ./tests/e2e/

# Run specific test
go test -tags=e2e -v ./tests/e2e/ -run TestRateLimitEnforce
```

## Test Scenarios

### TestHealth
Verifies the health endpoint responds correctly.

### TestRateLimitAllow
Tests that requests under the rate limit are allowed and proper headers are returned.

### TestRateLimitEnforce
Sends many requests to verify the rate limiter blocks excess requests after the limit is reached.

**Expected behavior:**
- First ~100 requests: Allowed (200 OK)
- Subsequent requests: Blocked (429 Too Many Requests)

### TestConcurrentClients
Launches multiple concurrent clients making simultaneous requests to verify:
- No race conditions
- Consistent rate limiting
- Acceptable latency under load

### TestDifferentClientIPs
Verifies that different client IPs have independent rate limits.

### TestResponseHeaders
Checks that all required rate limit headers are present:
- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`

### TestBackendProxying
Verifies that requests are properly proxied to the backend service.

## Configuration

Tests use the following URLs:
- **Gateway**: `http://localhost:3000`
- **Proxy Path**: `/proxy/`
- **Backend**: Configured via `BACKEND_URL` env var (default: `http://localhost:8080`)

Rate limit defaults:
- **Limit**: 100 requests (via `RATE_LIMIT_REQUESTS`)
- **Window**: 60 seconds (via `RATE_LIMIT_WINDOW_SECONDS`)

## Debugging

### View detailed test output
```bash
go test -tags=e2e -v ./tests/e2e/ -count=1
```

### Check service status
```bash
# Gateway health
curl http://localhost:3000/health

# Redis
docker-compose exec redis redis-cli ping

# Test backend
curl http://localhost:8080/status/200
```

### View logs
```bash
# Docker services
docker-compose logs -f

# Gateway (if running in separate terminal)
# Check the terminal where you ran: go run ./cmd/gatify
```

## CI Integration

E2E tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Start services
  run: docker-compose up -d
  
- name: Start gateway
  run: go run ./cmd/gatify &
  
- name: Wait for services
  run: sleep 5
  
- name: Run E2E tests
  run: make test-e2e
```

## Tips

1. **Clean slate**: Run `docker-compose down -v` before tests to reset Redis state
2. **Port conflicts**: Ensure ports 3000, 5432, 6379, 8080 are available
3. **Timeouts**: If tests fail with timeouts, services may not be fully started
4. **Rate limits**: Tests may affect each other if run too quickly - use `-count=1` to disable caching

## Troubleshooting

### "Gatify gateway not available"
- Ensure gateway is running: `go run ./cmd/gatify`
- Check port 3000 is not in use: `lsof -i :3000`

### "Connection refused"
- Verify Docker services: `docker-compose ps`
- Check service health: `docker-compose ps` (look for "healthy" status)

### Tests are flaky
- Reset Redis state: `docker-compose restart redis`
- Increase delays between tests
- Run with `-count=1` to disable test caching
