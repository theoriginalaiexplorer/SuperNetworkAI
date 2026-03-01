# AGENTS.md

This file guides agentic coding assistants working in the SuperNetworkAI repository.

## Build, Lint, and Test Commands

### Backend (Go, run from `backend/`)
```bash
make dev           # Hot reload via air
make build         # CGO_ENABLED=0 go build -ldflags="-s -w" -o api .
make swagger       # swag init -g main.go --output docs/
make migrate-up    # golang-migrate up (requires DATABASE_URL)
make migrate-down  # golang-migrate down 1
make sqlc          # sqlc generate
make test          # go test ./...
go test ./internal/service -v              # Run specific package, verbose
go test -run TestOllamaEmbedding ./... -v # Run specific test by name
```

### Frontend (Bun BFF, run from `frontend/`)
```bash
bun run dev        # Hot reload via bun --hot run server/index.ts
bun run start      # Production mode (requires build first)
bun run build      # tsc --noEmit && vite build
bun run typecheck  # tsc --noEmit
bun run lint       # eslint . --ext .ts
```

### Dev Infrastructure
```bash
docker compose up -d ollama              # Start Ollama (port 11434)
docker exec supernetwork_ollama ollama pull nomic-embed-text  # First run only
```

**Testing notes:**
- Some tests require `GROQ_API_KEY` in `backend/.env` (skip if unset)
- Embedding tests require Ollama running
- Use standard Go testing (no testify)

## Code Style Guidelines

### Go Backend

**Imports:** Grouped into 3 blocks with blank lines: stdlib, external, internal. Sort alphabetically.

**Formatting:** `gofmt` applied automatically. No trailing whitespace, tabs for indentation.

**Types:** Use concrete types only in `main.go`. All handlers/services depend on interfaces defined in `internal/service/interfaces.go`.

**Naming:**
- Handlers: `*UserHandler`, `*MatchHandler`
- Services: `*MatchService`, `*CVStructurer`
- Interfaces: `MatchService`, `EmbeddingProvider`, `NLSearchParser` (no "I" prefix)
- Constants: `ErrNotFound`, `ErrValidation`
- Unexported: `internalVar`, `privateFunc`
- Functions: `NewUserHandler`, `GetMatches`, `ParseSearchQuery` (PascalCase for exported)

**Error handling:**
- Always return `*model.AppError` from handlers: `return model.NewAppError(model.ErrNotFound, "user not found")`
- Error codes: `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `VALIDATION_ERROR`, `RATE_LIMITED`, `INTERNAL_ERROR`, `SERVICE_UNAVAILABLE`
- Global error handler registered in `main.go` converts `AppError` → HTTP status + JSON
- Use `%w` for wrapping: `return fmt.Errorf("groq request: %w", err)`

**SOLID/DRY:**
- Each handler file handles ONE resource group
- Handlers depend on interfaces, never concrete implementations
- Single shared functions: `UserFromCtx(c)`, `BuildEmbeddingText()`, `IsBlocked()`
- `main.go` is the composition root: wire concrete impls → inject into handlers

**LLM calls:**
- All Groq calls MUST use `response_format: {type: "json_object"}`
- Model selection: `llama-3.1-8b-instant` (NL search, Ikigai), `llama-3.3-70b-versatile` (CV, match explanations)
- Timeout: `context.WithTimeout(ctx, 10*time.Second)` for Groq, 5s for embeddings

**Logging:** Use `slog` (stdlib JSON format). Never log secrets (JWT, API keys, passwords).

### TypeScript Frontend

**Imports:** Use `import type` for types only. Group imports: stdlib, external, internal.

**Formatting:** Strict TypeScript, ESLint. No `any` types.

**Types:** Use `type` for object shapes, `interface` for contracts. Prefer explicit types.

**Naming:**
- Functions: `apiClient`, `signBffJwt`, `setSessionCookies`
- Types: `Method`, `ProfileData` (PascalCase)
- Constants: `GO_API_URL`, `BFF_JWT_SECRET`
- Files: `api.ts`, `auth.ts`, `middleware/session.ts` (kebab-case directories)

**Error handling:**
- Fetch errors: throw with `{status, code}` attached
- API client is the SINGLE source for BFF → Go API calls
- JWT from `getSession(c)` middleware helper

**Patterns:**
- Hono routes use functional handlers, not classes
- Eta templates in `server/templates/`
- Alpine.js stores in `client/stores/`
- HTMX partials rendered via `renderPartial(c, template, data)`

**No Bootstrap JS** — use Alpine.js directives only.

### Database

**Migrations:**
- Paired files: `00N_name.up.sql`, `00N_name.down.sql` in `backend/db/migrations/`
- Run `make migrate-up` to apply, `make migrate-down` to rollback
- pgvector enabled via Supabase dashboard (not migration)

**Queries:**
- Write SQL in `backend/db/queries/*.sql`
- Run `make sqlc` to generate type-safe Go code in `backend/internal/db/`
- Use generated methods: `queries.GetProfile(ctx, id)`

### Architecture Constraints

**NO JSON to browser:** Browser receives HTML from BFF only. WebSocket direct from Go API only.

**BFF ↔ Go API:** JSON REST over HTTP, `Authorization: Bearer <jwt>`.

**WebSocket:** First-message auth pattern (token in first JSON message, NOT URL). 60s HMAC tokens.

**Pagination:** Cursor-based using `(created_at, id)` tuples. Default 20, max 100.

**Embeddings:** Fixed 768-dim (nomic-embed-text). Never change without full re-migration.

### Testing

**Backend tests:**
- Use `testing.T`, no testify
- Setup: `_ = godotenv.Load("../../../.env")`
- Timeouts: `ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)`
- Log output: `t.Logf()` for debugging
- Run single test: `go test -run TestName ./... -v`

**Frontend tests:** Not yet implemented. Use `bun run typecheck` and `bun run lint`.

### Environment Variables

**Backend** (`backend/.env`): `DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_KEY`, `GROQ_API_KEY`, `OLLAMA_BASE_URL`, `NOMIC_API_KEY`, `EMBEDDING_PROVIDER` (ollama|nomic), `WS_TOKEN_SECRET`, `INTERNAL_API_SECRET`, `UPLOADTHING_SECRET`, `BFF_JWT_SECRET`

**Frontend** (`frontend/.env`): `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `GO_API_URL`, `SESSION_SECRET`, `FIREBASE_API_KEY`, `FIREBASE_AUTH_DOMAIN`, `FIREBASE_PROJECT_ID`, `BFF_JWT_SECRET`

All required vars validated at startup — process exits immediately if missing.

### Dependencies

**Backend:** Go 1.24.1, Fiber v3.0.0-beta.4, pgx v5.7.2, go-openai (Groq client), lestrrat-go/jwx (JWT)

**Frontend:** Bun runtime, Hono v4.7.4, Firebase v11.0.0, Eta v3.5.0, jose v5.0.0, TypeScript 5.8.2

**No new dependencies without justification.** Check existing patterns first.
