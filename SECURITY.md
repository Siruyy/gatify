# Security Policy

## Supported versions

Gatify is currently in active development.

Security fixes are applied on:

- `dev` (primary active branch)
- `main` (latest stable release line, when applicable)

Older branches may not receive security updates.

## Reporting a vulnerability

Do not open public GitHub issues for security vulnerabilities.

Please report vulnerabilities privately via email:

- [nulysses.roda@siruyy.dev](mailto:nulysses.roda@siruyy.dev)

Include as much detail as possible:

- Affected version/commit
- Reproduction steps or proof of concept
- Impact assessment
- Suggested fix (if available)

## Response and disclosure process

Target timelines:

- Initial response: within 72 hours
- Triage and severity assessment: within 7 days
- Fix timeline: depends on severity and complexity

After a fix is available, we will coordinate disclosure and publish remediation guidance.

## Security best practices for users

- Set a strong `ADMIN_API_TOKEN` in all non-local environments.
- Keep `TRUST_PROXY=false` unless running behind a trusted proxy that sets `X-Forwarded-For`.
- Restrict network access to Redis and backend services.
- Keep dependencies and container base images up to date.
