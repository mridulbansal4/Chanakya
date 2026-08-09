# Phase 0 - Project Understanding (no findings)

## 0.1 Inventory (188 tracked files; `node_modules` excluded, manifests/lockfiles inspected)

| Directory | Files | ~LOC | Purpose |
|---|---|---|---|
| `backend/cmd/{api,seed,compile}` | 3 | ~240 | Executable entrypoints (HTTP server; manual seed; manual compile). |
| `backend/db/migrations` | 6 | ~250 | Ordered, embedded SQL migrations (bi-temporal schema). |
| `backend/internal/domain` | 2 | ~270 | Pure types + invariants, no I/O. |
| `backend/internal/store` | 13 | ~1,900 | Bi-temporal SQLite; parameterized queries; migration runner. |
| `backend/internal/compiler` | 3 | ~420 | Schema-validated LLM extraction + mandatory citation. |
| `backend/internal/policy` | 4 | ~500 | Rego compile (now injection-safe) + OPA eval. |
| `backend/internal/signoff` | 2 | ~260 | Ed25519 canonical content signing + verify. |
| `backend/internal/{feed,vec,llm,fixtures,bootstrap,config}` | ~14 | ~1,100 | Feed schema/validator; embeddings; extractors; seed data; env config. |
| `backend/internal/httpapi` | 8 | ~1,050 | chi router, middleware, REST handlers. |
| `frontend/apps/web/app` | 11 | ~2,300 | Next.js App-Router pages (11 screens). |
| `frontend/apps/web/components` | ~30 | ~2,700 | UI components (modals, graphs, shell, badges). |
| `frontend/apps/web/lib` | 3 | ~840 | Typed API client, formatters, amendment sim data. |
| `frontend/packages/{ui,eslint-config,typescript-config}` | ~20 | ~600 | Shared design system + config presets. |
| `Documents/`, `docs/`, `scripts/` | ~30 | ~700 | Generated demo PDFs/CSV, screenshots, Python generators. |
| `description/` | 3 | ~870 | AGENTS/ARCHITECTURE/DEMO docs. |

No directory of unknown purpose.

## 0.2 Stack and build
- **Backend:** Go 1.26 (`backend/go.mod:3`), module `chanakya`. Deps: `chi/v5`, `chi/cors`, `chi/httprate`, `open-policy-agent/opa v1.18.2`, `santhosh-tekuri/jsonschema/v6`, `modernc.org/sqlite v1.34.4` (pure-Go, no cgo). `go.work` uses `./backend`.
- **Frontend:** Turborepo (`frontend/turbo.json`), Next.js **16.2.12** (`apps/web/package.json:23`, bumped for security), React 19.2.4, Tailwind v4 (CSS-first), TanStack Query/Table, `@xyflow/react`, `@dagrejs/dagre`, framer-motion. Package manager npm@11 (`frontend/package.json:20`).
- Scripts: backend `go run ./backend/cmd/{api,seed,compile}`; frontend `turbo {dev,build,lint,format,typecheck}`; `dev.ps1` launches both.

## 0.3 Configuration surface
- Env vars read (all documented in `.env.example` files - **no undocumented vars**): `CHANAKYA_ADDR`, `CHANAKYA_DB_PATH`, `CHANAKYA_CORS_ORIGINS`, `CHANAKYA_SIGNING_KEY_PATH`, `CHANAKYA_SIGNING_KEY_B64`, `CHANAKYA_LLM_API_KEY`, `CHANAKYA_LLM_MODEL` (backend); `NEXT_PUBLIC_API_BASE_URL` (frontend).
- Configs: `.eslintrc.js`, `eslint.config.js`, `.prettierrc/.prettierignore`, `tsconfig.*`, `postcss.config.mjs`, `next.config.ts`, `turbo.json`, `components.json`, `.npmrc` (empty), `go.work`. No CI config, no Dockerfile, no IaC.

## 0.4 Runtime flow (primary path, POST /api/signoff)
`cmd/api/main.go:run` → `config.Load` → `store.Open` (WAL, FK on, migrations) → `bootstrap.EnsureSeeded` → `signoff.LoadOrCreateKey` → `httpapi.NewRouter` [`httpapi.go:73`] → chi middleware (RequestID/RealIP/Logger/Recoverer/CleanPath/Timeout 30s/CORS/`httprate` 240/min) → `handlers.postSignoff` [`signoff.go:55`] → validate → optional `ApplyObligationCorrection` → `signer.Sign` (Ed25519) → `store.UpsertSignoff` + `SetObligationStatus` → JSON. Graceful shutdown on SIGINT/SIGTERM.

## 0.5 Data layer
SQLite file `chanakya.db` (WAL, `synchronous=NORMAL`, `busy_timeout=10000`, `foreign_keys=1`, `SetMaxOpenConns(1)`). Bi-temporal model: every graph table carries `valid_from/valid_to` (world) + `tx_from/tx_to` (system). Tables: `app_meta`, `schema_migrations`, `circular`, `clause` (self-ref tree), `entity`, `obligation`, `control`, `evidence`, `obligation_control`, `control_evidence`, `ticket`, `signoff`, `policy`, `policy_eval`. Migration runner applies embedded `db/migrations/*.sql` in lexical order, each in its own transaction. Indexes on FK + temporal columns. No ORM; hand-written parameterized SQL + recursive CTEs.

## 0.6 Identity and trust
**None.** No authentication, sessions, tokens, or authorization. `signed_by` on a sign-off is free text supplied in the request body. The UI models a single hardcoded persona (`OFFICER` in `app-shell.tsx:21`). The Ed25519 key is server-held (acknowledged MVP simplification). → Phase 2 input, not a Phase 0 finding.

## 0.7 Integrations
One outbound: Anthropic Messages API (`llm/anthropic.go:52`, fixed URL, `x-api-key` from env, used only when `CHANAKYA_LLM_API_KEY` set; default is the offline deterministic extractor). No queues, caches, object storage, mail, payments, or telemetry.

## 0.8 Dependency graph (intended layering)
`domain` (leaf) ← `store`, `signoff`, `policy`, `compiler`, `feed`, `vec`; `compiler` ← `llm`; `bootstrap` ← `compiler`+`fixtures`+`store`; `httpapi` ← `store`+`signoff`+`feed`+`policy`+`domain`; `cmd/*` wire it together. Frontend: `app/*` ← `components/*` ← `lib/api` + `@workspace/ui`. Clean, acyclic.

## 0.9 Conventions in force (the audit baseline)
Wrapped errors (`fmt.Errorf("...: %w")`); exclusively `?`-parameterized SQL; the as-of predicate repeated verbatim; deterministic surrogate IDs; RFC3339-UTC strings (lexical=chronological); frontend typed models (no `any`), TanStack Query for all fetches, semantic design tokens (`bg-surface`, `text-foreground`, `border-line`) in most components.

## 0.10 Narrative
CHANAKYA is a "regulatory operating system" for the SEBI TechSprint. It ingests a SEBI circular (seeded fixture: the IA Master Circular), parses it into a bi-temporal clause tree, and runs a Regulation Compiler that extracts typed, cited obligations - every obligation must carry a verbatim source sentence (enforced) and a confidence score; low-confidence ones route to human review. A compliance officer reviews each obligation and produces an Ed25519 cryptographic sign-off over the obligation's canonical content; only a signed obligation can be compiled into a deterministic OPA/Rego policy, which is evaluated against firm state under staged enforcement (audit → soft → hard). Everything is reconstructable "as of" any date via the bi-temporal model, and a schema-validated regulator feed republishes obligations with provenance. The frontend is 11 Next.js screens over a typed client; one screen (`/regulatory-feed`) is a self-contained scripted demo (`lib/amendment-sim.ts`) not backed by the API.

**Open questions:** Is any authentication planned before real use? Is the `/regulatory-feed` simulation meant to be understood as mocked? Is the timezone convention for `as_of` intentional?

**No findings reported in Phase 0.**
