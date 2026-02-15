# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary (stripped of symbol/debug info to reduce size)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gatify ./cmd/gatify

# Runtime stage
FROM alpine:3.20

# Intentionally unpinned to always receive up-to-date root certificates and avoid TLS issues
RUN apk add --no-cache ca-certificates curl

# Create non-root user and group
RUN addgroup -S gatify && adduser -S -G gatify gatify

COPY --from=builder /gatify /usr/local/bin/gatify

EXPOSE 3000

USER gatify

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:3000/health || exit 1

ENTRYPOINT ["gatify"]
