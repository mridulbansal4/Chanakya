# CHANAKYA — Architecture

This document is the durable design record. It is updated every phase.

## 1. What CHANAKYA is

CHANAKYA is a **system of record** for regulatory compliance in the Indian
securities market. It ingests a SEBI circular and maintains, over time, a graph
that answers auditor-grade questions:

- *What obligations are in force, on whom, as of any given date?*
- *When this clause is amended, exactly which controls, evidence, and workflows
  are affected?*
- *Who signed off on treating this sentence as this obligation, and can that
  signature still be verified?*
- *What was the compliant state as-of a past date?*

Because those answers must survive restarts and be independently auditable,
CHANAKYA **persists everything** in a single SQLite file. The database is not an
optional cache; it is the product.

## 2. The safety model (invariants preserved every phase)

1. **The LLM produces DATA, never code, never enforcement.** Every LLM output is
   validated against a strict JSON schema. It cannot execute anything.
2. **Enforcement is deterministic and gated.** Only the OPA/Rego engine
   enforces, and only after a human has cryptographically (Ed25519) signed the
   obligation.
3. **Evidence connectors are READ-ONLY.** Nothing writes back to a customer
   system.
4. **Enforcement is staged:** audit → soft → hard. Nothing hard-blocks before a
   sign-off exists.
5. **Provenance is mandatory.** Every obligation carries a source clause id and
   the exact source sentence. No citation → rejected before it enters the graph.

## 3. Storage — SQLite, no Docker

- Driver: `modernc.org/sqlite` (pure Go; no cgo, no gcc; works on Windows).
- One file, `./chanakya.db`, created on first run.
- Opened with `foreign_keys=ON`, `journal_mode=WAL`, `synchronous=NORMAL`,
  `busy_timeout=10000` via the modernc `_pragma` DSN. `MaxOpenConns(1)` keeps the
  single writer serialised.
- Migrations are embedded with `go:embed` (`backend/db/migrations/*.sql`) and
  applied in-process on boot by a runner that records each applied file in
  `schema_migrations` inside its own transaction. No goose, no external step.
- **Bi-temporal** model (from Phase 1): `valid_from`/`valid_to` = world time,
  `tx_from`/`tx_to` = system time. Graph traversal uses `WITH RECURSIVE` CTEs.
- **Semantic diff** (Phase 4): embeddings stored as JSON/BLOB; cosine similarity
  computed in Go over the small corpus — no pgvector.

## 4. Backend module layout (`./backend`, module `chanakya`)

```
cmd/api/            main, wiring, graceful HTTP server
cmd/seed/           loads the IA circular fixture (Phase 1)
db/migrations/      embedded .sql
internal/config/    env-only configuration (rule 4: no secrets in code)
internal/store/     SQLite open, migration runner, parameterized queries, CTEs
internal/compiler/  clause-tree parser + schema-validated LLM extraction (P2)
internal/evidence/  read-only mock connectors + gap detection (P5)
internal/policy/    Rego compilation + OPA evaluation (P7)
internal/signoff/   Ed25519 signing + verification (P6)
internal/httpapi/   chi router, middleware, handlers
internal/llm/       strict-JSON-schema LLM client, timeout + retries (P2)
internal/vec/       embeddings + cosine similarity in Go (P4)
```

**Conventions:** every function returns wrapped errors (`fmt.Errorf("…: %w",
err)`); no panics in request paths; `context.Context` propagated everywhere; all
SQL parameterized with `?` placeholders.

### HTTP middleware stack (chi)

`RequestID → RealIP → Logger → Recoverer → CleanPath → Timeout(30s) → CORS`
(scoped to the web origin). Rate limiting + per-handler input validation land in
Phase 9.

## 5. Frontend — "Operational Ink"

Next.js 16 monorepo (Turborepo, npm). Shared UI in `packages/ui`; the design
tokens in `packages/ui/src/styles/globals.css` are overridden to a dark-first,
hairline-bordered, tabular-mono system — deliberately *not* a default shadcn
look. Status semantics: teal = verified/enforced, amber = gap/pending, red =
breach, blue = accent. The obligation **graph is the hero** (React Flow, added in
Phase 3). No chatbot. An as-of-date control sits on every data view. Sign-off is
a deliberate multi-step modal with mandatory typed justification — friction is a
feature. Motion (Framer Motion) communicates causation (blast-radius
propagation), never decoration.

Typed API access is centralised in `apps/web/lib/api.ts`; server state is
managed by TanStack Query.

## 6. Product capabilities (A–I) and phase mapping

| Cap | Capability                     | Phase |
| --- | ------------------------------ | ----- |
| A   | Regulation Compiler            | 2     |
| B   | Living Obligation Graph        | 1, 3  |
| C   | Amendment / Blast Radius       | 4     |
| D   | Evidence Mapping & Gaps        | 5     |
| E   | Remediation Tickets            | 5     |
| F   | HITL Review Queue + Sign-off   | 6     |
| G   | Policy-as-Code (Rego/OPA)      | 7     |
| H   | Bi-temporal Audit Lineage      | 8     |
| I   | Regulator Feed                 | 8     |

## 7. Phase 0 — what was built

- Go module + `go.work` at the repo root so `go run ./backend/cmd/api` works.
- `internal/store`: opens SQLite (WAL + foreign keys), runs embedded migrations,
  exposes `Health`.
- `internal/httpapi`: chi router with the production middleware stack, CORS,
  `GET /health` (returns 200 ok / 503 degraded with a DB check) and `GET /version`.
- `cmd/api`: config load → store open → serve with graceful shutdown.
- Web: `lib/api.ts` typed client, `HealthIndicator` polling `/health` every 5s,
  a CHANAKYA landing page, and the Operational Ink tokens + editorial/mono fonts.

**Proven:** `go vet` + `go build` clean; server boots, `/health` returns
`status: ok`; `chanakya.db` created with `foreign_keys=1`, `journal_mode=wal`,
migration `0001_meta.sql` recorded; monorepo `typecheck` + `build` green; health
indicator renders live.

## 8. Phase 1 — the bi-temporal data layer

- **Migration `0002_graph.sql`** creates the graph: `circular`, `clause`
  (self-referential tree), `entity`, `obligation`, `control`, `evidence`, and
  the edge tables `obligation_control` / `control_evidence`. Every node/edge
  table carries the four bi-temporal columns. Timestamps are RFC3339 UTC strings
  so lexical = chronological comparison. `obligation` bakes the safety
  invariants into the schema: `deontic_type`/`status` `CHECK` constraints and
  `NOT NULL` on `source_clause_ref` + `source_sentence` (provenance mandatory).
- **`internal/domain`** — pure types (`Circular`, `Entity`, `Clause`,
  `ClauseNode`, `Obligation`) with `DeonticType`/`ObligationStatus` validity and
  `Obligation.Validate` (rejects missing provenance / out-of-range confidence).
  `ClauseID(circular, ref)` gives deterministic surrogate ids → idempotent seeds.
- **`internal/store/graph.go`** — parameterized `Upsert{Circular,Entity,Clause}`
  (idempotent via `ON CONFLICT`), `CountClauses`, `ListTopLevelClauses`, and
  `GetClauseSubtree` — a `WITH RECURSIVE` traversal returning a subtree in
  document pre-order with `depth`/`path`, filtered to the as-of world time and
  the latest system time (`tx_to IS NULL`).
- **`internal/fixtures`** — the embedded 12-clause SEBI IA Master Circular
  fixture (registration threshold 300 clients / INR 3 crore, 30-day application,
  fee disclosure, client-level segregation, 5-year retention, 7-day client
  notification) + a loader that stamps bi-temporal columns and validates the
  parent-before-child ordering the FK needs.
- **`cmd/seed`** — loads the fixture and prints the reconstructed tree.

**Proven:** `go vet`/`build`/`test` all clean (`internal/store` table-driven
tests: subtree order + depth, as-of world-time filtering incl.
future/retired exclusion, idempotent re-seed); `go run ./backend/cmd/seed`
loads 12 clauses and the recursive-CTE traversal prints the 4-chapter tree;
re-running keeps the count at 12; `schema_phase = 1`.

**Safety invariants preserved:** the schema *enforces* provenance
(`source_*` `NOT NULL`) and the deontic/status domains (`CHECK`); nothing here
runs LLM output or enforces anything; no evidence is written anywhere.

## 9. Phase 2 — the Regulation Compiler

The compiler turns clause text into typed, cited obligations. The whole pipeline
treats extractor output as untrusted DATA and validates it before anything
enters the graph.

- **`internal/llm`** — the `Extractor` interface (`Extract(clause) → raw JSON`).
  Two implementations behind it:
  - `OfflineExtractor` (default) — deterministic, dependency-free. Splits a
    clause into verbatim sentences, classifies the deontic modal with
    word-boundary matching (`must`/`shall` → MUST, `must not` → MUST_NOT — and
    critically *not* "must notify"), extracts numeric thresholds and
    `within N days` deadlines, and scores confidence. No API key; fully testable.
  - `AnthropicExtractor` — real Claude Messages API over raw HTTP, forcing the
    compiler's strict schema via **strict tool use** (`strict: true` +
    `tool_choice: {type:"tool"}`), with timeout and retry/backoff. Used only when
    `CHANAKYA_LLM_API_KEY` is set. No sampling params (they 400 on Opus 4.8).
- **`internal/compiler`** — owns `schema.json` (the single strict schema used
  *both* to validate output and as the Anthropic tool `input_schema`). For each
  clause it: (1) calls the extractor, (2) validates the document against the
  schema with `santhosh-tekuri/jsonschema/v6`, (3) enforces the **causal
  citation** — the cited clause ref must match and the `source_sentence` must be
  a verbatim substring of the clause text (hallucinated citations are rejected),
  (4) runs `domain.Obligation.Validate`, and (5) routes by confidence:
  `≥ 0.75 → pending`, else `needs_review`. Nothing is ever auto-approved.
- **`cmd/compile`** — runs the compiler over the seeded clauses and persists
  survivors via `store.UpsertObligation` (which re-validates and relies on the
  DB's `NOT NULL` provenance + `CHECK` constraints as the last guard).

**Proven:** `go vet`/`build`/`test` clean. Validator tests reject: an
obligation whose `source_sentence` is not in the clause (missing citation), a
wrong `source_clause_ref`, an invalid `deontic_type` enum, unknown fields
(`additionalProperties:false` — including a smuggled `"exec"` field), and
missing required fields; confidence routing (pending vs needs_review) and
deterministic ids are covered too. `go run ./backend/cmd/compile` extracts **10
obligations from the 12-clause fixture** (4 pending, 6 needs_review, 0 rejected),
correctly typing 4.2 as MUST_NOT and 5.2 as MUST.

**Safety invariants preserved:** the LLM/extractor emits **DATA only**, schema-
validated before use; **no code is executed**; **provenance is mandatory** and
enforced three times (schema `required`, compiler substring check,
`domain.Validate`); low-confidence extractions are **flagged, never
auto-trusted**; nothing is enforced and no evidence is touched.

## 10. Phase 3 — Graph API + Register UI

Read-only API and the first two screens. Every data endpoint takes `?as_of=`
(YYYY-MM-DD or RFC3339; a date is treated as end-of-day UTC) and reconstructs
the graph in world + system time.

- **`internal/store/queries.go`** — read models + queries: `ListObligations`
  (obligation ⋈ clause, parameterized filters on bearer/deontic/status),
  `GetObligation` (with clause text), `PostureAsOf` (status roll-up),
  `GraphAsOf` (clause-tree + obligation nodes/edges), `FirstCircularID`.
- **`internal/httpapi`** — routes under `/api`: `GET /obligations`,
  `GET /obligation?id=` (ids embed the circular id and contain `/`, so detail is
  a query param, not a path param), `GET /graph`, `GET /posture`. Handlers
  validate `as_of` (400 on malformed) and the deontic/status filter values
  against the domain enums.
- **Web (`apps/web`)** — a global **`AsOfProvider`** context feeds one as-of
  date to every view via the `AsOfControl` in the app shell. `lib/api.ts` gains
  typed `listObligations`/`getObligation`/`getGraph`/`getPosture`.
  - **Command Overview** (`/`) — thin posture strip (obligations in force,
    pending sign-off, needs-review, gaps) above the **React Flow** obligation
    graph (the hero): clause tree laid left→right by depth, obligations hanging
    off their clause, coloured by status. Edges are static (motion is reserved
    for Phase 4 blast-radius, never idle decoration).
  - **Obligation Register** (`/register`) — **TanStack Table** of obligations
    with deontic/status filters and the as-of control; clicking a row opens a
    detail panel showing the full record and the **citation** (the exact source
    sentence highlighted in the clause text) — "every claim, its citation one
    click away".

**Proven:** `go` + web `typecheck`/`build` clean. Live end-to-end against the
seeded+compiled DB: `/api/posture` → 10 in force; `/api/obligations` → 10 (0 as
of 2024-01-01, before the circular's 2024-05-15 issue); `deontic=MUST_NOT` → 1;
`/api/graph` → 22 nodes / 18 edges; malformed `as_of` → 400. In the browser:
the Register renders all 10 obligations; setting the as-of date to 2024-01-01
empties it ("No obligations in force as of 2024-01-01"); the MUST_NOT filter
narrows to clause 4.2; clicking it opens the detail panel with the verbatim
citation for source clause 4.2. The Overview posture strip and graph render live
from the API.

**Safety invariants preserved:** the API is strictly **read-only** — no writes,
no enforcement, no evidence access. Every view is a bi-temporal reconstruction,
so what was compliant "as-of" any past date is answerable, not just today.

## 11. Phase 4 — Amendment / Blast Radius

Given an amended clause, compute exactly what downstream work it creates —
without touching the graph.

- **`internal/vec`** — dependency-free text embeddings (hashed term-frequency,
  256-dim, L2-normalised, light singular stemming so "fee"/"fees" align) and
  cosine similarity in Go. No pgvector, no embedding service.
- **Controls/evidence layer** — an embedded `ia_controls.json` fixture defines
  the firm's controls, read-only evidence sources, which clauses each control
  covers, and the evidence it relies on. `compile` wires it after obligations
  exist: `obligation→control` (by clause coverage) and `control→evidence` edges.
  Clause 5.2 (client-notification) is deliberately left uncovered → a gap for
  Phase 5.
- **`store.BlastRadius`** — resolves the amended clause, embeds the amended
  text, cosine-diffs it against every obligation's stored embedding, unions the
  semantic matches (≥ threshold) with the obligations structurally on the clause,
  then traverses `obligation→control→evidence`. Returns nodes (layered
  amended→obligation→control→evidence, obligations tagged direct/semantic with
  similarity), edges, a change list, and summary counts. **Nothing is persisted**
  — it is a what-if preview.
- **`POST /api/amendments/blast-radius`** — validated body (missing fields → 400,
  unknown clause → 404, threshold ∈ [0,1]), 1 MiB cap, unknown-field rejection.
  `GET /api/clauses` feeds the clause picker.
- **Web** — the **Blast Radius** screen (`/amendments`): pick a clause (text
  prefills), edit it, compute. The right pane renders a React Flow graph whose
  nodes animate in **layer by layer via Framer Motion** — the impact visibly
  propagates clause→obligation→control→evidence (motion = causation). The left
  pane lists exactly what changed and the work created, with cosine scores.

**Proven:** `go`/web `typecheck`/`build`/`test` clean (incl. `internal/vec`
tests: L2-normalised embeddings, fee/fee > fee/retention similarity, marshal
round-trip). Live: amending the fee clause 4.1 directly hits its own obligation
(cosine 0.81) and **semantically** pulls in the fee-threshold obligation 3.1
(0.41) — which propagates to the **IA Registration Monitor** control, a
non-obvious downstream impact; raising the threshold to 0.9 collapses it to the
direct hit only; unknown clause → 404, missing field → 400. In the browser the
change list renders 6 obligations / 3 controls / 4 evidence with per-obligation
cosine scores and direct-vs-semantic labels.

**Safety invariants preserved:** the blast radius is a **read-only simulation**
— no clause version is written, no obligation changed, nothing enforced.
Evidence remains a read-only reference (source system, never written).

## 12. Phase 5 — Evidence, Gaps & Tickets

- **`store.EvidenceMap`** — joins each in-force obligation to its controls and
  the evidence reachable through them, and flags a **gap** when no control is
  mapped (or a mapped control has no evidence). Every evidence source is
  returned with `read_only: true` — connectors never write back.
- **`store.GenerateDraftTickets` / migration `0004_tickets.sql`** — for each gap
  it DRAFTS a ticket (deterministic id `tkt:<obligation>`, `state='draft'`,
  owner, deadline inherited from the obligation, and the obligation's source
  sentence as the citation). Idempotent. **CHANAKYA never files tickets** — the
  state enum includes `filed`/`resolved` for lifecycle completeness, but only
  `draft` is ever written. `compile` runs gap detection + ticket drafting as its
  last step.
- **`GET /api/evidence` / `GET /api/tickets`** — the mapping + draft tickets,
  both as-of aware.
- **Web** — the **Evidence & Gaps** screen (`/evidence`): a summary strip
  (satisfied / gaps / read-only sources), the obligation↔control↔evidence table
  with gaps highlighted and each evidence tagged by source system, a read-only
  connectors footer, and a draft-tickets panel (each stamped `DRAFT` with its
  citation and the "never filed" note).

**Proven:** `go`/web `typecheck`/`build`/`test` clean (store tests:
`EvidenceMap` flags a covered obligation satisfied and an uncovered one a gap;
`GenerateDraftTickets` drafts one ticket per gap, idempotently, in `draft`
state with a citation). Live: `compile` reports 5 satisfied, 5 gaps, 5 draft
tickets; `/api/evidence` shows clause 5.2 (client-notification, deliberately
uncovered) as a gap; `/api/tickets` returns its DRAFT ticket with deadline P7D
and the full citation. The Evidence & Gaps screen renders all of it.

**Safety invariants preserved:** evidence connectors are **read-only**
(`read_only: true`, source-system-labelled); remediation tickets are only ever
**drafted, never filed** into a customer system; nothing is enforced.

## 13. Phase 6 — HITL Review Queue + Ed25519 Sign-off

The human gate: no obligation reaches `approved` (the precondition for Phase 7
enforcement) without a person cryptographically signing it.

- **`internal/signoff`** — pure crypto: `Canonical(obligation)` produces a
  deterministic JSON of the obligation's material CONTENT (id, clause, bearer,
  deontic, condition, threshold, deadline, penalty, source ref + sentence,
  valid_from — **status is excluded**). `Signer.Sign` returns the sha256 hash
  (hex) and an **Ed25519** signature over the canonical bytes; `Verify` re-derives
  the canonical form of the *current* obligation and checks the signature.
  `LoadOrCreateKey` resolves the key from `CHANAKYA_SIGNING_KEY_B64`, a seed file
  (gitignored, created on first run), or generates one. (MVP: a server-held key
  stands in for the reviewer's key; the verification model is identical to a
  client-side/HSM key.)
- **Migration `0005_signoff.sql`** — the `signoff` table (action `approve`/`reject`,
  obligation hash, base64 signature + public key, `signed_by`, mandatory
  justification).
- **`store` sign-off methods** — `ReviewQueue` (pending/needs-review, lowest
  confidence first), `GetObligationDomain`, `ApplyObligationCorrection`,
  `SetObligationStatus`, `UpsertSignoff`, `GetSignoff`.
- **Endpoints** — `GET /api/review-queue`; `POST /api/signoff` (validates: action
  ∈ approve/reject, `signed_by` present, **justification ≥ 20 chars**; on approve
  it optionally applies corrections, signs, records, and sets status →
  `approved`; on reject → `rejected`); `GET /api/signoff?obligation_id=` returns
  the record plus a **live verification** against current content.
- **Web** — the **Review Queue** screen (`/review`) lists pending obligations
  beside their source sentence + reasoning chain. "Review & sign" opens a
  deliberate **multi-step modal** (Review → Decide → Sign): the reviewer chooses
  approve/reject (optionally correcting deontic/deadline first), must type a
  ≥20-char justification (Continue is gated on it — *friction is a feature*), and
  only then produces the Ed25519 signature, whose hash/signature/public key are
  shown with a **VERIFIED ✓** badge.

**Proven:** `go`/web `typecheck`/`build`/`test` clean. `internal/signoff` tests:
a valid signature verifies; tampering **any** of five material fields breaks it;
a status flip to `approved` does **not** break it; a wrong public key fails.
Live: approving obligation clause-1 returned `verified:true` and set status
`approved`; re-editing its content directly in the DB then made
`GET /api/signoff` report **`valid:false`** with mismatched signed vs current
hashes; a <20-char justification → 400; reject → `rejected`; the queue dropped
10→8 after one approve + one reject. In the browser the multi-step modal signed
obligation clause-1 end-to-end (hash + Ed25519 signature + public key shown,
VERIFIED ✓) and the queue dropped 10→9.

**Safety invariants preserved:** approval is the **only** path to `approved`, and
it requires a human + a mandatory justification + an Ed25519 signature — the LLM
never approves. The signature attests to obligation *content*, so any later
tampering is cryptographically detectable. Enforcement (Phase 7) will gate on a
valid `approve` sign-off.

## 14. Phase 7 — Policy-as-Code (OPA/Rego)

Enforcement is done **only** by a deterministic policy engine, and only for a
signed obligation.

- **`internal/policy`** — `Compile(obligation)` deterministically emits a Rego
  module: the obligation's structured **threshold becomes the applicability
  gate** (`input.metrics[metric] >= value`), firm compliance is the clause
  attestation, and a `deny` set carries the reason. `Evaluate` runs the module
  against firm-state input with the **embedded OPA engine**
  (`github.com/open-policy-agent/opa/v1/rego`), returning `{compliant,
  applicable, denies, trace}` — the trace captured via `topdown.BufferTracer` +
  `PrettyTrace`. Pure and deterministic.
- **Migration `0006_policy.sql`** — `policy` (compiled Rego + `stage` audit/soft/
  hard) and `policy_eval` (recorded decisions incl. `blocked` + trace).
- **Endpoints** — `POST /api/policy/compile` (**SAFETY GATE**: 409 unless the
  obligation is `approved` *and* has an approving sign-off), `GET /api/policy`,
  `POST /api/policy/stage`, `POST /api/policy/evaluate` (records the result;
  `blocked` only when `stage == hard` and non-compliant), `GET /api/policies`
  (approved-obligation candidates), `GET /api/firm-state` (suggested input from
  entity metrics + evidence-derived attestations).
- **Web** — the **Policy** screen (`/policy`): approved obligations on the left;
  selecting one shows the compiled Rego, an enforcement-stage selector
  (audit/soft/hard), an editable firm-state JSON input, and an **Evaluate**
  action that renders the deterministic decision (compliant / non-compliant,
  applicable, **BLOCKED** at hard) plus the OPA trace.

**Proven:** `go`/web `typecheck`/`build`/`test` clean. `internal/policy` tests:
compile emits valid Rego; evaluation is **deterministic** (evaluated twice,
identical) with correct pass/fail across triggered+attested / triggered+not /
below-threshold, denies present only on failure, trace non-empty. Live: compiling
an unsigned obligation → **409**; approving clause 3.1 then compiling produced the
threshold policy; firm state (clients 412, attested) → **compliant**; un-attested
at **soft** → non-compliant + deny, **not** blocked; at **hard** → **blocked**;
clients 100 (below 300) → **not applicable**, compliant; the OPA trace is
returned. In the browser: selecting 3.1 shows the Rego + prefilled firm state;
Evaluate → **Compliant** with trace; promoting to **hard** + an un-attested input
→ **Non-compliant + BLOCKED + deny**.

**Safety invariants preserved:** enforcement is done **only** by the
deterministic OPA/Rego engine, and a policy exists **only** for an obligation a
human signed (the compile gate returns 409 otherwise). Enforcement is **staged**
audit → soft → hard; only `hard` marks a decision `blocked`, and policies start
at `audit` — nothing hard-blocks before sign-off.

## 15. Phase 8 — Bi-temporal Audit Lineage + Regulator Feed

- **Temporal semantics fix** — a sign-off / policy becomes a **world-time fact
  when it is made** (`valid_from = now`), not retroactively at the clause's issue
  date. So reconstructing lineage as-of a date before it was signed shows the
  obligation *unsigned and un-enforced* — the whole point of the bi-temporal view.
- **`store.Lineage(circular, asOf)`** — reconstructs the full
  clause→obligation→control→evidence→sign-off→policy chain in force as-of a date
  (world time + current system knowledge), returning nodes, edges, and per-type
  counts. No new migration — it is a read-only query over the existing tables.
- **`store.RegulatorFeed`** + **`internal/feed`** — a versioned, machine-readable
  SupTech feed of obligations with causal **provenance** (source sentence +
  extractor confidence + the Ed25519 sign-off where signed). A JSON schema
  (`internal/feed/schema.json`) is compiled once; the feed is **self-validated
  against it before emission** (`santhosh-tekuri/jsonschema/v6`).
- **Endpoints** — `GET /api/lineage`, `GET /api/feed` (emits with
  `X-CHANAKYA-Feed-Version` and 500s if it fails self-validation),
  `GET /api/feed/schema`.
- **Web** — the **Audit** screen (`/audit`): a per-type counts strip + a React
  Flow lineage graph, both driven by the global as-of date. The **Feed** screen
  (`/feed`): feed metadata + a "validated against schema" badge + raw-feed and
  raw-schema links + each obligation with its provenance and sign-off status.

**Proven:** `go`/web `typecheck`/`build`/`test` clean (`internal/feed` tests: a
valid feed passes; missing `feed_version`, a bad deontic enum, missing
provenance, and an unknown top-level field all fail). Live: lineage **as-of
today** → 12 clause / 10 obligation / 4 control / 5 evidence / **1 signoff / 1
policy**; **as-of 2024-06-01** → same graph but **0 signoff / 0 policy**;
**as-of 2024-01-01** → **empty** (before the circular). The feed is version 1.0,
carries provenance + the sign-off for clause 3.1 (signed, with the obligation
hash) and `signoff: null` for unsigned obligations, and self-validates (HTTP
200). In the browser, both screens render live and changing the as-of date to
2024-06-01 drops the lineage signoff/policy counts to 0.

**Safety invariants preserved:** the lineage and feed are strictly **read-only**
reconstructions. The feed carries the full **causal provenance** for every
obligation (source clause + exact sentence), and the cryptographic sign-off
where one exists — so a downstream regulator can independently verify. Every
answer is **as-of** a date, so "what was compliant then" is reconstructable, not
just "what is compliant now".

## 16. Phase 9 — Polish + demo

- **`internal/bootstrap`** — the shared seed + compile pipeline (`Seed`,
  `Compile`, `EnsureSeeded`). `cmd/seed` and `cmd/compile` are now thin wrappers
  over it, and `cmd/api` calls `EnsureSeeded` on startup: on an empty DB it
  seeds the IA fixture and compiles it with the offline extractor, so the **two
  documented run commands (api + web) yield a fully-seeded working app** with no
  manual step.
- **Rate limiting** — `httprate.LimitByIP(240/min)` on the `/api` surface (429 +
  Retry-After beyond the cap).
- **Input validation** — every handler validates: as-of parsing (400 on
  malformed), JSON bodies capped at 1 MiB with unknown-field rejection, required
  fields, deontic/status/stage enum checks, threshold/confidence range checks,
  and the sign-off (`justification ≥ 20`) + policy-compile (approved+signed)
  gates.
- **`dev.ps1`**, **`DEMO.md`** (90-second walkthrough across the seven screens),
  and README run instructions.

**Proven:** `go`/web `typecheck`/`build`/`test` clean. From a **clean checkout**
(deleted `chanakya.db`), `go run ./backend/cmd/api` logs *"bootstrapped demo
data"* and immediately serves 10 obligations / 5 gaps / 5 tickets / 22 graph
nodes with **no seed step**; a 260-request burst returns 236×200 + **24×429**
(rate limit engaged); the web app renders the Overview (backend online, graph
hero) and all eight nav screens.

**Safety invariants preserved:** unchanged and intact end-to-end — the LLM emits
schema-validated data only, provenance is mandatory, evidence is read-only,
tickets are drafted never filed, approval requires a human Ed25519 signature,
and enforcement is done only by the deterministic OPA/Rego engine, staged
audit → soft → hard.

---

## Build & test summary

Backend: `go build ./...`, `go vet ./...`, `go test ./...` — all green (unit
tests across `domain`, `store`, `llm`, `compiler`, `vec`, `signoff`, `policy`,
`feed`). Frontend: `npm run typecheck` + `npm run build` — green. No secrets in
code; all config via environment (`backend/.env.example`, `apps/web/.env.example`).
No Docker anywhere; SQLite (`modernc.org/sqlite`, pure Go) is the single-file
system of record.
