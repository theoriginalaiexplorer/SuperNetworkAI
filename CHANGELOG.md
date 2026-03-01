# Changelog

All notable changes are documented here per phase.
Each phase is tagged in git as `phase/N-name` on completion.

Format: each entry lists what was added, what was changed, and what the phase tag is.

---

## [Unreleased]

---

## [phase/0-foundation] — 2026-03-01

### Added
- `backend/go.mod` + `backend/go.sum` — Go module (Go 1.23, Fiber v3, pgx v5, jwx v2, godotenv)
- `backend/main.go` — composition root: Fiber app, middleware registration, graceful shutdown
- `backend/internal/config/config.go` — env loading with fail-fast on missing required vars
- `backend/internal/model/errors.go` — `AppError` type, exhaustive error code registry, Fiber `ErrorHandler`
- `backend/internal/model/request.go` — `PaginationQuery` struct with validation
- `backend/internal/model/response.go` — `StatusOK` envelope
- `backend/internal/db/client.go` — `pgxpool` with `SimpleProtocol` mode (Supavisor-compatible)
- `backend/internal/health/handler.go` — `GET /healthz` (liveness) + `GET /readyz` (DB ping)
- `backend/internal/middleware/recovery.go` — panic → 500, never crashes server
- `backend/internal/middleware/cors.go` — BFF-origin-only CORS
- `backend/internal/middleware/logger.go` — structured `slog` request logging (no sensitive fields)
- `backend/internal/middleware/auth.go` — JWT verify via JWKS (30min cache) + `UserFromCtx()` helper
- `backend/db/migrations/` — 9 up + 9 down SQL migration files (001–009)
- `backend/Makefile` — `dev`, `build`, `swagger`, `migrate-up`, `migrate-down`, `sqlc`, `test` targets
- `backend/.air.toml` — hot reload config
- `backend/sqlc.yaml` — sqlc codegen config (pgvector + uuid overrides)
- `backend/Dockerfile` — multi-stage: golang:1.23-alpine → distroless/static:nonroot
- `backend/.env.example` — all required env vars documented
- `frontend/package.json` — Hono, Eta, Supabase JS, Vite, TypeScript, ESLint
- `frontend/tsconfig.json` — strict TypeScript config for Bun runtime
- `frontend/vite.config.ts` — builds `client/` → `dist/public/`
- `frontend/server/index.ts` — Hono app with `GET /healthz` + `Bun.serve()`
- `frontend/server/lib/eta.ts` — Eta engine (async, auto-escape, production cache)
- `frontend/server/lib/render.ts` — `renderPartial()` helper for HTMX partials
- `frontend/server/lib/api.ts` — `apiClient(jwt)` typed fetch client → Go API (SINGLE source)
- `frontend/client/main.ts` — Vite entry point (Alpine stores registered in Phase 1)
- `frontend/Dockerfile` — multi-stage: oven/bun:1 builder → oven/bun:1-slim runtime
- `frontend/.env.example` — all required env vars documented
- `CLAUDE.md` — Claude Code project guidance file

### Test criteria passed
- [x] `curl localhost:3001/healthz` → `{"status":"ok"}`
- [x] `curl localhost:3001/readyz` → does not crash
- [x] `curl localhost:3000/healthz` → `{"status":"ok"}`
- [x] `docker build backend/` → succeeds, image < 30MB
- [x] `docker build frontend/` → succeeds
- [ ] `make migrate-up` → requires live Supabase DB creds

---

<!-- New phase entries added above this line on tag -->
