# Quick Start — SuperNetworkAI

Get both servers running locally in under 10 minutes.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.22+ | https://go.dev/dl |
| Bun | 1.1+ | `curl -fsSL https://bun.sh/install \| bash` |
| Docker | 24+ | https://docs.docker.com/get-docker |
| golang-migrate | latest | `go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |

---

## 1. Clone

```bash
git clone https://github.com/theoriginalaiexplorer/SuperNetworkAI.git
cd SuperNetworkAI
```

---

## 2. External Services Setup

You need accounts for three free-tier services before running anything.

### Firebase (Authentication)

1. Go to [Firebase Console](https://console.firebase.google.com/) → Create project
2. Enable **Authentication → Sign-in method → Email/Password**
3. Project Settings → General → note **Project ID** and **API Key** (`FIREBASE_API_KEY`, `FIREBASE_PROJECT_ID`)
4. Set **Auth Domain** = `<project-id>.firebaseapp.com`
5. In Authentication → Settings → **Authorized domains** — add `localhost`

### Supabase (PostgreSQL + pgvector)

1. Go to [Supabase](https://supabase.com/) → New project
2. SQL Editor → run: `CREATE EXTENSION IF NOT EXISTS vector;`
3. Project Settings → Database → note the **Connection string** (mode: URI) → `DATABASE_URL`
4. Project Settings → API → note **Project URL** and **anon key**

### Groq API (LLM — free tier)

1. Go to [Groq Console](https://console.groq.com/) → Create API Key → `GROQ_API_KEY`

### Nomic Embed (Embeddings — free tier for dev)

1. Go to [Nomic Atlas](https://atlas.nomic.ai/) → API Keys → `NOMIC_API_KEY`

> **Local alternative**: Use Ollama instead of Nomic (free, no account needed):
> ```bash
> docker compose up -d
> docker exec supernetwork_ollama ollama pull nomic-embed-text
> # Set EMBEDDING_PROVIDER=ollama in backend/.env
> ```

---

## 3. Backend Setup

```bash
cd backend
cp .env.example .env
```

Edit `backend/.env`:

```env
PORT=3001

# Supabase — from step 2
DATABASE_URL=postgresql://postgres:<password>@db.<ref>.supabase.co:5432/postgres

# Groq — from step 2
GROQ_API_KEY=gsk_...

# Embeddings — choose one
EMBEDDING_PROVIDER=nomic          # or: ollama
NOMIC_API_KEY=nk-...              # if using nomic
OLLAMA_BASE_URL=http://localhost:11434   # if using ollama

# Security — generate two random hex strings
WS_TOKEN_SECRET=<64-char hex>
INTERNAL_API_SECRET=<64-char hex>

# Shared with frontend — must match BFF_JWT_SECRET below
BFF_JWT_SECRET=<64-char hex>

# BFF origin for CORS
BFF_ORIGIN=http://localhost:3000
```

> Generate secrets: `openssl rand -hex 32`

### Run migrations

```bash
# Using pgx5 driver (handles Supabase IPv6 correctly)
migrate -database "pgx5://postgres:<password>@db.<ref>.supabase.co:5432/postgres" \
        -path db/migrations up
```

### Start the API

```bash
make dev        # hot reload via air
# OR
go run .        # no hot reload
```

Verify: `curl http://localhost:3001/healthz` → `{"status":"ok"}`

---

## 4. Frontend Setup

```bash
cd frontend
cp .env.example .env
bun install
```

Edit `frontend/.env`:

```env
PORT=3000
GO_API_URL=http://localhost:3001

# Firebase — from step 2
FIREBASE_API_KEY=AIza...
FIREBASE_AUTH_DOMAIN=<project-id>.firebaseapp.com
FIREBASE_PROJECT_ID=<project-id>

# Must match backend BFF_JWT_SECRET exactly
BFF_JWT_SECRET=<same 64-char hex as backend>
```

### Build client assets

```bash
bun run build
```

> After each `bun run build`, update the asset filename in `server/templates/layouts/base.eta`:
> ```html
> <script src="/public/assets/main-<hash>.js" defer></script>
> ```
> The hash is printed at the end of the build output.

### Start the BFF

```bash
bun run dev     # hot reload
```

Verify: `curl http://localhost:3000/healthz` → `{"status":"ok"}`

---

## 5. Open the App

Navigate to **http://localhost:3000**

### Full user lifecycle to test

1. **Sign up** at `/signup` with any email + password (min 8 chars)
2. **Onboarding** — 5 steps:
   - Step 1: Display name + tagline
   - Step 2: Ikigai answers (min 10 chars each — be specific!)
   - Step 3: Skills & interests
   - Step 4: Intent (cofounder / teammate / client) + availability
   - Step 5: Optional links (LinkedIn, GitHub, portfolio)
3. **Dashboard** — landing page post-onboarding
4. **Discover** — match list (needs at least 2 users with embeddings to show results)
5. **Connect** — send a connection request from a match card
6. **Messages** — chat after a connection is accepted

> Matches only appear after the embedding pipeline runs. This happens asynchronously (a few seconds) after you complete the Ikigai step. Refresh `/discover` after ~10 seconds.

---

## 6. Development Commands

### Backend (`cd backend`)

```bash
make dev            # hot reload (requires air: go install github.com/air-verse/air@latest)
make build          # production binary
make test           # go test ./...
make swagger        # regenerate Swagger docs
make migrate-up     # apply all pending migrations
make migrate-down   # rollback last migration
make sqlc           # regenerate db query code from SQL
```

### Frontend (`cd frontend`)

```bash
bun run dev         # hot reload BFF
bun run build       # compile client assets (Vite)
bun run typecheck   # tsc --noEmit
bun run lint        # eslint
```

### Embeddings (Ollama local)

```bash
docker compose up -d                              # start Ollama container
docker exec supernetwork_ollama ollama pull nomic-embed-text   # first run only
docker exec supernetwork_ollama ollama list       # verify model loaded
```

---

## 7. Environment Variables Reference

### Backend (`backend/.env`)

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No | API port (default: 3001) |
| `DATABASE_URL` | **Yes** | Postgres connection string |
| `GROQ_API_KEY` | **Yes** | Groq API key for LLM calls |
| `EMBEDDING_PROVIDER` | **Yes** | `nomic` or `ollama` |
| `NOMIC_API_KEY` | If nomic | Nomic Atlas API key |
| `OLLAMA_BASE_URL` | If ollama | Ollama base URL |
| `BFF_JWT_SECRET` | **Yes** | Shared secret for BFF JWT verification |
| `WS_TOKEN_SECRET` | **Yes** | HMAC secret for WebSocket auth tokens |
| `INTERNAL_API_SECRET` | **Yes** | Secret for Cloud Scheduler endpoints |
| `BFF_ORIGIN` | **Yes** | BFF URL for CORS (e.g. `http://localhost:3000`) |
| `UPLOADTHING_SECRET` | No | File upload token (for avatar uploads) |

### Frontend (`frontend/.env`)

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No | BFF port (default: 3000) |
| `GO_API_URL` | **Yes** | Go API base URL |
| `FIREBASE_API_KEY` | **Yes** | Firebase Web API key |
| `FIREBASE_AUTH_DOMAIN` | **Yes** | Firebase auth domain |
| `FIREBASE_PROJECT_ID` | **Yes** | Firebase project ID |
| `BFF_JWT_SECRET` | **Yes** | Must match backend `BFF_JWT_SECRET` |

---

## 8. Common Issues

**`migrate: dirty database version`**
```bash
migrate -database "pgx5://..." -path db/migrations force <last-clean-version>
migrate -database "pgx5://..." -path db/migrations up
```

**`/discover` shows 500 after login**
Template compilation error — restart the BFF: `pkill -f "bun.*server" && bun run dev`

**No matches on `/discover`**
- Embedding hasn't run yet — wait 10–15 seconds and refresh
- Check backend logs for embedding errors (`NOMIC_API_KEY` or Ollama not reachable)
- Need at least 2 users with completed onboarding and successful embeddings

**Login redirects back to `/login`**
The `sn_access` cookie wasn't set. Check BFF logs for `[AUTH] Login failed` with the Firebase error code.

**WebSocket disconnects immediately**
`WS_TOKEN_SECRET` mismatch between frontend and backend, or the 60-second WS token TTL expired — navigate to `/messages` again to get a fresh token.

---

## 9. Architecture Deep Dive

See [PLAN.md](./PLAN.md) for:
- Full API contract (§7)
- Database schema with all 9 migrations (§6)
- Authentication & session design (§8)
- Security design (§12)
- Deployment architecture for GCP Cloud Run (§13)
