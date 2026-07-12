// Scripted data for the Regulatory Amendment Simulation module.
//
// The entire story originates from ONE real regulatory event — SEBI circular
// SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2025/19 (17 Feb 2025), "Most Important Terms and
// Conditions (MITC) for Investment Advisers". Nothing here calls the backend; the
// simulation is a self-contained, deterministic front-end demo that layers on top
// of the existing (already-compliant) baseline. No obligation is invented beyond
// what the MITC circular actually requires.

export const MITC_REF = "SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2025/19"
export const MITC_DATE = "17 Feb 2025"
export const MITC_DEADLINE = "30 June 2025"

export interface Circular {
  id: string
  title: string
  ref?: string
  date: string
  status: "Processed" | "New"
}

/** The regulatory inbox — the firm has already processed prior circulars. */
export const CIRCULARS: Circular[] = [
  { id: "ia-master", title: "IA Master Circular", date: "May 2024", status: "Processed" },
  { id: "cyber", title: "Cybersecurity & Cyber Resilience Framework", date: "Aug 2024", status: "Processed" },
  { id: "charter", title: "Investor Charter for Investment Advisers", date: "Dec 2024", status: "Processed" },
  {
    id: "mitc",
    title: "Most Important Terms and Conditions (MITC) for Investment Advisers",
    ref: MITC_REF,
    date: MITC_DATE,
    status: "New",
  },
]

/** Screen 2 — the processing pipeline stages, played in order. */
export const PIPELINE_STAGES = [
  "Fetching circular",
  "Extracting clauses",
  "Parsing regulation",
  "Comparing previous version",
  "Generating obligation diff",
  "Computing operational impact",
] as const

/** Screen 3 — clause-level before/after diff, faithful to the MITC circular. */
export interface ClauseDiff {
  id: string
  title: string
  kind: "added" | "modified"
  before: string
  after: string
}

export const CLAUSE_DIFFS: ClauseDiff[] = [
  {
    id: "mitc-intro",
    title: "MITC introduced",
    kind: "added",
    before: "— (no equivalent requirement)",
    after:
      "The Investment Adviser shall provide every client the standardized Most Important Terms and Conditions (MITC) specified by SEBI.",
  },
  {
    id: "existing-clients",
    title: "Existing clients must be informed",
    kind: "added",
    before: "— (no equivalent requirement)",
    after: `Existing clients shall be informed of the standardized MITC on or before ${MITC_DEADLINE}.`,
  },
  {
    id: "agreement",
    title: "Agreement template updated",
    kind: "modified",
    before:
      "The client agreement shall record the terms of engagement, fees, and scope of advice.",
    after:
      "The client agreement shall record the terms of engagement, fees, scope of advice, and shall incorporate the standardized MITC.",
  },
  {
    id: "acknowledgement",
    title: "Client acknowledgement required",
    kind: "modified",
    before: "Client consent is recorded at onboarding.",
    after:
      "The client shall acknowledge receipt and understanding of the MITC; the acknowledgement shall be retained as record.",
  },
  {
    id: "deadline",
    title: "Compliance deadline introduced",
    kind: "added",
    before: "— (no equivalent requirement)",
    after: `MITC compliance is mandatory; existing clients are to be covered on or before ${MITC_DEADLINE}.`,
  },
]

/** Screen 4 — extracted obligations (3 modified + 1 new). */
export interface SimObligation {
  ref: string
  summary: string
  kind: "new" | "modified"
  confidence: number
  citation: string
  source: string
  status: "Pending Review"
}

export const SIM_OBLIGATIONS: SimObligation[] = [
  {
    ref: "MITC-1",
    summary:
      "Provide the standardized MITC to every client and obtain the client's acknowledgement.",
    kind: "new",
    confidence: 0.98,
    citation: `${MITC_REF} · ¶2`,
    source: "MITC Circular",
    status: "Pending Review",
  },
  {
    ref: "3.1",
    summary:
      "Client agreement must incorporate the standardized MITC as part of the terms of engagement.",
    kind: "modified",
    confidence: 0.97,
    citation: `${MITC_REF} · ¶3`,
    source: "MITC Circular",
    status: "Pending Review",
  },
  {
    ref: "3.4",
    summary:
      "Retain each client's MITC acknowledgement as part of the firm's records.",
    kind: "modified",
    confidence: 0.96,
    citation: `${MITC_REF} · ¶4`,
    source: "MITC Circular",
    status: "Pending Review",
  },
  {
    ref: "5.2",
    summary: `Inform all existing clients of the MITC on or before ${MITC_DEADLINE}.`,
    kind: "modified",
    confidence: 0.98,
    citation: `${MITC_REF} · ¶5`,
    source: "MITC Circular",
    status: "Pending Review",
  },
]

/** Screen 6 — the blast-radius propagation flow + impact counters. */
export const BLAST_FLOW = [
  "MITC Amendment",
  "Regulation",
  "Obligation",
  "Agreement Template",
  "Client Register",
  "Notification Workflow",
  "Evidence",
  "Audit Pack",
] as const

export interface Counter {
  label: string
  value: number
  suffix?: string
}

export const BLAST_COUNTERS: Counter[] = [
  { label: "Agreements impacted", value: 1 },
  { label: "Clients impacted", value: 2 },
  { label: "Policies updated", value: 1 },
  { label: "Evidence required", value: 5 },
  { label: "Controls affected", value: 2 },
]

/** Screen 7 — auto-generated operational workflow tasks. */
export interface WorkflowTask {
  id: string
  title: string
  owner: string
  priority: "Critical" | "High" | "Normal"
  deadline: string
}

export const WORKFLOWS: WorkflowTask[] = [
  { id: "w1", title: "Update Agreement Template (v1 → v2)", owner: "Compliance Team", priority: "Critical", deadline: "Immediate" },
  { id: "w2", title: "Notify existing clients of the MITC", owner: "Client Servicing", priority: "Critical", deadline: MITC_DEADLINE },
  { id: "w3", title: "Collect client acknowledgements", owner: "Client Servicing", priority: "High", deadline: MITC_DEADLINE },
  { id: "w4", title: "Update internal compliance policy", owner: "Compliance Team", priority: "High", deadline: "Immediate" },
  { id: "w5", title: "Generate audit report", owner: "Compliance Team", priority: "Normal", deadline: "On completion" },
]

/** Screen 8 — the human-review governance card. */
export const REVIEW = {
  sourceClause:
    "Existing clients shall be informed of the standardized MITC on or before 30 June 2025, and the client agreement shall incorporate the MITC with the client's acknowledgement retained as record.",
  aiInterpretation:
    "The firm must update its client agreement to v2 (incorporating the MITC), send the MITC to all existing clients, obtain and retain their acknowledgements, and update the internal policy — completed for existing clients on or before 30 June 2025.",
  suggestedAction:
    "Generate 5 operational tasks: update agreement → v2, notify 2 existing clients, collect acknowledgements, update policy v2, and produce the audit report.",
  affectedSystems: ["Agreement Template", "Client Register", "Notification Workflow", "Internal Policy", "Evidence Register"],
  confidence: 0.98,
  citation: MITC_REF,
}

/** Screen 9 — execution steps, played one after another. */
export const EXECUTION_STEPS = [
  "Agreement template updated to v2",
  "Client notifications generated",
  "Emails sent to existing clients",
  "Acknowledgements received",
  "Internal policy updated to v2",
] as const

/** Screen 10 — evidence collected and linked. */
export interface EvidenceItem {
  label: string
  value: string
}

export const EVIDENCE_ITEMS: EvidenceItem[] = [
  { label: "Email evidence", value: "2 files" },
  { label: "Agreement version", value: "v2" },
  { label: "Client acknowledgements", value: "2 received" },
  { label: "Policy approval", value: "Recorded" },
  { label: "Human sign-off", value: "Completed" },
]

/**
 * The document bundle assembled by the amendment lifecycle. These are the real
 * firm/regulatory artefacts (generated separately) that make up the audit pack.
 */
export const DOCUMENTS: { name: string; kind: "regulation" | "agreement" | "policy" | "register" | "report" | "evidence" }[] = [
  { name: "IA_Master_Circular_2025.pdf", kind: "regulation" },
  { name: "MITC_Circular_17Feb2025.pdf", kind: "regulation" },
  { name: "IA_Agreement_v1.pdf", kind: "agreement" },
  { name: "IA_Agreement_v2.pdf", kind: "agreement" },
  { name: "Internal_Compliance_Policy_v1.pdf", kind: "policy" },
  { name: "Internal_Compliance_Policy_v2.pdf", kind: "policy" },
  { name: "Client_Register.csv", kind: "register" },
  { name: "Blast_Radius_Report.pdf", kind: "report" },
  { name: "Compliance_Change_Report.pdf", kind: "report" },
  { name: "Clause_to_Evidence_Lineage.pdf", kind: "report" },
  { name: "Audit_Pack_Reg19(3).pdf", kind: "report" },
  { name: "Email_Notification_Log.pdf", kind: "evidence" },
  { name: "Client_Acknowledgement_Register.pdf", kind: "evidence" },
  { name: "Human_Approval_Record.pdf", kind: "evidence" },
]

/** Screen 11 — the audit-pack lineage chain. */
export const AUDIT_LINEAGE = [
  "SEBI Circular",
  "Clause",
  "Obligation",
  "Workflow",
  "Evidence",
  "Human Approval",
  "Audit Pack",
] as const

/** Builds the downloadable audit-pack JSON (client-side; no backend). */
export function buildAuditPack(approver: string) {
  return {
    audit_pack: "CHANAKYA — MITC Amendment Compliance",
    generated_for: "Investment Adviser (baseline: IA Master Circular)",
    regulatory_event: {
      circular: MITC_REF,
      date: MITC_DATE,
      subject: "Most Important Terms and Conditions (MITC) for Investment Advisers",
      compliance_deadline: MITC_DEADLINE,
    },
    clause_diff: CLAUSE_DIFFS.map((d) => ({ change: d.title, kind: d.kind, before: d.before, after: d.after })),
    obligations: SIM_OBLIGATIONS.map((o) => ({ ref: o.ref, kind: o.kind, summary: o.summary, confidence: o.confidence, citation: o.citation })),
    impact: Object.fromEntries(BLAST_COUNTERS.map((c) => [c.label, c.value])),
    workflows: WORKFLOWS.map((w) => ({ task: w.title, owner: w.owner, priority: w.priority, deadline: w.deadline, status: "Completed" })),
    human_approval: { approver, decision: "Approved", confidence: REVIEW.confidence, citation: REVIEW.citation },
    evidence: EVIDENCE_ITEMS.map((e) => ({ item: e.label, value: e.value })),
    audit_status: "Ready",
    regulation_ref: "Reg 19(3) — Compliant",
  }
}
