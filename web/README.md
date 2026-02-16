# React + TypeScript + Vite

This template provides a minimal setup to get React working in Vite with HMR and some ESLint rules.

Currently, two official plugins are available:

- [@vitejs/plugin-react](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react) uses [Babel](https://babeljs.io/) (or [oxc](https://oxc.rs) when used in [rolldown-vite](https://vite.dev/guide/rolldown)) for Fast Refresh
- [@vitejs/plugin-react-swc](https://github.com/vitejs/vite-plugin-react/blob/main/packages/plugin-react-swc) uses [SWC](https://swc.rs/) for Fast Refresh

# Gatify Dashboard (`web/`)

Frontend scaffold for GAT-21.

## Stack

- React 19 + TypeScript + Vite
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
- `VITE_ADMIN_API_TOKEN` (required to call protected `/api/rules` and `/api/stats` endpoints)

## Build and lint

- `make web-lint`
- `make web-build`

## Pages included

- `/dashboard` – traffic overview cards + timeline chart scaffold
- `/rules` – rules management table scaffold

## Notes

- This is a scaffold baseline for the dashboard roadmap and intentionally keeps feature logic minimal.
- Backend API is expected to run on `http://localhost:3000` during local development.
      parserOptions: {
        project: ['./tsconfig.node.json', './tsconfig.app.json'],
        tsconfigRootDir: import.meta.dirname,
      },
      // other options...
    },
  },
])
```
