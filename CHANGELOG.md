# Changelog

All notable changes are documented here per phase.
Each phase is tagged in git as `phase/N-name` on completion.

Format: each entry lists what was added, what was changed, and what the phase tag is.

---

## [Unreleased]

---

## [infra/firebase-neon] — 2026-03-01

> Emergency infrastructure swap during Supabase India outage.

### Changed
- **Auth provider**: Supabase Auth → **Firebase Auth** (Email/Password, global CDN)
  - `frontend/server/routes/auth.ts` — full rewrite: `signInWithEmailAndPassword`, `createUserWithEmailAndPassword`, `sendEmailVerification`, `applyActionCode`; BFF issues its own HS256 JWT (`sub` = UUID, 1h TTL) after Firebase credential exchange
  - `frontend/server/middleware/session.ts` — replaced Supabase `refreshSession` with Firebase REST token endpoint (`securetoken.googleapis.com/v1/token`); re-signs BFF JWT using `decodeJwt` (expiry-tolerant UUID extraction) + `jose` `SignJWT`
- **JWT validation**: RS256 + JWKS (Supabase) → **HS256 shared secret** (BFF-signed)
  - `backend/internal/middleware/auth.go` — removed JWKS cache (`jwksCache`, `getJWKS`); replaced with `jwt.WithKey(jwa.HS256, bffSecret)` (single-pass, no network call)
  - `backend/internal/config/config.go` — removed `SupabaseURL`, `SupabaseKey`; added `BffJWTSecret` (required, validated at startup)
  - `backend/main.go` — removed `jwksURL := fmt.Sprintf(...)`; passes `[]byte(cfg.BffJWTSecret)` to `RequireAuth`
- **Database**: Supabase PostgreSQL → **Neon.tech PostgreSQL** (`aws-us-east-1`, pgvector supported)
  - `backend/.env` — `DATABASE_URL` updated to Neon pooled connection string; `BFF_JWT_SECRET` added
  - `frontend/package.json` — removed `@supabase/supabase-js`; added `firebase@^11`, `jose@^5`

### Test criteria passed
- [x] `go build ./...` — no errors
- [x] `go vet ./...` — no warnings
- [x] `bun typecheck` — no errors
- [x] `GET /healthz` → `{"status":"ok"}`
- [x] `GET /readyz` (Neon DB) → `{"status":"ready"}`
- [x] `GET /api/v1/users/me` (no token) → 401 UNAUTHORIZED
- [x] `POST /internal/matches/refresh` (no secret) → 401 UNAUTHORIZED
- [x] `GET /api/v1/users/me` (valid HS256 BFF JWT) → 404 NOT_FOUND (middleware accepts token; user not yet in Neon)
- [x] `GET localhost:3000/` → 200
- [x] `GET localhost:3000/login` → 200
- [x] `GET localhost:3000/dashboard` (no cookie) → 302 → /login
- [x] `TestOllamaEmbedding` — 768-dim vector confirmed (2.31s)
- [x] `TestCVPipeline` — download → extract → Groq LLM → CVData parsed (0.48s)

### Post-emergency follow-up (when Supabase recovers or as production hardening)
- Add `auth_mapping (firebase_uid TEXT PK, user_uuid UUID DEFAULT gen_random_uuid())` Neon table to stabilise UUID-per-Firebase-UID across devices
- Optionally keep Neon as primary DB (no schema changes required — identical migrations)
- Optionally keep Firebase Auth (avoids Supabase vendor lock-in)

---

## [phase/7-messaging] — 2026-03-01

### Added
- `backend/internal/ws/hub.go` — WebSocket Hub using `github.com/fasthttp/websocket` (Fiber v3 compatible via `FastHTTPUpgrader`); rooms map + `sync.RWMutex`; per-connection write mutex; first-message auth (10s deadline); join_room/leave_room/message dispatch; all messaging guards (membership, accepted connection, blocks); `hub.Stop()` for graceful shutdown
- `backend/internal/handler/conversations.go` — `POST /api/v1/conversations` (idempotent: returns existing or creates new, requires accepted connection); `GET /api/v1/conversations` (with last-message preview + unread count); `GET /api/v1/conversations/:id/messages` (cursor pagination: `?before=`, `?after=`, default newest-50); `PATCH /api/v1/conversations/:id/read`
- `frontend/server/routes/messages.ts` — `GET /messages`, `GET /messages/new?userId=`, `GET /messages/:convId`, `GET /messages/partials/ws-token` (fresh WS token for reconnect)
- `frontend/server/templates/pages/messages.eta` — conversation list sidebar + chat area; Alpine chat store with connect/auth/join/send/receive/reconnect (exponential backoff: 1s→2s→4s→…→30s); `window.__CHAT__` pattern for safe server-to-Alpine data handoff

### Changed
- `backend/go.mod` — added `github.com/fasthttp/websocket v1.5.12` as direct dependency (removed unused `gofiber/contrib/websocket` which targets Fiber v2)
- `backend/main.go` — wired `convH`, `hub`; added conversation routes; added `GET /ws` WebSocket route using `c.RequestCtx()` (Fiber v3 API); added `hub.Stop()` before server shutdown
- `frontend/server/routes/pages.ts` — mounts `messageRoutes` at `/messages`

### Test criteria passed
- [x] `go build ./...` — no errors
- [x] `go vet ./...` — no warnings
- [x] `bun typecheck` — no errors
- [ ] `GET /ws` → first message auth succeeds (blocked: requires live server)
- [ ] `POST /api/v1/conversations` without accepted connection → 403 (blocked: live server)
- [ ] Two users send messages in real time via WS (blocked: local Supabase)
- [ ] WS message >4000 chars → error response (blocked: live server)
- [ ] Reconnect after disconnect → fresh token fetched, messages resumed (blocked: live server)

---

## [phase/6-connections] — 2026-03-01

### Added
- `backend/internal/handler/connections.go` — `POST /api/v1/connections` (bidirectional duplicate check → 409), `GET /api/v1/connections?status=`, `PATCH /api/v1/connections/:id` (recipient-only accept/reject), `GET /api/v1/connections/status/:userId`
- `frontend/server/routes/connections.ts` — full connections BFF: page, HTMX list partial, request/accept/reject actions
- `frontend/server/templates/pages/connections.eta` — connections page with accepted/pending tab switcher (HTMX, no Bootstrap JS)
- `frontend/server/templates/partials/connection-list.eta` — connection cards with direction-aware action buttons (Accept/Reject for received, "Request sent" badge for sent)

### Changed
- `backend/main.go` — wired `connH`; added `/connections`, `/connections/status/:userId`, `/connections/:id` routes
- `frontend/server/routes/pages.ts` — profile page now fetches connection status in parallel with profile data; passes `connectionStatus`, `connectionId`, `connectionDirection` to template; mounts `connectionRoutes` at `/connections`
- `frontend/server/templates/pages/profile.eta` — shows Connect / Pending / Accept request / Message button depending on connection status
- `frontend/server/templates/partials/match-card.eta` — added "Connect" button with HTMX swap to "Pending…" on click
- `frontend/server/templates/pages/dashboard.eta` — reordered nav: Discover → Connections → Messages

### Test criteria passed
- [x] `go build ./...` — no errors
- [x] `go vet ./...` — no warnings
- [x] `bun typecheck` — no errors
- [ ] User A sends connection to User B → User B sees pending request (blocked: local Supabase)
- [ ] User B accepts → status = 'accepted' for both (blocked: requires auth + data)
- [ ] User B rejects → status = 'rejected' (blocked: requires auth + data)
- [ ] `GET /connections/status/:userId` → correct status (blocked: requires auth)
- [ ] Profile page shows correct button per connection state (blocked: requires auth)
- [ ] Duplicate connection request → 409 CONFLICT (blocked: requires auth)

---

## [phase/5-ai-search] — 2026-03-01

### Added
- `backend/internal/service/llm/search.go` — `NLSearchParser.ParseSearchQuery()`: Groq `llama-3.1-8b-instant` (JSON mode, 5s timeout) parses free-text query into `{ embedding_text, intent_filter, availability_filter }`
- `backend/internal/service/llm/explain.go` — `MatchExplainer.ExplainMatch()`: Groq `llama-3.3-70b-versatile` (JSON mode, 10s timeout) generates 2-3 sentence match explanation from two profile snapshots
- `backend/internal/handler/search.go` — `POST /api/v1/search`: NL parse → embed → pgvector search (visibility + block guards) → ranked results
- `backend/internal/handler/matches.go` — `GET /api/v1/matches/:matchedUserId/explanation`: check match_cache; call Groq on first request; cache result; return on subsequent calls
- `frontend/server/routes/discover.ts` — `GET /discover/matches/:id/explanation` (HTMX lazy-load), `POST /discover/search` (NL search HTMX endpoint; empty query restores match list)
- `frontend/server/templates/partials/search-results.eta` — search result cards grid with avatar, score, intent badges, skills, profile link
- `frontend/server/templates/pages/discover.eta` — NL search bar with 500ms HTMX debounce + `htmx-indicator` spinner
- `frontend/server/templates/partials/match-card.eta` — "Why this match?" lazy-load button (HTMX `hx-trigger="click once"`, inline explanation text)

### Changed
- `backend/internal/service/matching.go` — `matchService` gains `explainer MatchExplainer` field; `GetExplanation` now fetches both profiles and calls Groq on cache miss, then persists result
- `backend/main.go` — wired `nlSearchParser`, `matchExplainer`, `searchH`; `NewMatchService` now accepts explainer; added `GET /api/v1/matches/:id/explanation` and `POST /api/v1/search` routes

### Test criteria passed
- [x] `go build ./...` — no errors
- [x] `go vet ./...` — no warnings
- [x] `bun typecheck` — no errors
- [ ] `POST /api/v1/search {"query":"React dev"}` → ranked profiles (blocked: local Supabase setup pending)
- [ ] `GET /api/v1/matches/:id/explanation` first call → Groq generates + caches (blocked: requires auth + data)
- [ ] Second call to explanation → returns cached (no new Groq call) (blocked: requires auth + data)
- [ ] Search with no results → "No results found" empty state

---

## [phase/4-matches] — 2026-03-01

### Added
- `backend/internal/service/matching.go` — `matchService`: `GetMatches` (pgvector cosine similarity with block exclusion + category filter), `DismissMatch`, `GetExplanation`, `RefreshCacheForUser` (upsert preserving dismissed state + explanation)
- `backend/internal/handler/matches.go` — `GET /api/v1/matches` (ranked, filtered, paginated) + `POST /api/v1/matches/:matchedUserId/dismiss`
- `backend/internal/handler/internal.go` — `POST /internal/matches/refresh`: batch-refreshes match_cache for all users with stale caches; secured by `X-Internal-Secret` header
- `backend/internal/middleware/internal.go` — `RequireInternal()` middleware for Cloud Scheduler routes
- `frontend/server/routes/discover.ts` — `GET /discover`, `GET /discover/matches` (HTMX partial), `POST /discover/matches/:id/dismiss`
- `frontend/server/templates/pages/discover.eta` — Discover page with Bootstrap category tabs and HTMX match grid
- `frontend/server/templates/partials/match-card.eta` — Match card: avatar, name, match %, categories, skills, View/Dismiss
- `frontend/server/templates/partials/match-list.eta` — Match list partial (HTMX swap target); empty state + load-more

### Changed
- `backend/internal/service/interfaces.go` — `MatchService` interface: added `DismissMatch`; `Match` struct expanded with profile snapshot fields (`DisplayName`, `Tagline`, `Skills`, `Intent`, `AvatarURL`)
- `backend/internal/handler/profiles.go` — `ProfileHandler` gains `matchSvc` field; triggers `RefreshCacheForUser` after embedding update
- `backend/internal/handler/onboarding.go` — `OnboardingHandler` gains `matchSvc` field; triggers `RefreshCacheForUser` after ikigai embedding
- `backend/main.go` — wired `matchSvc`, `matchH`, `internalH`; updated `profileH` + `onboardingH` constructors; added match + internal routes
- `frontend/server/routes/pages.ts` — mounted `discoverRoutes` at `/discover`

### Test criteria passed
- [x] `go build ./...` — no errors
- [x] `go vet ./...` — no warnings
- [x] `bun typecheck` — no errors
- [ ] End-to-end: User A visits /discover → User B in results (blocked: local Supabase setup pending)
- [ ] Category filter "cofounder" → only cofounder matches (blocked: requires DB data)
- [ ] `POST /api/v1/matches/:id/dismiss` → match disappears (blocked: requires auth)
- [ ] `POST /internal/matches/refresh` → recomputes cache (blocked: requires DB)

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
