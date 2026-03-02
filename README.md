# SuperNetworkAI

> **Ikigai-powered AI networking** — find cofounders, teammates, and clients through meaningful self-discovery, not keyword matching.

SuperNetworkAI uses your **Ikigai** (what you love × what you're good at × what the world needs × what you can be paid for) as its core matching signal. The AI reads between the lines — understanding intent, not just titles — and explains every match in plain English.

---

## Why It Exists

Online communities waste their collective potential. Rich context — skills, goals, passions — stays invisible while networking remains superficial and slow. SuperNetworkAI surfaces the right people at the right time through AI that understands *purpose*, not just proximity.

---

## Feature Highlights

| Feature | Detail |
|---|---|
| **Ikigai Onboarding** | 4-question self-discovery wizard powers all matching |
| **CV Import** | Paste a PDF URL — LLM extracts skills, bio, and links automatically |
| **Vector Matching** | 768-dim embeddings (nomic-embed-text) + pgvector HNSW index |
| **AI Match Explanations** | Groq LLaMA 70B generates why two people fit, on demand |
| **Natural Language Search** | "React developer open to equity" → structured vector search |
| **Category Matching** | Ranked by cofounder / teammate / client intent |
| **Real-Time Messaging** | WebSocket hub with conversation threads and unread counts |
| **Connection Graph** | Request → accept → message flow with block controls |
| **Privacy Controls** | Public / private profile visibility per user |

---

## Architecture

```
Browser (HTMX + Alpine.js + Bootstrap 5)
  │  HTML partials only          │  WebSocket only
  ▼                              ▼
Bun BFF (Hono + Eta)  ──JWT──▶  Go API (Fiber v3, :3001)
  :3000                            │
                                   ├─ Supabase PostgreSQL + pgvector
                                   ├─ Groq API  (LLaMA 3.1 8B / 3.3 70B)
                                   ├─ Nomic / Ollama  (embeddings, 768-dim)
                                   └─ Uploadthing  (file storage)
```

**Hard boundaries enforced in code:**
- Browser → BFF: HTTP only (HTMX partial swaps — zero JSON to browser)
- Browser → Go API: Native WebSocket only (real-time chat)
- BFF → Go API: JSON REST with `Authorization: Bearer <HS256-JWT>`
- Firebase Auth → BFF: Session exchange; Go API never sees Firebase tokens

---

## Tech Stack

**Backend (Go)**
- [Fiber v3](https://gofiber.io/) — HTTP framework
- [pgx v5](https://github.com/jackc/pgx) + [pgvector-go](https://github.com/pgvector/pgvector-go) — Postgres + vector queries
- [golang-migrate](https://github.com/golang-migrate/migrate) — schema migrations
- [lestrrat-go/jwx](https://github.com/lestrrat-go/jwx) — JWT validation
- [Groq API](https://groq.com/) — ultra-fast LLM inference
- [Nomic Embed](https://www.nomic.ai/) / [Ollama](https://ollama.ai/) — local & cloud embeddings

**Frontend (TypeScript / Bun)**
- [Hono](https://hono.dev/) — edge-native BFF framework
- [Eta](https://eta.js.org/) — server-side templating
- [HTMX 2](https://htmx.org/) — server-driven HTML
- [Alpine.js](https://alpinejs.dev/) — local UI state
- [Bootstrap 5](https://getbootstrap.com/) — CSS (no Bootstrap JS)
- [Vite](https://vitejs.dev/) — client asset bundling

**Infrastructure**
- [Supabase](https://supabase.com/) — PostgreSQL + pgvector hosting
- [Firebase Auth](https://firebase.google.com/products/auth) — identity provider
- [GCP Cloud Run](https://cloud.google.com/run) — containerised deployment target

---

## Project Status

| Phase | Status |
|---|---|
| 0 — Foundation | ✅ |
| 1 — Authentication | ✅ |
| 2 — Onboarding & Profiles | ✅ |
| 3 — CV Import | ✅ |
| 4 — Match Discovery | ✅ |
| 5 — AI Search & Explanations | ✅ |
| 6 — Connections | ✅ |
| 7 — Real-Time Messaging | ✅ |
| 8 — Privacy & Safety | ✅ |
| 9 — Polish & Hardening | 🔄 In Progress |
| 10 — GCP Deployment | ⬜ |

---

## Quick Start

See [QUICK_START.md](./QUICK_START.md) for full setup instructions.

```bash
# Clone
git clone https://github.com/theoriginalaiexplorer/SuperNetworkAI.git
cd SuperNetworkAI

# Backend
cp backend/.env.example backend/.env   # fill in secrets
cd backend && make migrate-up && make dev

# Frontend (new terminal)
cp frontend/.env.example frontend/.env  # fill in secrets
cd frontend && bun install && bun run dev
```

Open **http://localhost:3000** — sign up and complete onboarding to start matching.

---

## Repository Layout

```
SuperNetworkAI/
├── backend/                    # Go API
│   ├── db/
│   │   ├── migrations/         # 9 up/down SQL migrations
│   │   └── queries/            # sqlc query definitions
│   ├── internal/
│   │   ├── handler/            # HTTP request handlers
│   │   ├── middleware/         # Auth, CORS, logger, recovery
│   │   ├── model/              # Shared types, errors
│   │   ├── service/            # Matching, embedding, downloader
│   │   └── ws/                 # WebSocket hub
│   ├── main.go
│   └── Makefile
└── frontend/                   # Bun BFF
    ├── client/                 # Vite entry (Alpine.js + HTMX bundle)
    └── server/
        ├── lib/                # API client, Eta config, render helper
        ├── middleware/         # Session, auth
        ├── routes/             # Hono route handlers
        └── templates/          # Eta server-side templates
            ├── layouts/
            ├── pages/
            └── partials/
```

---

## API Overview

Base URL: `http://localhost:3001`

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Liveness probe |
| GET | `/readyz` | Readiness (DB + embeddings) |
| GET | `/api/v1/users/me` | Own user + profile + ikigai |
| GET | `/api/v1/users/:id` | Another user's public profile |
| PATCH | `/api/v1/profiles/me` | Update profile fields |
| POST | `/api/v1/onboarding/ikigai` | Save Ikigai answers |
| POST | `/api/v1/onboarding/import-cv` | Import CV from PDF URL |
| POST | `/api/v1/onboarding/complete` | Mark onboarding complete |
| GET | `/api/v1/matches` | Paginated match list |
| GET | `/api/v1/matches/:id/explanation` | AI match explanation |
| POST | `/api/v1/search` | Natural language search |
| GET | `/api/v1/connections` | Connection list |
| POST | `/api/v1/connections/request/:id` | Send connection request |
| PATCH | `/api/v1/connections/:id` | Accept / decline |
| GET | `/api/v1/conversations` | Conversation list |
| GET | `/api/v1/conversations/:id/messages` | Message history |
| POST | `/api/v1/blocks` | Block a user |
| WS | `/ws` | Real-time messaging |

Full contract in [PLAN.md](./PLAN.md) §7.

---

## Contributing

1. Fork and create a branch from `main`
2. Run `go test ./...` (backend) and `bun run typecheck` (frontend) before committing
3. Follow the error code convention: `AppError` with `SNAKE_CASE` codes
4. Keep the browser↔BFF↔API separation hard — no REST calls from the browser

---

## License

MIT
