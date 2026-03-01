# Changelog

All notable changes are documented here per phase.
Each phase is tagged in git as `phase/N-name` on completion.

Format: each entry lists what was added, what was changed, and what the phase tag is.

---

## [Unreleased]

---

## [phase/3-cv-import] — 2026-03-01

### Added
- `backend/internal/service/downloader.go` — `DownloadPDF()`: SSRF-safe HTTP fetch (URL allowlist: uploadthing.com/utfs.io/ufs.sh/localhost, 10s timeout, 5MB cap, MIME check)
- `backend/internal/service/pdf.go` — `ExtractPDFText()`: PDF-to-plaintext via `ledongthuc/pdf`
- `backend/internal/service/llm/cv.go` — `CVStructurer.StructureCV()`: Groq `llama-3.3-70b-versatile` in JSON mode → `service.CVData`
- `backend/internal/handler/files.go` — `POST /api/v1/files/presign` stub (Uploadthing Phase 3+ SDK integration pending)
- `backend/internal/handler/onboarding.go` — `ImportCV` method: download → extract → LLM → JSON; wired to `POST /api/v1/onboarding/import-cv`
- `frontend/server/routes/onboarding.ts` — `POST /onboarding/import-cv` BFF proxy route
- `frontend/server/templates/pages/onboarding/step5-links.eta` — CV import card (Alpine.js fetch + DOM pre-fill for LinkedIn/GitHub/portfolio)

### Changed
- `backend/go.mod` — added `ledongthuc/pdf`, removed incorrect `pdfcpu` API usage
- `backend/main.go` — wired `cvStructurer` and `POST /onboarding/import-cv` route
- `backend/internal/handler/onboarding.go` — `OnboardingHandler` extended with `cvStructurer service.CVStructurer` field
- `.gitignore` — added `.claude/settings.local.json`

### Fixed
- Eta template layout paths: `"layouts/base"` → `"../layouts/base"` (pages) and `"../../layouts/base"` (onboarding sub-dir) — fixes 500 on all page routes

### Test criteria passed
- [x] `go build ./...` — no errors
- [x] `bun typecheck` — no errors
- [x] `GET localhost:3001/healthz` → `{"status":"ok"}`
- [x] `GET localhost:3001/readyz` → `{"status":"ready"}`
- [x] `GET localhost:3000/healthz` → `{"status":"ok"}`
- [x] `GET localhost:3000/` → 200 (landing page renders)
- [x] `GET localhost:3000/login` → 200
- [x] `GET localhost:3000/signup` → 200
- [x] `GET localhost:3000/dashboard` (no auth) → 302
- [x] `POST /api/v1/onboarding/import-cv` (no auth) → 401
- [x] Groq API (llama-3.3-70b-versatile) — reachable via CVStructurer
- [x] CV pipeline unit test (localhost PDF → extract 1,472 chars → Groq LLM → CVData in 1.28s)
- [ ] End-to-end CV import via HTTP with authenticated user (blocked: Supabase project paused)

---

## [phase/2-profiles] — 2026-03-01

### Added
- `backend/internal/service/interfaces.go` — all service interfaces (ISP: EmbeddingProvider, NLSearchParser, CVStructurer, MatchExplainer, IkigaiSummariser, MatchService)
- `backend/internal/service/embedding/text.go` — `BuildEmbeddingText()` single source of truth for embedding input
- `backend/internal/service/embedding/ollama.go` — Ollama embedding provider (nomic-embed-text, 5s timeout)
- `backend/internal/service/embedding/nomic.go` — Nomic Embed API provider (nomic-embed-text-v1.5, 5s timeout)
- `backend/internal/service/llm/ikigai.go` — `IkigaiSummariser` via Groq llama-3.1-8b-instant (JSON mode)
- `backend/internal/handler/users.go` — `GET /api/v1/users/me`, `GET /api/v1/users/:id` (block + visibility enforcement)
- `backend/internal/handler/profiles.go` — `PATCH /api/v1/profiles/me`, `PATCH /api/v1/profiles/me/visibility`; async embedding goroutine with WaitGroup + panic recovery
- `backend/internal/handler/onboarding.go` — `POST /api/v1/onboarding/ikigai`, `POST /api/v1/onboarding/complete`; async AI summary + embedding
- `backend/db/queries/users.sql` + `profiles.sql` — sqlc query files
- `backend/main.go` — wired embedding provider, LLM services, WaitGroup, all Phase 2 routes
- `frontend/server/types.ts` — Hono `Variables` type for typed `c.get("session")`
- `frontend/server/routes/onboarding.ts` — 5-step onboarding HTMX route handlers (GET + POST each step)
- `frontend/server/routes/pages.ts` — profile view routes (`/profile/me`, `/profile/:id`)
- `frontend/server/templates/pages/onboarding/step2-ikigai.eta` — 4 Ikigai question form
- `frontend/server/templates/pages/onboarding/step3-skills.eta` — tag-input skills & interests (Alpine.js)
- `frontend/server/templates/pages/onboarding/step4-intent.eta` — intent checkboxes, availability, working style
- `frontend/server/templates/pages/onboarding/step5-links.eta` — social links (optional), completes onboarding
- `frontend/server/templates/pages/profile.eta` — profile view page (own + others)

### Test criteria passed
- [x] `go build ./...` — no errors
- [x] `bun typecheck` — no errors
- [x] Groq API (llama-3.1-8b-instant) — reachable, JSON mode response confirmed
- [x] Ollama `nomic-embed-text` — model pulled, 768-dim embeddings verified
- [x] `GET /api/v1/users/me` (no auth) → 401 UNAUTHORIZED
- [x] `PATCH /api/v1/profiles/me` (no auth) → 401 UNAUTHORIZED
- [x] `POST /api/v1/onboarding/ikigai` (no auth) → 401 UNAUTHORIZED
- [ ] Complete 5 onboarding steps → `profile.onboarding_complete = true` (requires signed-in user)
- [x] Ollama embedding unit test — 768-dim vector in 1.33s (nomic-embed-text confirmed)
- [ ] Complete 5 onboarding steps → `profile.onboarding_complete = true` (blocked: Supabase project paused)
- [ ] `profile.embedding_status` → `current` within 5s of profile update (blocked: Supabase project paused)

---

## [phase/1-auth] — 2026-03-01

### Added
- `backend/internal/handler/auth.go` — `POST /api/v1/auth/ws-token` (HMAC-SHA256, 60s TTL, single-use via `sync.Map`); background token purger goroutine
- `backend/internal/handler/swagger.go` — CDN-based Swagger UI at `/swagger`; spec served at `/swagger/doc.json`
- `backend/docs/docs.go` — stub swagger spec (regenerated by `make swagger`)
- `backend/main.go` — wired JWT auth middleware on `/api/v1/*`, auth handler, swagger routes
- `frontend/server/middleware/session.ts` — `getSession()`, `refreshSession()` with mutex guard, `setSessionCookies()`, `clearSessionCookies()`
- `frontend/server/middleware/auth.ts` — `requireAuth` Hono middleware; silent refresh on expired access token
- `frontend/server/routes/auth.ts` — `POST /auth/login`, `POST /auth/signup`, `GET /auth/logout`, `GET /auth/confirm`
- `frontend/server/routes/pages.ts` — page routes with `requireAuth` + onboarding guard
- `frontend/server/index.ts` — registered auth + page routes
- `frontend/server/templates/layouts/base.eta` — base HTML layout (Bootstrap 5 CSS, HTMX, Alpine.js, morph plugin via CDN)
- `frontend/server/templates/pages/landing.eta` — public landing page
- `frontend/server/templates/pages/login.eta` — login form (HTMX POST)
- `frontend/server/templates/pages/signup.eta` — signup form (HTMX POST)
- `frontend/server/templates/pages/dashboard.eta` — authenticated dashboard shell
- `frontend/server/templates/pages/onboarding/step1-basic.eta` — onboarding step 1 placeholder

### Test criteria passed
- [x] `go build ./...` — no errors
- [x] `bun typecheck` — no errors
- [x] `GET /dashboard` (no session cookie) → 302 redirect to /login
- [x] `POST /api/v1/auth/ws-token` (no auth) → 401 UNAUTHORIZED
- [x] `GET /swagger/` → 200; `GET /swagger/doc.json` → 200 valid JSON spec
- [ ] `POST /auth/login` (valid creds) → HX-Redirect to /dashboard (blocked: Supabase project paused)
- [ ] `POST /api/v1/auth/ws-token` (valid JWT) → returns `{token, expires_at}` (blocked: Supabase project paused)

---

## [phase/0-foundation] — 2026-03-01

### Added
- `backend/go.mod` + `backend/go.sum` — Go module (Go 1.24, Fiber v3, pgx v5, jwx v2, godotenv)
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
- [x] `curl localhost:3001/readyz` → `{"status":"ready"}` (DB connection verified)
- [x] `curl localhost:3000/healthz` → `{"status":"ok"}`
- [ ] `docker build backend/` → pending (no Docker daemon in dev env)
- [ ] `docker build frontend/` → pending (no Docker daemon in dev env)
- [ ] `make migrate-up` → pending (requires running migrations against Supabase)

---

<!-- New phase entries added above this line on tag -->
