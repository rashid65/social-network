# Frontend — Next.js App

This is the React/Next.js frontend of the Social Network.

- Next.js 15 (App Router), React 19, TypeScript
- Tailwind CSS v4 (via `@tailwindcss/postcss`), DaisyUI
- Talks to the Go backend via REST and WebSockets

## Prerequisites

- Node.js 22.x
- npm

## Environment

Create `frontend/.env.local` for local development:

```
NEXT_PUBLIC_API_URL=http://localhost:4000
NEXT_PUBLIC_WS_URL=ws://localhost:4000
```

When running with Docker Compose, these are provided as build args and envs and resolve to the backend service hostname.

## Install & run (local)

```powershell
npm install
npm run dev
```

Open http://localhost:3000.

## Scripts

- `npm run dev` — start dev server (Turbopack)
- `npm run build` — production build
- `npm run start` — start production server
- `npm run lint` — run Next.js ESLint

## Docker

The `frontend/Dockerfile` builds the Next.js app in a multi-stage image. When using `docker-compose.yml`, the container exposes port 3000 and is configured with `NEXT_PUBLIC_API_URL` and `NEXT_PUBLIC_WS_URL` pointing to the backend service.

## Images

Allowed external image domains are configured in `next.config.ts` (Unsplash, Giphy). Add more domains as needed.

## Project structure (high-level)

- `app/` — routes, layouts, and pages
- `components/` — UI components (chat, feed, profile, etc.)
- `context/` — React contexts (Auth, Notifications, WebSocket, Theme)
- `lib/` and `utils/` — helpers and shared utilities
- `types/` — TypeScript types for API data
