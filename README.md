# 🐰 Bugsbunny — Bug Tracking, Enforced

A full-stack bug/issue tracker built around a **real, server-enforced lifecycle engine** — not a flat list of tickets with a status dropdown you can set to anything. Every state change, assignment, and relationship is validated, logged, and explainable after the fact.

Built for CloneFest — Legacy Modernisation: PrivateBin/Bug Tracker challenge.

**Demo login:** `admin@bugsbunny.local` / `admin123`

---

## Why this stands out

Most student bug trackers are CRUD wrappers around a `status` column. Bugsbunny enforces the entire lifecycle **server-side**, keeps a full audit trail of everything that happens to an issue, and ships five deterministic intelligence features — **zero external AI/API dependency, nothing to fail at demo time** — that most trackers, student or commercial, don't bother building:

| Feature | What it actually does |
|---|---|
| 🧬 **Bug DNA** | Clusters open bugs by component + shared keywords to surface *where* the codebase is actually fragile |
| 💥 **Blast Radius** | Walks the `blocks` / `blocked-by` graph to show exactly what unblocks if a given bug is resolved |
| ⏱ **SLA Breach Forecast** | Predicts a breach *before* it happens, using severity/priority-weighted deadlines |
| 🕰 **Time-Travel Replay** | Reconstructs an issue's exact state at any past moment by replaying its own audit log |
| 📋 **Auto Summary Report** | One-click, copy-ready incident summary generated from real activity data |

Plus a **WASM plugin architecture** (via [wazero](https://wazero.io)): third-party logic — like our SLA checker — runs sandboxed *inside* the Go server itself, compiled ahead-of-time, no external microservice or exec() call needed.

---

## Core Features

- **Enforced lifecycle**: `OPEN → TRIAGED → IN PROGRESS → RESOLVED → VERIFIED → CLOSED`, with reopening. Invalid transitions are rejected server-side, not just hidden in the UI.
- Full field set: title, description, **priority** (urgency) and **severity** (impact) kept intentionally distinct, component, environment, module, target release.
- Assignment, labels, watch/follow.
- **Relationships**: related-to, duplicate-of, blocks, blocked-by — self-links and duplicate entries are rejected.
- **Comments** with `@mention` detection, logged to the activity feed.
- **Attachments** with upload/download.
- **Complete audit trail** — every field change, status transition, assignment, comment, and relationship is logged with actor, old value, new value, and timestamp. This audit log is what powers Time-Travel Replay and the Auto Summary Report — it's a first-class part of the data model, not an afterthought bolted on for one feature.
- Project-level dashboard with live bug metrics (open / critical / overdue / aging).
- Role-based admin panel: user management, password resets, project archiving.

---

## Tech Stack

**Backend:** Go (stdlib `net/http`, no framework) · PostgreSQL · JWT auth · [pgx/v5](https://github.com/jackc/pgx) · [wazero](https://wazero.io) WASM runtime
**Frontend:** Vue 3 (Composition API) · Vite · vue-router
**Infra:** Docker (backend + Postgres)

## Architecture

```
bugsbunny-ui (Vue 3, Vite)  ──HTTP/JSON──►  bugsbunny (Go REST API)  ──►  PostgreSQL
                                                    │
                                                    └──► wazero WASM runtime
                                                              └── sla-checker.wasm
```

- Backend is a single Go binary — no framework, using Go 1.22+'s native `net/http` pattern-matching router (`GET /issues/{id}`, etc).
- Every mutating endpoint writes to `activity_stream`, which every insight feature reads from.
- Frontend talks to the backend exclusively through `VITE_API_URL` — no hardcoded hosts, portable across local / Codespaces / production.
- Config (`DATABASE_URL`, `PORT`, `JWT_SECRET`, `BASE_URL`) is environment-driven throughout, so the same binary runs locally and in the cloud unchanged.

---

## Running Locally

```bash
# 1. Start Postgres
cd bugsbunny
docker compose up -d

# 2. Load schema + seed data
docker exec -i bugsbunny-db psql -U postgres -d bugsbunny < schema.sql
docker exec -i bugsbunny-db psql -U postgres -d bugsbunny < seed.sql

# 3. Start the backend
go run .

# 4. Start the frontend (new terminal)
cd ../bugsbunny-ui
npm install
cp .env.example .env      # points VITE_API_URL at http://localhost:8080
npm run dev
```

---

## Deploying

The frontend and backend deploy separately — this is a normal split-stack app, not a Next.js monolith:

| Piece | Where | Why |
|---|---|---|
| `bugsbunny-ui` | Vercel / Netlify | Static Vite build, `vercel.json` included for SPA routing |
| `bugsbunny` (Go API) | Render / Railway / Fly.io | Needs a persistent process + Postgres connection — not a fit for Vercel's serverless functions. `Dockerfile` included |
| Postgres | Render/Railway managed Postgres, or Neon/Supabase | Any managed Postgres works — schema is plain SQL, no vendor lock-in |

**Backend environment variables** (set in your host's dashboard):

| Variable | Example | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://user:pass@host:5432/bugsbunny` | Falls back to local Docker Postgres if unset |
| `JWT_SECRET` | any long random string | Falls back to a dev default if unset — **set this in production** |
| `PORT` | set automatically by most hosts | Falls back to `8080` |
| `BASE_URL` | `https://your-backend.onrender.com` | Used to build absolute attachment-download links |

After the schema is loaded on your managed Postgres (`psql $DATABASE_URL < schema.sql` then `seed.sql`), deploy the backend from the `bugsbunny/Dockerfile`, then point the frontend's `VITE_API_URL` at the deployed backend URL and deploy `bugsbunny-ui` to Vercel.

---

## Where to Look (for judges)

| Page | What to check |
|---|---|
| Dashboard | Live metrics + create-issue form (all fields) |
| Issue Queue | Full list, filters, **Bug DNA** clustering |
| Any issue | Lifecycle edit, relationships, labels, watch, **7 insight buttons**, activity trail, comments |
| Admin | User & project management |

---

## Known Limitations

- No automated test suite yet.
- No analytics dashboards (charts/aging visualizations) — metrics are numeric, not charted.
- CORS is currently open (`*`) for demo convenience; a production deployment would restrict it to the frontend's origin.
