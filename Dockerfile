# Build stage — pinned digest for reproducible builds.
# To update: docker pull golang:1.22-alpine && docker inspect --format='{{index .RepoDigests 0}}' golang:1.22-alpine
FROM golang:1.22-alpine@sha256:1699c10032ca2582ec89a24a1312d986a3f094aed3d5c1147b19880afe40e052 AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary (stripped of symbol/debug info to reduce size)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gatify ./cmd/gatify

# Runtime stage — pinned digest for reproducible builds.
# To update: docker pull alpine:3.20 && docker inspect --format='{{index .RepoDigests 0}}' alpine:3.20
FROM alpine:3.20@sha256:1e42bbe2508154c9126d48c2b8a75420c3544343bf86fd041fb7527e017a4b4a
RUN apk add --no-cache ca-certificates curl

# Create non-root user and group
RUN addgroup -S gatify && adduser -S -G gatify gatify

COPY --from=builder /gatify /usr/local/bin/gatify

EXPOSE 3000

USER gatify

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:3000/health || exit 1

ENTRYPOINT ["gatify"]
