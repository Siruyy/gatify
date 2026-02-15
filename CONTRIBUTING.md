# Contributing to Gatify

Thank you for considering contributing to Gatify.

## Development Process

We use a `dev` → `main` branching strategy:

1. **Feature branches** are created from `dev`
2. PRs from feature branches merge into `dev` for testing
3. Once stable, `dev` is merged into `main` for releases

## Getting Started

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Make
- golangci-lint

### Setup

```bash
# Fork and clone the repository
git clone https://github.com/YOUR_USERNAME/gatify.git
cd gatify

# Install dependencies
make deps

# Start development environment
make dev

# Run tests
make test

# Run linter
make lint
```

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/) for clear git history:

```text
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `chore`: Maintenance (deps, config)
- `refactor`: Code restructure (no behavior change)
- `test`: Adding/updating tests
- `perf`: Performance improvement
- `ci`: CI/CD changes

### Examples

```text
feat(gateway): implement sliding window rate limiter
fix(redis): handle connection timeout gracefully
docs: add installation guide
chore(deps): update go dependencies
test(proxy): add integration tests for reverse proxy
```

## Pull Request Process

1. **Create a feature branch from `dev`**:

   ```bash
   git checkout dev
   git pull origin dev
   git checkout -b feature/gat-XX-description
   ```

2. **Make your changes**:
   - Write tests for new features
   - Update documentation as needed
   - Follow Go best practices
   - Run `make check` before committing

3. **Commit your changes**:

   ```bash
   git add .
   git commit -m "feat(component): add new feature"
   ```

4. **Push and create PR**:

   ```bash
   git push origin feature/gat-XX-description
   ```

   - Open PR against `dev` branch
   - Fill out the PR template
   - Link related GitHub issue (e.g., `Closes #123`)

5. **Code Review**:
   - Address review comments
   - Keep commits clean
   - Ensure CI passes (test, lint, build)

6. **Merge**:
   - Squash and merge into `dev`
   - Delete branch after merge

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `golangci-lint` and fix warnings
- Write clear comments for complex logic
- Keep functions small and focused

## Testing

- Write unit tests for all new functionality
- Aim for >80% test coverage
- Add integration tests for critical paths
- Test with `make test`

## Issue Tracking

We use GitHub Issues for public collaboration. When contributing:

1. Check existing issues: [GitHub Issues](https://github.com/Siruyy/gatify/issues)
2. Comment on the issue you'd like to work on
3. Reference the issue in your PR (e.g., "Closes #123")

## Questions?

- Email: [nulysses.roda@siruyy.dev](mailto:nulysses.roda@siruyy.dev)
- [GitHub Discussions](https://github.com/Siruyy/gatify/discussions)
- [Report Issues](https://github.com/Siruyy/gatify/issues)

## Code of Conduct

Be respectful, inclusive, and professional. We're all here to build something great together.
