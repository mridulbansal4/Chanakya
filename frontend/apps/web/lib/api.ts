// Typed client for the CHANAKYA backend API.
//
// The base URL comes from NEXT_PUBLIC_API_BASE_URL (see .env.example); it
// defaults to the local dev backend. Every response shape is typed here so
// screens consume real, checked data - never `any`.

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL?.replace(/\/$/, "") ??
  "http://localhost:8080"

/** A single named health check (e.g. the database). */
export interface HealthCheck {
  ok: boolean
  error: string
}

/** Response shape of GET /health. */
export interface HealthResponse {
  status: "ok" | "degraded"
  version: string
  checks: {
    database: HealthCheck
  }
}

/** Error thrown when an API call fails at the transport or HTTP layer. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status?: number,
  ) {
    super(message)
    this.name = "ApiError"
  }
}

/**
 * apiFetch performs a typed JSON request against the backend. It throws
 * ApiError on network failure or a non-2xx status so callers can distinguish
 * "backend unreachable" from a valid degraded payload.
 */
export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const url = `${API_BASE_URL}${path.startsWith("/") ? path : `/${path}`}`
  let res: Response
  try {
    res = await fetch(url, {
      ...init,
      headers: { Accept: "application/json", ...init?.headers },
    })
  } catch (cause) {
    throw new ApiError(
      `network error contacting ${url}: ${(cause as Error).message}`,
    )
  }
  if (!res.ok) {
    throw new ApiError(`request to ${url} failed`, res.status)
  }
  return (await res.json()) as T
}

/**
 * getHealth fetches backend health, reading the JSON body on BOTH 200 (ok) and
 * 503 (degraded) so the UI can tell "backend up but database degraded" apart
 * from "backend unreachable". A network failure throws ApiError → treat as down.
 */
export async function getHealth(signal?: AbortSignal): Promise<HealthResponse> {
  const url = `${API_BASE_URL}/health`
  let res: Response
  try {
    res = await fetch(url, { headers: { Accept: "application/json" }, signal })
  } catch (cause) {
    throw new ApiError(`network error contacting ${url}: ${(cause as Error).message}`)
  }
  return (await res.json()) as HealthResponse
}

// ---- Obligation graph API (Phase 3) --------------------------------------

export type DeonticType = "MUST" | "MUST_NOT" | "MAY"
export type ObligationStatus =
  | "pending"
  | "needs_review"
  | "approved"
  | "rejected"

export interface Obligation {
  id: string
  clause_id: string
  clause_ref: string
  clause_heading: string
  bearer: string
  deontic_type: DeonticType
  condition: string
  threshold: Record<string, unknown>
  deadline: string
  penalty: string
  status: ObligationStatus
  confidence: number
  source_clause_ref: string
  source_sentence: string
  valid_from: string
  valid_to?: string
}

export interface ObligationDetail extends Obligation {
  clause_text: string
}

export interface ObligationListResponse {
  as_of: string
  count: number
  obligations: Obligation[]
}

export interface Posture {
  as_of: string
  obligations_in_force: number
  pending: number
  needs_review: number
  approved: number
  gaps: number
  pending_signoffs: number
}

export interface GraphNode {
  id: string
  type: "clause" | "obligation"
  label: string
  sublabel?: string
  ref?: string
  status?: ObligationStatus
  deontic?: DeonticType
}

export interface GraphEdge {
  id: string
  source: string
  target: string
  kind: "clause_parent" | "clause_obligation"
}

export interface GraphPayload {
  as_of: string
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface ObligationFilters {
  asOf?: string // YYYY-MM-DD
  bearer?: string
  deontic?: DeonticType | ""
  status?: ObligationStatus | ""
}

function qs(params: Record<string, string | undefined>): string {
  const s = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v) s.set(k, v)
  }
  const str = s.toString()
  return str ? `?${str}` : ""
}

export function listObligations(
  f: ObligationFilters,
  signal?: AbortSignal,
): Promise<ObligationListResponse> {
  return apiFetch<ObligationListResponse>(
    `/api/obligations${qs({ as_of: f.asOf, bearer: f.bearer, deontic: f.deontic, status: f.status })}`,
    { signal },
  )
}

export function getObligation(
  id: string,
  signal?: AbortSignal,
): Promise<ObligationDetail> {
  return apiFetch<ObligationDetail>(
    `/api/obligation?id=${encodeURIComponent(id)}`,
    { signal },
  )
}

export function getPosture(asOf?: string, signal?: AbortSignal): Promise<Posture> {
  return apiFetch<Posture>(`/api/posture${qs({ as_of: asOf })}`, { signal })
}

export function getGraph(asOf?: string, signal?: AbortSignal): Promise<GraphPayload> {
  return apiFetch<GraphPayload>(`/api/graph${qs({ as_of: asOf })}`, { signal })
}

// ---- Amendment blast radius (Phase 4) ------------------------------------

export interface Clause {
  id: string
  clause_ref: string
  heading: string
  text: string
  parent_id?: string
}

export interface BlastNode {
  id: string
  type: "clause" | "obligation" | "control" | "evidence"
  label: string
  sublabel?: string
  ref?: string
  status?: ObligationStatus
  deontic?: DeonticType
  kind: "amended" | "direct" | "semantic" | "control" | "evidence"
  layer: number
  similarity?: number
}

export interface BlastEdge {
  id: string
  source: string
  target: string
  kind: "clause_obligation" | "semantic" | "obligation_control" | "control_evidence"
}

export interface BlastChange {
  category: "obligation" | "control" | "evidence"
  ref?: string
  detail: string
}

export interface BlastRadius {
  as_of: string
  clause_ref: string
  amended_text: string
  threshold: number
  nodes: BlastNode[]
  edges: BlastEdge[]
  changes: BlastChange[]
  summary: { obligations: number; controls: number; evidence: number }
}

export function listClauses(
  asOf?: string,
  signal?: AbortSignal,
): Promise<{ clauses: Clause[] }> {
  return apiFetch<{ clauses: Clause[] }>(`/api/clauses${qs({ as_of: asOf })}`, {
    signal,
  })
}

export interface BlastRequest {
  clause_ref: string
  amended_text: string
  threshold?: number
  as_of?: string
}

export function computeBlastRadius(req: BlastRequest): Promise<BlastRadius> {
  return apiFetch<BlastRadius>("/api/amendments/blast-radius", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  })
}

// ---- Evidence, gaps & tickets (Phase 5) ----------------------------------

export interface MappedEvidence {
  id: string
  name: string
  source_system: string
}

export interface ObligationEvidence {
  id: string
  clause_ref: string
  clause_heading: string
  deontic_type: DeonticType
  status: ObligationStatus
  deadline: string
  source_sentence: string
  valid_from: string
  controls: string[]
  evidence: MappedEvidence[]
  satisfied: boolean
  gap_reason?: string
}

export interface EvidenceSource {
  id: string
  name: string
  source_system: string
  kind: string
  read_only: boolean
}

export interface EvidenceMapping {
  as_of: string
  obligations: ObligationEvidence[]
  sources: EvidenceSource[]
  satisfied: number
  gaps: number
}

export type TicketState = "draft" | "filed" | "resolved"

export interface Ticket {
  id: string
  obligation_id: string
  clause_ref: string
  title: string
  detail: string
  owner: string
  deadline: string
  citation: string
  state: TicketState
  valid_from: string
}

export interface TicketsResponse {
  as_of: string
  count: number
  tickets: Ticket[]
}

export function getEvidenceMap(
  asOf?: string,
  signal?: AbortSignal,
): Promise<EvidenceMapping> {
  return apiFetch<EvidenceMapping>(`/api/evidence${qs({ as_of: asOf })}`, {
    signal,
  })
}

export function getTickets(
  asOf?: string,
  signal?: AbortSignal,
): Promise<TicketsResponse> {
  return apiFetch<TicketsResponse>(`/api/tickets${qs({ as_of: asOf })}`, {
    signal,
  })
}

// ---- Review queue & Ed25519 sign-off (Phase 6) ---------------------------

export interface ReviewQueueResponse {
  as_of: string
  count: number
  obligations: Obligation[]
}

export interface SignoffRecord {
  id: string
  obligation_id: string
  action: "approve" | "reject"
  obligation_hash: string
  signature?: string
  public_key?: string
  signed_by: string
  justification: string
  created_at: string
}

export interface Verification {
  valid: boolean
  reason: string
  signed_hash?: string
  current_hash?: string
}

export interface SignoffResponse {
  signoff: SignoffRecord
  verified: boolean
}

export interface SignoffCorrections {
  deontic_type?: DeonticType
  condition?: string
  deadline?: string
  threshold?: Record<string, unknown>
}

export interface SignoffRequest {
  obligation_id: string
  action: "approve" | "reject"
  signed_by: string
  justification: string
  corrections?: SignoffCorrections
}

export function getReviewQueue(
  asOf?: string,
  signal?: AbortSignal,
): Promise<ReviewQueueResponse> {
  return apiFetch<ReviewQueueResponse>(`/api/review-queue${qs({ as_of: asOf })}`, {
    signal,
  })
}

export function postSignoff(req: SignoffRequest): Promise<SignoffResponse> {
  return apiFetch<SignoffResponse>("/api/signoff", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  })
}

export function getSignoffFor(
  obligationId: string,
  signal?: AbortSignal,
): Promise<{ signoff: SignoffRecord; verification: Verification }> {
  return apiFetch<{ signoff: SignoffRecord; verification: Verification }>(
    `/api/signoff?obligation_id=${encodeURIComponent(obligationId)}`,
    { signal },
  )
}

// ---- Policy-as-Code (Phase 7) --------------------------------------------

export type PolicyStage = "audit" | "soft" | "hard"

export interface PolicyCandidate {
  obligation_id: string
  clause_ref: string
  clause_heading: string
  deontic_type: DeonticType
  compiled: boolean
  stage?: PolicyStage
}

export interface PolicyRecord {
  id: string
  obligation_id: string
  package_name: string
  rego: string
  stage: PolicyStage
  compiled_at: string
}

export interface PolicyEvalResult {
  obligation_id: string
  stage: PolicyStage
  compliant: boolean
  applicable: boolean
  denies: string[]
  blocked: boolean
  trace: string
}

export type FirmState = Record<string, unknown>

export function listPolicies(
  asOf?: string,
  signal?: AbortSignal,
): Promise<{ as_of: string; candidates: PolicyCandidate[] }> {
  return apiFetch(`/api/policies${qs({ as_of: asOf })}`, { signal })
}

export function getFirmState(asOf?: string, signal?: AbortSignal): Promise<FirmState> {
  return apiFetch<FirmState>(`/api/firm-state${qs({ as_of: asOf })}`, { signal })
}

export function compilePolicy(obligationId: string): Promise<{ policy: PolicyRecord }> {
  return apiFetch("/api/policy/compile", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ obligation_id: obligationId }),
  })
}

export function getPolicy(
  obligationId: string,
  signal?: AbortSignal,
): Promise<{ policy: PolicyRecord; eval?: PolicyEvalResult }> {
  return apiFetch(
    `/api/policy?obligation_id=${encodeURIComponent(obligationId)}`,
    { signal },
  )
}

export function setPolicyStage(
  obligationId: string,
  stage: PolicyStage,
): Promise<unknown> {
  return apiFetch("/api/policy/stage", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ obligation_id: obligationId, stage }),
  })
}

export function evaluatePolicy(req: {
  obligation_id: string
  input: FirmState
  stage?: PolicyStage
}): Promise<PolicyEvalResult> {
  return apiFetch("/api/policy/evaluate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  })
}

// ---- Audit lineage & regulator feed (Phase 8) ----------------------------

export type LineageNodeType =
  | "clause"
  | "obligation"
  | "control"
  | "evidence"
  | "signoff"
  | "policy"

export interface LineageNode {
  id: string
  type: LineageNodeType
  label: string
  sublabel?: string
  ref?: string
  status?: string
}

export interface LineageEdge {
  id: string
  source: string
  target: string
  kind: string
}

export interface Lineage {
  as_of: string
  nodes: LineageNode[]
  edges: LineageEdge[]
  counts: Record<string, number>
}

export interface FeedSignoff {
  action: string
  signed_by: string
  obligation_hash: string
  signature?: string
  public_key?: string
}

export interface FeedProvenance {
  source_clause_ref: string
  source_sentence: string
  extractor_confidence: number
  signoff?: FeedSignoff
}

export interface FeedObligation {
  id: string
  clause_ref: string
  bearer: string
  deontic_type: DeonticType
  condition?: string
  threshold: Record<string, unknown>
  deadline?: string
  status: ObligationStatus
  valid_from: string
  provenance: FeedProvenance
}

export interface RegulatorFeed {
  feed_version: string
  source: string
  regulator: string
  generated_as_of: string
  circular: { id: string; title: string; issued_on: string }
  obligations: FeedObligation[]
}

export function getLineage(asOf?: string, signal?: AbortSignal): Promise<Lineage> {
  return apiFetch<Lineage>(`/api/lineage${qs({ as_of: asOf })}`, { signal })
}

export function getFeed(asOf?: string, signal?: AbortSignal): Promise<RegulatorFeed> {
  return apiFetch<RegulatorFeed>(`/api/feed${qs({ as_of: asOf })}`, { signal })
}

// ---- PDF ingestion pipeline (Phase 2) ------------------------------------
//
// Nothing an ingestion run produces reaches the regulatory graph until
// approveIngest() is called. Everything below the upload is a PROPOSAL.

export type IngestState =
  | "queued"
  | "running"
  | "preview"
  | "approved"
  | "discarded"
  | "failed"

export type DocKind =
  | "master_circular"
  | "circular"
  | "amendment"
  | "notification"
  | "faq"
  | "guidance_note"
  | "consultation_paper"

export type UnitRole =
  | "norm"
  | "condition"
  | "exception"
  | "deadline"
  | "penalty"
  | "definition"
  | "cross_ref"
  | "scope"

export interface IngestAccepted {
  ingest_id: string
  job_id?: string
  state: IngestState
  duplicate: boolean
  sha256: string
  filename: string
  page_count?: number
  stages?: string[]
}

export interface IngestProgress {
  stage: string
  done: number
  total: number
  detail: string
  index: number
  count: number
}

export interface IngestStatus {
  ingest_id: string
  state: IngestState
  stage: string
  filename: string
  sha256: string
  doc_kind: string
  error: string
  stages: string[]
  counts: {
    clauses: number
    obligations: number
    semantic_units: number
    resolved_references: number
    dangling_references: number
    rejected: number
  }
  progress?: IngestProgress
}

export interface CircularMeta {
  circular_no: string
  title: string
  issued_on: string
  effective_from: string
  regulator: string
  department: string
  supersedes: string[] | null
  amends: string[] | null
  references: string[] | null
  applies_to: string[] | null
  doc_kind: DocKind
  from_regex: string[] | null
}

export interface ProposedClause {
  ID: string
  ClauseRef: string
  ParentID: string
  Heading: string
  Text: string
  Ordinal: number
}

export interface ProposedObligation {
  obligation: {
    ID: string
    ClauseID: string
    Bearer: string
    DeonticType: DeonticType
    Condition: string
    Deadline: string
    SourceClauseRef: string
    SourceSentence: string
    Confidence: number
    Status: ObligationStatus
  }
  clause_ref: string
  clause_text: string
}

export interface ProposedUnit {
  id: string
  clause_id: string
  ordinal: number
  role: UnitRole
  text: string
  start_offset: number
  end_offset: number
}

export interface DanglingReference {
  id: string
  clause_id: string
  raw_text: string
  kind: string
  reason: string
}

export interface ProposedRelation {
  id: string
  from_circular: string
  to_ref: string
  kind: "supersedes" | "amends" | "references"
}

export interface IngestProposal {
  sha256: string
  filename: string
  meta: CircularMeta
  circular: { ID: string; Title: string; Regulator: string; IssuedOn: string }
  clauses: ProposedClause[] | null
  units: ProposedUnit[] | null
  clause_refs: { from_clause_id: string; to_clause_id: string; raw_text: string }[] | null
  dangling_references: DanglingReference[] | null
  circular_relations: ProposedRelation[] | null
  obligations: ProposedObligation[] | null
  rejected: { clause_ref: string; reason: string }[] | null
  degraded: boolean
  extractor: string
  compiler: string
  /** Present only when this document amends one already in the graph. */
  amendment?: AmendmentDiff
}

export interface IngestPreview {
  ingest_id: string
  state: IngestState
  committed: boolean
  proposal: IngestProposal
}

export interface IngestRunSummary {
  id: string
  sha256: string
  filename: string
  state: IngestState
  stage: string
  doc_kind: string
  circular_id: string
  created_at: string
}

/**
 * uploadPdf posts the PDF and returns as soon as the run is queued. The
 * pipeline runs server-side; progress arrives over ingestEventsUrl().
 */
export async function uploadPdf(file: File): Promise<IngestAccepted> {
  const form = new FormData()
  form.append("file", file)
  const url = `${API_BASE_URL}/api/ingest`
  let res: Response
  try {
    res = await fetch(url, { method: "POST", body: form })
  } catch (cause) {
    throw new ApiError(`network error contacting ${url}: ${(cause as Error).message}`)
  }
  if (!res.ok) {
    // Stage 0 rejections carry a specific, actionable message (encrypted,
    // scanned, too large) - surface it rather than a generic failure.
    let detail = `upload failed (${res.status})`
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) detail = body.error
    } catch {
      // fall through to the generic message
    }
    throw new ApiError(detail, res.status)
  }
  return (await res.json()) as IngestAccepted
}

export function getIngestStatus(
  id: string,
  signal?: AbortSignal,
): Promise<IngestStatus> {
  return apiFetch<IngestStatus>(`/api/ingest/${encodeURIComponent(id)}`, { signal })
}

export function listIngestRuns(
  signal?: AbortSignal,
): Promise<{ count: number; runs: IngestRunSummary[] | null }> {
  return apiFetch(`/api/ingest`, { signal })
}

export function getIngestPreview(
  id: string,
  signal?: AbortSignal,
): Promise<IngestPreview> {
  return apiFetch<IngestPreview>(
    `/api/ingest/${encodeURIComponent(id)}/preview`,
    { signal },
  )
}

export interface IngestApproveResult {
  ingest_id: string
  state: IngestState
  circular_id: string
  approved_by: string
  approved_at: string
  committed: {
    clauses: number
    obligations: number
    units: number
    relations: number
    dangling: number
  }
}

/** approveIngest is the human gate: the only path into the regulatory graph. */
export function approveIngest(
  id: string,
  signedBy: string,
  justification: string,
): Promise<IngestApproveResult> {
  return apiFetch<IngestApproveResult>(
    `/api/ingest/${encodeURIComponent(id)}/approve`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ signed_by: signedBy, justification }),
    },
  )
}

export function discardIngest(id: string): Promise<{ ingest_id: string; state: IngestState }> {
  return apiFetch(`/api/ingest/${encodeURIComponent(id)}`, { method: "DELETE" })
}

/** ingestEventsUrl is the SSE endpoint for live stage-by-stage progress. */
export function ingestEventsUrl(id: string): string {
  return `${API_BASE_URL}/api/ingest/${encodeURIComponent(id)}/events`
}

// ---- Enterprise graph & projection (Phase 3) ------------------------------
//
// The firm as queryable, as-of-able data. Every gap below is the RESULT of a
// graph traversal, not a constant - change the seeded data and the gaps change.

export interface EnterpriseFirm {
  id: string
  name: string
  kind: string
  pan: string
  meta_json: string
}

export interface EnterpriseDepartment {
  id: string
  name: string
  function: string
  head_employee_id: string
  head_name: string
  headcount: number
}

export interface EnterpriseGap {
  kind: string
  title: string
  detail: string
  count: number
  subject?: string
  names?: string[] | null
}

export interface EnterpriseSystem {
  id: string
  kind: string
  vendor: string
  connector_id: string
  criticality: string
  owner_dept: string
}

export interface EnterpriseRegister {
  id: string
  kind: string
  row_count: number
  source_system: string
  last_updated: string
  owner_dept: string
  stale_days: number
}

export interface EnterpriseEmployee {
  id: string
  name: string
  role: string
  department_id: string
  department_name: string
  email: string
  certifications: string[] | null
  depth: number
}

export interface EnterpriseSummary {
  as_of: string
  firm: EnterpriseFirm
  departments: EnterpriseDepartment[] | null
  counts: Record<string, number>
  gaps: EnterpriseGap[] | null
  systems: EnterpriseSystem[] | null
  registers: EnterpriseRegister[] | null
  org: EnterpriseEmployee[] | null
}

export interface EnterpriseClient {
  id: string
  name: string
  segment: string
  onboarded_on: string
  risk_profile: string
  adviser_id: string
  adviser_name: string
  service_kind: string
  template_version: string
  agreement_id: string
}

export interface EnterpriseDocument {
  id: string
  kind: string
  title: string
  version: number
  owner_dept: string
  owner_dept_name: string
  status: string
  last_reviewed: string
  months_since_review: number
  stale: boolean
}

export interface ObligationBinding {
  obligation_id: string
  target_type: string
  target_id: string
  target_label: string
  confidence: number
  human_confirmed: boolean
  rationale: string
}

export interface ImpactedControl {
  id: string
  name: string
  kind: string
  owner_dept: string
  owner_dept_name: string
}

export interface ImpactedDepartment {
  id: string
  name: string
  head_name: string
  reason: string
}

export interface EnterpriseImpact {
  as_of: string
  obligation_id: string
  clause_ref: string
  summary: string
  bindings: ObligationBinding[] | null
  controls: ImpactedControl[] | null
  documents: EnterpriseDocument[] | null
  registers: EnterpriseRegister[] | null
  systems: EnterpriseSystem[] | null
  clients: EnterpriseClient[] | null
  departments: ImpactedDepartment[] | null
  owners: EnterpriseEmployee[] | null
  counts: Record<string, number>
  unbound: boolean
}

export function getEnterpriseSummary(
  asOf?: string,
  signal?: AbortSignal,
): Promise<EnterpriseSummary> {
  return apiFetch<EnterpriseSummary>(
    `/api/enterprise/summary${qs({ as_of: asOf })}`,
    { signal },
  )
}

export function getEnterpriseImpact(
  obligationId: string,
  asOf?: string,
  signal?: AbortSignal,
): Promise<EnterpriseImpact> {
  return apiFetch<EnterpriseImpact>(
    `/api/enterprise/impact${qs({ obligation_id: obligationId, as_of: asOf })}`,
    { signal },
  )
}

export function listEnterpriseClients(
  params: { asOf?: string; segment?: string; adviser?: string; template?: string; limit?: string },
  signal?: AbortSignal,
): Promise<{ as_of: string; count: number; clients: EnterpriseClient[] | null }> {
  return apiFetch(
    `/api/clients${qs({
      as_of: params.asOf,
      segment: params.segment,
      adviser: params.adviser,
      template: params.template,
      limit: params.limit,
    })}`,
    { signal },
  )
}

export function listEnterpriseDocuments(
  params: { asOf?: string; stale?: boolean },
  signal?: AbortSignal,
): Promise<{ as_of: string; count: number; documents: EnterpriseDocument[] | null }> {
  return apiFetch(
    `/api/documents${qs({ as_of: params.asOf, stale: params.stale ? "true" : undefined })}`,
    { signal },
  )
}

// ---- Amendment matcher (Phase 3, Stage 9) --------------------------------

export type ChangeKind = "unchanged" | "modified" | "added" | "deleted"

export interface ClauseChange {
  kind: ChangeKind
  new_clause_id?: string
  new_clause_ref?: string
  new_text?: string
  old_clause_id?: string
  old_clause_ref?: string
  old_text?: string
  score: number
  cosine: number
  jaccard: number
  ref_equal: boolean
  text_identical: boolean
  rationale: string
}

export interface AmendmentDiff {
  changes: ClauseChange[] | null
  counts: Record<string, number>
  reused_obligations: number
}

// ---- Workflows, connectors, regulatory corpus (Phase 4) ------------------

export interface WorkflowTask {
  id: string
  workflow_id: string
  title: string
  detail: string
  owner_role: string
  owner_employee_id: string
  owner_name: string
  owner_unresolved: boolean
  state: string
  deadline: string
  ordinal: number
  depends_on: string[] | null
}

export interface WorkflowApproval {
  approver: string
  decision: string
  note: string
  decided_at: string
}

export interface GeneratedWorkflow {
  id: string
  template: string
  title: string
  obligation_id: string
  clause_ref: string
  verb: string
  state: string
  sla: string
  rationale: string
  generated_at: string
  task_count: number
  unresolved_owners: number
  tasks?: WorkflowTask[]
  approval?: WorkflowApproval
}

export function listWorkflows(
  asOf?: string,
  signal?: AbortSignal,
): Promise<{
  as_of: string
  count: number
  draft: number
  workflows: GeneratedWorkflow[] | null
  dispatch_note: string
}> {
  return apiFetch(`/api/workflows${qs({ as_of: asOf })}`, { signal })
}

export function getWorkflowTasks(
  id: string,
  asOf?: string,
  signal?: AbortSignal,
): Promise<GeneratedWorkflow> {
  return apiFetch<GeneratedWorkflow>(
    `/api/workflows/${encodeURIComponent(id)}/tasks${qs({ as_of: asOf })}`,
    { signal },
  )
}

export function approveWorkflow(
  id: string,
  approver: string,
  note: string,
): Promise<{ workflow: GeneratedWorkflow; dispatched: boolean; dispatch_note: string }> {
  return apiFetch(`/api/workflows/${encodeURIComponent(id)}/approve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ approver, note }),
  })
}

export interface ConnectorDescriptor {
  id: string
  kind: string
  vendor: string
  mode: "mock" | "simulated" | "live"
  read_only: boolean
  scopes: string[]
  rate_limit: { requests: number; per: string }
  description: string
  health: { ok: string; detail: string; checked_at: string }
}

export function listConnectors(
  signal?: AbortSignal,
): Promise<{
  count: number
  read_only_count: number
  connectors: ConnectorDescriptor[]
  guarantee: string
}> {
  return apiFetch(`/api/connectors`, { signal })
}

export interface CircularRelation {
  kind: "supersedes" | "amends" | "references"
  to_ref: string
  to_circular: string
}

export interface ClauseLineageChange {
  relation: ChangeKind
  score: number
  new_clause_id: string
  old_clause_id: string
  clause_ref: string
  new_text: string
  old_text: string
}

export interface RegulatoryFeedItem {
  circular_id: string
  title: string
  regulator: string
  issued_on: string
  doc_kind: string
  source: string
  ingest_run_id?: string
  ingest_state?: string
  approved_by?: string
  approved_at?: string
  clauses: number
  obligations: number
  relations: CircularRelation[] | null
  amendment?: {
    counts: Record<string, number>
    changes: ClauseLineageChange[] | null
  }
}

export function getRegulatoryFeed(
  asOf?: string,
  signal?: AbortSignal,
): Promise<{
  as_of: string
  count: number
  circulars: RegulatoryFeedItem[] | null
  runs: IngestRunSummary[] | null
  monitoring_note: string
}> {
  return apiFetch(`/api/regulatory-feed${qs({ as_of: asOf })}`, { signal })
}

/** feedUrl / feedSchemaUrl are the raw endpoints (for "open raw" links). */
export function feedUrl(asOf?: string): string {
  return `${API_BASE_URL}/api/feed${qs({ as_of: asOf })}`
}
export function feedSchemaUrl(): string {
  return `${API_BASE_URL}/api/feed/schema`
}
