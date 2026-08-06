# CHANAKYA - Architecture

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

## 3. Storage - SQLite, no Docker

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
  computed in Go over the small corpus - no pgvector.

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

## 5. Frontend - "Operational Ink"

Next.js 16 monorepo (Turborepo, npm). Shared UI in `packages/ui`; the design
tokens in `packages/ui/src/styles/globals.css` are overridden to a dark-first,
hairline-bordered, tabular-mono system - deliberately *not* a default shadcn
look. Status semantics: teal = verified/enforced, amber = gap/pending, red =
breach, blue = accent. The obligation **graph is the hero** (React Flow, added in
Phase 3). No chatbot. An as-of-date control sits on every data view. Sign-off is
a deliberate multi-step modal with mandatory typed justification - friction is a
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

## 7. Phase 0 - what was built

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

## 8. Phase 1 - the bi-temporal data layer

- **Migration `0002_graph.sql`** creates the graph: `circular`, `clause`
  (self-referential tree), `entity`, `obligation`, `control`, `evidence`, and
  the edge tables `obligation_control` / `control_evidence`. Every node/edge
  table carries the four bi-temporal columns. Timestamps are RFC3339 UTC strings
  so lexical = chronological comparison. `obligation` bakes the safety
  invariants into the schema: `deontic_type`/`status` `CHECK` constraints and
  `NOT NULL` on `source_clause_ref` + `source_sentence` (provenance mandatory).
- **`internal/domain`** - pure types (`Circular`, `Entity`, `Clause`,
  `ClauseNode`, `Obligation`) with `DeonticType`/`ObligationStatus` validity and
  `Obligation.Validate` (rejects missing provenance / out-of-range confidence).
  `ClauseID(circular, ref)` gives deterministic surrogate ids → idempotent seeds.
- **`internal/store/graph.go`** - parameterized `Upsert{Circular,Entity,Clause}`
  (idempotent via `ON CONFLICT`), `CountClauses`, `ListTopLevelClauses`, and
  `GetClauseSubtree` - a `WITH RECURSIVE` traversal returning a subtree in
  document pre-order with `depth`/`path`, filtered to the as-of world time and
  the latest system time (`tx_to IS NULL`).
- **`internal/fixtures`** - the embedded 12-clause SEBI IA Master Circular
  fixture (registration threshold 300 clients / INR 3 crore, 30-day application,
  fee disclosure, client-level segregation, 5-year retention, 7-day client
  notification) + a loader that stamps bi-temporal columns and validates the
  parent-before-child ordering the FK needs.
- **`cmd/seed`** - loads the fixture and prints the reconstructed tree.

**Proven:** `go vet`/`build`/`test` all clean (`internal/store` table-driven
tests: subtree order + depth, as-of world-time filtering incl.
future/retired exclusion, idempotent re-seed); `go run ./backend/cmd/seed`
loads 12 clauses and the recursive-CTE traversal prints the 4-chapter tree;
re-running keeps the count at 12; `schema_phase = 1`.

**Safety invariants preserved:** the schema *enforces* provenance
(`source_*` `NOT NULL`) and the deontic/status domains (`CHECK`); nothing here
runs LLM output or enforces anything; no evidence is written anywhere.

## 9. Phase 2 - the Regulation Compiler

The compiler turns clause text into typed, cited obligations. The whole pipeline
treats extractor output as untrusted DATA and validates it before anything
enters the graph.

- **`internal/llm`** - the `Extractor` interface (`Extract(clause) → raw JSON`).
  Two implementations behind it:
  - `OfflineExtractor` (default) - deterministic, dependency-free. Splits a
    clause into verbatim sentences, classifies the deontic modal with
    word-boundary matching (`must`/`shall` → MUST, `must not` → MUST_NOT - and
    critically *not* "must notify"), extracts numeric thresholds and
    `within N days` deadlines, and scores confidence. No API key; fully testable.
  - `AnthropicExtractor` - real Claude Messages API over raw HTTP, forcing the
    compiler's strict schema via **strict tool use** (`strict: true` +
    `tool_choice: {type:"tool"}`), with timeout and retry/backoff. Used only when
    `CHANAKYA_LLM_API_KEY` is set. No sampling params (they 400 on Opus 4.8).
- **`internal/compiler`** - owns `schema.json` (the single strict schema used
  *both* to validate output and as the Anthropic tool `input_schema`). For each
  clause it: (1) calls the extractor, (2) validates the document against the
  schema with `santhosh-tekuri/jsonschema/v6`, (3) enforces the **causal
  citation** - the cited clause ref must match and the `source_sentence` must be
  a verbatim substring of the clause text (hallucinated citations are rejected),
  (4) runs `domain.Obligation.Validate`, and (5) routes by confidence:
  `≥ 0.75 → pending`, else `needs_review`. Nothing is ever auto-approved.
- **`cmd/compile`** - runs the compiler over the seeded clauses and persists
  survivors via `store.UpsertObligation` (which re-validates and relies on the
  DB's `NOT NULL` provenance + `CHECK` constraints as the last guard).

**Proven:** `go vet`/`build`/`test` clean. Validator tests reject: an
obligation whose `source_sentence` is not in the clause (missing citation), a
wrong `source_clause_ref`, an invalid `deontic_type` enum, unknown fields
(`additionalProperties:false` - including a smuggled `"exec"` field), and
missing required fields; confidence routing (pending vs needs_review) and
deterministic ids are covered too. `go run ./backend/cmd/compile` extracts **10
obligations from the 12-clause fixture** (4 pending, 6 needs_review, 0 rejected),
correctly typing 4.2 as MUST_NOT and 5.2 as MUST.

**Safety invariants preserved:** the LLM/extractor emits **DATA only**, schema-
validated before use; **no code is executed**; **provenance is mandatory** and
enforced three times (schema `required`, compiler substring check,
`domain.Validate`); low-confidence extractions are **flagged, never
auto-trusted**; nothing is enforced and no evidence is touched.

## 10. Phase 3 - Graph API + Register UI

Read-only API and the first two screens. Every data endpoint takes `?as_of=`
(YYYY-MM-DD or RFC3339; a date is treated as end-of-day UTC) and reconstructs
the graph in world + system time.

- **`internal/store/queries.go`** - read models + queries: `ListObligations`
  (obligation ⋈ clause, parameterized filters on bearer/deontic/status),
  `GetObligation` (with clause text), `PostureAsOf` (status roll-up),
  `GraphAsOf` (clause-tree + obligation nodes/edges), `FirstCircularID`.
- **`internal/httpapi`** - routes under `/api`: `GET /obligations`,
  `GET /obligation?id=` (ids embed the circular id and contain `/`, so detail is
  a query param, not a path param), `GET /graph`, `GET /posture`. Handlers
  validate `as_of` (400 on malformed) and the deontic/status filter values
  against the domain enums.
- **Web (`apps/web`)** - a global **`AsOfProvider`** context feeds one as-of
  date to every view via the `AsOfControl` in the app shell. `lib/api.ts` gains
  typed `listObligations`/`getObligation`/`getGraph`/`getPosture`.
  - **Command Overview** (`/`) - thin posture strip (obligations in force,
    pending sign-off, needs-review, gaps) above the **React Flow** obligation
    graph (the hero): clause tree laid left→right by depth, obligations hanging
    off their clause, coloured by status. Edges are static (motion is reserved
    for Phase 4 blast-radius, never idle decoration).
  - **Obligation Register** (`/register`) - **TanStack Table** of obligations
    with deontic/status filters and the as-of control; clicking a row opens a
    detail panel showing the full record and the **citation** (the exact source
    sentence highlighted in the clause text) - "every claim, its citation one
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

**Safety invariants preserved:** the API is strictly **read-only** - no writes,
no enforcement, no evidence access. Every view is a bi-temporal reconstruction,
so what was compliant "as-of" any past date is answerable, not just today.

## 11. Phase 4 - Amendment / Blast Radius

Given an amended clause, compute exactly what downstream work it creates -
without touching the graph.

- **`internal/vec`** - dependency-free text embeddings (hashed term-frequency,
  256-dim, L2-normalised, light singular stemming so "fee"/"fees" align) and
  cosine similarity in Go. No pgvector, no embedding service.
- **Controls/evidence layer** - an embedded `ia_controls.json` fixture defines
  the firm's controls, read-only evidence sources, which clauses each control
  covers, and the evidence it relies on. `compile` wires it after obligations
  exist: `obligation→control` (by clause coverage) and `control→evidence` edges.
  Clause 5.2 (client-notification) is deliberately left uncovered → a gap for
  Phase 5.
- **`store.BlastRadius`** - resolves the amended clause, embeds the amended
  text, cosine-diffs it against every obligation's stored embedding, unions the
  semantic matches (≥ threshold) with the obligations structurally on the clause,
  then traverses `obligation→control→evidence`. Returns nodes (layered
  amended→obligation→control→evidence, obligations tagged direct/semantic with
  similarity), edges, a change list, and summary counts. **Nothing is persisted**
  - it is a what-if preview.
- **`POST /api/amendments/blast-radius`** - validated body (missing fields → 400,
  unknown clause → 404, threshold ∈ [0,1]), 1 MiB cap, unknown-field rejection.
  `GET /api/clauses` feeds the clause picker.
- **Web** - the **Blast Radius** screen (`/amendments`): pick a clause (text
  prefills), edit it, compute. The right pane renders a React Flow graph whose
  nodes animate in **layer by layer via Framer Motion** - the impact visibly
  propagates clause→obligation→control→evidence (motion = causation). The left
  pane lists exactly what changed and the work created, with cosine scores.

**Proven:** `go`/web `typecheck`/`build`/`test` clean (incl. `internal/vec`
tests: L2-normalised embeddings, fee/fee > fee/retention similarity, marshal
round-trip). Live: amending the fee clause 4.1 directly hits its own obligation
(cosine 0.81) and **semantically** pulls in the fee-threshold obligation 3.1
(0.41) - which propagates to the **IA Registration Monitor** control, a
non-obvious downstream impact; raising the threshold to 0.9 collapses it to the
direct hit only; unknown clause → 404, missing field → 400. In the browser the
change list renders 6 obligations / 3 controls / 4 evidence with per-obligation
cosine scores and direct-vs-semantic labels.

**Safety invariants preserved:** the blast radius is a **read-only simulation**
- no clause version is written, no obligation changed, nothing enforced.
Evidence remains a read-only reference (source system, never written).

## 12. Phase 5 - Evidence, Gaps & Tickets

- **`store.EvidenceMap`** - joins each in-force obligation to its controls and
  the evidence reachable through them, and flags a **gap** when no control is
  mapped (or a mapped control has no evidence). Every evidence source is
  returned with `read_only: true` - connectors never write back.
- **`store.GenerateDraftTickets` / migration `0004_tickets.sql`** - for each gap
  it DRAFTS a ticket (deterministic id `tkt:<obligation>`, `state='draft'`,
  owner, deadline inherited from the obligation, and the obligation's source
  sentence as the citation). Idempotent. **CHANAKYA never files tickets** - the
  state enum includes `filed`/`resolved` for lifecycle completeness, but only
  `draft` is ever written. `compile` runs gap detection + ticket drafting as its
  last step.
- **`GET /api/evidence` / `GET /api/tickets`** - the mapping + draft tickets,
  both as-of aware.
- **Web** - the **Evidence & Gaps** screen (`/evidence`): a summary strip
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

## 13. Phase 6 - HITL Review Queue + Ed25519 Sign-off

The human gate: no obligation reaches `approved` (the precondition for Phase 7
enforcement) without a person cryptographically signing it.

- **`internal/signoff`** - pure crypto: `Canonical(obligation)` produces a
  deterministic JSON of the obligation's material CONTENT (id, clause, bearer,
  deontic, condition, threshold, deadline, penalty, source ref + sentence,
  valid_from - **status is excluded**). `Signer.Sign` returns the sha256 hash
  (hex) and an **Ed25519** signature over the canonical bytes; `Verify` re-derives
  the canonical form of the *current* obligation and checks the signature.
  `LoadOrCreateKey` resolves the key from `CHANAKYA_SIGNING_KEY_B64`, a seed file
  (gitignored, created on first run), or generates one. (MVP: a server-held key
  stands in for the reviewer's key; the verification model is identical to a
  client-side/HSM key.)
- **Migration `0005_signoff.sql`** - the `signoff` table (action `approve`/`reject`,
  obligation hash, base64 signature + public key, `signed_by`, mandatory
  justification).
- **`store` sign-off methods** - `ReviewQueue` (pending/needs-review, lowest
  confidence first), `GetObligationDomain`, `ApplyObligationCorrection`,
  `SetObligationStatus`, `UpsertSignoff`, `GetSignoff`.
- **Endpoints** - `GET /api/review-queue`; `POST /api/signoff` (validates: action
  ∈ approve/reject, `signed_by` present, **justification ≥ 20 chars**; on approve
  it optionally applies corrections, signs, records, and sets status →
  `approved`; on reject → `rejected`); `GET /api/signoff?obligation_id=` returns
  the record plus a **live verification** against current content.
- **Web** - the **Review Queue** screen (`/review`) lists pending obligations
  beside their source sentence + reasoning chain. "Review & sign" opens a
  deliberate **multi-step modal** (Review → Decide → Sign): the reviewer chooses
  approve/reject (optionally correcting deontic/deadline first), must type a
  ≥20-char justification (Continue is gated on it - *friction is a feature*), and
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
it requires a human + a mandatory justification + an Ed25519 signature - the LLM
never approves. The signature attests to obligation *content*, so any later
tampering is cryptographically detectable. Enforcement (Phase 7) will gate on a
valid `approve` sign-off.

## 14. Phase 7 - Policy-as-Code (OPA/Rego)

Enforcement is done **only** by a deterministic policy engine, and only for a
signed obligation.

- **`internal/policy`** - `Compile(obligation)` deterministically emits a Rego
  module: the obligation's structured **threshold becomes the applicability
  gate** (`input.metrics[metric] >= value`), firm compliance is the clause
  attestation, and a `deny` set carries the reason. `Evaluate` runs the module
  against firm-state input with the **embedded OPA engine**
  (`github.com/open-policy-agent/opa/v1/rego`), returning `{compliant,
  applicable, denies, trace}` - the trace captured via `topdown.BufferTracer` +
  `PrettyTrace`. Pure and deterministic.
- **Migration `0006_policy.sql`** - `policy` (compiled Rego + `stage` audit/soft/
  hard) and `policy_eval` (recorded decisions incl. `blocked` + trace).
- **Endpoints** - `POST /api/policy/compile` (**SAFETY GATE**: 409 unless the
  obligation is `approved` *and* has an approving sign-off), `GET /api/policy`,
  `POST /api/policy/stage`, `POST /api/policy/evaluate` (records the result;
  `blocked` only when `stage == hard` and non-compliant), `GET /api/policies`
  (approved-obligation candidates), `GET /api/firm-state` (suggested input from
  entity metrics + evidence-derived attestations).
- **Web** - the **Policy** screen (`/policy`): approved obligations on the left;
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
at `audit` - nothing hard-blocks before sign-off.

## 15. Phase 8 - Bi-temporal Audit Lineage + Regulator Feed

- **Temporal semantics fix** - a sign-off / policy becomes a **world-time fact
  when it is made** (`valid_from = now`), not retroactively at the clause's issue
  date. So reconstructing lineage as-of a date before it was signed shows the
  obligation *unsigned and un-enforced* - the whole point of the bi-temporal view.
- **`store.Lineage(circular, asOf)`** - reconstructs the full
  clause→obligation→control→evidence→sign-off→policy chain in force as-of a date
  (world time + current system knowledge), returning nodes, edges, and per-type
  counts. No new migration - it is a read-only query over the existing tables.
- **`store.RegulatorFeed`** + **`internal/feed`** - a versioned, machine-readable
  SupTech feed of obligations with causal **provenance** (source sentence +
  extractor confidence + the Ed25519 sign-off where signed). A JSON schema
  (`internal/feed/schema.json`) is compiled once; the feed is **self-validated
  against it before emission** (`santhosh-tekuri/jsonschema/v6`).
- **Endpoints** - `GET /api/lineage`, `GET /api/feed` (emits with
  `X-CHANAKYA-Feed-Version` and 500s if it fails self-validation),
  `GET /api/feed/schema`.
- **Web** - the **Audit** screen (`/audit`): a per-type counts strip + a React
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
where one exists - so a downstream regulator can independently verify. Every
answer is **as-of** a date, so "what was compliant then" is reconstructable, not
just "what is compliant now".

## 16. Phase 9 - Polish + demo

- **`internal/bootstrap`** - the shared seed + compile pipeline (`Seed`,
  `Compile`, `EnsureSeeded`). `cmd/seed` and `cmd/compile` are now thin wrappers
  over it, and `cmd/api` calls `EnsureSeeded` on startup: on an empty DB it
  seeds the IA fixture and compiles it with the offline extractor, so the **two
  documented run commands (api + web) yield a fully-seeded working app** with no
  manual step.
- **Rate limiting** - `httprate.LimitByIP(240/min)` on the `/api` surface (429 +
  Retry-After beyond the cap).
- **Input validation** - every handler validates: as-of parsing (400 on
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

**Safety invariants preserved:** unchanged and intact end-to-end - the LLM emits
schema-validated data only, provenance is mandatory, evidence is read-only,
tickets are drafted never filed, approval requires a human Ed25519 signature,
and enforcement is done only by the deterministic OPA/Rego engine, staged
audit → soft → hard.

## 17. Rebuild Phase 1 - Foundation, bi-temporal versioning, ingestion core

### Demo-critical fixes

- **Artificial delay removed.** `lib/api.ts` had a hardcoded 3.5s `setTimeout`
  on `/graph`, `/lineage`, `/obligations`, `/simulate`. Gone.
- **CORS actually enforced.** `httpapi.originChecker` matches the request origin
  exactly (scheme + host + port) against `Options.CORSOrigins`; the previous
  `AllowOriginFunc` returned `true` unconditionally, making the configured
  allowlist a no-op. An EMPTY allowlist preserves the permissive local-dev
  behaviour but logs a warning ONCE at construction, not per request.
- **`/regulatory-feed` copy is honest.** The nav hint, the screen banner, and the
  in-screen inbox description now say "simulated ... live SEBI monitoring lands
  in a later phase" instead of claiming CHANAKYA monitors SEBI.

### Bi-temporal versioning (`0007_versioning.sql`, `store/versioning.go`)

The four bi-temporal columns were decorative: every `UpsertX` did
`ON CONFLICT(id) DO UPDATE`, and `tx_to`/`valid_to` were never assigned, so
amending a circular DESTROYED the prior clause text.

- `circular`, `clause` and `obligation` are rebuilt with a deterministic
  surrogate primary key `row_uid = id || '@' || tx_from`; `id` becomes the
  LOGICAL key shared by every version. A partial unique index
  `UNIQUE(id) WHERE tx_to IS NULL` makes "at most one current version" a
  database guarantee.
- **Deliberate divergence from the phase prompt.** SQLite requires a foreign
  key's parent columns to be covered by a NON-partial unique index; a partial one
  fails with `foreign key mismatch` (verified empirically before designing the
  migration). The prompt's fallback - a full `UNIQUE(id)` - is self-defeating,
  since it forbids a second version. So the FK clauses on columns pointing at the
  three versioned tables are dropped; the partial index plus store-level
  validation replace them. FKs to the non-versioned tables (control, evidence,
  policy) are unchanged. `TestCurrentRowUniqueness` documents and pins this.
- The rebuild runs in the migration runner's single transaction under
  `PRAGMA defer_foreign_keys`. `policy_eval`'s rows are parked in a
  constraint-free table across the `policy` rebuild: dropping a parent records one
  deferred violation per surviving child row, and that count is not cleared by
  rebuilding the child afterwards.
- `store.SupersedeAndInsert(ctx, tx, table, id, next, at)` closes the current
  row's system-time interval and inserts the new version as current knowledge.
  The table name is resolved through a CLOSED lookup (a table name cannot be a
  `?` placeholder), a nonexistent id is an explicit error rather than a silent
  insert, and an obligation version is re-validated for mandatory provenance.
  Existing `UpsertX` call sites are untouched; they now target the current
  version via `ON CONFLICT(id) WHERE tx_to IS NULL`.

### PDF ingestion core - `internal/ingest`, Stages 0-2 (`0008_ingest.sql`)

Deterministic front-end; no LLM. Its only output contract is `[]domain.Clause`
(+ a minimal `Circular`), exactly what `compiler.CompileClause` consumes.

- **Stage 0 - intake.** `RawDoc{SHA256, Bytes, Filename, PageCount, MIME}`.
  Rejects >25 MiB, non-PDF, and encrypted documents with DISTINCT sentinel errors
  (encryption is detected from pdfcpu's `/Encrypt` entry, never inferred from a
  parse failure, so "encrypted" and "damaged" are never confused). A document
  with under 50 extractable characters is rejected as scanned - a product
  decision, since OCR output cannot support the verbatim-citation guarantee.
  Bytes are stored content-addressed in `document_blob`.
- **Stage 1 - layout.** `PageExtractor` interface with three registered
  implementations: `RSCExtractor` (default, `rsc.io/pdf`, pure Go, gives
  positions + font sizes; each page read under `recover` so one malformed page
  degrades instead of failing the document), `ExternalCmdExtractor`
  (`pdftotext -layout`, opt-in via `CHANAKYA_PDF_EXTRACTOR`, clear error when the
  binary is absent, never a silent fallback), and `OCRExtractor` returning
  `ErrNotEnabled`. Text is NFKC-normalised and typographic quotes/dashes folded
  to ASCII; glyph fragments are coalesced into line runs, with a wide gap ending
  a run so table columns survive.
- **Stage 2 - structure.** Modal-font-size body baseline, a precedence-ordered
  numbering lexer (deepest-first, so "3.1.1" is not read as "3"), roman-vs-alpha
  `(i)` resolved by sequence continuity and defaulted to alpha at REDUCED
  confidence when there is no preceding sibling, independent numbering-vs-indent
  level votes with disagreement RECORDED as `Confidence`, a stack machine
  emitting parents before children, provisos/explanations/illustrations as
  siblings under the clause they qualify, footnotes attached to their host
  clause, and table detection gated on BOTH >=3 rows AND consistent aligned
  columns (a table's serialised text stays a verbatim superset of the page runs).
  An unrecognisable document degrades to a flat `p{page}.¶{n}` list rather than
  failing.

**Proven:** `go vet`/`build`/`test` all green; frontend `typecheck` + `build`
green. Store tests: superseding a clause then querying as-of a system time
*before* the supersession still returns the ORIGINAL text (the regression guard
for the destroyed-history defect); exactly one current version survives; a
nonexistent id, an unknown table name, and a wrong `next` type are all rejected.
CORS tests: an allowlisted origin is echoed back, a non-allowlisted one is not,
and neither trailing slashes nor case are implicitly normalised. Ingest tests:
a 21-node golden clause tree (chapters, clauses, a proviso, a lettered list, an
explanation, a table) reproduced byte-for-byte and stable across repeated runs;
clause ordering satisfies the parent-before-child FK requirement. The migration
was applied to a copy of the real `chanakya.db` and every existing endpoint
(`/api/posture`, `/api/evidence`, `/api/lineage`, `/api/tickets`, `/api/feed`,
`/api/policies`) still serves correctly.

**Known limitation, recorded rather than worked around:** neither
`Documents/MITC_Circular_17Feb2025.pdf` nor `Documents/IA_Master_Circular_2025.pdf`
can be a parsing fixture - both are image-only "Microsoft: Print To PDF" scans
with zero font objects, i.e. exactly the class Stage 0 refuses.
`TestMITCIsRejectedAsScanned` pins that refusal. The remaining `Documents/*.pdf`
are ReportLab output using the `ASCII85Decode` stream filter, which `rsc.io/pdf`
does not implement (it panics), so they also yield no text. The golden fixture is
therefore a digitally-generated circular built in `pdffixture_test.go` from the
repo's own `ia_master_circular.json` clause text, so the golden input is
reviewable as source rather than committed as a binary.

**Safety invariants preserved:** ingestion produces a faithful transcription -
normalised for Unicode form and whitespace only, never rewritten - because the
downstream citation gate proves an obligation is real by substring-matching
against this text. No LLM runs in Stages 0-2. Nothing is enforced, no evidence is
touched, and superseding preserves history instead of overwriting it.

## 18. Rebuild Phase 2 - Async ingestion runtime, Stages 3-6, the approval gate

**The invariant this phase adds: an uploaded circular does not enter the graph
until a human approves it.** Everything the pipeline produces is a PROPOSAL,
held in `ingest_run.proposal_json`, outside `circular`/`clause`/`obligation`.

### Job queue + worker pool (`0011_jobs.sql`, `internal/jobs`)

- A real run with a live model is 40-150s. It cannot live in an HTTP request:
  the client would time out and, because `store.go` pins `MaxOpenConns(1)`, a
  long handler would block every other request behind it.
- `job` is a queue inside SQLite - no broker, so the job history lives in the
  same file as the audit trail it belongs to. Rows are NEVER deleted.
- `store.ClaimJob` is a single `UPDATE ... RETURNING` so two workers cannot take
  the same job. The pool runs `N = min(4, NumCPU)` workers.
- **Every stage runs under `recover()`**: a panic marks THAT job failed, naming
  the stage it died in, and leaves the pool and other in-flight jobs untouched.
- `AcquireLLM`/`ReleaseLLM` bound concurrent model calls (4) across ALL workers,
  so a 40-clause circular does not open 40 connections to a rate-limited API.
- Progress fan-out is **non-blocking**: a browser tab that closed mid-ingestion
  cannot stall the pipeline. `Subscribe` replays the last known state, so a
  reconnecting client catches up instead of restarting anything.

### Stages 3-6 (`0012_ingest_pipeline.sql`)

- **Stage 3 - metadata.** Deterministic regex pass (circular number, dates,
  department, "in supersession of" / "read with" / "stands modified", applies-to,
  `DocKind`), then an LLM pass for gaps only. **Precedence is fixed**: the regex
  pass always wins - a model cannot overwrite a circular number read off the
  page. `DocKind` is never taken from the model, because it decides how the rest
  of the pipeline treats the document (an `faq` produces no obligations at all).
  LLM output is schema-validated (`meta_schema.json`, `additionalProperties:false`)
  before any field is used; with no model configured Stage 3 is regex-only and
  fully correct. `llm.JSONCompleter` adds a general strict-JSON completion behind
  the same safety model as `Extractor`.
- **Stage 4 - normalization.** Whitespace/quote/dash canonicalisation and Indian
  numeric forms (`₹3,00,00,000` / `Rs. 3,00,00,000` / `INR 3 crore` → 30000000
  INR). The Indian digit grouping is not the international one: read with a
  Western thousands assumption, three crore becomes three hundred thousand.
  Output goes to a **parallel** field - `Clause.Text` is never touched, because
  the citation gate substring-matches against it. The whitespace rule is
  identical to `compiler.normalizeWS`, not wider.
- **Stage 5 - semantic segmentation.** Splits clauses on discourse markers
  (`provided that`, `unless`, `except`, `subject to`, `notwithstanding`, ...)
  into units tagged `norm|condition|exception|deadline|penalty|definition|
  cross_ref|scope`. Each unit keeps **character offsets into its parent clause**,
  so it is provably a slice and not a paraphrase. Overlapping markers do not
  compound (`provided further that` is not also cut at `provided that`), and
  sentence boundaries no longer split on `3.1` or `Rs.` the way
  `llm.splitSentences` did.
- **Stage 6 - cross references.** Resolves `clause 3.1` intra-document (reusing
  `normalizeClauseRef`'s vocabulary); `regulation 15` is external and
  `the said circular` is anaphoric, so both become **`dangling_reference` rows**
  rather than guessed edges. Cycles record each edge once and stop. Nothing is
  silently dropped: a graph that looks complete but is not is worse than one
  that admits its gaps.

### Preview + approval gate

- `POST /api/ingest` runs Stage 0 synchronously (its rejections are answers the
  user needs now) and returns **202 with an ingest_id immediately**. Content
  addressing makes duplicate handling exact: the same bytes already queued or
  running return the EXISTING run.
- `GET /api/ingest/:id/events` is SSE. It is exempted from the router's 30s
  timeout, and a dropped connection never cancels the run - the reconnect path is
  `GET /api/ingest/:id`.
- `POST /api/ingest/:id/approve` is the gate: named approver + >=20-char
  justification, then **one transaction** commits circular, clauses, obligations,
  embeddings, semantic units, relations, dangling references and the audit
  record. On failure nothing partial enters the graph and the run is recorded as
  failed with its stage. A second approve, or an approve after discard, is a
  409 - never a silent no-op, never a second commit.

**Proven:** `go vet`/`build`/`test` and web `typecheck`/`build` all green.
Store tests: the graph is empty before approve and holds 1 circular / 2 clauses /
1 obligation after; a forced mid-commit failure leaves it **completely**
unchanged and records the run failed; double-approve and approve-after-discard
both return `ErrRunSettled`; a duplicate upload reuses the run; `ClaimJob` is
exclusive; job rows are retained. Pool tests: a panicking job is marked failed
**naming its stage** while a sibling job completes normally; an unread subscriber
does not stall the pipeline. Stage tests cover the fixed metadata precedence
(a stub model's fabricated circular number does NOT overwrite the regex value),
schema rejection of an unknown `"exec"` field, six Indian-numeral forms, unit
offsets that literally slice their parent, nested provisos, dangling references
and reference cycles.

**Live, against a running server with a real Gemini extractor:** upload → 202 in
milliseconds; a duplicate upload returns the same `ingest_id`; SSE streamed all
nine stages (`intake → layout → structure → metadata → normalize → segment →
cross_reference → compile → ready_for_review`); the run produced 22 clauses, 20
obligations, 25 semantic units, 1 resolved and 2 dangling references, and
correctly identified the document as a `master_circular`
(`SEBI/HO/IMD/IMD-PoD-1/P/CIR/2024/49`, issued 2024-05-15); `/api/posture`
showed **10 obligations before approve and 26 after**; a 19-character
justification returned 400; a second approve returned 409; and a blast-radius
query against a newly-ingested clause returned 200, confirming the embeddings
committed inside the approval transaction are live.

**Two defects found by that live run and fixed:**
1. The Phase 1 table rebuild silently dropped `obligation.embedding_json`, a
   column `0003` had added by `ALTER TABLE`. `TestRebuiltTablesKeepEveryColumn`
   now asserts the full column list of every rebuilt table.
2. Document front matter (issuing authority, circular number, date line) sits
   before the first numbered clause and was being discarded, leaving Stage 3
   nothing to read. It is now captured as an explicit `preamble` node.

**Safety invariants preserved:** the pipeline writes NO graph data; only
`ApproveIngestRun` does, and only a human calls it. Extractor and completer output
is schema-validated DATA. Every proposed obligation carries its verbatim source
sentence, and the store re-validates mandatory provenance one last time inside
the commit transaction.

## 19. Rebuild Phase 3 - Enterprise graph, projection, amendment matching

This is where the product's central claim stops being an assertion: a real
company changes when a regulation changes.

### The firm as data (`0009_enterprise.sql`, `internal/fixtures/enterprise/`)

- 15 new bi-temporal tables (`department`, `employee`, `client`, `agreement`,
  `document`, `register`, `system`, `workflow`, `task`, `approval`, `risk`,
  `training`, `communication`, `calendar_event`, plus `binds_to`). Workflow/task/
  approval get their final shape now so Phase 4 needs no migration.
- **Two namespaces, one seam.** The regulatory graph (external, immutable,
  regulator-authored) and the enterprise graph (internal, mutable, firm-authored)
  stay separate. They join at `control` and at `binds_to` - and `binds_to`
  carries a confidence and a `human_confirmed` flag, because a guess about which
  internal policy governs a clause must never be indistinguishable from the
  clause itself.
- 12 embedded fixture files (Alpha Wealth Advisors, SEBI reg INA000000001,
  Mumbai, inc. 2019): 8 departments, 24 employees incl. Priya Menon as Principal
  Officer & Compliance Officer, 140 clients, 22 documents, 7 registers, 7
  systems, 14 risks, ~90 communications.
- **Recency is stored as an OFFSET, not a date.** "Reviewed 14 months ago" is the
  fact; the calendar date depends on when you look. This was found the hard way -
  with absolute dates baked in, every policy in the firm went overdue once a year
  passed and the single deliberate annual-review breach was buried under 21
  accidental ones.

### The deliberate gaps - all discovered BY QUERY

None is recorded anywhere as a problem; each is what a traversal returns.

| Gap | How it is found |
| --- | --- |
| 118 clients on the superseded agreement template | `client ⋈ agreement` filtered to the in-force agreement |
| 3 employees without current training | `training` rows in the latest period with no completion date |
| 1 adviser serving both advisory and distribution clients | group `client` by adviser, count distinct service kinds |
| complaint register 90 days stale | register freshness |
| cybersecurity policy 14 months since review | document review recency |

The 22 re-papered clients hold **two** agreement rows - the original, closed in
world time on the re-papering date, and its replacement. Modelling it as a
supersession rather than "these clients were always on v2" is what makes the
time-travel claim real.

### Projection (`internal/enterprise`)

- `Project` scores an obligation's clause text against firm objects using a
  small, readable topic table rather than an embedding lookup: a compliance
  officer must be able to see WHY a binding was proposed, and "cosine 0.41" does
  not tell them. Bindings below 0.25 confidence are not recorded at all.
- `ImpactOf` traverses obligation → {control, binds_to} → {department, system} →
  named head, and resolves the affected client population as-of the query date.
  It returns **names, not counts**. An unbound obligation is reported as unbound -
  an empty blast radius is a real answer, not a failed query.
- New endpoints: `GET /api/enterprise/summary`, `GET /api/enterprise/impact`,
  `GET /api/clients` (incl. `?impacted_by=`), `GET /api/documents?stale=true`.
- New screen `app/enterprise/page.tsx`.

### Stage 9 - the amendment matcher (`internal/ingest/amendment.go`)

`score = 0.45·cosine + 0.35·jaccard(word trigrams) + 0.20·refEquality`, with the
thresholds applied exactly: `≥0.92` **and byte-identical text** → unchanged;
`0.55 ≤ score < 0.92` → modified; `<0.55` → added. **Both conditions are required
for `unchanged`** - two clauses can score 0.95 and still differ by the one word
that changes what the firm must do. `deleted` requires the document to actually
supersede its predecessor; inferring a deletion from a document that merely
references another would retire live regulation. Many-to-one takes the single
best match with a deterministic tie-break (ref equality, then lowest `row_uid`) -
never a multi-way merge.

Every classification is a PROPOSAL. It lands in the Phase 2 preview queue with
old and new text side by side, and only approval applies it - at which point a
`modified` clause goes through `SupersedeAndInsert` rather than being overwritten.

**Proven:** `go vet`/`test` and web `typecheck`/`build` green. Tests pin the exact
fixture counts (140 clients, 22 on v2 → 118 on v1, 3 training gaps, 14 risks, 24
employees), assert each gap is found by query, prove the org-chart CTE terminates
**with a manager cycle deliberately introduced**, and check that as-of a
pre-re-papering date all 140 clients come back on v1 and none on v2.

**Live, against a running server:** `/api/enterprise/summary` returns Alpha Wealth
with exactly 6 gaps - 118 clients, 3 named employees (Aditya Joshi, Deepa Shetty,
Nikhil Bose), Vikram Rao's segregation breach, 1 stale register, 1 stale policy.
`/api/enterprise/impact` for clause 5.1 returns **118 named clients with their
named advisers**, three owning departments each with its named head (Arjun Desai,
Farida Merchant, Manish Gupta), and the Records Retention System control.
Uploading an amended circular produced **20 unchanged / 2 modified / 1 added,
with 20 obligations reused without re-extraction**; clause 5.1 scored **0.928 -
above the unchanged threshold - and was still classified `modified` because the
text differs (5 years → 8 years)**, exactly the safety property. After approval
the database holds two versions of clause 5.1, and the superseded one still reads
"5 years".

**Safety invariants preserved:** the projection layer INFERS and says so
(confidence + `human_confirmed`, which only a person can set and which a re-run
never clears). Nothing here enforces anything or writes to a firm system. The
amendment matcher proposes; the human gate applies.

## 20. Rebuild Phase 4 - Workflows, the real feed, connectors, testing corpus

### Workflow synthesis (`0010_workflow.sql`, `internal/workflow`)

- **Verb-driven, not free generation.** A closed vocabulary of 25 regulatory
  verbs (`maintain`, `retain`, `disclose`, `notify`, ...) selects one of 8
  reviewed templates through a fixed LOOKUP TABLE. The mapping from a regulatory
  act to an operational response is a decision a compliance professional made
  once and can review - not something inferred per obligation. Synthesis needs no
  LLM at all, which is what makes it unit-testable.
- **An unrecognised verb goes to the review queue as unclassified.** It is not
  mapped to the nearest-looking template and not silently dropped.
- `owners.go` resolves each task's role to a **real employee** through Phase 3's
  enterprise graph. A role with no resolvable department head leaves the task
  UNASSIGNED and flagged - assigning an arbitrary employee to satisfy a non-null
  column would put a real person's name against work nobody agreed they own.
- **Everything is `state='draft'`**, enforced again at the store boundary
  regardless of what the synthesis layer produced. Approving a workflow records a
  human decision and dispatches NOTHING: no email, no ticket, no calendar invite.
- New endpoints `GET /api/workflows`, `GET /api/workflows/:id/tasks`,
  `POST /api/workflows/:id/approve`; new screen `app/workflows/page.tsx`.

### `/regulatory-feed` rewritten on real data

`lib/amendment-sim.ts` (281 lines of hardcoded MITC data) and the 668-line
`components/amendment/steps.tsx` that consumed it are **deleted**. The screen now
renders `GET /api/regulatory-feed`: which circulars the graph holds, how each
arrived (ingested upload vs seeded fixture), its `circular_relation` edges, and -
for an amendment - the `clause_lineage` diff with both texts, where the OLD text
comes from the superseded clause version. The "CHANAKYA does not poll SEBI"
statement is served by the API rather than written into the page, so it cannot
drift from what the system does.

### Connectors (`internal/connect`)

**Read-only enforced by the TYPE SYSTEM, not by convention.** The `Connector`
interface exposes exactly three methods - `Descriptor`, `Health`, `Fetch`. There
is no `Write`, no `Send`, no `Delete`: not unimplemented, ABSENT. A connector
cannot write to a customer system because the vocabulary to do so does not exist
in the interface it must satisfy. `TestConnectorInterfaceHasNoWriteMethod`
asserts this reflectively, so adding one would fail CI.

14 adapters plus a webhook receiver (Gmail, Outlook, both calendars, Drive,
OneDrive, SharePoint, Dropbox, Jira, Slack, Teams, Notion, Confluence, internal
REST), all `mode: mock` reading Phase 3's seeded data with **zero network calls**.
`SelectConnector` mirrors `llm.SelectExtractor`; a configured live credential
returns an **explicit error** rather than silently serving mock data as though it
came from the firm's real systems. An unsupported query kind returns a typed
error and an unwired data source returns an empty STALE result - neither
fabricates records, because invented evidence in a compliance audit trail is the
worst failure this system could have.

### Testing corpus (`testdata/`, `internal/corpus`)

~20 documents, not 45. `manifest.json` carries one entry per document
(`governed_by`, `provides_evidence_for`, `stale_if_clause_amended`) plus the
**expected value of every deliberate gap**, so CI regression-tests the demo
NARRATIVE: if a fixture refactor turns the 118-client gap into 117, everything
still compiles and every other test passes while the product's central claim has
silently changed. Assertions: every manifest document exists, every
obligation-bearing clause has a mapped document, every control has evidence, and
every seeded gap still reports its exact expected value.

**Adversarial test.** A corpus document embeds a prompt-injection payload. Three
independent guards are asserted: the strict schema rejects the requested shape
(`additionalProperties:false` kills `exec`), the citation gate makes a fabricated
obligation impossible, and nothing arrives approved. Documented in the README -
including the honest finding that the extractor DOES quote the injected sentence,
because it is genuinely in the document, and that this is the citation gate
working rather than failing.

**Proven live:** `/api/connectors` → **15/15 read-only**, all mock, all healthy.
`/api/workflows` → **10 draft workflows / 40 draft tasks**, verb-selected
(`certify`→Attestation, `disclose`→Policy update + Client notification), owners
resolved to Priya Menon and Manish Gupta by name. A 2-character note → 400; a
valid approval → 200 with `dispatched:false` and **0 tasks moved out of draft**;
a second approval → 409. After ingesting an amended circular, `/api/regulatory-feed`
shows three circulars with their real relations and the actual recorded diff
(`added: 1, modified: 2, unchanged: 20`; clause 5.1 at score 0.928).
`go vet ./...`, `go test ./...` and the frontend `typecheck`/`build` all pass with
`amendment-sim.ts` gone.

---

## Build & test summary

Backend: `go build ./...`, `go vet ./...`, `go test ./...` - all green (unit
tests across `domain`, `store`, `llm`, `compiler`, `vec`, `signoff`, `policy`,
`feed`). Frontend: `npm run typecheck` + `npm run build` - green. No secrets in
code; all config via environment (`backend/.env.example`, `apps/web/.env.example`).
No Docker anywhere; SQLite (`modernc.org/sqlite`, pure Go) is the single-file
system of record.
