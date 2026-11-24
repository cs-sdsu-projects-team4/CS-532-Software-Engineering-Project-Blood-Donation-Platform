# Web — Clinic/Admin Dashboard

React + Vite + TypeScript + Tailwind CSS. The clinic/admin dashboard for the
Blood Donation Coordination platform; it consumes the Go backend REST API.

## Layout

```
src/
  main.tsx           app entry (mounts React, imports Tailwind)
  App.tsx            router setup
  layouts/           shared page shells (nav + <Outlet/>)
  pages/             route components
  lib/               api client, env config, shared utilities
```

## Prerequisites

- Node.js 20.19+ or 22.12+ (Vite 7 requirement)

## Configuration

Config is read from the environment at build time; only `VITE_`-prefixed
variables reach the client. Copy `.env.example` to `.env` and adjust:

- `VITE_API_BASE_URL` — backend API base (default `/api`, served via the Vite
  dev proxy locally and the Nginx reverse proxy in production).

## Commands

```bash
npm install          # install dependencies
npm run dev          # start the dev server (http://localhost:5173)
npm run build        # type-check and build for production (dist/)
npm run preview      # preview the production build
npm run lint         # ESLint
npm run typecheck    # TypeScript, no emit
```

During development, requests to `/api` are proxied to the Go backend on
`http://localhost:8080` (see `vite.config.ts`), so the browser stays same-origin
and no CORS setup is needed.
