// Typed client for the CHANAKYA backend API.
//
// The base URL comes from NEXT_PUBLIC_API_BASE_URL (see .env.example); it
// defaults to the local dev backend. Every response shape is typed here so
// screens consume real, checked data — never `any`.

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

/** feedUrl / feedSchemaUrl are the raw endpoints (for "open raw" links). */
export function feedUrl(asOf?: string): string {
  return `${API_BASE_URL}/api/feed${qs({ as_of: asOf })}`
}
export function feedSchemaUrl(): string {
  return `${API_BASE_URL}/api/feed/schema`
}
