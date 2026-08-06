"use client"

import * as React from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { motion } from "framer-motion"

import {
  approveIngest,
  discardIngest,
  getIngestPreview,
  getIngestStatus,
  ingestEventsUrl,
  listIngestRuns,
  uploadPdf,
  type IngestAccepted,
  type IngestPreview,
  type IngestProgress,
  type IngestStatus,
} from "@/lib/api"
import { PageHeader } from "@/components/page-header"

/** The reviewer identity recorded on approval. Auth is a stated non-goal. */
const REVIEWER = "Priya Menon"

const MIN_JUSTIFICATION = 20

/** Human labels for the backend's stage identifiers. */
const STAGE_LABELS: Record<string, string> = {
  intake: "Intake & content addressing",
  layout: "Layout extraction",
  structure: "Clause-tree parsing",
  metadata: "Metadata extraction",
  normalize: "Normalization",
  segment: "Semantic segmentation",
  cross_reference: "Cross-reference resolution",
  compile: "Obligation extraction",
  ready_for_review: "Ready for review",
}

function stageLabel(stage: string): string {
  return STAGE_LABELS[stage] ?? stage
}

/**
 * useIngestStream subscribes to the server's SSE progress stream.
 *
 * A dropped connection does NOT cancel the run - the pipeline keeps going
 * server-side - so on error we fall back to polling GET /api/ingest/:id, which
 * is exactly the reconnect path the backend documents.
 */
function useIngestStream(ingestId: string | null): {
  progress: IngestProgress | null
  streamClosed: boolean
} {
  const [progress, setProgress] = React.useState<IngestProgress | null>(null)
  const [streamClosed, setStreamClosed] = React.useState(false)

  React.useEffect(() => {
    if (!ingestId) return
    setProgress(null)
    setStreamClosed(false)

    const source = new EventSource(ingestEventsUrl(ingestId))
    source.addEventListener("progress", (event) => {
      const data = JSON.parse((event as MessageEvent<string>).data) as IngestProgress
      setProgress(data)
    })
    source.addEventListener("done", () => {
      setStreamClosed(true)
      source.close()
    })
    source.onerror = () => {
      // Never treat this as a failed run: the job is unaffected by the browser.
      setStreamClosed(true)
      source.close()
    }
    return () => source.close()
  }, [ingestId])

  return { progress, streamClosed }
}

export default function IngestPage() {
  const queryClient = useQueryClient()
  const [ingestId, setIngestId] = React.useState<string | null>(null)
  const [uploadError, setUploadError] = React.useState<string | null>(null)
  const [accepted, setAccepted] = React.useState<IngestAccepted | null>(null)
  const [justification, setJustification] = React.useState("")
  const [approveError, setApproveError] = React.useState<string | null>(null)
  const [approving, setApproving] = React.useState(false)
  const [committed, setCommitted] = React.useState<string | null>(null)

  const { progress, streamClosed } = useIngestStream(ingestId)

  const status = useQuery<IngestStatus>({
    queryKey: ["ingest-status", ingestId],
    queryFn: ({ signal }) => getIngestStatus(ingestId as string, signal),
    enabled: ingestId !== null,
    // Poll while the run is in flight; this is also the recovery path when the
    // SSE stream drops.
    refetchInterval: (query) => {
      const state = query.state.data?.state
      return state === "queued" || state === "running" ? 1500 : false
    },
  })

  const settled =
    status.data?.state === "preview" ||
    status.data?.state === "approved" ||
    status.data?.state === "failed" ||
    status.data?.state === "discarded"

  const preview = useQuery<IngestPreview>({
    queryKey: ["ingest-preview", ingestId],
    queryFn: ({ signal }) => getIngestPreview(ingestId as string, signal),
    enabled: ingestId !== null && (status.data?.state === "preview" || status.data?.state === "approved"),
  })

  const runs = useQuery({
    queryKey: ["ingest-runs"],
    queryFn: ({ signal }) => listIngestRuns(signal),
  })

  async function onFile(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    setUploadError(null)
    setCommitted(null)
    setApproveError(null)
    setJustification("")
    try {
      const res = await uploadPdf(file)
      setAccepted(res)
      setIngestId(res.ingest_id)
      void queryClient.invalidateQueries({ queryKey: ["ingest-runs"] })
    } catch (cause) {
      setUploadError((cause as Error).message)
    } finally {
      event.target.value = ""
    }
  }

  async function onApprove() {
    if (!ingestId) return
    setApproving(true)
    setApproveError(null)
    try {
      const res = await approveIngest(ingestId, REVIEWER, justification)
      setCommitted(
        `${res.committed.clauses} clauses and ${res.committed.obligations} obligations committed to ${res.circular_id}`,
      )
      await queryClient.invalidateQueries()
    } catch (cause) {
      setApproveError((cause as Error).message)
    } finally {
      setApproving(false)
    }
  }

  async function onDiscard() {
    if (!ingestId) return
    try {
      await discardIngest(ingestId)
      await status.refetch()
      void queryClient.invalidateQueries({ queryKey: ["ingest-runs"] })
    } catch (cause) {
      setApproveError((cause as Error).message)
    }
  }

  const stages = status.data?.stages ?? accepted?.stages ?? Object.keys(STAGE_LABELS)
  const currentStage = progress?.stage ?? status.data?.stage ?? ""
  const currentIndex = stages.indexOf(currentStage)
  const proposal = preview.data?.proposal

  return (
    <div className="mx-auto w-full max-w-6xl px-6 py-6">
      <PageHeader
        eyebrow="Ingestion"
        title="Ingest a circular"
        description="Upload a SEBI circular. CHANAKYA parses it into a clause tree, extracts metadata, segments each clause, resolves cross-references and proposes obligations - then stops. Nothing enters the graph until you approve it."
      />

      {/* Upload */}
      <section className="mt-4 rounded-lg border border-line-subtle bg-raised p-5">
        <label className="flex cursor-pointer flex-col items-start gap-2">
          <span className="text-sm font-medium text-fg">Source document (PDF, max 25 MiB)</span>
          <input
            type="file"
            accept="application/pdf"
            onChange={onFile}
            className="block w-full max-w-md text-sm text-fg-muted file:mr-3 file:rounded-md file:border file:border-line-strong file:bg-elevated file:px-3 file:py-1.5 file:text-sm file:text-fg hover:file:bg-raised"
          />
        </label>
        <p className="mt-2 text-xs text-fg-muted">
          Digitally-generated PDFs only. Scanned documents are rejected: a verbatim citation
          cannot be guaranteed from an OCR transcription.
        </p>
        {uploadError && (
          <p className="mt-3 rounded-md border border-risk/40 bg-risk/10 px-3 py-2 text-sm text-risk">
            {uploadError}
          </p>
        )}
        {accepted?.duplicate && (
          <p className="mt-3 text-sm text-warn">
            This document has already been uploaded - showing the existing run.
          </p>
        )}
      </section>

      {/* Live pipeline */}
      {ingestId && (
        <section className="mt-5 rounded-lg border border-line-subtle bg-raised p-5">
          <div className="flex items-baseline justify-between">
            <h2 className="font-display text-lg tracking-tight">Pipeline</h2>
            <span className="tnum text-xs text-fg-muted">{ingestId}</span>
          </div>

          <ol className="mt-4 space-y-1.5">
            {stages.map((stage, i) => {
              const done = currentIndex > i || status.data?.state === "preview" || status.data?.state === "approved"
              const active = currentIndex === i && !settled
              return (
                <li key={stage} className="flex items-center gap-3 text-sm">
                  <span
                    className={
                      "inline-flex size-5 shrink-0 items-center justify-center rounded-full border text-[10px] " +
                      (done
                        ? "border-ok/50 bg-ok/15 text-ok"
                        : active
                          ? "border-accent/60 bg-accent/15 text-accent"
                          : "border-line-subtle text-fg-muted")
                    }
                  >
                    {done ? "✓" : i + 1}
                  </span>
                  <span className={active ? "text-fg" : done ? "text-fg-muted" : "text-fg-muted/60"}>
                    {stageLabel(stage)}
                  </span>
                  {active && progress && progress.total > 1 && (
                    <span className="tnum text-xs text-fg-muted">
                      {progress.done}/{progress.total}
                    </span>
                  )}
                  {active && progress?.detail && (
                    <span className="truncate text-xs text-fg-muted">{progress.detail}</span>
                  )}
                  {active && (
                    <motion.span
                      aria-hidden
                      className="h-px flex-1 bg-accent/40"
                      initial={{ scaleX: 0 }}
                      animate={{ scaleX: 1 }}
                      style={{ originX: 0 }}
                      transition={{ duration: 0.4 }}
                    />
                  )}
                </li>
              )
            })}
          </ol>

          {streamClosed && !settled && (
            <p className="mt-3 text-xs text-fg-muted">
              Live stream closed - the run continues on the server; polling for its state.
            </p>
          )}

          {status.data?.state === "failed" && (
            <p className="mt-4 rounded-md border border-risk/40 bg-risk/10 px-3 py-2 text-sm text-risk">
              Failed at <strong>{stageLabel(status.data.stage)}</strong>: {status.data.error}
            </p>
          )}

          {status.data && settled && (
            <dl className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
              {[
                ["Clauses", status.data.counts.clauses],
                ["Obligations", status.data.counts.obligations],
                ["Semantic units", status.data.counts.semantic_units],
                ["Refs resolved", status.data.counts.resolved_references],
                ["Refs dangling", status.data.counts.dangling_references],
                ["Rejected", status.data.counts.rejected],
              ].map(([label, value]) => (
                <div key={label as string} className="rounded-md border border-line-subtle px-3 py-2">
                  <dt className="text-[11px] uppercase tracking-wide text-fg-muted">{label}</dt>
                  <dd className="tnum text-xl text-fg">{value}</dd>
                </div>
              ))}
            </dl>
          )}
        </section>
      )}

      {/* Proposal preview */}
      {proposal && (
        <section className="mt-5 rounded-lg border border-line-subtle bg-raised p-5">
          <div className="flex items-baseline justify-between">
            <h2 className="font-display text-lg tracking-tight">Proposal</h2>
            <span className="rounded-full border border-warn/50 bg-warn/10 px-2 py-0.5 text-[11px] uppercase tracking-wide text-warn">
              {preview.data?.committed ? "committed" : "not yet in the graph"}
            </span>
          </div>

          <dl className="mt-4 grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-xs uppercase tracking-wide text-fg-muted">Circular number</dt>
              <dd className="tnum text-fg">{proposal.meta.circular_no || "-"}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wide text-fg-muted">Document kind</dt>
              <dd className="text-fg">{proposal.meta.doc_kind}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wide text-fg-muted">Issued on</dt>
              <dd className="tnum text-fg">{proposal.meta.issued_on?.slice(0, 10) || "-"}</dd>
            </div>
            <div>
              <dt className="text-xs uppercase tracking-wide text-fg-muted">Effective from</dt>
              <dd className="tnum text-fg">{proposal.meta.effective_from?.slice(0, 10) || "-"}</dd>
            </div>
          </dl>

          {proposal.degraded && (
            <p className="mt-3 rounded-md border border-warn/40 bg-warn/10 px-3 py-2 text-xs text-warn">
              This document&apos;s structure was not recognised; it was parsed as a flat paragraph
              list. Obligations and citations are still exact - only the clause hierarchy is poorer.
            </p>
          )}

          {/* Every proposed obligation shows its VERBATIM source sentence. */}
          <h3 className="mt-5 text-sm font-medium text-fg">
            Proposed obligations ({proposal.obligations?.length ?? 0})
          </h3>
          <ul className="mt-2 space-y-2">
            {(proposal.obligations ?? []).map((po) => (
              <li key={po.obligation.ID} className="rounded-md border border-line-subtle px-3 py-2">
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <span className="tnum rounded bg-elevated px-1.5 py-0.5 text-fg">{po.clause_ref}</span>
                  <span className="rounded bg-elevated px-1.5 py-0.5 text-fg">{po.obligation.DeonticType}</span>
                  <span className="text-fg-muted">confidence {po.obligation.Confidence.toFixed(2)}</span>
                </div>
                <p className="mt-1.5 text-sm text-fg">{po.obligation.Condition || po.obligation.Bearer}</p>
                <blockquote className="mt-1.5 border-l-2 border-accent/50 pl-2 text-xs italic text-fg-muted">
                  {po.obligation.SourceSentence}
                </blockquote>
              </li>
            ))}
            {(proposal.obligations?.length ?? 0) === 0 && (
              <li className="text-sm text-fg-muted">
                No obligations proposed for this document kind.
              </li>
            )}
          </ul>

          {(proposal.dangling_references?.length ?? 0) > 0 && (
            <>
              <h3 className="mt-5 text-sm font-medium text-fg">
                Unresolved references ({proposal.dangling_references?.length})
              </h3>
              <p className="text-xs text-fg-muted">
                Recorded, never dropped - a citation the pipeline could not follow is something a
                reviewer needs to see.
              </p>
              <ul className="mt-2 space-y-1 text-xs">
                {(proposal.dangling_references ?? []).slice(0, 12).map((d) => (
                  <li key={d.id} className="flex flex-wrap gap-2 text-fg-muted">
                    <span className="tnum text-fg">{d.raw_text}</span>
                    <span>- {d.reason}</span>
                  </li>
                ))}
              </ul>
            </>
          )}

          {/* Version diff - old and new clause text side by side.
              Every classification here is PROPOSED. Approving the run is what
              applies it, and a `modified` clause supersedes its predecessor
              rather than overwriting it. */}
          {proposal.amendment && (
            <>
              <h3 className="mt-5 text-sm font-medium text-fg">
                Amendment diff against the existing corpus
              </h3>
              <div className="mt-1 flex flex-wrap gap-3 text-xs text-fg-muted">
                {(["unchanged", "modified", "added", "deleted"] as const).map((k) => (
                  <span key={k}>
                    <span className="tnum text-fg">{proposal.amendment?.counts?.[k] ?? 0}</span> {k}
                  </span>
                ))}
                <span>
                  <span className="tnum text-fg">{proposal.amendment.reused_obligations}</span>{" "}
                  obligations reused without re-extraction
                </span>
              </div>

              <ul className="mt-3 space-y-2">
                {(proposal.amendment.changes ?? [])
                  .filter((c) => c.kind !== "unchanged")
                  .slice(0, 20)
                  .map((c) => (
                    <li
                      key={`${c.kind}-${c.new_clause_ref ?? c.old_clause_ref}`}
                      className="rounded-md border border-line-subtle px-3 py-2"
                    >
                      <div className="flex flex-wrap items-center gap-2 text-xs">
                        <span
                          className={
                            "rounded px-1.5 py-0.5 uppercase tracking-wide " +
                            (c.kind === "modified"
                              ? "bg-warn/15 text-warn"
                              : c.kind === "deleted"
                                ? "bg-risk/15 text-risk"
                                : "bg-ok/15 text-ok")
                          }
                        >
                          {c.kind}
                        </span>
                        <span className="tnum text-fg">
                          {c.new_clause_ref ?? c.old_clause_ref}
                        </span>
                        {c.score > 0 && (
                          <span className="tnum text-fg-muted">
                            score {c.score.toFixed(3)} (cos {c.cosine.toFixed(2)} / jac{" "}
                            {c.jaccard.toFixed(2)})
                          </span>
                        )}
                      </div>
                      <p className="mt-1 text-xs text-fg-muted">{c.rationale}</p>
                      {(c.old_text || c.new_text) && (
                        <div className="mt-2 grid gap-2 md:grid-cols-2">
                          <div className="rounded border border-risk/30 bg-risk/5 p-2">
                            <div className="text-[10px] uppercase tracking-wide text-fg-muted">
                              Before
                            </div>
                            <p className="mt-1 text-xs text-fg">{c.old_text || "-"}</p>
                          </div>
                          <div className="rounded border border-ok/30 bg-ok/5 p-2">
                            <div className="text-[10px] uppercase tracking-wide text-fg-muted">
                              After
                            </div>
                            <p className="mt-1 text-xs text-fg">{c.new_text || "-"}</p>
                          </div>
                        </div>
                      )}
                    </li>
                  ))}
              </ul>
            </>
          )}

          {/* The human gate */}
          {preview.data?.state === "preview" && (
            <div className="mt-6 rounded-md border border-line-strong bg-elevated p-4">
              <h3 className="text-sm font-medium text-fg">Approve this document into the graph</h3>
              <p className="mt-1 text-xs text-fg-muted">
                Approving commits the circular, its clauses, obligations, semantic units and
                relations in one transaction. Until then, nothing above exists in the graph.
              </p>
              <textarea
                value={justification}
                onChange={(e) => setJustification(e.target.value)}
                rows={3}
                placeholder="Why is this document being accepted? (minimum 20 characters)"
                className="mt-3 w-full rounded-md border border-line-subtle bg-canvas px-3 py-2 text-sm text-fg placeholder:text-fg-muted/60"
              />
              <div className="mt-3 flex items-center gap-3">
                <button
                  type="button"
                  onClick={onApprove}
                  disabled={approving || justification.trim().length < MIN_JUSTIFICATION}
                  className="rounded-md border border-ok/50 bg-ok/15 px-3 py-1.5 text-sm text-ok disabled:opacity-40"
                >
                  {approving ? "Committing..." : `Approve as ${REVIEWER}`}
                </button>
                <button
                  type="button"
                  onClick={onDiscard}
                  className="rounded-md border border-line-strong px-3 py-1.5 text-sm text-fg-muted hover:text-fg"
                >
                  Discard proposal
                </button>
                <span className="tnum text-xs text-fg-muted">
                  {justification.trim().length}/{MIN_JUSTIFICATION}
                </span>
              </div>
            </div>
          )}

          {committed && (
            <p className="mt-4 rounded-md border border-ok/40 bg-ok/10 px-3 py-2 text-sm text-ok">
              {committed}
            </p>
          )}
          {approveError && (
            <p className="mt-4 rounded-md border border-risk/40 bg-risk/10 px-3 py-2 text-sm text-risk">
              {approveError}
            </p>
          )}
        </section>
      )}

      {/* Run history */}
      <section className="mt-5 rounded-lg border border-line-subtle bg-raised p-5">
        <h2 className="font-display text-lg tracking-tight">Recent runs</h2>
        <ul className="mt-3 divide-y divide-line-subtle">
          {(runs.data?.runs ?? []).map((run) => (
            <li key={run.id} className="flex flex-wrap items-center gap-3 py-2 text-sm">
              <button
                type="button"
                onClick={() => setIngestId(run.id)}
                className="text-fg underline-offset-2 hover:underline"
              >
                {run.filename}
              </button>
              <span className="rounded bg-elevated px-1.5 py-0.5 text-xs text-fg-muted">{run.state}</span>
              {run.doc_kind && (
                <span className="text-xs text-fg-muted">{run.doc_kind}</span>
              )}
              <span className="tnum ml-auto text-xs text-fg-muted">{run.created_at.slice(0, 16).replace("T", " ")}</span>
            </li>
          ))}
          {(runs.data?.count ?? 0) === 0 && (
            <li className="py-2 text-sm text-fg-muted">No documents ingested yet.</li>
          )}
        </ul>
      </section>
    </div>
  )
}
