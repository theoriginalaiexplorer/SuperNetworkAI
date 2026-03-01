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
- [x] `GET /ws` → `{type:"auth_ok"}` after sending valid WS token as first message
- [x] `POST /api/v1/conversations` without accepted connection → 403 FORBIDDEN
- [x] `POST /api/v1/conversations` with accepted connection → 200 + conversation_id
- [x] WS: auth → join_room → `{type:"new_message"}` broadcast with persisted ID
- [x] WS message >4000 chars → `{type:"error","message":"content must be 1–4000 characters"}`
- [x] `GET /conversations/:id/messages` → persisted message returned
- [x] `PATCH /conversations/:id/read` → 204
- [ ] Reconnect with fresh WS token after disconnect → blocked (requires multi-session simulation)
- [ ] Two separate browser sessions exchanging messages live → blocked (manual test)

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
- [x] `POST /api/v1/connections` (A→B) → 201 pending
- [x] `PATCH /connections/:id` by non-recipient → 403 FORBIDDEN
- [x] `PATCH /connections/:id` by recipient (Bob accepts) → 200 `{"status":"accepted"}`
- [x] `GET /connections/status/:userId` → `{status, direction, connection_id}`
- [x] `GET /connections?status=accepted` → returns accepted connections list
- [x] Duplicate connection request → 409 CONFLICT
- [ ] User B rejects → status = 'rejected' (rejected path not independently tested; accept path verified)
- [ ] Profile page Connect button states → blocked (requires browser session)

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
- [x] `POST /api/v1/search {"query":"React developer looking for cofounder"}` → 1 result (Bob, score 0.52)
- [x] `GET /api/v1/matches/:id/explanation` first call → Groq generates explanation + caches in match_cache
- [x] Second call → identical response returned from cache (no new Groq call)
- [x] Low-relevance query → API returns low-score results; UI empty state is a frontend concern

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
- [x] `POST /internal/matches/refresh` → `{"queued":2}` — refreshed Alice and Bob
- [x] `GET /api/v1/matches` → Bob in Alice's results, score 0.755, categories `[cofounder, client]`
- [x] `GET /api/v1/matches?category=cofounder` → filtered results (Bob has cofounder intent)
- [x] `POST /api/v1/matches/:id/dismiss` → `{"status":"dismissed"}` 200
- [x] Fixed bug: `match_cache` has no `updated_at` column — all SQL references corrected to `computed_at`
- [ ] End-to-end Discover page with live browser session → blocked (manual test)

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
- [x] CV pipeline unit test (localhost PDF → extract chars → Groq LLM → CVData)
- [x] `POST /api/v1/onboarding/import-cv` (no auth) → 401
- [x] `POST /api/v1/onboarding/import-cv` (authenticated, localhost PDF) → 200 + structured CVData (bio, skills, interests parsed by Groq)

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
- [x] Ollama embedding unit test — 768-dim vector confirmed (nomic-embed-text)
- [x] `GET /api/v1/users/me` → returns full user + profile + ikigai
- [x] `GET /api/v1/users/:id` → returns public profile snapshot
- [x] `PATCH /api/v1/profiles/me` → 200; async embedding goroutine triggers
- [x] `profile.embedding_status` → `current` within 10s of profile PATCH (Ollama confirmed)
- [ ] Complete 5 onboarding steps end-to-end → blocked (requires browser + Firebase login)

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
- [x] `POST /api/v1/auth/ws-token` (valid HS256 BFF JWT) → `{token, expires_at}` — HMAC token generated
- [ ] `POST /auth/login` (Firebase email/password) → HX-Redirect to /dashboard — blocked (requires browser + Firebase project)

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
- [x] `migrate-up` (9 migrations applied to Neon via `go run ./cmd/migrate/`; pgvector extension enabled)
- [ ] `docker build backend/` → blocked (no Docker daemon in dev env)
- [ ] `docker build frontend/` → blocked (no Docker daemon in dev env)

---

<!-- New phase entries added above this line on tag -->
