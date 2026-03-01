# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Status

SuperNetworkAI is an Ikigai-based AI networking app (cofounders, teammates, clients). Phase 0 (Foundation) is in progress. See `PLAN.md` for the authoritative implementation spec and `Progress.md` for phase status.

## Commands

### Backend (Go — run from `backend/`)
```bash
make dev           # hot reload via air
make build         # CGO_ENABLED=0 go build -ldflags="-s -w" -o api .
make swagger       # swag init -g main.go --output docs/
make migrate-up    # golang-migrate up
make migrate-down  # golang-migrate down 1
make sqlc          # sqlc generate
make test          # go test ./...
go test ./internal/service -v              # run specific package with verbose output
go test -run TestCVPipeline ./... -v       # run specific test by name
```

### Frontend (Bun BFF — run from `frontend/`)
```bash
bun run dev        # hot reload via bun --hot run server/index.ts
bun run start      # production mode (requires build first)
bun run build      # tsc + vite build
bun run typecheck  # tsc --noEmit
bun run lint       # eslint .
```

**Directory structure:**
- `server/` — Hono app, Eta templates, API client
- `client/` — Browser JS (Alpine.js, HTMX)

### Dev Infrastructure
```bash
docker compose up -d    # Start Ollama (port 11434)
# First run only — pull embedding model:
docker exec supernetwork_ollama ollama pull nomic-embed-text
```

**Testing requirements:**
- Some tests require `GROQ_API_KEY` in `backend/.env` (will skip if not set)
- Embedding tests require Ollama running (`docker compose up -d`)
- Tests use standard Go testing (no testify dependency)

## Architecture

```
Browser (HTMX + Alpine.js + Bootstrap CSS)
  │ HTML partials only          │ WebSocket only
  ▼                             ▼
Bun BFF (Hono + Eta)   ←REST→  Go API (Fiber v3, port 3001)
  port 3000                       │
                                  ├─ Supabase PostgreSQL + pgvector
                                  ├─ Groq API (LLM)
                                  ├─ Ollama/Nomic (embeddings, 768-dim)
                                  └─ Uploadthing (files)
```

**Hard constraints:**
- Browser never calls Go API via REST — HTML from BFF, WebSocket from Go only
- BFF calls Go API server-side with Bearer JWT
- No JSON served to browser — HTMX partials only
- Go API is the sole auth/authorization layer (Supabase RLS disabled)

## Key Design Patterns

### SOLID / DRY (strictly enforced)
- Handlers depend on interfaces (`MatchService`, `EmbeddingProvider`, etc.) — never concrete types
- `main.go` is the only place concrete implementations are wired
- Block exclusion: single `IsBlocked()` function used everywhere
- Visibility filter: named CTE `visible_profiles` reused across queries
- Embedding text: single `BuildEmbeddingText(profile, ikigai)` function
- Error responses: `AppError` type with `{"code":"SNAKE_CASE","message":"..."}` shape
  - Exhaustive error codes in `internal/model/errors.go`: `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `VALIDATION_ERROR`, `RATE_LIMITED`, `INTERNAL_ERROR`, `SERVICE_UNAVAILABLE`
  - Return errors using: `return model.NewAppError(model.ErrNotFound, "profile not found")`
  - Global error handler registered in `main.go` converts `AppError` → HTTP status + JSON
- JWT extraction: `UserFromCtx(c)` helper used in all handlers

### Auth Flow
1. Supabase Auth issues JWT → BFF stores in HttpOnly cookie
2. BFF forwards Bearer token to Go API on every server-side request
3. WebSocket auth: HMAC-signed token (60s TTL, single-use via `sync.Map`)
4. Token refresh guarded by mutex (Supabase refresh tokens are single-use)

### Embedding Pipeline
- Triggered async (goroutine) on: Ikigai save, profile update
- Status field: `pending → current | failed`
- Cloud Scheduler re-embeds `stale` entries as recovery
- All embeddings are 768-dim fixed (nomic-embed-text)

### LLM Usage (Groq API)
All calls must use `response_format: {type: "json_object"}`. Model selection:
- `llama-3.1-8b-instant` — NL search parsing, Ikigai summary
- `llama-3.3-70b-versatile` — CV structuring, match explanations

### Pagination
Cursor-based using `(created_at, id)` tuples. Default limit: 20, max: 100.

## Database

9 migrations in `backend/db/migrations/`. Key tables:
- `profiles` — includes `embedding VECTOR(768)` with HNSW index (m=16, ef=64)
- `ikigai_profiles` — 4 Ikigai answers + AI summary
- `match_cache` — upserted scores; dismissed state preserved across refreshes
- `blocks` — checked in both directions at the query level

**Migration workflow:**
1. Create paired files: `00N_name.up.sql`, `00N_name.down.sql` in `backend/db/migrations/`
2. Run `make migrate-up` to apply (requires `DATABASE_URL` in `.env`)
3. Run `make migrate-down` to rollback last migration

**Query workflow:**
1. Write SQL in `backend/db/queries/*.sql` (e.g., `profiles.sql`)
2. Run `make sqlc` to generate type-safe Go code in `backend/internal/db/`
3. Use generated methods in services (e.g., `queries.GetProfile(ctx, id)`)

## API Structure

Base: `http://localhost:3001`
- `GET /healthz` — liveness (always 200)
- `GET /readyz` — readiness (checks DB)
- `POST /api/v1/auth/ws-token` — issue WebSocket HMAC token
- `/api/v1/users/*`, `/api/v1/profiles/*` — profile management
- `/api/v1/onboarding/*` — Ikigai + CV import
- `/api/v1/matches`, `/api/v1/search` — matching and NL search
- `/api/v1/connections`, `/api/v1/conversations/*`, `/api/v1/blocks` — social graph
- `ws://localhost:3001/ws` — WebSocket (real-time messaging)
- `/internal/*` — Cloud Scheduler endpoints (authenticated by `INTERNAL_API_SECRET`)

Full API contract in `PLAN.md` §7.

## Environment Variables

**Backend** (`backend/.env`): `DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_KEY`, `GROQ_API_KEY`, `OLLAMA_BASE_URL`, `NOMIC_API_KEY`, `EMBEDDING_PROVIDER`, `WS_TOKEN_SECRET`, `INTERNAL_API_SECRET`, `UPLOADTHING_SECRET`

**Frontend** (`frontend/.env`): `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `GO_API_URL`, `SESSION_SECRET`

## Deployment

- Go: `CGO_ENABLED=0` → `gcr.io/distroless/static:nonroot` (~15MB image) on GCP Cloud Run
- Cloud Run: `min-instances=1` required (WebSocket connections)
- Secrets via GCP Secret Manager
