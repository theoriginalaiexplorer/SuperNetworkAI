# SuperNetworkAI — Build Progress

> Updated after each phase completion. Current phase is marked 🔄. Completed phases ✅. Not started ⬜.

---

## Phase Status

| # | Phase | Status | Tag | Date |
|---|---|---|---|---|
| 0 | Foundation | ✅ Complete | `phase/0-foundation` | 2026-03-01 |
| 1 | Authentication | ✅ Complete | `phase/1-auth` | 2026-03-01 |
| 2 | Onboarding & Profiles | ⬜ Not Started | `phase/2-profiles` | — |
| 3 | CV Import | ⬜ Not Started | `phase/3-cv-import` | — |
| 4 | Match Discovery | ⬜ Not Started | `phase/4-matches` | — |
| 5 | AI Search & Explanations | ⬜ Not Started | `phase/5-ai-search` | — |
| 6 | Connections | ⬜ Not Started | `phase/6-connections` | — |
| 7 | Real-Time Messaging | ⬜ Not Started | `phase/7-messaging` | — |
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
**Blocked by**: Phase 1

---

## How to Update This File

After each phase, update:
1. Phase Status table — change status + add date
2. Add a new phase section with checklist + test results + notes
3. Update CHANGELOG.md with what was added/changed
4. Run: `git tag -a phase/N-name -m "Phase N: <one line description>"`
