# Phases 6–10 — Frontend, Backend, Testing, DevOps, Dependencies

## Phase 6 — Frontend
- **F-1 — Design-token inconsistency (see HC-8).** `app/page.tsx` and `components/app-shell.tsx` use raw hex (`#08090E`, `#11131C`) and raw Tailwind palette (`bg-blue-600`, `text-amber-400`, `text-slate-400`) while the rest of the app (signoff-modal, register, review, feed, audit, policy) uses semantic tokens (`bg-surface`, `text-foreground`, `border-line`, `text-ok/warn/risk`). Two sources of truth for color; dark theme on Overview is hardcoded rather than token-driven. LOW, CONFIRMED.
- **F-2 — Accessibility gaps (LIKELY; not tested with AT).** Modals set `role="dialog"`, `aria-modal`, focus via `useDialog` (good). But: form controls in `signoff-modal.tsx` / `register` filters rely on placeholder text and `eyebrow` spans rather than programmatic `<label htmlFor>` associations; the officer avatar in `app-shell.tsx:146` is a `div` with `title` only; several icon-only buttons depend on `aria-label` (present in most, e.g. close buttons). Color-contrast of `text-slate-400/500` on `#08090E` not verified against WCAG AA. Recommend an axe/Lighthouse pass. LOW–MEDIUM depending on audience.
- **F-3 — SEO/metadata minimal.** `app/layout.tsx:25` sets title+description only; no per-route metadata, canonical, OG, robots, sitemap. Acceptable for an authenticated internal tool; note if public. INFO.
- Strengths: consistent loading skeletons, explicit error states with retry, empty states everywhere, `next/font` (no layout shift), `suppressHydrationWarning` on `<html>` for the theme, responsive flex/grid, framer-motion transitions. Good UX baseline.

## Phase 7 — Backend
- Endpoint table (22 routes): `/health`, `/version`, and `/api/{obligations,obligation,graph,posture,clauses,amendments/blast-radius,evidence,tickets,review-queue,signoff(GET/POST),policies,policy,policy/compile,policy/stage,policy/evaluate,firm-state,lineage,feed,feed/schema}`. Consistent: `?as_of` everywhere, JSON envelopes, correct verbs, `writeError` taxonomy, `ErrNotFound`→404 mapping, `409` for the sign-off gate.
- **B-1 — No tests on the HTTP layer.** `backend/internal/httpapi/` has zero `_test.go` (confirmed: `go test` → "[no test files]"). This is the layer holding the sign-off gate, body validation, and the (now-fixed) injection-reachable path. MEDIUM, CONFIRMED.
- **B-2 — `as_of` date is end-of-day UTC.** `httpapi/obligations.go:28-30` maps `YYYY-MM-DD` → `23:59:59Z`; for an IST firm this can shift day-boundary obligations by up to 5.5h. Given the product's core "as-of any date" promise, make the timezone convention explicit + tested. MEDIUM, LIKELY (no boundary test exists).
- Migrations are forward-only (no down-migrations) and idempotent (`CREATE TABLE IF NOT EXISTS`, deterministic upserts); acceptable for embedded SQLite. Transactions wrap each migration; multi-write sign-off/correction paths are not wrapped in a single tx (correction then sign then status are separate statements on one serialized conn) — low risk under `MaxOpenConns(1)` but not atomic. INFO/LOW.
- Graceful shutdown (`main.go:110`), `ReadHeaderTimeout` set, health check verifies DB. Good ops hygiene for the scope.

## Phase 8 — Testing
- Tooling: Go `testing` (backend); no frontend test runner configured. Backend suite **passes** (`go test ./backend/...` all ok). Coverage is strong for logic packages: `domain`, `compiler`, `policy` (incl. new injection regressions), `signoff`, `store`, `feed`, `llm`, `vec` all have tests.
- **T-1 — Untested critical layer:** `httpapi` (the auth-less write surface, the sign-off gate) — see B-1. Highest-risk gap.
- **T-2 — No frontend tests at all** (no unit/component/E2E). The typed client, formatters (`format.ts` duration parsing), and the sign-off flow are untested. MEDIUM.
- No flaky patterns observed in existing tests (seeded times passed in, deterministic extractor/embeddings). Tests assert behavior, not mocks.

## Phase 9 — DevOps
- **No CI/CD, no Dockerfile, no IaC, no monitoring/alerting.** Intentional for a hackathon MVP (README/ARCHITECTURE say "no Docker, no Postgres"). Consequence: nothing gates merges (tests, `go vet`, typecheck, `npm audit` are manual); no deployment/rollback story; secrets management is env-only. Acceptable for the stated scope; a blocker for production. INFO (scope-appropriate) trending MEDIUM if productionized.
- `.gitignore` correctly excludes `.env*` (except example), the DB + WAL sidecars, the signing key, `node_modules`, `.next`. `go.work.sum` is untracked (commit or ignore explicitly). LOW.

## Phase 10 — Dependencies
- **Go:** current, well-chosen, pure-Go SQLite (no cgo). No known-vulnerable versions. No unused/undeclared. `go.sum` intact.
- **npm:** `next` at 16.2.12 (security-bumped). `npm audit` → **9 high**, all **without a non-breaking fix**: `brace-expansion`/`minimatch` (dev-only, transitive under ESLint) and `postcss`/`sharp` (bundled inside `next`, flagged across all 16.x). `npm audit fix --force` would downgrade `next` to 9.x and break the app — **do not**. Reachability in this app is minimal (build-time/image tooling). No unused top-level deps (framer-motion, @xyflow/react, @dagrejs/dagre, TanStack all used). No duplicate/heavy first-party concerns. Supply chain: no suspicious install scripts in first-party manifests.
