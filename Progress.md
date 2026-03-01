# SuperNetworkAI — Build Progress

> Updated after each phase completion. Current phase is marked 🔄. Completed phases ✅. Not started ⬜.

---

## Phase Status

| # | Phase | Status | Tag | Date |
|---|---|---|---|---|
| 0 | Foundation | ✅ Complete | `phase/0-foundation` | 2026-03-01 |
| 1 | Authentication | ✅ Complete | `phase/1-auth` | 2026-03-01 |
| 2 | Onboarding & Profiles | ✅ Complete | `phase/2-profiles` | 2026-03-01 |
| 3 | CV Import | ✅ Complete | `phase/3-cv-import` | 2026-03-01 |
| 4 | Match Discovery | ✅ Complete | `phase/4-matches` | 2026-03-01 |
| 5 | AI Search & Explanations | ✅ Complete | `phase/5-ai-search` | 2026-03-01 |
| 6 | Connections | ✅ Complete | `phase/6-connections` | 2026-03-01 |
| 7 | Real-Time Messaging | ✅ Complete | `phase/7-messaging` | 2026-03-01 |
| — | **Infra: Firebase + Neon** | ✅ Complete | `infra/firebase-neon` | 2026-03-01 |
| 8 | Privacy & Safety | ⬜ Not Started | `phase/8-privacy` | — |
| 9 | Polish & Hardening | ⬜ Not Started | `phase/9-polish` | — |
| 10 | GCP Deployment | ⬜ Not Started | `phase/10-deploy` | — |

---

## Phase 0 — Foundation

**Goal**: Both servers start and respond to health checks. DB schema created. Docker builds.

**Completed**: 2026-03-01

### Checklist
- [x] PLAN.md — finalized (v4)
- [x] CHANGELOG.md — created
- [x] Progress.md — created (this file)
- [x] backend/go.mod + main.go
- [x] backend/internal/config/config.go
- [x] backend/internal/model/errors.go + request.go + response.go
- [x] backend/internal/health/handler.go
- [x] backend/internal/db/client.go
- [x] backend/internal/middleware/ (auth.go, recovery.go, cors.go, logger.go)
- [x] backend/db/migrations/ (9 up + 9 down files)
- [x] backend/Makefile + .air.toml + sqlc.yaml + Dockerfile + .env.example
- [x] frontend/package.json + tsconfig.json + vite.config.ts
- [x] frontend/server/index.ts (Hono + /healthz)
- [x] frontend/server/lib/eta.ts + render.ts + api.ts
- [x] frontend/client/main.ts
- [x] frontend/.env.example + Dockerfile
- [x] CLAUDE.md
- [x] .gitignore + docker-compose.yml

### Test Results
```
go build ./...               →  [x] pass (no errors)
bun typecheck                →  [x] pass (no errors)
curl localhost:3001/healthz  →  [ ] pending (need .env)
curl localhost:3001/readyz   →  [ ] pending (need .env)
curl localhost:3000/healthz  →  [ ] pending (need .env)
docker build backend/        →  [ ] pending
docker build frontend/       →  [ ] pending
make migrate-up              →  [ ] pending (requires Supabase creds)
```

### Notes
- pgvector extension must be enabled manually in Supabase dashboard before running migrate-up
- Ollama: `ollama pull nomic-embed-text` after `docker compose up ollama`
- Go binary in distroless image; `go mod tidy` removed future-phase deps (added back as phases are implemented)

---

## Phase 1 — Authentication

**Goal**: Users can sign up, log in, see an empty dashboard, and log out. Session cookies work. Onboarding guard redirects incomplete users.

**Completed**: 2026-03-01

### Checklist
- [x] Go: `POST /api/v1/auth/ws-token` endpoint (HMAC-SHA256, 60s TTL, single-use)
- [x] Go: JWT auth middleware on all `/api/v1/*` routes (JWKS 30min cache)
- [x] Go: Swagger UI at `/swagger` + spec at `/swagger/doc.json`
- [x] BFF: `server/middleware/session.ts` — cookie read/write + mutex refresh guard
- [x] BFF: `server/middleware/auth.ts` — requireAuth + silent refresh
- [x] BFF: `server/routes/auth.ts` — login, logout, signup, confirm
- [x] BFF: `server/routes/pages.ts` — page routes with onboarding guard
- [x] UI: Base layout (Bootstrap 5 CSS, HTMX, Alpine + morph CDN)
- [x] UI: Landing, login, signup, dashboard pages
- [x] UI: Onboarding step 1 placeholder template

### Test Results
```
go build ./...               →  [x] pass
bun typecheck                →  [x] pass
POST /auth/login             →  [ ] pending (requires live Supabase)
GET /dashboard (no cookie)   →  [ ] pending (requires live server)
POST /api/v1/auth/ws-token   →  [ ] pending (requires live server)
```

---

## Phase 2 — Onboarding & Profiles

**Completed**: 2026-03-01

### Checklist
- [x] Go: `service/interfaces.go` — all service interfaces
- [x] Go: `service/embedding/` — provider interface, Ollama impl, Nomic impl, `BuildEmbeddingText()`
- [x] Go: `service/llm/ikigai.go` — IkigaiSummariser via Groq
- [x] Go: `handler/users.go` — GET /api/v1/users/me + /users/:id (visibility + block)
- [x] Go: `handler/profiles.go` — PATCH /api/v1/profiles/me + visibility; async embedding
- [x] Go: `handler/onboarding.go` — POST /api/v1/onboarding/ikigai + complete
- [x] Go: sqlc query files for users + profiles
- [x] BFF: `server/types.ts` — Hono typed context variables
- [x] BFF: `server/routes/onboarding.ts` — 5-step onboarding handlers
- [x] BFF: `server/routes/pages.ts` — profile view routes
- [x] UI: Onboarding steps 2–5 templates
- [x] UI: Profile view template

### Test Results
```
go build ./...                    →  [x] pass
bun typecheck                     →  [x] pass
5-step onboarding completion      →  [ ] pending (requires live server)
embedding_status → current        →  [ ] pending (requires Ollama)
```

---

## Phase 3 — CV Import

**Goal**: Users can paste a PDF URL to auto-fill social links during onboarding step 5. Backend downloads the PDF, extracts text, calls Groq LLM, and returns structured profile fields.

**Completed**: 2026-03-01

### Checklist
- [x] Go: `service/downloader.go` — SSRF-safe PDF downloader (allowlist, 10s timeout, 5MB cap)
- [x] Go: `service/pdf.go` — PDF text extraction via `ledongthuc/pdf`
- [x] Go: `service/llm/cv.go` — CVStructurer via Groq `llama-3.3-70b-versatile` (JSON mode)
- [x] Go: `handler/files.go` — Presign stub (Uploadthing Phase 3+)
- [x] Go: `handler/onboarding.go` — `ImportCV` method added
- [x] Go: `main.go` — `POST /api/v1/onboarding/import-cv` wired
- [x] BFF: `routes/onboarding.ts` — `POST /onboarding/import-cv` proxy route
- [x] UI: step5-links.eta — CV import card with Alpine.js pre-fill
- [x] Fix: Eta layout paths corrected (`../layouts/base`, `../../layouts/base`)
- [x] Fix: pdfcpu swapped for `ledongthuc/pdf` (correct API for text extraction)

### Test Results
```
go build ./...                            →  [x] pass
bun typecheck                             →  [x] pass
POST /api/v1/onboarding/import-cv (401)  →  [x] pass (auth guard working)
GET  localhost:3001/healthz               →  [x] pass
GET  localhost:3001/readyz                →  [x] pass
GET  localhost:3000/healthz               →  [x] pass
GET  localhost:3000/ (landing)            →  [x] pass — 200
GET  localhost:3000/login                 →  [x] pass — 200
GET  localhost:3000/dashboard (no auth)  →  [x] pass — 302 redirect
CV import with real PDF URL              →  [ ] pending (requires auth token)
```

---

## Phase 6 — Connections

**Goal**: Users can send, accept, and reject connection requests. Profile pages show correct action button.

**Completed**: 2026-03-01

### Checklist
- [x] Go: `handler/connections.go` — POST /api/v1/connections (409 on duplicate), GET /api/v1/connections?status=, PATCH /api/v1/connections/:id (recipient-only), GET /api/v1/connections/status/:userId
- [x] Go: main.go — all 4 connection routes wired
- [x] BFF: `routes/connections.ts` — page, HTMX list partial, request/accept/reject
- [x] UI: `pages/connections.eta` — accepted/pending tab switcher (HTMX)
- [x] UI: `partials/connection-list.eta` — direction-aware action buttons
- [x] UI: `pages/profile.eta` — Connect/Pending/Accept/Message button based on connection status
- [x] UI: `partials/match-card.eta` — Connect button with HTMX "Pending" swap
- [x] BFF: `routes/pages.ts` — profile page fetches connection status in parallel

### Test Results
```
go build ./...                                         →  [x] pass
go vet ./...                                           →  [x] pass
bun typecheck                                          →  [x] pass
POST /api/v1/connections (send request)                →  [ ] pending (requires live server)
POST /api/v1/connections (duplicate)                   →  [ ] pending (409 CONFLICT check)
PATCH /api/v1/connections/:id (accept)                 →  [ ] pending (requires auth)
PATCH /api/v1/connections/:id (non-recipient attempt)  →  [ ] pending (403 check)
GET  /api/v1/connections/status/:userId                →  [ ] pending (requires auth)
/connections page (accepted/pending tabs)              →  [ ] pending (requires local Supabase)
Profile page Connect button states                     →  [ ] pending (requires data)
```

---

## Phase 5 — AI Search & Explanations

**Goal**: Natural language search returns relevant results. On-demand AI explanations generated for matches.

**Completed**: 2026-03-01

### Checklist
- [x] Go: `service/llm/search.go` — NLSearchParser (llama-3.1-8b-instant, JSON mode, 5s timeout)
- [x] Go: `service/llm/explain.go` — MatchExplainer (llama-3.3-70b-versatile, JSON mode, 10s timeout)
- [x] Go: `handler/search.go` — POST /api/v1/search full flow (NL → embed → pgvector → results)
- [x] Go: `handler/matches.go` — GET /api/v1/matches/:matchedUserId/explanation with cache-first logic
- [x] Go: `service/matching.go` — GetExplanation: profile fetch + Groq call on cache miss + persist
- [x] BFF: `routes/discover.ts` — POST /discover/search (NL search HTMX endpoint)
- [x] BFF: `routes/discover.ts` — GET /discover/matches/:id/explanation (explanation lazy-load)
- [x] UI: `pages/discover.eta` — NL search bar with 500ms HTMX debounce + spinner
- [x] UI: `partials/search-results.eta` — search result cards with score, skills, profile link
- [x] UI: `partials/match-card.eta` — "Why this match?" button (HTMX once, inline text)

### Test Results
```
go build ./...                                        →  [x] pass
go vet ./...                                          →  [x] pass
bun typecheck                                         →  [x] pass
POST /api/v1/search (no auth)                         →  [ ] pending (requires live server)
GET  /api/v1/matches/:id/explanation (first call)     →  [ ] pending (requires auth + data)
GET  /api/v1/matches/:id/explanation (second call)    →  [ ] pending (requires auth + data)
/discover search bar → results grid                   →  [ ] pending (requires local Supabase)
"Why this match?" button → inline explanation          →  [ ] pending (requires data)
```

---

## Phase 4 — Match Discovery

**Goal**: Two users with different profiles see each other on the Discover page ranked by similarity.

**Completed**: 2026-03-01

### Checklist
- [x] Go: `service/matching.go` — MatchService, cache computation, category algorithm (§9.3)
- [x] Go: `GET /api/v1/matches`, `POST /api/v1/matches/:matchedUserId/dismiss`
- [x] Go: `POST /internal/matches/refresh` (X-Internal-Secret secured)
- [x] Go: `middleware/internal.go` — RequireInternal middleware
- [x] Go: Match cache triggered after embedding completes (profiles + onboarding handlers)
- [x] BFF: `routes/discover.ts` — GET /discover, GET /discover/matches (HTMX), POST /discover/matches/:id/dismiss
- [x] UI: `pages/discover.eta` — discover page with category tabs + HTMX match grid
- [x] UI: `partials/match-card.eta` — match card with dismiss button
- [x] UI: `partials/match-list.eta` — match list partial with empty state + load-more

### Test Results
```
go build ./...                               →  [x] pass
go vet ./...                                 →  [x] pass
bun typecheck                                →  [x] pass
GET  /api/v1/matches (no auth)               →  [ ] pending (requires live server)
POST /api/v1/matches/:id/dismiss (no auth)   →  [ ] pending (requires live server)
POST /internal/matches/refresh (no secret)   →  [ ] pending (requires live server)
GET  /discover (authenticated)               →  [ ] pending (requires local Supabase)
Category filter cofounder tab                →  [ ] pending (requires DB data)
Dismiss button removes card from UI          →  [ ] pending (requires DB data)
```

---

## Phase 7 — Real-Time Messaging

**Goal**: Accepted connections can exchange real-time messages. History persists. Two tabs see each other's messages live.

**Completed**: 2026-03-01

### Checklist
- [x] Go: `internal/ws/hub.go` — Hub (rooms map + RWMutex, per-connection write mutex, validateToken injection, Stop())
- [x] Go: `hub.Upgrade()` — FastHTTP upgrader; first-message auth (10s deadline), join_room, message with all guards
- [x] Go: `handler/conversations.go` — POST/GET /api/v1/conversations, GET /api/v1/conversations/:id/messages, PATCH /:id/read
- [x] Go: `main.go` — wired convH, hub, GET /ws route, hub.Stop() in shutdown
- [x] Go: `go get github.com/fasthttp/websocket` — WS library (Fiber v3 compatible via FastHTTPUpgrader)
- [x] BFF: `routes/messages.ts` — GET /messages, GET /messages/new?userId=, GET /messages/:convId, GET /messages/partials/ws-token
- [x] BFF: `routes/pages.ts` — mounted messageRoutes at /messages
- [x] UI: `pages/messages.eta` — conversation sidebar + chat area + Alpine chat store
- [x] UI: Alpine chat store — connect/auth/join/send/receive/reconnect with exponential backoff

### Test Results
```
go build ./...                                              →  [x] pass
go vet ./...                                                →  [x] pass
bun typecheck                                               →  [x] pass
GET  /ws (no auth token in first message)                   →  [ ] pending (requires live server)
POST /api/v1/conversations (no accepted connection)         →  [ ] pending (403 check)
POST /api/v1/conversations (accepted connection)            →  [ ] pending (creates conv)
GET  /api/v1/conversations/:id/messages (cursor)            →  [ ] pending (requires data)
WS: auth → join_room → message → new_message broadcast      →  [ ] pending (requires two sessions)
WS message > 4000 chars                                     →  [ ] pending (error response check)
/messages page (two tabs, live message delivery)            →  [ ] pending (requires local Supabase)
Reconnect with fresh WS token after disconnect              →  [ ] pending (requires live server)
```

### Notes
- `gofiber/contrib/websocket` v1.3.4 targets Fiber v2 (incompatible); used `github.com/fasthttp/websocket` directly with `FastHTTPUpgrader` + Fiber v3's `c.RequestCtx()`
- WS auth is first-message (`{type:"auth",token}`) not header-based, keeping `/ws` outside JWT middleware
- BFF derives WS URL from `GO_API_URL` by replacing `http:` → `ws:` / `https:` → `wss:`
- `window.__CHAT__` pattern avoids double-escaping of JSON in Eta templates

---

---

## Infra Migration — Firebase Auth + Neon PostgreSQL

**Date**: 2026-03-01 | **Trigger**: Supabase India outage (Auth + DB both down)

### What Changed
- **Auth**: Supabase Auth → Firebase Auth (Email/Password). BFF signs HS256 JWT (`BFF_JWT_SECRET`) after Firebase credential exchange; Go API validates HS256 instead of RS256/JWKS.
- **Database**: Supabase PostgreSQL → Neon.tech PostgreSQL (`aws-us-east-1`, pgvector supported). Zero schema changes — identical migrations applied.
- **No changes** to any handler, service, ws, or template files.

### Test Results
```
go build ./...                                    →  [x] pass
go vet ./...                                      →  [x] pass
bun typecheck                                     →  [x] pass
GET  /healthz                                     →  [x] pass — {"status":"ok"}
GET  /readyz (Neon)                               →  [x] pass — {"status":"ready"}
GET  /api/v1/users/me (no token)                  →  [x] pass — 401 UNAUTHORIZED
POST /internal/matches/refresh (no secret)        →  [x] pass — 401 UNAUTHORIZED
GET  /api/v1/users/me (valid HS256 JWT)           →  [x] pass — 404 (auth accepted, user not in DB)
GET  localhost:3000/ (landing)                    →  [x] pass — 200
GET  localhost:3000/login                         →  [x] pass — 200
GET  localhost:3000/dashboard (no cookie)         →  [x] pass — 302 → /login
TestOllamaEmbedding                               →  [x] pass — 768-dim (2.31s)
TestCVPipeline                                    →  [x] pass — download+extract+LLM (0.48s)
```

### Post-emergency TODOs
- [ ] Add `auth_mapping` table (firebase_uid → stable UUID) for multi-device consistency
- [ ] Decide: keep Firebase + Neon permanently, or revert to Supabase when recovered

---

## How to Update This File

After each phase, update:
1. Phase Status table — change status + add date
2. Add a new phase section with checklist + test results + notes
3. Update CHANGELOG.md with what was added/changed
4. Run: `git tag -a phase/N-name -m "Phase N: <one line description>"`
