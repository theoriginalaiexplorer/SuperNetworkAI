# SuperNetworkAI — Implementation Plan
> **Status**: Final pre-implementation plan. Read fully before writing any code.
> **Last updated**: 2026-03-01 (v4 — 22-issue review fixes applied)

---

## Table of Contents
1. [Problem Summary](#1-problem-summary)
2. [Architecture Overview](#2-architecture-overview)
3. [Technology Decisions](#3-technology-decisions)
4. [Software Engineering Principles](#4-software-engineering-principles)
5. [Project Structure](#5-project-structure)
6. [Database Schema](#6-database-schema)
7. [API Contract](#7-api-contract)
8. [Authentication & Session Design](#8-authentication--session-design)
9. [Key Feature Flows](#9-key-feature-flows)
10. [Error Handling Strategy](#10-error-handling-strategy)
11. [Resilience & Crash Safety](#11-resilience--crash-safety)
12. [Security Design](#12-security-design)
13. [Deployment Architecture](#13-deployment-architecture)
14. [Development Environment Setup](#14-development-environment-setup)
15. [Implementation Phases](#15-implementation-phases)
16. [Known Risks & Open Items](#16-known-risks--open-items)

---

## 1. Problem Summary

Online communities have members with rich context — skills, goals, passions — that never gets surfaced. Manual networking is slow and produces mismatched connections.

**SuperNetworkAI** uses **Ikigai** (self-discovery answers: what you love, what you're good at, what the world needs, what you can be paid for) as its core matching signal, combined with LLM-driven semantic search and AI-generated match explanations.

**Core capabilities:**
1. Onboarding capturing Ikigai, portfolio, social profiles, intent — importable from CV
2. AI pre-fills and updates matchmaking criteria (skills, interests)
3. Natural language search interface
4. Ranked match lists categorised as cofounder / client / teammate
5. AI-generated explanations for each match
6. In-app messaging (after accepted connection)
7. Public/private visibility and block controls

---

## 2. Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│  Browser                                                       │
│  ├── HTMX  (server-driven HTML partial swaps)                 │
│  ├── Alpine.js  (local UI state: modals, forms, WS client)    │
│  └── Bootstrap 5 CSS only  (no Bootstrap JS)                  │
└─────────────┬───────────────────────────┬─────────────────────┘
              │ HTTP → HTML pages/partials │ WebSocket (native)
              ▼                            ▼ (direct — NOT via BFF)
┌─────────────────────┐  Bearer JWT  ┌─────────────────────────┐
│  Bun BFF (port 3000) │ ───────────▶ │  Go API (port 3001)     │
│  Hono + Eta          │             │  Fiber v3               │
│  ├── Session mgmt    │             │  ├── REST (JSON only)   │
│  │  (HttpOnly cookie)│             │  ├── Swagger 2.0 UI     │
│  ├── SSR templates   │             │  ├── WS hub             │
│  └── Static assets   │             │  ├── JWT verification   │
│     (Vite build)     │             │  ├── pgx + sqlc         │
└─────────────────────┘             │  ├── Groq (LLM)         │
                                     │  ├── Ollama/Nomic (emb) │
                                     │  └── Uploadthing        │
                                     └────────────┬────────────┘
                        ┌───────────────────────┬─┘
                   ┌────▼──────┐  ┌─────────────▼──┐  ┌────────────┐
                   │ Supabase  │  │  Ollama (dev)   │  │  Groq API  │
                   │ PostgreSQL│  │  Nomic (prod)   │  │  (cloud)   │
                   │ +pgvector │  │  nomic-embed    │  │ llama-3.x  │
                   └───────────┘  └────────────────┘  └────────────┘
```

**Separation contract (NO ambiguity):**
- Browser → BFF: HTTP only (HTML full pages + HTMX partials). No JSON.
- Browser → Go API: Native WebSocket ONLY. No REST calls from browser.
- BFF → Go API: JSON REST over HTTP, `Authorization: Bearer <jwt>`.
- Go API → External: DB, Groq, Ollama/Nomic, Uploadthing.
- BFF → Supabase Auth: Only for login/logout/token-refresh (not Go API concern).

---

## 3. Technology Decisions

### 3.1 Go Backend: Fiber v3 (pinned v3.0.x)
- Express-like syntax, fastest Go HTTP benchmarks.
- WebSocket via `github.com/gofiber/contrib/websocket` (separate module — add explicitly).
- Panic recovery middleware registered at startup — catches all unhandled panics, returns 500, never crashes server.
- **Rejected**: Echo, Chi (fewer built-ins), Huma (immature).

### 3.2 BFF: Hono on Bun
- Bun-first, TypeScript-native, typed middleware, `Bun.serve()` compatible.
- Eta for server-side templates (async-capable — needed for `await api.call()` inside templates).
- **Rejected**: Elysia (Bun-only, less portable), raw `Bun.serve()` (too much boilerplate).

### 3.3 API Documentation: Swagger 2.0 (swaggo/swag)
- Generates Swagger 2.0 — NOT OpenAPI 3.0. Accepted for MVP (sufficient for docs + codegen).
- `github.com/gofiber/swagger` serves Swagger UI at `/swagger/*`.

### 3.4 Groq Models (specific, no ambiguity)
| Use case | Model | Why |
|---|---|---|
| NL search parsing | `llama-3.1-8b-instant` | Fast, simple structured extraction |
| CV structuring | `llama-3.3-70b-versatile` | Complex reasoning needed |
| Match explanation | `llama-3.3-70b-versatile` | Quality over speed (on-demand only) |
| Ikigai AI summary | `llama-3.1-8b-instant` | Simple summarisation |

- **All Groq calls MUST use `response_format: { type: "json_object" }`**. Without this, output may be markdown-wrapped JSON which breaks Go unmarshaling.
- **No official Groq Go SDK.** Use `github.com/sashabaranov/go-openai` with `BaseURL = "https://api.groq.com/openai/v1"`.
- **Mixtral is deprecated on Groq** — do not reference it.

### 3.5 Embeddings: Ollama (dev) → Nomic API (prod)
- `nomic-embed-text`, **768 dimensions, FIXED**. Changing dimension requires full re-embedding + schema migration.
- Abstracted via `EmbeddingProvider` interface — switch via `EMBEDDING_PROVIDER=ollama|nomic` env var.
- Prod uses Nomic Embed API (same model, same dims, no re-embedding migration).
- Groq calls use `context.WithTimeout(ctx, 10s)`. Embedding calls (Ollama/Nomic) use 5s timeout. No hanging requests.

### 3.6 Vector Index: HNSW
- `m=16, ef_construction=64`. Works on empty table. No training needed. Grows with data.
- **IVFFlat rejected**: Cannot build on empty table, requires tuned `lists` parameter.

### 3.7 Database Migrations: golang-migrate
- Pure SQL up/down files. Compatible with sqlc schema source.
- pgvector extension enabled via Supabase dashboard (not via migration — free tier restriction).

### 3.8 SQL Codegen: sqlc
- `sqlc.yaml` references migration files as schema source.
- pgvector type: declare `embedding vector(768)` (NOT `public.vector(768)`).
- Add sqlc override: `db_type: "vector"` → `github.com/pgvector/pgvector-go.Vector`.
- Connection: `pgxpool` with `DefaultQueryExecMode: pgx.QueryExecModeSimpleProtocol` (Supavisor transaction mode — no prepared statements).

### 3.9 WebSocket: Go Fiber contrib + Browser Native API
- No Socket.io on either side.
- Go hub: `map[string]map[*websocket.Conn]bool` + `sync.RWMutex`.
- Browser: `Alpine.store('chat')` wraps native `WebSocket`. No socket.io-client.
- WS authentication: **first-message pattern** (token in first JSON message, NOT in URL).
- **Cloud Run**: `min-instances=1` + session affinity (best-effort, MVP trade-off documented §16).

### 3.10 PDF Extraction: pdfcpu (pure Go, CGO_ENABLED=0 safe)
- Apache 2.0 licence. Handles modern text PDFs.
- Image-only PDFs → graceful error → user prompted to fill manually.
- Download: custom `http.Client` (10s timeout, 5MB cap, Uploadthing CDN allowlist, `application/pdf` MIME check).

### 3.11 Docker Base: `gcr.io/distroless/static:nonroot`
- Includes CA certificates (needed for HTTPS to Groq/Supabase/Nomic).
- Minimal `/etc/passwd`. No shell. Runs as nonroot.
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w"`.

### 3.12 Bootstrap v5.3 — CSS Only
- Import `bootstrap.min.css` ONLY. Do NOT import `bootstrap.bundle.min.js`.
- All interactive components (modals, dropdowns) via Alpine.js directives.

### 3.13 HTMX + Alpine.js Integration
- Use `@alpinejs/morph` plugin with `hx-ext="alpine-morph"` on HTMX-swapped regions.
- Alpine global stores (`Alpine.store()`) are outside HTMX swap targets — never destroyed.
- **`hx-boost` is NOT used**. Full page navigations use standard `<a>` links; HTMX only for partial swaps.

### 3.14 CSRF: SameSite=Lax Cookies
- BFF and browser on same origin. SameSite=Lax blocks cross-site POSTs. Sufficient for MVP.

### 3.15 Supabase RLS: Disabled
- Go API uses service-role key. Go code is the SOLE authorization layer.
- Every SQL query touching user data MUST have explicit WHERE clause. Enforced in code review.

### 3.16 Logging: Go `slog` (stdlib, JSON format)
- Structured JSON — Cloud Logging parses fields automatically.
- Sensitive fields (JWT, API keys, passwords) MUST never be logged. Lint rule enforced.

### 3.17 Configuration: godotenv (dev) + env vars (prod)
- Startup validates all required vars — process exits immediately if any are missing.
- Production secrets via GCP Secret Manager → Cloud Run env var references.

---

## 4. Software Engineering Principles

This section defines the patterns that MUST be followed when implementing. Violations are rejected in code review.

### 4.1 SOLID in Go API

#### Single Responsibility
- Each handler file handles ONE resource group (e.g., `matches.go` only handles match routes).
- Each service file handles ONE domain concern.
- **`llm.go` is split** into separate files per concern:
  - `service/llm/cv.go` — CV text structuring
  - `service/llm/search.go` — NL search query parsing
  - `service/llm/explain.go` — match explanation generation
  - `service/llm/ikigai.go` — Ikigai answer summarisation
- `cv_import.go` is split into:
  - `service/downloader.go` — PDF URL download (SSRF-safe)
  - `service/pdf.go` — text extraction (pdfcpu)
  - `service/llm/cv.go` — Groq structuring (above)

#### Open/Closed
- Adding a new LLM provider requires implementing `LLMProvider` interface, not modifying existing code.
- Adding a new embedding provider requires implementing `EmbeddingProvider` interface, not modifying existing code.

#### Liskov Substitution
- `OllamaProvider` and `NomicProvider` are interchangeable via `EmbeddingProvider` interface.
- `GroqProvider` is the concrete implementation of all `LLM*` interfaces.

#### Interface Segregation
- Interfaces are small and specific to the consumer — not one large `LLMService`:
  ```go
  // service/interfaces.go
  type MatchExplainer interface {
      ExplainMatch(ctx context.Context, a, b Profile) (string, error)
  }
  type NLSearchParser interface {
      ParseSearchQuery(ctx context.Context, query string) (*SearchParams, error)
  }
  type CVStructurer interface {
      StructureCV(ctx context.Context, text string) (*CVData, error)
  }
  type IkigaiSummariser interface {
      SummariseIkigai(ctx context.Context, answers IkigaiAnswers) (string, error)
  }
  type EmbeddingProvider interface {
      Embed(ctx context.Context, text string) ([]float32, error)
  }
  type MatchService interface {
      GetMatches(ctx context.Context, userID uuid.UUID, f MatchFilter) ([]Match, error)
      GetExplanation(ctx context.Context, matchID uuid.UUID) (string, error)
      RefreshCacheForUser(ctx context.Context, userID uuid.UUID) error
  }
  ```

#### Dependency Inversion
- Handlers depend on interfaces, never on concrete service structs directly.
- All dependencies injected via constructor:
  ```go
  // handler/matches.go
  type MatchHandler struct {
      svc    MatchService
      logger *slog.Logger
  }
  func NewMatchHandler(svc MatchService, logger *slog.Logger) *MatchHandler {
      return &MatchHandler{svc: svc, logger: logger}
  }
  ```
- `main.go` is the composition root: wire concrete impls → inject into handlers.

### 4.2 DRY in Go API

The following logic appears in multiple places — each MUST be a single shared function:

| Duplicated logic | Single source location |
|---|---|
| Block exclusion SQL filter (both directions) | `db/queries/shared.sql` → `IsBlocked(userA, userB)` function |
| Visibility + embedding_status filter | Named CTE `visible_profiles` in each query referencing it |
| Embedding text construction | `service/embedding/text.go` → `BuildEmbeddingText(p Profile, ik Ikigai) string` |
| HTTP timeout context for external calls | `service/http.go` → `NewTimeoutClient(timeout) *http.Client` |
| Standard error response | `model/errors.go` → `AppError` type + `ErrorHandler` Fiber middleware |
| JWT user extraction from Fiber context | `middleware/auth.go` → `UserFromCtx(c *fiber.Ctx) uuid.UUID` |
| Pagination params (limit/offset) | `model/request.go` → `PaginationQuery` struct + `Validate()` |

### 4.3 DRY in BFF (TypeScript)

| Duplicated logic | Single source location |
|---|---|
| Go API fetch with Bearer token | `server/lib/api.ts` → `apiClient(jwt).get(path)` |
| Read JWT from session cookie | `server/middleware/session.ts` → `getSession(c)` |
| Redirect to login if unauthenticated | `server/middleware/auth.ts` → `requireAuth` Hono middleware |
| Render Eta partial for HTMX response | `server/lib/render.ts` → `renderPartial(c, template, data)` |

---

## 5. Project Structure

```
supernetwork-ai/
├── PLAN.md
├── problem.md
├── .gitignore
├── docker-compose.yml              ← Ollama for local dev
│
├── backend/
│   ├── go.mod                      ← module supernetwork/backend, Go 1.23
│   ├── go.sum
│   ├── Makefile
│   │    dev:        air -c .air.toml
│   │    build:      CGO_ENABLED=0 go build ...
│   │    swagger:    swag init -g main.go --output docs/
│   │    migrate-up: migrate -path db/migrations -database $DATABASE_URL up
│   │    migrate-dn: migrate -path db/migrations -database $DATABASE_URL down 1
│   │    sqlc:       sqlc generate
│   │    test:       go test ./...
│   ├── .air.toml                   ← air hot-reload config
│   ├── Dockerfile
│   ├── .env.example
│   ├── sqlc.yaml
│   ├── main.go                     ← composition root only: wire deps, start server
│   ├── docs/                       ← swaggo output (never edit)
│   └── internal/
│       ├── config/
│       │   └── config.go           ← struct with all env vars; fail-fast on missing
│       ├── middleware/
│       │   ├── auth.go             ← JWT verify (lestrrat-go/jwx/v2) + JWKS cache
│       │   │                          UserFromCtx(c) helper
│       │   ├── recovery.go         ← panic → 500, never crash server
│       │   ├── cors.go
│       │   ├── ratelimit.go        ← per-route limits, in-memory (min-instances=1)
│       │   └── logger.go           ← slog request logging (no sensitive fields)
│       ├── handler/
│       │   ├── auth.go             ← POST /api/v1/auth/ws-token
│       │   ├── users.go            ← GET /api/v1/users/me
│       │   │                          GET /api/v1/users/:id
│       │   ├── profiles.go         ← PATCH /api/v1/profiles/me (partial update)
│       │   │                          PATCH /api/v1/profiles/me/visibility
│       │   ├── onboarding.go       ← POST /api/v1/onboarding/ikigai
│       │   │                          POST /api/v1/onboarding/import-cv
│       │   │                          GET  /api/v1/onboarding/skill-suggestions
│       │   ├── matches.go          ← GET /api/v1/matches
│       │   │                          POST /api/v1/matches/:matchedUserId/dismiss
│       │   │                          GET  /api/v1/matches/:matchedUserId/explanation
│       │   ├── search.go           ← POST /api/v1/search
│       │   ├── connections.go      ← POST /api/v1/connections
│       │   │                          GET  /api/v1/connections
│       │   │                          PATCH /api/v1/connections/:id
│       │   │                          GET  /api/v1/connections/status/:userId
│       │   ├── conversations.go    ← GET  /api/v1/conversations
│       │   │                          POST /api/v1/conversations (create with userId)
│       │   │                          GET  /api/v1/conversations/:id/messages
│       │   │                          PATCH /api/v1/conversations/:id/read
│       │   ├── blocks.go           ← POST   /api/v1/blocks
│       │   │                          DELETE /api/v1/blocks/:userId
│       │   ├── files.go            ← POST /api/v1/files/presign
│       │   ├── internal.go         ← POST /internal/matches/refresh
│       │   │                          (secured by INTERNAL_API_SECRET header)
│       │   └── account.go          ← DELETE /api/v1/account (hard delete cascade)
│       ├── ws/
│       │   ├── hub.go              ← Hub struct: rooms, register/unregister, broadcast
│       │   │                          WaitGroup for goroutine tracking
│       │   │                          graceful shutdown: close all conns on Stop()
│       │   └── handler.go          ← WS upgrade, first-message auth, join_room, message
│       ├── service/
│       │   ├── interfaces.go       ← ALL service interfaces (ISP — small, focused)
│       │   ├── embedding/
│       │   │   ├── text.go         ← BuildEmbeddingText(profile, ikigai) string (SINGLE source)
│       │   │   ├── provider.go     ← EmbeddingProvider interface
│       │   │   ├── ollama.go       ← Ollama impl
│       │   │   └── nomic.go        ← Nomic API impl
│       │   ├── llm/
│       │   │   ├── client.go       ← Groq go-openai client setup (BaseURL, key, timeouts)
│       │   │   ├── cv.go           ← CVStructurer impl
│       │   │   ├── search.go       ← NLSearchParser impl
│       │   │   ├── explain.go      ← MatchExplainer impl
│       │   │   └── ikigai.go       ← IkigaiSummariser impl
│       │   ├── matching.go         ← MatchService impl: pgvector search, cache computation
│       │   ├── downloader.go       ← PDF URL download (SSRF-safe, timeout, size cap)
│       │   └── pdf.go              ← pdfcpu text extraction
│       ├── db/
│       │   ├── client.go           ← pgxpool: SimpleProtocol mode, pool config, startup ping
│       │   ├── migrations/         ← golang-migrate SQL up+down files
│       │   │   ├── 001_extensions.sql      ← placeholder (pgvector enabled in dashboard)
│       │   │   ├── 002_users.sql
│       │   │   ├── 003_profiles.sql
│       │   │   ├── 004_ikigai.sql
│       │   │   ├── 005_connections.sql
│       │   │   ├── 006_conversations.sql
│       │   │   ├── 007_messages.sql
│       │   │   ├── 008_blocks.sql
│       │   │   └── 009_match_cache.sql
│       │   ├── queries/
│       │   │   ├── users.sql
│       │   │   ├── profiles.sql
│       │   │   ├── matches.sql
│       │   │   ├── connections.sql
│       │   │   ├── conversations.sql
│       │   │   └── blocks.sql
│       │   └── generated/          ← sqlc output — never edit
│       ├── model/
│       │   ├── request.go          ← request structs + PaginationQuery
│       │   ├── response.go         ← response structs
│       │   └── errors.go           ← AppError type, error codes registry, Fiber ErrorHandler
│       └── health/
│           └── handler.go          ← GET /healthz, GET /readyz
│
└── frontend/
    ├── package.json                ← scripts: dev, build, typecheck, lint
    ├── bun.lockb
    ├── tsconfig.json
    ├── vite.config.ts              ← builds client/ → dist/public/
    ├── .env.example
    └── server/                     ← Hono BFF (Bun.serve, port 3000)
        ├── index.ts                ← Hono app, Bun.serve(), register middleware + routes
        ├── lib/
        │   ├── api.ts              ← typed fetch client → Go API (SINGLE source, adds Bearer)
        │   ├── render.ts           ← renderPartial(c, template, data) helper
        │   └── eta.ts              ← Eta engine configuration
        ├── middleware/
        │   ├── session.ts          ← getSession(c): reads sn_access cookie; token refresh
        │   │                          concurrent refresh guard (mutex — only one refresh at a time)
        │   └── auth.ts             ← requireAuth: redirect to /login if no session
        ├── routes/
        │   ├── auth.ts             ← POST /auth/login, GET /auth/logout, GET /auth/signup, GET /auth/confirm
        │   ├── pages.ts            ← full page routes with onboarding guard
        │   └── partials/
        │       ├── matches.ts
        │       ├── search.ts
        │       ├── messages.ts
        │       ├── profile.ts
        │       └── ws-token.ts     ← GET /partials/ws-token (re-fetches WS token on reconnect)
        ├── templates/
        │   ├── layouts/base.eta
        │   ├── pages/
        │   │   ├── landing.eta
        │   │   ├── login.eta
        │   │   ├── signup.eta
        │   │   ├── onboarding/
        │   │   │   ├── step1-basic.eta       ← display_name, tagline, avatar
        │   │   │   ├── step2-ikigai.eta      ← 4 Ikigai questions
        │   │   │   ├── step3-skills.eta      ← skills + interests (AI-suggested)
        │   │   │   ├── step4-intent.eta      ← intent + availability + working style
        │   │   │   └── step5-links.eta       ← social links + CV import (optional)
        │   │   ├── dashboard.eta
        │   │   ├── profile.eta
        │   │   ├── discover.eta
        │   │   └── messages.eta
        │   └── partials/
        │       ├── match-card.eta
        │       ├── match-list.eta
        │       ├── msg-bubble.eta
        │       └── search-results.eta
        └── client/                 ← Vite-bundled browser assets
            ├── main.ts             ← Alpine.store() registrations
            ├── stores/
            │   ├── chat.ts         ← WS + reconnect + re-auth logic
            │   └── onboarding.ts   ← step progress state
            └── styles/
                └── main.scss       ← Bootstrap 5 SCSS + custom

```

---

## 6. Database Schema

> **BEFORE running migrations**: enable the `vector` extension in Supabase dashboard → Database → Extensions.

```sql
-- ============================================================
-- Migration 001 (placeholder — pgvector enabled in dashboard)
-- ============================================================
-- No SQL. Document the manual step.
-- Down: no-op


-- ============================================================
-- Migration 002: users
-- ============================================================
-- Up:
CREATE TABLE users (
  id          UUID PRIMARY KEY,         -- = auth.users.id (Supabase Auth)
  email       TEXT NOT NULL UNIQUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Down:
-- DROP TABLE users;


-- ============================================================
-- Migration 003: profiles
-- ============================================================
-- Up:
CREATE TABLE profiles (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  display_name        TEXT NOT NULL DEFAULT '',
  tagline             TEXT NOT NULL DEFAULT '',
  bio                 TEXT NOT NULL DEFAULT '',
  avatar_url          TEXT,
  portfolio_url       TEXT,
  linkedin_url        TEXT,
  github_url          TEXT,
  twitter_url         TEXT,
  location            TEXT,
  timezone            TEXT,
  skills              TEXT[] NOT NULL DEFAULT '{}',
  interests           TEXT[] NOT NULL DEFAULT '{}',
  -- intent: array so user can be open to multiple roles simultaneously
  intent              TEXT[] NOT NULL DEFAULT '{}'
                      CHECK (intent <@ ARRAY['cofounder','teammate','client']),
  availability        TEXT NOT NULL DEFAULT 'open'
                      CHECK (availability IN ('open','part-time','not-available')),
  working_style       TEXT NOT NULL DEFAULT 'async'
                      CHECK (working_style IN ('async','sync','hybrid')),
  visibility          TEXT NOT NULL DEFAULT 'public'
                      CHECK (visibility IN ('public','private')),
  embedding           VECTOR(768),           -- nomic-embed-text, 768-dim FIXED
  embedding_status    TEXT NOT NULL DEFAULT 'pending'
                      CHECK (embedding_status IN ('pending','current','stale','failed')),
  embedding_updated_at TIMESTAMPTZ,
  onboarding_complete BOOLEAN NOT NULL DEFAULT FALSE,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX profiles_embedding_hnsw_idx
  ON profiles USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);
CREATE INDEX profiles_user_id_idx ON profiles (user_id);
-- Down:
-- DROP TABLE profiles;


-- ============================================================
-- Migration 004: ikigai_profiles
-- ============================================================
-- Up:
CREATE TABLE ikigai_profiles (
  id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id                  UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  what_you_love            TEXT NOT NULL DEFAULT '',
  what_youre_good_at       TEXT NOT NULL DEFAULT '',
  what_world_needs         TEXT NOT NULL DEFAULT '',
  what_you_can_be_paid_for TEXT NOT NULL DEFAULT '',
  ai_summary               TEXT,
  created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Down:
-- DROP TABLE ikigai_profiles;


-- ============================================================
-- Migration 005: connections
-- ============================================================
-- Up:
CREATE TABLE connections (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  requester_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status        TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','accepted','rejected')),
  message       TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT connections_no_self CHECK (requester_id != recipient_id),
  CONSTRAINT connections_unique_pair UNIQUE (requester_id, recipient_id)
);
CREATE INDEX connections_recipient_status_idx ON connections (recipient_id, status);
CREATE INDEX connections_requester_status_idx ON connections (requester_id, status);
-- NOTE: DB-level UNIQUE (requester_id, recipient_id) prevents exact-duplicate rows only.
-- Bidirectional uniqueness (A→B and B→A simultaneously) is enforced in the POST /connections
-- handler: before INSERT, check WHERE (requester_id=$a AND recipient_id=$b)
--                                    OR (requester_id=$b AND recipient_id=$a).
-- Returns 409 CONFLICT if any row found.
-- Down:
-- DROP TABLE connections;


-- ============================================================
-- Migration 006: conversations
-- ============================================================
-- Up:
CREATE TABLE conversations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE conversation_members (
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (conversation_id, user_id)
);

-- NOTE: Pair uniqueness (one conversation per user pair) enforced at app layer in
-- POST /conversations handler: SELECT 1 FROM conversation_members WHERE conversation_id IN
-- (SELECT conversation_id FROM conversation_members WHERE user_id=$a)
-- AND user_id=$b — returns existing conversation if found, creates new one if not.
-- No DB-level unique index needed (PRIMARY KEY already prevents duplicate membership rows).
-- Down:
-- DROP TABLE conversation_members; DROP TABLE conversations;


-- ============================================================
-- Migration 007: messages
-- ============================================================
-- Up:
CREATE TABLE messages (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  sender_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content         TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000),
  read_at         TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX messages_conv_created_idx ON messages (conversation_id, created_at DESC);
-- Down:
-- DROP TABLE messages;


-- ============================================================
-- Migration 008: blocks
-- ============================================================
-- Up:
CREATE TABLE blocks (
  blocker_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (blocker_id, blocked_id),
  CONSTRAINT blocks_no_self CHECK (blocker_id != blocked_id)
);
-- Down:
-- DROP TABLE blocks;


-- ============================================================
-- Migration 009: match_cache
-- ============================================================
-- Up:
CREATE TABLE match_cache (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  matched_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  score           FLOAT NOT NULL CHECK (score BETWEEN 0 AND 1),
  -- categories: TEXT[] because a match can qualify as multiple roles simultaneously
  categories      TEXT[] NOT NULL DEFAULT '{}'
                  CHECK (categories <@ ARRAY['cofounder','teammate','client']),
  explanation     TEXT,
  dismissed       BOOLEAN NOT NULL DEFAULT FALSE,
  computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, matched_user_id)
);
CREATE INDEX match_cache_user_score_idx ON match_cache (user_id, score DESC);
-- Down:
-- DROP TABLE match_cache;
```

**Schema decisions resolved:**
- `match_cache.categories` is `TEXT[]` (not singular `TEXT`) because a user can have `intent=['cofounder','teammate']` and a match may qualify for multiple roles simultaneously.
- Message `content` is capped at 4000 characters at DB level (not just app level).
- Conversations: uniqueness between two users enforced in Go handler (check before create).
- All migrations include Down comments for `golang-migrate` reversibility.

---

## 7. API Contract

**Base URL**: `http://localhost:3001` (dev) | `https://api.supernetworkai.com` (prod)
**Auth**: All `/api/v1/*` require `Authorization: Bearer <supabase_jwt>`.
**Errors**: All errors → `{"code": "SNAKE_CASE_CODE", "message": "human readable string"}`.
**Pagination**: All list endpoints accept `?limit=20&offset=0` (default limit 20, max 100).

### Error Codes (exhaustive registry)
| Code | HTTP Status | Meaning |
|---|---|---|
| `UNAUTHORIZED` | 401 | Missing or expired JWT |
| `FORBIDDEN` | 403 | Authenticated but not allowed |
| `NOT_FOUND` | 404 | Resource does not exist |
| `CONFLICT` | 409 | Duplicate resource (e.g., connection already exists) |
| `VALIDATION_ERROR` | 422 | Request body/params failed validation |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Unexpected server error |
| `SERVICE_UNAVAILABLE` | 503 | External dependency (Groq, Nomic) unavailable |

### Health
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/healthz` | None | Liveness — always 200 if process running |
| GET | `/readyz` | None | Readiness — checks DB ping |

### Auth
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/auth/ws-token` | Bearer | Returns `{token, expires_at}` — 60s WS token |

### Users & Profiles
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/users/me` | Bearer | Returns `{user, profile, ikigai}` — own data |
| PATCH | `/api/v1/profiles/me` | Bearer | Partial profile update (PATCH not PUT) |
| PATCH | `/api/v1/profiles/me/visibility` | Bearer | Body: `{visibility: "public"\|"private"}` |
| GET | `/api/v1/users/:id` | Bearer | Other-user profile (visibility + connection + block enforced) |

> **Ambiguity resolved**: `PATCH` (not `PUT`) is used for profile updates because profiles have many optional fields. PUT would require sending all fields.

> **PATCH /api/v1/profiles/me body**:
> - Accepted fields: `display_name`, `tagline`, `bio`, `avatar_url`, `portfolio_url`, `linkedin_url`, `github_url`, `twitter_url`, `location`, `timezone`, `skills`, `interests`, `intent`, `availability`, `working_style`
> - All fields optional (omit to leave unchanged). Unknown fields rejected with 422.
> - Embedding-triggering fields: `skills`, `interests`, `bio`, `intent` — if any of these change, handler sets `embedding_status='stale'` and triggers async re-embedding.
> - Non-triggering fields: `avatar_url`, `portfolio_url`, `linkedin_url`, `github_url`, `twitter_url`, `tagline`, `display_name`, `location`, `timezone`, `availability`, `working_style`.

> **GET /api/v1/users/:id visibility rule**: block check first (403 if blocked in either direction). Then: `visibility='public'` → return profile. `visibility='private'` → check for accepted connection → return if connected, else 403.

### Onboarding
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/onboarding/ikigai` | Bearer | Save Ikigai answers + trigger async embed |
| POST | `/api/v1/onboarding/import-cv` | Bearer | Body: `{file_url}`. Returns pre-fill data |
| GET | `/api/v1/onboarding/skill-suggestions` | Bearer | Query: `?context=<text>`. Groq suggests skills |

### Matching & Search
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/matches` | Bearer | Ranked matches from cache. Query: `?category=cofounder&limit=20&offset=0` |
| GET | `/api/v1/matches/:matchedUserId/explanation` | Bearer | On-demand explanation. `:matchedUserId` is a user UUID (not cache row UUID) |
| POST | `/api/v1/matches/:matchedUserId/dismiss` | Bearer | Dismiss a match from results |
| POST | `/api/v1/search` | Bearer | Body: `{query, filters?}`. Returns ranked profiles |

> **Ambiguity resolved**: `matches/:id` uses `matchedUserId` (a user UUID) not the internal cache row UUID. This is stable and meaningful to callers.

### Connections
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/connections` | Bearer | Body: `{recipient_id, message?}`. Returns 409 if connection exists in either direction. |
| GET | `/api/v1/connections` | Bearer | Query: `?status=pending\|accepted` |
| PATCH | `/api/v1/connections/:id` | Bearer | Body: `{status: "accepted"\|"rejected"}` |
| GET | `/api/v1/connections/status/:userId` | Bearer | Returns `{status: "none"\|"pending"\|"accepted"\|"rejected"}` — used by profile page to render correct button |

> **Bidirectional uniqueness**: `POST /connections` handler checks for an existing row in BOTH directions before inserting: `WHERE (requester_id=$me AND recipient_id=$them) OR (requester_id=$them AND recipient_id=$me)`. Returns 409 CONFLICT if found. This prevents A→B and B→A existing simultaneously.

### Conversations & Messages
| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/conversations` | Bearer | List conversations with unread count |
| POST | `/api/v1/conversations` | Bearer | Body: `{user_id}`. Creates conv if not exists; returns conv. Returns 403 if no accepted connection. |
| GET | `/api/v1/conversations/:id/messages` | Bearer | Paginated message history. Cursors: `?before=<created_at>&before_id=<uuid>` or `?after=<created_at>` |
| PATCH | `/api/v1/conversations/:id/read` | Bearer | Marks all messages in conv as read |

> **Ambiguity resolved**: Conversation creation is explicit via `POST /api/v1/conversations`. Accepts a connection request does NOT auto-create a conversation. User must explicitly start one from the connections list or profile page.

> **Message cursor format**: ISO 8601 UTC timestamp, e.g. `2026-01-15T14:30:00Z`. Backward pagination (`?before`): `WHERE (created_at, id) < ($cursor_ts, $cursor_id)` to break ties by UUID. Forward pagination for catch-up (`?after`): `WHERE created_at > $after_ts ORDER BY created_at ASC`. Both parameters are optional; omit for the most recent page.

### Blocks
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/blocks` | Bearer | Body: `{user_id}` |
| DELETE | `/api/v1/blocks/:userId` | Bearer | Unblock |

### Files
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/files/presign` | Bearer | Body: `{type: "avatar"\|"cv", filename, size}`. Returns Uploadthing presigned URL |

### Internal (Cloud Scheduler)
| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/internal/matches/refresh` | `X-Internal-Secret` header | Recomputes match cache for stale entries |

> **Ambiguity resolved**: Internal endpoints use `X-Internal-Secret: <INTERNAL_API_SECRET>` header (not Bearer JWT). Go middleware checks this header before allowing access to `/internal/*` routes.

### WebSocket
**URL**: `ws://localhost:3001/ws` (dev) | `wss://api.supernetworkai.com/ws` (prod)

```jsonc
// Auth flow (MUST be first message after connect)
C→S: { "type": "auth", "token": "<60s_hmac_token>" }
S→C: { "type": "auth_ok", "user_id": "<uuid>" }
S→C: { "type": "auth_fail", "reason": "expired|invalid" }

// Room management (only after auth_ok)
C→S: { "type": "join_room", "conversation_id": "<uuid>" }
C→S: { "type": "leave_room", "conversation_id": "<uuid>" }

// Messaging
C→S: { "type": "message", "conversation_id": "<uuid>", "content": "<max 4000 chars>" }
S→C: { "type": "new_message", "id": "<uuid>", "conversation_id": "<uuid>",
        "sender_id": "<uuid>", "content": "...", "created_at": "<iso8601>" }

// Errors
S→C: { "type": "error", "code": "NOT_CONNECTED|BLOCKED|TOO_LONG|UNAUTHORIZED" }
```

---

## 8. Authentication & Session Design

### 8.1 Signup Flow (NEW — was missing)
```
1. Browser submits POST /auth/signup (BFF route)
   Body: { email, password }

2. BFF calls Supabase Auth:
   POST {SUPABASE_URL}/auth/v1/signup
   Body: { email, password }
   → Returns: { user, access_token, refresh_token }
     OR requires email verification (config dependent)

3. If email verification disabled (recommended for MVP, configurable in Supabase):
   Same cookie flow as login → redirect to /onboarding/step1

4. If email verification enabled:
   BFF renders "Check your email" page. User clicks link → redirects to
   /auth/confirm?token=... → BFF exchanges token → sets cookies → /onboarding/step1
```

### 8.2 Login Flow
```
1. POST /auth/login (BFF)
2. BFF → POST {SUPABASE_URL}/auth/v1/token?grant_type=password
3. Response: { access_token, refresh_token, expires_in }
4. BFF sets cookies:
   sn_access:  HttpOnly; Secure; SameSite=Lax; Path=/;            Max-Age=3600
   sn_refresh: HttpOnly; Secure; SameSite=Lax; Path=/auth/refresh; Max-Age=604800
5. HX-Redirect: /dashboard (or /onboarding if !onboarding_complete)
```

### 8.3 Onboarding Guard
```
BFF pages.ts middleware (applied to all authenticated pages except /onboarding/*):
1. Read JWT from sn_access cookie
2. Parse JWT locally (verify signature with Supabase public key) to extract sub (user ID)
   — this avoids a Go API round-trip just to check onboarding status
3. Call Go API: GET /api/v1/users/me
4. If profile.onboarding_complete == false → HX-Redirect: /onboarding/step1
```

### 8.4 Token Refresh (with Concurrent Guard)
```
BFF session middleware wraps Go API calls:

mutex.Lock()  // only ONE token refresh at a time
  if already refreshed (check cookie timestamp) → mutex.Unlock(); retry with new token
  else:
    call Supabase POST /auth/v1/token?grant_type=refresh_token
    on success: update both cookies, mutex.Unlock(), retry Go API call
    on failure: clear cookies, mutex.Unlock(), redirect to /login
```

> **Why mutex**: Supabase refresh tokens are single-use. If 3 concurrent HTMX requests all hit 401 simultaneously, all 3 would try to refresh. Only the first succeeds; the other two would fail and log the user out. The mutex ensures exactly one refresh happens.

### 8.5 JWT Verification in Go API
```
- Algorithm: RS256 (Supabase default)
- JWKS: {SUPABASE_URL}/auth/v1/.well-known/jwks.json
- Cache: 30-minute TTL. Re-fetch on verify failure (handles key rotation).
- Library: github.com/lestrrat-go/jwx/v2
- user.ID stored in Fiber context locals via UserFromCtx(c) helper
```

### 8.6 WebSocket Token
```
1. BFF renders messages.eta → calls POST /api/v1/auth/ws-token
2. Go: sign(userID + expiry, WS_TOKEN_SECRET) via HMAC-SHA256, TTL=60s
3. BFF embeds in data attribute (NOT in URL, NOT in localStorage):
   <div data-ws-token="<token>" data-conv-id="<uuid>" ...>
4. Alpine reads token → sends as first WS message
5. Token is single-use: Go marks used in sync.Map; auto-purges after 60s TTL.
   Purge mechanism: background goroutine (started at server init) ticks every 60s,
   iterates sync.Map and deletes entries whose stored expiry timestamp has passed.
   Expiry is also validated on every WS auth attempt from the HMAC token payload itself —
   the sync.Map check only prevents replay; expiry is authoritative.
6. On WS reconnect: Alpine triggers HTMX GET /partials/ws-token
   → BFF calls Go API for fresh token → re-embeds in hidden DOM element
   → Alpine reads new token → re-authenticates
```

---

## 9. Key Feature Flows

### 9.1 Onboarding (5 Steps, HTMX-driven)

**Step count (FIXED — was 5 templates but 6 described):**
1. Basic info (display_name, tagline, avatar upload via Uploadthing)
2. Ikigai questionnaire (4 questions)
3. Skills & interests (tag input + `GET /api/v1/onboarding/skill-suggestions`)
4. Intent + availability + working style
5. Social links + optional CV import (Uploadthing PDF → Go API → Groq → pre-fill)

On final step completion → Go sets `onboarding_complete = true` → BFF redirects to `/dashboard`.

**Async embedding triggered** after Ikigai save and after any profile update (§9.2).

### 9.2 Embedding Pipeline (Async, Resilient)
```
1. Go handler saves profile/ikigai to DB → returns 200 immediately
2. Goroutine spawned (tracked by hub.WaitGroup):
   defer wg.Done()
   defer func() { if r := recover(); r != nil { log.Error(r) } }()  // panic safety
   a. embedding_text := BuildEmbeddingText(profile, ikigai)  // SINGLE function
   b. vec, err := embeddingProvider.Embed(ctx, embedding_text)  // 10s timeout
   c. if err: UPDATE embedding_status='failed'; log; return
   d. UPDATE profiles SET embedding=$vec, embedding_status='current', ...
   e. wg.Add(1); go func() { defer wg.Done(); refreshMatchCacheForUser(userID) }()
      // Inner goroutine MUST be tracked by WaitGroup — graceful shutdown waits for it.

3. Profiles with embedding_status != 'current' excluded from all search results.

4. Recovery: Cloud Scheduler → POST /internal/matches/refresh every 6h also
   re-embeds any profiles with embedding_status IN ('pending','failed','stale').

refreshMatchCacheForUser upsert strategy:
  - Compute new candidate matches via pgvector search (score ≥ 0.6 threshold)
  - INSERT ... ON CONFLICT (user_id, matched_user_id) DO UPDATE SET
      score = EXCLUDED.score,
      categories = EXCLUDED.categories,
      computed_at = NOW()
    -- PRESERVE dismissed and explanation — never overwrite them on refresh
  - After upsert: DELETE FROM match_cache WHERE user_id=$uid
      AND matched_user_id NOT IN (<new candidate set>)
      AND dismissed = false  -- only remove undismissed rows below threshold
  This ensures: dismissed matches stay dismissed, stale rows are cleaned up,
  explanation text is preserved across refreshes.
```

### 9.3 Match Categories (Resolved Algorithm)
```
Determine categories for a match pair (User A, User B):

categories = []
if 'cofounder' ∈ A.intent AND 'cofounder' ∈ B.intent → add 'cofounder'
if 'teammate'  ∈ A.intent AND 'teammate'  ∈ B.intent → add 'teammate'
if 'client'    ∈ A.intent AND ('cofounder' ∈ B.intent OR 'teammate' ∈ B.intent) → add 'client'
if 'client'    ∈ B.intent AND ('cofounder' ∈ A.intent OR 'teammate' ∈ A.intent) → add 'client'

if categories is empty → match not stored (score below 0.6 threshold already filters most)
match_cache.categories = categories (TEXT[])
```

### 9.4 Match Discovery
```
GET /api/v1/matches?category=cofounder&limit=20&offset=0

SELECT mc.*, p.display_name, p.tagline, p.skills, p.intent, p.avatar_url
FROM match_cache mc
JOIN profiles p ON p.user_id = mc.matched_user_id
WHERE mc.user_id = $currentUser
  AND mc.dismissed = false
  AND p.visibility = 'public'
  AND ($category IS NULL OR $category = ANY(mc.categories))
  AND NOT EXISTS (
    SELECT 1 FROM blocks
    WHERE (blocker_id = $currentUser AND blocked_id = mc.matched_user_id)
       OR (blocker_id = mc.matched_user_id AND blocked_id = $currentUser)
  )
ORDER BY mc.score DESC
LIMIT $limit OFFSET $offset

Explanation (on-demand, GET /api/v1/matches/:matchedUserId/explanation):
  Looks up match_cache row for (currentUser, matchedUserId).
  If explanation IS NULL: calls Groq → saves → returns.
  If already exists: returns cached.
  Rate limited: 10/min per user.
```

### 9.5 Natural Language Search
```
POST /api/v1/search
Body: { "query": "Python dev open to equity, prefers async" }

1. Groq llama-3.1-8b-instant (JSON mode, 5s timeout):
   → { embedding_text, intent_filter, availability_filter }

2. Embed embedding_text (Ollama/Nomic, 3s timeout) → query_vec float[768]

3. pgvector search with all guards:
   SELECT p.user_id, p.display_name, p.tagline, p.skills, p.intent, p.avatar_url,
          1 - (p.embedding <=> $query_vec) AS score
   FROM profiles p
   WHERE p.user_id != $currentUser
     AND p.visibility = 'public'
     AND p.embedding_status = 'current'
     AND ($intent_filter IS NULL OR $intent_filter = ANY(p.intent))
     AND ($availability IS NULL OR p.availability = $availability)
     AND NOT EXISTS (
       SELECT 1 FROM blocks
       WHERE (blocker_id = $currentUser AND blocked_id = p.user_id)
          OR (blocker_id = p.user_id AND blocked_id = $currentUser)
     )
   ORDER BY p.embedding <=> $query_vec
   LIMIT 20

4. Returns ranked results. NO explanations — fetched on-demand via HTMX click.
```

### 9.6 Real-Time Messaging
```
Open /messages/:convId:
  BFF guards: user must be member of conversation (Go API check)
  BFF fetches: last 50 messages (cursor pagination)
  BFF fetches: WS token (POST /api/v1/auth/ws-token)
  BFF renders: messages.eta with history + Alpine store init data

Browser Alpine chat store:
  init(wsToken, convId):
    ws = new WebSocket("wss://api.supernetworkai.com/ws")
    ws.onopen: send { type: "auth", token: wsToken }
    ws.onmessage: handle auth_ok → send join_room; handle new_message → append bubble
    ws.onclose: exponential backoff reconnect (1s, 2s, 4s, 8s, max 30s)
      on reconnect: HTMX GET /partials/ws-token to refresh token first
    on auth_ok + join_room acknowledged:
      fetch GET /api/v1/conversations/:id/messages?after=<last_received_created_at>
      → append any missed messages to the chat UI before resuming live flow
      (last_received_created_at tracked in Alpine store as messages arrive)

Sending:
  Alpine: ws.send({ type: "message", conversation_id, content })
  Go hub:
    1. Validate content length (≤4000 chars)
    2. Verify user is member of conversation (DB)
    3. Verify users have accepted connection (DB)
    4. Verify neither user has blocked the other (DB)
    5. INSERT INTO messages
    6. Broadcast new_message to all conn's in room

Mark read:
  HTMX PATCH /api/v1/conversations/:id/read triggered when user opens conversation
```

---

## 10. Error Handling Strategy

### 10.1 Go API — Centralised Error Handler
```go
// model/errors.go
type AppError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Status  int    `json:"-"`
}

// Sentinel errors
var (
    ErrUnauthorized       = &AppError{"UNAUTHORIZED", "Authentication required", 401}
    ErrForbidden          = &AppError{"FORBIDDEN", "Access denied", 403}
    ErrNotFound           = &AppError{"NOT_FOUND", "Resource not found", 404}
    ErrConflict           = &AppError{"CONFLICT", "Resource already exists", 409}
    ErrValidation         = &AppError{"VALIDATION_ERROR", "Invalid request", 422}
    ErrRateLimited        = &AppError{"RATE_LIMITED", "Too many requests", 429}
    ErrInternal           = &AppError{"INTERNAL_ERROR", "Internal server error", 500}
    ErrServiceUnavailable = &AppError{"SERVICE_UNAVAILABLE", "External service unavailable", 503}
)

// Fiber error handler — registered in main.go
app.Use(recovery.New())  // panic → ErrInternal
app.Use(ErrorHandler)    // AppError → JSON response
```

### 10.2 External Service Failures
| Service | Timeout | On failure |
|---|---|---|
| Groq API | 10s | Return `ErrServiceUnavailable`, log; user sees "AI unavailable, try again" |
| Nomic/Ollama | 5s | Set `embedding_status='failed'`; log; profile saved, search stale |
| Supabase DB | pgxpool timeout (5s acquire) | Return `ErrServiceUnavailable` |
| Uploadthing | 10s | Return `ErrServiceUnavailable` |
| PDF download | 10s | Return 422 "Could not download file" |

### 10.3 BFF Error Handling
- Go API 5xx → BFF renders error page (not raw JSON to browser)
- Go API 401 → token refresh middleware handles (§8.4)
- Go API 403/404 → BFF renders appropriate error page
- Network timeout to Go API → BFF renders "Service temporarily unavailable"

---

## 11. Resilience & Crash Safety

### 11.1 Go API Startup Sequence
```
1. Load config → fail-fast if any required env var missing
2. Connect pgxpool → if initial ping fails, log warning and continue.
   Server starts anyway; Cloud Run startup probe (GET /readyz) blocks traffic
   until DB is reachable. Pool reconnects automatically.
3. Fetch JWKS → cache → if Supabase unreachable, log warning and continue.
   Auth middleware returns 503 "authentication unavailable" until JWKS cached.
   Rationale for asymmetry vs DB: DB can become reachable after a brief cold start;
   JWKS is also recoverable. Both follow warn+continue to maximise Cloud Run
   startup probe compatibility. The /readyz endpoint reflects DB status only
   (JWKS is re-fetched lazily on verify failure, so no separate readiness signal needed).
4. Register Fiber middleware: recovery, cors, logger, ratelimit
5. Register routes
6. Start listening (Cloud Run sends traffic only after /readyz returns 200)
```

### 11.2 Graceful Shutdown
```go
// main.go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
<-quit

// 1. Stop accepting new HTTP connections
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
app.ShutdownWithContext(ctx)

// 2. Wait for in-flight embedding goroutines to finish (up to 15s)
done := make(chan struct{})
go func() { embeddingWG.Wait(); close(done) }()
select {
case <-done:
case <-ctx.Done():
    log.Warn("shutdown timeout: some embedding goroutines may be incomplete")
}

// 3. Close WebSocket hub (broadcasts close to all connections)
wsHub.Stop()

// 4. Close DB pool
dbPool.Close()
```

### 11.3 Goroutine Safety
- All async embedding goroutines: tracked by `sync.WaitGroup`, wrapped in `recover()`.
- WS hub: all map mutations under `sync.RWMutex`. No channel-based goroutine leak risk.
- WS connections: Fiber WS middleware handles connection lifecycle; hub removes on disconnect.
- In-flight Groq/Nomic calls: bound by `context.WithTimeout` — never hang indefinitely.

### 11.4 Database Resilience
- `pgxpool`: `MaxConns=10` (Supabase free: 60 total, transaction mode multiplexes).
- `MinConns=2`: keep warm connections, reduce cold-start latency.
- `MaxConnIdleTime=5min`, `MaxConnLifetime=30min`: prevent stale connections.
- pgxpool handles reconnection automatically on connection loss.
- `HealthCheckPeriod=1min`: pool proactively validates connections.

### 11.5 BFF Resilience
- All Go API calls have `fetch` timeout (5s default, configurable per endpoint).
- Token refresh mutex prevents concurrent refresh races (§8.4).
- Health probe at `/healthz` — Cloud Run uses for BFF liveness.

### 11.6 Cloud Run Deployment Resilience
- `--min-instances=1` on both services: eliminates cold starts for active users.
- Rolling deployments: Cloud Run routes traffic to new instances only after `/readyz` passes.
- WS connections on old instance are dropped during deploy — Alpine reconnect handles this.
- `SIGTERM` → graceful 15s shutdown before Cloud Run force-kills.

---

## 12. Security Design

### 12.1 Authentication
- JWTs: HttpOnly; Secure; SameSite=Lax cookies. No localStorage ever.
- Refresh token: separate cookie, Path=/auth/refresh, never forwarded to Go API.
- WS tokens: HMAC-SHA256, 60s TTL, single-use (sync.Map with TTL auto-purge).
- JWKS: 30min cache, re-fetch on verify failure.

### 12.2 Input Validation
- Go: all request structs validated by Fiber validator before handler logic.
- Groq responses: strict JSON unmarshal — any failure returns 422 (never trust LLM output blindly).
- PDF URL: must match `^https://utfs\.io/f/[A-Za-z0-9_-]+$` before download.
- PDF download: 5MB cap, 10s timeout, Content-Type must be `application/pdf`.
- Groq CV prompt: user PDF text capped at 4000 tokens. System prompt fixed (prevents injection).
- WS message content: max 4000 chars (enforced at DB level AND WS handler level).

### 12.3 Rate Limiting
| Endpoint | Limit | Storage |
|---|---|---|
| POST /auth/login (BFF) | 5/min per IP | BFF in-memory |
| POST /auth/signup (BFF) | 3/min per IP | BFF in-memory |
| POST /api/v1/search | 30/min per user | Go in-memory |
| POST /api/v1/onboarding/import-cv | 5/hour per user | Go in-memory |
| GET /api/v1/matches/:id/explanation | 10/min per user | Go in-memory |
| POST /api/v1/connections | 20/hour per user | Go in-memory |
| WS connections | 3 concurrent per user | Go WS hub |
| WS messages | 60/min per connection | Go WS handler |

> In-memory rate limiting works only with `min-instances=1`. This is the documented trade-off. If scaling beyond 1 instance, switch to Redis-backed limiter.

### 12.4 Authorization (Go API — no RLS)
Rules enforced in every handler:
1. Own-data reads/writes: `WHERE user_id = $currentUser`
2. Other-user profile reads: block check first (403 if blocked in either direction).
   Then: `visibility='public'` → return profile.
   `visibility='private'` → check for accepted connection → return if connected, else 403.
3. Messages: sender must be accepted connection to recipient; neither must block the other
4. WS room join: validated connection + no block (checked on join_room, not on every message)
5. Connection PATCH: only recipient can accept/reject (not requester)
6. Match explanation: can only request explanation for own matches

### 12.5 CORS
```go
cors.New(cors.Config{
    AllowOrigins:     []string{os.Getenv("BFF_ORIGIN")},
    AllowMethods:     []string{"GET","POST","PATCH","DELETE"},
    AllowHeaders:     []string{"Authorization","Content-Type"},
    AllowCredentials: false,
})
```

---

## 13. Deployment Architecture

### 13.1 GCP Services
```
GCP Project: supernetwork-prod

Cloud Run: supernetwork-api
  Image: REGION-docker.pkg.dev/PROJECT/repo/api:SHA
  Min instances: 1
  Max instances: 3 (MVP)
  Session affinity: enabled (WS stickiness, best-effort)
  Request timeout: 3600s
  Startup probe: GET /readyz (30s timeout, 3 retries)
  Liveness probe: GET /healthz

Cloud Run: supernetwork-web (BFF)
  Image: REGION-docker.pkg.dev/PROJECT/repo/web:SHA
  Min instances: 1
  Startup probe: GET /healthz

Cloud Scheduler: match-refresh
  Schedule: 0 */6 * * * (every 6 hours)
  Target: POST https://api.supernetworkai.com/internal/matches/refresh
  Headers: X-Internal-Secret: <secret from Secret Manager>

Secret Manager: (all secrets referenced by Cloud Run env var bindings)
  supernetwork-supabase-url
  supernetwork-supabase-service-role-key
  supernetwork-database-url
  supernetwork-groq-api-key
  supernetwork-nomic-api-key
  supernetwork-ws-token-secret
  supernetwork-internal-api-secret
  supernetwork-cookie-secret
  supernetwork-supabase-anon-key

Artifact Registry: supernetwork-images (Docker repo)
```

### 13.2 Dockerfile — Go API
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o api ./main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/api /api
EXPOSE 3001
USER nonroot:nonroot
ENTRYPOINT ["/api"]
```

### 13.3 Dockerfile — BFF
```dockerfile
FROM oven/bun:1 AS builder
WORKDIR /app
COPY package.json bun.lockb ./
RUN bun install --frozen-lockfile
COPY . .
RUN bun run build      # vite → dist/public/

FROM oven/bun:1-slim
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/server ./server
COPY --from=builder /app/node_modules ./node_modules
EXPOSE 3000
# No HEALTHCHECK — Cloud Run uses its own startup/liveness probes (GET /healthz).
# Docker HEALTHCHECK is ignored by Cloud Run and bun:1-slim may not have curl.
CMD ["bun", "run", "server/index.ts"]
```

### 13.4 Environment Variables (complete, no ambiguity)

**Go API** (`backend/.env.example`):
```env
PORT=3001
SUPABASE_URL=https://xxxx.supabase.co
SUPABASE_SERVICE_ROLE_KEY=eyJ...
DATABASE_URL=postgresql://postgres.[ref]:[pw]@aws-0-[region].pooler.supabase.com:6543/postgres
GROQ_API_KEY=gsk_...
EMBEDDING_PROVIDER=ollama                  # ollama | nomic
OLLAMA_URL=http://localhost:11434
NOMIC_API_KEY=                             # required when EMBEDDING_PROVIDER=nomic
BFF_ORIGIN=http://localhost:3000
WS_TOKEN_SECRET=<32-char random hex>
INTERNAL_API_SECRET=<32-char random hex>   # X-Internal-Secret header for /internal/* routes
LOG_LEVEL=info                             # debug | info | warn | error
DB_MAX_CONNS=10                            # pgxpool max connections (Supabase free tier: 60 total)
DB_MIN_CONNS=2                             # pgxpool min warm connections
```

**BFF** (`frontend/.env.example`):
```env
PORT=3000
API_BASE_URL=http://localhost:3001
SUPABASE_URL=https://xxxx.supabase.co
SUPABASE_ANON_KEY=eyJ...
COOKIE_SECRET=<32-char random hex>         # signs cookie integrity
NODE_ENV=development
```

### 13.5 `package.json` scripts (frontend — explicit)
```json
{
  "scripts": {
    "dev":       "concurrently \"bun run dev:bff\" \"bun run dev:vite\"",
    "dev:bff":   "bun --watch server/index.ts",
    "dev:vite":  "vite dev",
    "build":     "vite build",
    "start":     "bun run server/index.ts",
    "typecheck": "tsc --noEmit",
    "lint":      "eslint server/ client/"
  }
}
```

---

## 14. Development Environment Setup

### Prerequisites (complete, in order)
```bash
# 1. Go 1.23+
go version  # verify

# 2. Bun 1.x
curl -fsSL https://bun.sh/install | bash

# 3. air (Go hot reload)
go install github.com/air-verse/air@latest

# 4. golang-migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 5. sqlc CLI
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# 6. swag CLI (swagger codegen)
go install github.com/swaggo/swag/cmd/swag@latest

# 7. Ollama
# macOS:  brew install ollama
# Linux:  curl -fsSL https://ollama.com/install.sh | sh
ollama pull nomic-embed-text
# Verify: curl http://localhost:11434/api/tags

# 8. Docker Desktop (for docker-compose Ollama in dev)
```

### First-time Setup
```bash
git clone <repo>
cd supernetwork-ai

# Terminal 1: Ollama (or use docker-compose up ollama)
ollama serve

# Terminal 2: Go API
cd backend
cp .env.example .env
# Fill in: SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, DATABASE_URL, GROQ_API_KEY
make migrate-up    # apply all migrations to Supabase
make swagger       # generate docs/swagger.json
make dev           # hot reload with air on :3001

# Terminal 3: Frontend BFF
cd frontend
cp .env.example .env
# Fill in: API_BASE_URL, SUPABASE_URL, SUPABASE_ANON_KEY
bun install
bun run dev        # Hono BFF on :3000 + Vite HMR
```

### `.air.toml` (Go hot reload config)
```toml
[build]
  cmd = "go build -o ./tmp/api ./main.go"
  bin = "./tmp/api"
  delay = 1000
  include_ext = ["go"]
  exclude_dir = ["docs", "tmp", "vendor"]
[log]
  time = true
```

### `docker-compose.yml`
```yaml
version: '3.9'
services:
  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama_data:/root/.ollama
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:11434/api/tags"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  ollama_data:

# After first start, pull model:
# docker exec -it <container_name> ollama pull nomic-embed-text
```

---

## 15. Implementation Phases

Each phase has: a single demonstrable goal, explicit test criteria, a git tag on completion, and a CHANGELOG entry. No phase begins until the previous phase's test criteria pass.

| Phase | Name | Git Tag | Demonstrates |
|---|---|---|---|
| 0 | Foundation | `phase/0-foundation` | Both servers start, health endpoints live, DB migrated |
| 1 | Authentication | `phase/1-auth` | Sign up → login → dashboard → logout |
| 2 | Onboarding & Profiles | `phase/2-profiles` | Complete 5-step onboarding, view own profile |
| 3 | CV Import | `phase/3-cv-import` | Upload PDF → fields auto-populated |
| 4 | Match Discovery | `phase/4-matches` | Two users see each other ranked on Discover page |
| 5 | AI Search & Explanations | `phase/5-ai-search` | NL query returns results + on-demand explanation |
| 6 | Connections | `phase/6-connections` | Send → accept → see in connections list |
| 7 | Real-Time Messaging | `phase/7-messaging` | Two tabs, messages appear live |
| 8 | Privacy & Safety | `phase/8-privacy` | Block user → disappears from all results |
| 9 | Polish & Hardening | `phase/9-polish` | Rate limits, empty states, mobile-ready |
| 10 | GCP Deployment | `phase/10-deploy` | App live at public URL, monitoring active |

---

### Phase 0 — Foundation (Week 1)
**Goal**: Both servers start and respond to health checks. DB schema created. Docker builds. No user-facing UI yet.

**Test criteria** (all must pass before tagging):
```
curl localhost:3001/healthz  →  {"status":"ok"}
curl localhost:3001/readyz   →  {"status":"ready"} or {"status":"db_unavailable"} (no crash)
curl localhost:3000/healthz  →  {"status":"ok"}
docker build backend/ -t api:phase0  →  build succeeds, image < 30MB
docker build frontend/ -t web:phase0 →  build succeeds
make migrate-up (with real DB creds) →  all 9 migrations applied
```

- [ ] Go: `go mod init`, Fiber v3, recovery+cors+logger middleware
- [ ] Go: `internal/config/config.go` — env loading + fail-fast
- [ ] Go: `internal/model/errors.go` — AppError + all error codes
- [ ] Go: `internal/health/handler.go` — /healthz + /readyz
- [ ] Go: `internal/db/client.go` — pgxpool (SimpleProtocol, startup ping)
- [ ] Go: All 9 DB migration files (up + down)
- [ ] Go: `Makefile`, `.air.toml`, `sqlc.yaml`, `Dockerfile`
- [ ] Go: `.env.example` with all required vars
- [ ] BFF: Hono app, `Bun.serve()`, /healthz route
- [ ] BFF: `server/lib/eta.ts` — Eta engine setup
- [ ] BFF: `package.json` scripts (dev, build, start, typecheck)
- [ ] BFF: `tsconfig.json`, `vite.config.ts`, `Dockerfile`
- [ ] BFF: `.env.example`
- [ ] Root: `.gitignore`, `docker-compose.yml` (Ollama)

**Git tag**: `phase/0-foundation`

---

### Phase 1 — Authentication (Week 2)
**Goal**: Users can sign up, log in, see an empty dashboard, and log out. Session cookies work. Onboarding guard redirects incomplete users.

**Test criteria**:
```
POST /auth/signup (new email)  →  redirects to /onboarding/step1
POST /auth/login  (valid creds) →  HX-Redirect to /dashboard
GET  /dashboard  (no cookie)   →  redirects to /login
GET  /auth/logout              →  cookies cleared, redirects to /
BFF → Go API (Bearer)          →  Go API returns 200 (JWT verified)
```

- [ ] Go: `POST /api/v1/auth/ws-token` endpoint
- [ ] Go: JWT auth middleware (JWKS fetch + 30min cache)
- [ ] Go: Swagger plugin wired (docs at /swagger)
- [ ] Go: `swag init` generates docs/swagger.json
- [ ] BFF: `/auth/login`, `/auth/logout`, `/auth/signup` routes
- [ ] BFF: `server/middleware/session.ts` — cookie read/write + refresh mutex
- [ ] BFF: `server/middleware/auth.ts` — requireAuth guard
- [ ] BFF: Onboarding guard on authenticated page routes
- [ ] UI: Base layout (Bootstrap CSS, HTMX, Alpine + morph plugin CDN)
- [ ] UI: Landing page, login form, signup form
- [ ] UI: Empty dashboard page (authenticated shell)

**Git tag**: `phase/1-auth`

---

### Phase 2 — Onboarding & Profiles (Week 3–4)
**Goal**: New user completes 5-step onboarding. Profile is saved to DB with embedding generated. Profile page viewable.

**Test criteria**:
```
Complete all 5 onboarding steps    →  profile.onboarding_complete = true
View own profile (/profile/me)     →  all fields shown correctly
View another user's public profile →  renders (visibility check passes)
profile.embedding_status           →  changes to 'current' within 5s of save
GET /api/v1/onboarding/skill-suggestions?context=... → returns suggestions array
```

- [ ] Go: sqlc generate for users + profiles queries
- [ ] Go: `GET /api/v1/users/me`, `PATCH /api/v1/profiles/me`, visibility endpoint
- [ ] Go: `GET /api/v1/users/:id` (visibility + block enforcement)
- [ ] Go: `POST /api/v1/onboarding/ikigai` (save + Groq summary)
- [ ] Go: `GET /api/v1/onboarding/skill-suggestions`
- [ ] Go: `service/interfaces.go` — all interfaces
- [ ] Go: `service/embedding/` — provider interface + Ollama impl + `BuildEmbeddingText()`
- [ ] Go: Async embedding goroutine (WaitGroup + recover)
- [ ] BFF: 5-step onboarding HTMX flow + Alpine onboarding store
- [ ] UI: Each onboarding step as Eta partial template
- [ ] UI: Profile view page (own + others)
- [ ] UI: Profile edit (inline HTMX)

**Git tag**: `phase/2-profiles`

---

### Phase 3 — CV Import (Week 5)
**Goal**: Upload a PDF CV → Groq extracts structured data → onboarding form pre-filled.

**Test criteria**:
```
Upload text-based PDF → onboarding fields pre-populated
Upload image-only PDF → graceful error: "Please fill manually"
Upload >5MB file      → rejected before download
Invalid URL           → rejected at allowlist check
```

- [ ] Go: `POST /api/v1/files/presign` (Uploadthing)
- [ ] Go: `service/downloader.go` (SSRF-safe)
- [ ] Go: `service/pdf.go` (pdfcpu extraction)
- [ ] Go: `service/llm/cv.go` (Groq structuring, JSON mode)
- [ ] Go: `POST /api/v1/onboarding/import-cv`
- [ ] BFF + UI: File upload in onboarding step 5, pre-fill from response

**Git tag**: `phase/3-cv-import`

---

### Phase 4 — Match Discovery (Week 6)
**Goal**: Two users with different profiles see each other on the Discover page ranked by similarity.

**Test criteria**:
```
Create User A (cofounder intent, Python skills)
Create User B (cofounder intent, React skills)
User A visits /discover → User B appears in results with score > 0
Category filter "cofounder" → only cofounder matches shown
POST /api/v1/matches/:id/dismiss → match disappears from list
POST /internal/matches/refresh → recomputes cache (with X-Internal-Secret header)
```

- [ ] Go: `service/matching.go` — MatchService, cache computation, category algorithm
- [ ] Go: `service/embedding/nomic.go` — Nomic API embedding provider
- [ ] Go: `GET /api/v1/matches`, `POST /api/v1/matches/:id/dismiss`
- [ ] Go: `POST /internal/matches/refresh` (X-Internal-Secret secured)
- [ ] Go: Match cache triggered after embedding completes
- [ ] BFF + UI: Discover page, match cards, category tabs, filter panel
- [ ] UI: HTMX match list partial, dismiss button

**Git tag**: `phase/4-matches`

---

### Phase 5 — AI Search & Explanations (Week 7)
**Goal**: Natural language search returns relevant results. On-demand AI explanations generated for matches.

**Test criteria**:
```
POST /api/v1/search {"query": "React developer open to equity"} → returns relevant profiles
GET /api/v1/matches/:userId/explanation → returns 2-3 sentence explanation
Second call to explanation → returns same text (cached, no new Groq call)
Search with no results → UI shows "No matches found" empty state
```

- [ ] Go: `service/llm/search.go` — NLSearchParser
- [ ] Go: `service/llm/explain.go` — MatchExplainer
- [ ] Go: `POST /api/v1/search` full flow
- [ ] Go: `GET /api/v1/matches/:userId/explanation` with caching
- [ ] BFF + UI: NL search bar (HTMX, 500ms debounce), search results partial
- [ ] UI: "Why this match?" lazy-load accordion (HTMX on click)

**Git tag**: `phase/5-ai-search`

---

### Phase 6 — Connections (Week 8)
**Goal**: Users can send, accept, and reject connection requests. Profile pages show correct action button.

**Test criteria**:
```
User A sends connection to User B  →  User B sees pending request
User B accepts                     →  status = 'accepted' for both
User B rejects                     →  status = 'rejected'
GET /connections/status/:userId    →  returns correct status
Profile page (unconnected user)    →  shows "Connect" button
Profile page (connected user)      →  shows "Message" button
Duplicate connection request       →  409 CONFLICT
```

- [ ] Go: sqlc queries for connections
- [ ] Go: `POST/GET/PATCH /api/v1/connections`
- [ ] Go: `GET /api/v1/connections/status/:userId`
- [ ] BFF + UI: Connection request button on match cards + profile pages
- [ ] UI: Connections list page (accepted + pending tabs)
- [ ] UI: Accept/reject actions (HTMX swap)

**Git tag**: `phase/6-connections`

---

### Phase 7 — Real-Time Messaging (Week 9)
**Goal**: Accepted connections can exchange real-time messages. History persists.

**Test criteria**:
```
Open /messages in two tabs (User A + User B)
User A sends message → appears instantly in User B's tab (no refresh)
Refresh page → message history still shows
Message > 4000 chars → rejected with WS error
Unconnected users    → WS join_room returns NOT_CONNECTED error
GET /conversations/:id/messages?before=<ts> → returns older messages (pagination)
```

- [ ] Go: sqlc queries for conversations + messages
- [ ] Go: `GET/POST /api/v1/conversations`, message history endpoint, mark-read
- [ ] Go: WS hub (`internal/ws/hub.go`) + handler with first-message auth
- [ ] Go: Graceful shutdown: hub.Stop() in SIGTERM handler
- [ ] BFF: `GET /partials/ws-token` route
- [ ] BFF + UI: Messages page, Eta renders history
- [ ] UI: Alpine chat store (connect, auth, join, send, receive, reconnect with backoff)

**Git tag**: `phase/7-messaging`

---

### Phase 8 — Privacy & Safety (Week 10)
**Goal**: Users can block others, set profiles private, and delete accounts. Blocked users disappear from all results.

**Test criteria**:
```
User A blocks User B            →  User B absent from A's matches, search, profile lookup
User A private, B not connected →  User B gets 403 on A's profile
User A private, B IS connected  →  User B can view A's profile (accepted connection grants access)
User A private                  →  User A absent from all search/match results
DELETE /account       →  all user data deleted (profile, messages, connections)
POST /blocks (already blocked) → 409 CONFLICT
```

- [ ] Go: `POST/DELETE /api/v1/blocks`
- [ ] Go: Verify ALL search + match + profile queries enforce block exclusion (both directions)
- [ ] Go: `DELETE /api/v1/account` — cascade deletion + Uploadthing file removal
- [ ] BFF + UI: Block user action on profile + message pages
- [ ] UI: Profile visibility toggle, confirmation dialogs

**Git tag**: `phase/8-privacy`

---

### Phase 9 — Polish & Hardening (Week 11)
**Goal**: Rate limiting enforced. All edge cases have UI feedback. Mobile-ready. Password reset works.

**Test criteria**:
```
POST /api/v1/search > 30/min  → 429 RATE_LIMITED
Empty discover page           → shows "No matches yet" with helpful copy
Loading state during HTMX     → skeleton/spinner visible
Mobile viewport (375px)       → no horizontal scroll, all elements accessible
Password reset email          → received + link works
```

- [ ] Go: Rate limiting on all endpoints (§12.3)
- [ ] Go: Prompt injection hardening (fixed system prompts)
- [ ] Supabase dashboard: configure password reset redirect URL (e.g. https://app.supernetworkai.com/auth/confirm)
- [ ] UI: Loading skeletons (hx-indicator on all requests)
- [ ] UI: Empty states for all list pages
- [ ] UI: Pending connection badge on nav (HTMX polling every 30s via GET /api/v1/connections?status=pending)
- [ ] UI: Error pages (404, 500, maintenance)
- [ ] UI: Mobile responsiveness audit + fixes

**Git tag**: `phase/9-polish`

---

### Phase 10 — GCP Deployment (Week 12)
**Goal**: Both services live at public URLs. Monitoring active. CI/CD pipeline running.

**Test criteria**:
```
https://app.supernetworkai.com   → loads landing page
https://api.supernetworkai.com/healthz → {"status":"ok"}
https://api.supernetworkai.com/swagger → Swagger UI loads
Deploy new commit → GitHub Actions builds + deploys automatically
Cloud Scheduler fires → /internal/matches/refresh returns 200
```

- [ ] GCP: Artifact Registry + push both images via GitHub Actions
- [ ] GCP: Deploy Go API Cloud Run (min-instances=1, session affinity, 3600s timeout)
- [ ] GCP: Deploy BFF Cloud Run (min-instances=1)
- [ ] GCP: All secrets in Secret Manager, bound to Cloud Run
- [ ] GCP: Cloud Scheduler for match refresh
- [ ] GCP: DNS + custom domains
- [ ] Supabase: Production project (Pro tier)
- [ ] Monitoring: Cloud Logging metrics + alerting on error rate

**Git tag**: `phase/10-deploy` = `v1.0.0`

---

## 16. Known Risks & Open Items

| Risk | Severity | Mitigation |
|---|---|---|
| Cloud Run WS session affinity is best-effort | HIGH | `min-instances=1` for MVP (single instance = no fan-out issue). Redis pub/sub planned for v2 when horizontal scaling needed. |
| Groq rate limit on free tier (6k TPM) | HIGH | Use paid tier in prod. Retry with exponential backoff + jitter on 429. |
| Supavisor transaction mode: no prepared statements | HIGH | `pgx.QueryExecModeSimpleProtocol` set on pool. All sqlc queries tested in transaction mode before deploy. |
| pdfcpu fails on image-only (scanned) CVs | MEDIUM | Graceful fallback to manual entry. Logged for product team review. |
| Nomic API unavailability in prod | MEDIUM | Embedding is async; `embedding_status='failed'` on error. Cloud Scheduler re-tries every 6h. |
| WS connections dropped on rolling deploy | MEDIUM | Alpine reconnect with backoff handles this transparently. |
| In-memory rate limiting breaks at >1 instance | MEDIUM | Documented constraint; only safe with `min-instances=1`. Redis limiter needed before scaling. |
| Token refresh race condition (concurrent 401s) | MEDIUM | BFF mutex on refresh (§8.4) prevents multi-refresh. |
| swaggo/swag generates Swagger 2.0 not OpenAPI 3.0 | LOW | Accepted for MVP. Add `swagger2openapi` post-processing if OAS3 needed for SDK gen. |
| Embedding dimension locked at 768 | LOW | Documented. Any model switch requires re-embedding all profiles + schema migration. Review at 6-month mark. |
| GDPR compliance | LOW | Account deletion cascades all data. Privacy policy + cookie notice needed before public launch. |
| CV/profile export | LOW | Deferred to post-MVP. problem.md scope says 'import/export' — only CV import implemented in MVP. Export feature tracked for v2. |
