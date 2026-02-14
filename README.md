# Gatify 🛡️

**Self-hosted API Gateway with intelligent rate limiting, real-time analytics, and zero vendor lock-in.**

> ⚠️ **Early Development**: Gatify is currently in active development. Not production-ready yet!

## Features

- 🚀 **High Performance** - Handle 10K+ requests per second with sub-millisecond latency
- 🎯 **Smart Rate Limiting** - Sliding window algorithm with Redis backend
- 📊 **Real-time Analytics** - Live dashboard with traffic insights
- 🔧 **Self-Hosted** - Full control over your infrastructure
- 🐳 **Easy Deployment** - One-command Docker Compose setup
- 🌐 **HTTP Reverse Proxy** - Seamless integration with your backend services

## Quick Start

```bash
# Clone the repository
git clone https://github.com/Siruyy/gatify.git
cd gatify

# Start all services (Redis, TimescaleDB, Gatify)
docker-compose up -d

# View logs
docker-compose logs -f gatify
```

## Development

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Make

### Setup

```bash
# Install dependencies
make deps

# Run tests
make test

# Run linter
make lint

# Build binary
make build

# Start development environment
make dev
```

## Project Status

Gatify is being built in public! Check out the [development roadmap](https://linear.app/siruyy/project/gatify-9245f3b8fbcf) for current progress.

**Current Phase**: Core Gateway Development (Phase 1)

- [x] Project setup
- [ ] Sliding window rate limiter
- [ ] Redis storage backend
- [ ] HTTP reverse proxy
- [ ] Rule matching engine
- [ ] Analytics logging

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────┐
│        Gatify Gateway           │
│  ┌──────────────────────────┐  │
│  │   Rate Limit Middleware  │  │
│  └──────────────────────────┘  │
│  ┌──────────────────────────┐  │
│  │    Reverse Proxy         │  │
│  └──────────────────────────┘  │
└────┬──────────────────────┬─────┘
     │                      │
     ▼                      ▼
┌──────────┐          ┌──────────┐
│  Redis   │          │TimescaleDB│
│ (State)  │          │(Analytics)│
└──────────┘          └──────────┘
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- 📧 Email: nulysses.roda@siruyy.dev
- 🐛 [Report Issues](https://github.com/Siruyy/gatify/issues)
- 💬 [Discussions](https://github.com/Siruyy/gatify/discussions)

---

Built with ❤️ by [Neria](https://github.com/Siruyy)
