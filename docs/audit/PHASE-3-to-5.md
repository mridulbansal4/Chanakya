# Phases 3–5 - Architecture, Code Quality, Performance

## Phase 3 - Architecture
Clean layered design; dependency graph is acyclic and matches the intended layering (Phase 0.8). No god classes, no circular deps, no cross-layer mixing (HTTP handlers stay thin; SQL confined to `store`; business rules in `domain`/`policy`).

- **A-1 - Largest file is a UI simulation.** `frontend/apps/web/components/amendment/steps.tsx` (660 LOC) holds the entire multi-step MITC simulation. Current shape: one large stepper component driving scripted stages from `lib/amendment-sim.ts`. Cost: hard to test/modify; conflates presentation with a long scripted flow. Target: split per-step components + a small state machine. Severity LOW (it is demo-only, self-contained).
- **A-2 - Two parallel "blast radius" concepts.** The real backend `/api/amendments/blast-radius` (`store/blast.go`, driven by `/amendments`) and the scripted `amendment-sim.ts` (driven by `/regulatory-feed`) both present "blast radius" with different data. Not a defect, but a reviewer/onboarding trap; the sim is fully mocked. Severity INFO.
- **Altitude:** `store/policy.go:FirmState` mixes data-shaping with a hardcoded business value (HC-1) and a metric-key remap (HC-2) - the one place domain knowledge leaks into the store layer.

No other architectural findings. Files >400 LOC: `steps.tsx` (660), `generate_documents.py` (624, a standalone generator), `policy/page.tsx` (525, cohesive), `lib/api.ts` (514, a flat typed client - appropriate).

## Phase 4 - Code Quality
Tooling (actual output this pass):
- `go build ./backend/...` → **OK**; `go vet ./backend/...` → **OK** (clean); `go test ./backend/...` → **all packages ok**.
- `turbo typecheck` → **2 successful, 0 errors**.
- `turbo lint` → **33 problems (0 errors, 33 warnings)** - all `react-hooks/set-state-in-effect` and `react-hooks/immutability`.

Findings:
- **Q-1 - Dead code.** `backend/internal/store/store.go:76-78`: `url.Values{}` allocated then discarded (`_ = q`). LOW, CONFIRMED, XS.
- **Q-2 - 33 ESLint warnings.** e.g. `screen-banner.tsx:17` (setState in effect), `overview-graph.tsx:182` (immutability), firm-state init in `policy/page.tsx`. Non-blocking; effect-driven setState cascades worth cleaning. LOW.
- No `TODO`/`FIXME`/`HACK`/`console.log`/`debugger` in first-party source (grep clean; sole hit is inside `package-lock.json`).
- No error-swallowing of note; catches surface to the UI via error states. No `any` in the typed client. Type escape hatches are rare and justified.
- `ui-demo` route (`app/ui-demo/page.tsx`) is a component showcase - reachable, so not dead, but dev-facing; confirm before shipping to a regulator-facing build.

## Phase 5 - Performance (demo-scale corpus; distinguish measured vs suspected)
- **P-1 - Posture recomputes the full evidence map.** `store/queries.go:202` calls `EvidenceMap` (≈4 queries) inside `PostureAsOf`, which is fetched on every screen via the shell. Suspected amplification at scale; negligible at n≈10. SUSPECTED (not measured).
- **P-2 - Blast radius is O(n·Dim) in Go.** `store/blast.go:96-126` loads all obligations + embeddings and cosine-diffs in-process. Documented as intentional (`vec/vec.go:2-8`); fine at this size. INFO.
- **P-3 - `SetMaxOpenConns(1)`** (`store/store.go:50`) serializes DB access - correct for SQLite-WAL single-writer, deliberate. INFO.
- No bundle analyzer configured → frontend bundle impact **not measured** (coverage gap). React data-fetching uses TanStack Query with per-screen keys; a few effect-driven setStates (Q-2) can cause minor extra renders but nothing measured as hot.
- No N+1 in loops issuing queries (the graph/evidence/lineage builders fetch sets then join in memory). No missing-index predicates observed - temporal/FK columns are indexed (`0002_graph.sql:54-56,97-98`).
