# Social Network

A full‑stack social network built with:

- Backend: Go (net/http), SQLite (WAL), WebSockets
- Frontend: Next.js (App Router), React, TypeScript, Tailwind CSS (via @tailwindcss/postcss) and DaisyUI
- Containerization: Docker, Docker Compose

Features include authentication, posts with media, comments and likes, groups with invitations/requests, events, private chats over WebSockets, notifications, search, and a Tenor GIF proxy.

See ERD and docs in `shared/docs/` (e.g., `ERD.png`).

## Architecture

- `backend/` — Go HTTP server exposing REST APIs under `/api/*`, WebSocket hub at `/ws`, SQLite DB and migrations, and static serving for uploaded media from `/uploads/media/`.
- `frontend/` — Next.js app that consumes the API; API and WS endpoints are configured via environment variables.
- `docker-compose.yml` — Runs `backend` on port 4000 and `frontend` on port 3000 on the same bridge network. Health check probes `http://localhost:4000/health`.

## Prerequisites

Choose one setup:

- Docker Desktop 4.x (recommended)
- Or local tools:
  - Go 1.23.x (CGO enabled) with a working C toolchain (required by `github.com/mattn/go-sqlite3`)
  - Node.js 22.x and npm

Optional: Create a Tenor API key and save it to `backend/tenor.key` to enable GIF search.

## Quick start (Docker)

From the repository root:

```powershell
# Build and start both services
docker compose up --build
```

Then open:

- Frontend: http://localhost:3000
- Backend health: http://localhost:4000/health

Notes:

- `backend` reads SQLite migrations from a bind mount at `/migrations` and runs them automatically on startup.
- `frontend` receives `NEXT_PUBLIC_API_URL` and `NEXT_PUBLIC_WS_URL` pointing to `http://backend:4000` and `ws://backend:4000` inside the Compose network.

## Local development (without Docker)

### Backend (Go + SQLite)

1) Ensure Go 1.23.x and a C toolchain are installed (CGO is required).
2) From `backend/`, run the server (migrations are applied on startup):

```powershell
go run server.go
```

The API will listen on http://localhost:4000 and media will be served from `/uploads/media/`.

### Frontend (Next.js)

1) From `frontend/`, install dependencies and create `.env.local`:

```
NEXT_PUBLIC_API_URL=http://localhost:4000
NEXT_PUBLIC_WS_URL=ws://localhost:4000
```

2) Start the dev server:

```powershell
npm install
npm run dev
```

Open http://localhost:3000.

## Key endpoints (selection)

- Health: `GET /health`
- Auth: `POST /api/register`, `POST /api/login`, `POST /api/logout`, `GET /api/dev/checkAuth`
- Posts: `GET /api/posts`, `POST /api/create-post`, `POST /api/edit-post`, `POST /api/delete-post`, `POST /api/like/post/`
- Comments: `GET /api/comment`, `POST /api/comment/create`, `POST /api/comment/edit`, `POST /api/comment/delete`, `POST /api/comment/like`
- Groups: `POST /api/group` and membership/admin operations under `/api/group/*`
- Events: `POST /api/event`, `GET /api/event/group`
- Follow: `/api/follow/*`, `/api/user/followers`, `/api/user/following`
- Search: `/api/search`, `/api/search/{users|groups|posts}`
- Media upload: `POST /api/upload/media` (files served from `/uploads/media/`)
- WebSocket: `GET /ws` (requires auth)
- Tenor proxy: `GET /api/tenor?endpoint=search&q=...`

Development utilities are exposed under `/api/dev/*` (e.g., migration status, WAL tools).

## Data & migrations

- Database file: `backend/social-network.db` (SQLite, WAL mode).
- Migrations directory: `backend/pkg/db/migrations/sqlite`.
- The backend runs migrations at startup. Use Makefile targets for manual control (see backend README).

## Scripts overview

- Frontend: `npm run dev | build | start | lint`
- Backend Makefile: `dev-backend`, `migrate-up`, `migrate-down`, `migrate-status`, `migrate-rollback`, `migrate-force`, `clean`.

## Folder structure (high-level)

```
backend/       Go server, handlers, middleware, models, sockets, DB, migrations
frontend/      Next.js app (App Router), components, contexts, API routes, types
shared/docs/   ERD and reference docs
docker-compose.yml
```

## Troubleshooting

- SQLite/CGO on Windows: ensure a C compiler is available (e.g., MSYS2/MinGW) or use Docker for a fully prepped toolchain.
- Tenor GIFs not working: create `backend/tenor.key` containing your API key.
- Images blocked by Next.js: allowed image domains are set in `frontend/next.config.ts` (Giphy/Unsplash). Add domains as needed.

---
