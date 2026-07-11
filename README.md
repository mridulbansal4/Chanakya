# CHANAKYA

**A Regulatory Operating System for the Indian securities market — from regulatory text to operational action.**

Built for the SEBI Securities Market TechSprint, Problem Statement 2 (*Agentic
Compliance*). CHANAKYA turns a SEBI circular into a living, bi-temporal graph of
typed **obligations**, maps them to **controls** and **evidence**, computes the
**blast radius** of amendments, routes low-confidence extractions to a **human
review queue**, and — only after a cryptographic sign-off — compiles each
obligation into a deterministic **policy** enforced in staged mode.

> **Safety model (non-negotiable).** The LLM only ever emits *data* validated
> against a strict JSON schema — never code, never enforcement. Enforcement is
> done solely by the deterministic OPA/Rego engine, and only after a human has
> Ed25519-signed the obligation. Evidence connectors are **read-only**.
> Enforcement is staged **audit → soft → hard**. Every obligation must carry a
> causal citation (source clause id + exact sentence) or it is rejected.

---

## Stack

| Layer      | Choice                                                             |
| ---------- | ----------------------------------------------------------------- |
| Backend    | Go (`chi` router), single module in `./backend`                   |
| Storage    | **SQLite** via `modernc.org/sqlite` (pure Go, no cgo, no Docker)  |
| Migrations | Embedded `.sql` via `go:embed`, applied in-process on boot        |
| Frontend   | Next.js 16 monorepo (Turborepo), shadcn/Base UI, Tailwind v4      |
| Data       | TanStack Query · (graph/table/motion added in later phases)       |

There is **no Docker, no Postgres, no external service**. The entire system of
record is a single file, `./chanakya.db`, created on first run.

---

## Prerequisites (Windows)

- **Go 1.24+** — `winget install --id GoLang.Go -e` (then open a fresh terminal)
- **Node 20+** and npm

Verify: `go version` and `node -v`.

---

## Run it

From the repository root (`C:\Projects\SEBI\CHANAKYA`), **two commands**:

```powershell
# Terminal 1 — backend. On first run it creates ./chanakya.db AND self-seeds
# the SEBI IA circular (seeds + compiles the fixture). Serves :8080.
go run ./backend/cmd/api

# Terminal 2 — web app (http://localhost:3000)
npm install    # first time only
npm run dev
```

That's it — the app is fully seeded and working. Or start both at once with
`.\dev.ps1`. Then open http://localhost:3000 and follow [DEMO.md](DEMO.md).

### Manual seed / compile (optional)

The API self-seeds, so these are only needed to (re)seed explicitly or to use
the real LLM extractor:

```powershell
go run ./backend/cmd/seed      # load the 12-clause fixture; prints the clause tree
go run ./backend/cmd/compile   # extract typed, cited obligations; wire controls; draft tickets
```

`compile` uses the **deterministic offline extractor** by default (no API key);
set `CHANAKYA_LLM_API_KEY` to use the real Anthropic strict-tool-use extractor
instead — both feed the identical strict-schema validation path.

Or start both at once:

```powershell
.\dev.ps1
```

Check health:

```powershell
Invoke-RestMethod http://localhost:8080/health | ConvertTo-Json
```

Expected:

```json
{ "status": "ok", "version": "0.0.0-dev",
  "checks": { "database": { "ok": true, "error": "" } } }
```

The web app's header shows a live **Backend online** indicator (teal) that polls
`/health` every 5 seconds.

---

## Configuration

All configuration is via environment variables (see `backend/.env.example` and
`apps/web/.env.example`). No secrets live in code.

| Variable                    | Default                 | Purpose                        |
| --------------------------- | ----------------------- | ------------------------------ |
| `CHANAKYA_ADDR`             | `:8080`                 | Backend listen address         |
| `CHANAKYA_DB_PATH`          | `./chanakya.db`         | SQLite file path               |
| `CHANAKYA_CORS_ORIGINS`     | `http://localhost:3000` | Allowed browser origins        |
| `NEXT_PUBLIC_API_BASE_URL`  | `http://localhost:8080` | Backend URL used by the web app |

---

## Repository layout

```
CHANAKYA/
├─ go.work                 # workspace so `go run ./backend/...` works from root
├─ backend/                # Go module (module path: chanakya)
│  ├─ cmd/api/             # HTTP server entrypoint
│  ├─ db/migrations/       # embedded .sql migrations
│  └─ internal/
│     ├─ config/           # env-only configuration loader
│     ├─ store/            # SQLite open + migration runner + queries
│     └─ httpapi/          # chi router, middleware, handlers
├─ apps/web/               # Next.js app (health indicator, screens)
└─ packages/ui/            # shared shadcn/Base UI components + design tokens
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for the design, the safety model, and the
bi-temporal data model.

---

## Working with UI components

Shared components live in `packages/ui`. Add more with the shadcn CLI (do **not**
re-init):

```powershell
npx shadcn@latest add table -c apps/web
```

Import them from the `@workspace/ui` package:

```tsx
import { Button } from "@workspace/ui/components/button"
```

---

## Phase status

- **Phase 0 ✅** — backend + `/health` (SQLite, WAL, foreign keys, embedded
  migrations); typed API client + live health indicator; "Operational Ink"
  design tokens.
- **Phase 1 ✅** — bi-temporal graph schema (circular, clause tree, entity,
  obligation, control, evidence + edges); parameterized store methods with a
  recursive-CTE clause traversal; `seed` command loading the IA fixture.
- **Phase 2 ✅** — Regulation Compiler: pluggable extractor (deterministic
  offline default + real Anthropic strict-tool-use client), strict JSON-schema
  validation, mandatory verbatim citation, confidence-based review routing;
  `compile` command.
- **Phase 3 ✅** — read-only graph API (`/api/obligations`, `/api/obligation`,
  `/api/graph`, `/api/posture`, all honouring `?as_of=`); Command Overview
  (posture strip + React Flow graph hero) and Obligation Register (TanStack
  Table, filters, as-of control, row → detail with citation) screens.
- **Phase 4 ✅** — Amendment / Blast Radius: in-Go embeddings + cosine diff of an
  amended clause against every obligation, traversal to affected controls +
  evidence, `POST /api/amendments/blast-radius` (a read-only what-if), and the
  animated Blast Radius screen. The `compile` command now also wires the firm
  controls/evidence layer.
- **Phase 5 ✅** — Evidence, Gaps & Tickets: gap detection over
  obligation→control→evidence, DRAFT remediation tickets (state=draft, never
  filed), `/api/evidence` + `/api/tickets`, and the Evidence & Gaps screen.
  Read-only connectors; clause 5.2 (client-notification) is a seeded gap.
- **Phase 6 ✅** — HITL Review Queue + Ed25519 sign-off: review queue,
  deliberate multi-step sign-off modal with mandatory typed justification,
  Ed25519 signature over a canonical obligation hash, and live verification
  (tampering fails). `/api/review-queue`, `/api/signoff`.
- **Phase 7 ✅** — Policy-as-Code: a *signed* obligation compiles to a
  deterministic **Rego** policy, evaluated against firm-state input by embedded
  **OPA** (Go lib) with a trace; staged **audit → soft → hard** enforcement
  (only hard blocks). `/api/policies`, `/api/policy/*`, `/api/firm-state`.
- **Phase 8 ✅** — Bi-temporal Audit Lineage + Regulator Feed: reconstruct the
  clause→obligation→control→evidence→sign-off→policy chain as-of any date (a past
  date correctly shows obligations unsigned/unenforced), and a versioned,
  schema-validated machine-readable SupTech feed with provenance.
  `/api/lineage`, `/api/feed`, `/api/feed/schema`.
- **Phase 9 ✅** — Polish + demo: API **self-bootstraps** the seeded fixture on
  first run (two-command startup), per-IP **rate limiting**, input validation on
  every handler, `dev.ps1`, and a 90-second walkthrough ([DEMO.md](DEMO.md)).

## Screens

Overview (obligation graph + posture) · Register (typed, cited obligations) ·
Blast Radius (amendment impact) · Evidence & Gaps · Review Queue (Ed25519
sign-off) · Policy (Rego/OPA, staged enforcement) · Audit (bi-temporal lineage)
· Feed (machine-readable SupTech feed). Every data view has an **as-of date**
control.

After seeding + compiling, open the app: the **Overview** shows the live
obligation graph and posture; the **Register** lists obligations with an as-of
date control (set it before 2024-05-15 and the register empties).
