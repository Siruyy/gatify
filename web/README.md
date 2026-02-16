# Gatify Dashboard (`web/`)

Frontend scaffold for GAT-21.

## Stack

- React 18 + TypeScript + Vite
- React Router
- TanStack Query
- Tailwind CSS
- Recharts

## Local development

From repository root:

- `make web-install`
- `make web-dev`

Or directly in this folder:

- `npm install`
- `npm run dev`

## Environment variables

Configure in the root `.env` or export in your shell:

- `VITE_API_BASE_URL` (default: `http://localhost:3000`)

## Runtime auth behavior

- API requests do **not** read a build-time token from environment variables.
- Preferred runtime sources:
  - An injected in-memory getter (`window.__GATIFY_ADMIN_TOKEN__`)
  - Secure httpOnly cookie session on the backend
- Legacy browser storage token lookup is disabled by default and only used when explicitly opted in (`useLegacyStorage: true`).
- Requests are sent with `credentials: include` to support secure cookie-based auth if configured.

## Build and lint

- `make web-lint`
- `make web-build`

## Pages included

- `/dashboard` – traffic overview cards + timeline chart scaffold
- `/rules` – rules management table scaffold

## Notes

- This is a scaffold baseline for the dashboard roadmap and intentionally keeps feature logic minimal.
- Backend API is expected to run on `http://localhost:3000` during local development.
