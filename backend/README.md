# Backend — Go API Server

Go HTTP server for the Social Network with SQLite (WAL) and WebSockets.

## Tech stack

- Go 1.23.x
- SQLite via `github.com/mattn/go-sqlite3` (requires CGO)
- `net/http` server with middleware and feature-based handlers
- WebSockets (gorilla/websocket)

## Running locally

Requirements:

- Go 1.23.x
- A working C toolchain (CGO) for sqlite3

From `backend/`:

```powershell
go run server.go
```

The server listens on http://localhost:4000

Health check: `GET /health`

Uploaded media is served from `/uploads/media/`.

## Database & migrations

- SQLite DB file: `./social-network.db`
- Migrations: `./pkg/db/migrations/sqlite`
- Migrations are applied automatically on server startup.

Makefile targets (optional manual control):

```text
migrate-up              Run all pending migrations
migrate-down            Roll back the last migration
migrate-status          Show current migration status
migrate-rollback        Roll back N steps (use STEPS=<n>) or to a version (use VERSION=<v>)
migrate-rollback-version
migrate-force           Force set DB version (use VERSION=<v>)
dev-backend             Run the server (go run server.go)
clean                   Remove build artifacts
```

Examples:

```powershell
# From backend/
make migrate-up
make migrate-down
make migrate-status
```

## Environment

- Tenor API key: create a file `tenor.key` in `backend/` containing your API key to enable `/api/tenor` GIF search proxy.
- SQLite migrations path is read via env in Docker (`SQLITE_MIGRATIONS_PATH=file:///migrations/sqlite`), but for local runs the code uses `./pkg/db/migrations/sqlite`.

## Selected endpoints

- Auth: `POST /api/register`, `POST /api/login`, `POST /api/logout`
- Posts: `GET /api/posts`, `POST /api/create-post`, `POST /api/edit-post`, `POST /api/delete-post`, `POST /api/like/post/`
- Comments: `GET /api/comment`, `POST /api/comment/create`, `POST /api/comment/edit`, `POST /api/comment/delete`, `POST /api/comment/like`
- Groups: `/api/group/*` (create, edit, requests, invitations, admin)
- Events: `POST /api/event`, `GET /api/event/group`
- Follow: `/api/follow/*`, `/api/user/followers`, `/api/user/following`
- Search: `/api/search`, `/api/search/{users|groups|posts}`
- Media: `POST /api/upload/media` and GET `/uploads/media/...`
- WebSocket: `GET /ws` (requires auth)
- Tenor proxy: `GET /api/tenor?endpoint=...`

Development helpers: `/api/dev/*` (migration status, WAL status/checkpoint, auth check).

## Docker

The `backend/Dockerfile` builds the server with CGO enabled and exposes port 4000. In Docker Compose, migrations are mounted into `/migrations` and applied automatically on startup.
