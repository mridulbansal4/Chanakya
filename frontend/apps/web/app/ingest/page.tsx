"use client"

import * as React from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { motion, AnimatePresence } from "framer-motion"
import Link from "next/link"
import { Check, Loader2, X } from "lucide-react"

import {
  getIngestStatus,
  getIngestPreview,
  listIngestRuns,
  ingestEventsUrl,
  uploadPdf,
  type IngestAccepted,
  type IngestProgress,
  type IngestStatus,
  type IngestPreview,
} from "@/lib/api"
import { PageHeader } from "@/components/page-header"
import { cn } from "@workspace/ui/lib/utils"

const ANIMATION_STEPS = [
  { id: "upload", label: "Upload Complete", time: 300 },
  { id: "intake", label: "Validating PDF", time: 300 },
  { id: "layout", label: "Parsing Document Structure", time: 350 },
  { id: "structure", label: "Building Clause Tree", time: 350 },
  { id: "metadata", label: "Extracting Metadata", time: 300 },
  { id: "normalize", label: "Normalizing Text", time: 250 },
  { id: "segment", label: "Segmenting Semantic Units", time: 350 },
  { id: "cross_reference", label: "Resolving Cross References", time: 350 },
  { id: "compare", label: "Comparing Against Existing Corpus", time: 350 },
  { id: "compile", label: "Extracting Draft Obligations", time: 400 },
  { id: "preview", label: "Preparing Review Package", time: 300 },
]

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

  const { progress, streamClosed } = useIngestStream(ingestId)

  const status = useQuery<IngestStatus>({
    queryKey: ["ingest-status", ingestId],
    queryFn: ({ signal }) => getIngestStatus(ingestId as string, signal),
    enabled: ingestId !== null,
    refetchInterval: (query) => {
      const state = query.state.data?.state
      return state === "queued" || state === "running" ? 1500 : false
    },
  })

  const preview = useQuery<IngestPreview>({
    queryKey: ["ingest-preview", ingestId],
    queryFn: ({ signal }) => getIngestPreview(ingestId as string, signal),
    enabled: ingestId !== null && (status.data?.state === "preview" || status.data?.state === "approved"),
  })

  const runs = useQuery({
    queryKey: ["ingest-runs"],
    queryFn: ({ signal }) => listIngestRuns(signal),
  })

  const settled =
    status.data?.state === "preview" ||
    status.data?.state === "approved" ||
    status.data?.state === "failed" ||
    status.data?.state === "discarded"

  const isFinished = status.data?.state === "preview" || status.data?.state === "approved"
  const isFailed = status.data?.state === "failed"

  const [animIndex, setAnimIndex] = React.useState(0)
  const [animationCompleted, setAnimationCompleted] = React.useState(false)

  React.useEffect(() => {
    if (!ingestId) {
      setAnimIndex(0)
      setAnimationCompleted(false)
      return
    }

    let targetIndex = 0
    if (isFinished) {
      targetIndex = ANIMATION_STEPS.length
    } else {
      const currentBackendStage = progress?.stage ?? status.data?.stage ?? ""
      if (currentBackendStage === "ready_for_review") {
        targetIndex = ANIMATION_STEPS.length - 1
      } else {
        const found = ANIMATION_STEPS.findIndex(s => s.id === currentBackendStage)
        targetIndex = found >= 0 ? found : 1
      }
    }

    // If the animation is lagging behind the target, increment it based on the step's specified duration.
    // If the backend has finished, we zip through any remaining steps automatically.
    if (animIndex < targetIndex || (isFinished && animIndex < ANIMATION_STEPS.length)) {
      const timer = setTimeout(() => {
        setAnimIndex((prev) => prev + 1)
      }, ANIMATION_STEPS[animIndex].time)
      return () => clearTimeout(timer)
    } else if (animIndex === ANIMATION_STEPS.length && !animationCompleted && isFinished) {
      setAnimationCompleted(true)
      // Once the animation fully finishes, refresh the recent runs to show the new document
      void queryClient.invalidateQueries({ queryKey: ["ingest-runs"] })
    }
  }, [ingestId, isFinished, progress?.stage, status.data?.stage, accepted?.stages, animIndex, animationCompleted, queryClient])

  async function onFile(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    setUploadError(null)
    setAnimIndex(0)
    setAnimationCompleted(false)
    try {
      const res = await uploadPdf(file)
      setAccepted(res)
      
      // Invalidate the status cache BEFORE setting the ID, so we don't
      // instantly render a stale 'failed' state from a previous attempt
      void queryClient.invalidateQueries({ queryKey: ["ingest-status", res.ingest_id] })
      void queryClient.invalidateQueries({ queryKey: ["ingest-preview", res.ingest_id] })
      
      setIngestId(res.ingest_id)
      if (res.duplicate) {
         void queryClient.invalidateQueries({ queryKey: ["ingest-runs"] })
      }
    } catch (cause) {
      setUploadError((cause as Error).message)
    } finally {
      event.target.value = ""
    }
  }

  return (
    <div className="mx-auto w-full max-w-6xl px-6 py-6">
      <PageHeader
        eyebrow="Regulatory Intake"
        title="Regulatory Intake"
        description="Upload a new regulatory document. CHANAKYA validates it, understands its legal structure, extracts draft obligations and prepares it for human review. Nothing becomes operational until approved."
      />

      {/* Upload */}
      <section className="mt-4 rounded-lg border border-line-subtle bg-raised p-5">
        <label className="flex cursor-pointer flex-col items-start gap-2">
          <span className="text-sm font-medium text-fg">Source document (PDF, max 25 MiB)</span>
          <input
            type="file"
            accept="application/pdf"
            onChange={onFile}
            className="block w-full max-w-md text-sm text-fg-muted file:mr-3 file:rounded-md file:border file:border-line-strong file:bg-elevated file:px-3 file:py-1.5 file:text-sm file:text-fg hover:file:bg-raised transition-colors focus:outline-none focus:ring-2 focus:ring-accent/50"
          />
        </label>
        
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

        <div className="mt-6 border-t border-line-subtle pt-5">
          <h3 className="text-sm font-medium text-fg">Accepted documents</h3>
          <ul className="mt-3 grid grid-cols-2 gap-y-3 gap-x-4 text-xs text-fg-muted sm:grid-cols-3">
            <li className="flex items-center gap-2"><Check className="size-3.5 text-ok" /> Master Circular</li>
            <li className="flex items-center gap-2"><Check className="size-3.5 text-ok" /> Amendment Circular</li>
            <li className="flex items-center gap-2"><Check className="size-3.5 text-ok" /> Circular</li>
            <li className="flex items-center gap-2"><Check className="size-3.5 text-ok" /> Parent Regulation</li>
            <li className="flex items-center gap-2"><Check className="size-3.5 text-ok" /> FAQ / Clarification</li>
            <li className="flex items-center gap-2"><Check className="size-3.5 text-ok" /> Consultation Paper</li>
          </ul>
          <p className="mt-4 text-xs text-fg-muted">
            Digitally generated PDFs only. Scanned PDFs are rejected because verbatim regulatory citations cannot be guaranteed.
          </p>
        </div>
      </section>

      {/* Live pipeline */}
      {ingestId && (
        <section className="mt-5 rounded-lg border border-line-subtle bg-raised p-5 overflow-hidden">
          <div className="flex items-baseline justify-between">
            <h2 className="font-display text-lg tracking-tight">Pipeline</h2>
            <span className="tnum text-xs text-fg-muted">{ingestId}</span>
          </div>

          <div className="mt-5 space-y-1">
            {ANIMATION_STEPS.map((step, i) => {
              const isCompleted = animIndex > i
              const isFailedStep = isFailed && animIndex === i
              const isProcessing = animIndex === i && !isFailed
              const isIdle = animIndex < i

              return (
                <motion.div
                  key={step.id}
                  className={cn(
                    "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-all duration-300",
                    isIdle ? "opacity-50" : "opacity-100",
                  )}
                  animate={
                    isFailedStep
                      ? { backgroundColor: "rgba(239, 68, 68, 0.1)" }
                      : isCompleted
                        ? { backgroundColor: ["rgba(16, 185, 129, 0.1)", "rgba(16, 185, 129, 0)"] }
                        : isProcessing
                          ? { backgroundColor: "rgba(139, 92, 246, 0.05)" }
                          : { backgroundColor: "rgba(0,0,0,0)" }
                  }
                  transition={{ duration: 0.6 }}
                >
                  <div className="relative flex size-5 shrink-0 items-center justify-center">
                    {isIdle && (
                      <div className="size-2.5 rounded-full border border-line-strong" />
                    )}
                    {isProcessing && (
                      <Loader2 className="size-4 animate-spin text-accent" />
                    )}
                    {isCompleted && (
                      <motion.div
                        initial={{ scale: 0 }}
                        animate={{ scale: 1 }}
                        className="flex size-5 items-center justify-center rounded-full bg-ok/15 text-ok"
                      >
                        <Check className="size-3" strokeWidth={3} />
                      </motion.div>
                    )}
                    {isFailedStep && (
                      <motion.div
                        initial={{ scale: 0 }}
                        animate={{ scale: 1 }}
                        className="flex size-5 items-center justify-center rounded-full bg-risk/15 text-risk"
                      >
                        <X className="size-3" strokeWidth={3} />
                      </motion.div>
                    )}
                  </div>
                  
                  <span className={cn(
                    "transition-colors duration-300",
                    isFailedStep ? "text-risk font-medium" : isProcessing ? "text-fg font-medium" : isCompleted ? "text-fg-muted" : "text-fg-muted"
                  )}>
                    {step.label}
                  </span>

                  {isProcessing && (
                    <motion.span 
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      className="ml-2 text-xs font-medium text-accent"
                    >
                      {progress && progress.stage === step.id && progress.total > 0
                        ? `Working... ${Math.round((progress.done / progress.total) * 100)}%`
                        : "Working..."}
                    </motion.span>
                  )}
                  {isFailedStep && (
                    <motion.span 
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      className="ml-2 text-xs font-medium text-risk"
                    >
                      Failed
                    </motion.span>
                  )}
                </motion.div>
              )
            })}
          </div>

          {isFailed && (
            <p className="mt-4 rounded-md border border-risk/40 bg-risk/10 px-3 py-2 text-sm text-risk">
              Failed: {status.data.error}
            </p>
          )}

          <AnimatePresence>
            {animationCompleted && isFinished && !isFailed && (
              <motion.div 
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: "auto" }}
                transition={{ duration: 0.5, ease: "easeOut" }}
                className="mt-8 border-t border-line-subtle pt-6"
              >
                 <h3 className="font-display text-base tracking-tight text-fg">Processing Summary</h3>
                 <dl className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">Document Type</dt>
                     <dd className="mt-1 font-medium text-fg capitalize">{(preview.data?.proposal?.meta?.doc_kind || status.data?.doc_kind || "Unknown").replace(/_/g, " ")}</dd>
                   </div>
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">Pages</dt>
                     <dd className="mt-1 font-medium text-fg">{accepted?.page_count || 1}</dd>
                   </div>
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">Clauses</dt>
                     <dd className="mt-1 font-medium text-fg">{status.data?.counts?.clauses || 0}</dd>
                   </div>
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">Cross References</dt>
                     <dd className="mt-1 font-medium text-fg">{status.data?.counts?.resolved_references || 0}</dd>
                   </div>
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">Draft Obligations</dt>
                     <dd className="mt-1 font-medium text-fg">{status.data?.counts?.obligations || 0}</dd>
                   </div>
                 </dl>
                 
                 <h3 className="mt-8 font-display text-base tracking-tight text-fg">Comparison Result</h3>
                 <dl className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-4">
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">New Clauses</dt>
                     <dd className="mt-1 font-medium text-ok">{preview.data?.proposal?.amendment?.counts?.added || 0}</dd>
                   </div>
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">Modified Clauses</dt>
                     <dd className="mt-1 font-medium text-warn">{preview.data?.proposal?.amendment?.counts?.modified || 0}</dd>
                   </div>
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">Deprecated Clauses</dt>
                     <dd className="mt-1 font-medium text-risk">{preview.data?.proposal?.amendment?.counts?.deleted || 0}</dd>
                   </div>
                   <div className="rounded-md border border-line-subtle bg-elevated px-3 py-2">
                     <dt className="text-[11px] uppercase tracking-wide text-fg-muted">Overall</dt>
                     <dd className="mt-1 font-medium text-fg">{preview.data?.proposal?.amendment ? "Amendment detected" : "New document"}</dd>
                   </div>
                 </dl>
  
                 <motion.div 
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.3 }}
                    className="mt-6 flex flex-wrap items-center justify-between gap-4 rounded-md border border-ok/30 bg-ok/5 p-4"
                  >
                   <div className="flex items-center gap-3">
                     <div className="flex size-8 items-center justify-center rounded-full bg-ok/20 text-ok">
                       <Check className="size-4" strokeWidth={3} />
                     </div>
                     <div>
                       <h4 className="text-sm font-medium text-fg">Intake Completed</h4>
                       <p className="mt-0.5 text-xs text-fg-muted">Ready for review. This document has been fully parsed and is awaiting human verification.</p>
                     </div>
                   </div>
                   <Link href="/review" className="shrink-0 rounded-md border border-ok/50 bg-ok/15 px-4 py-2 text-sm font-medium text-ok hover:bg-ok/25 transition-colors">
                     Review Extracted Obligations
                   </Link>
                 </motion.div>
              </motion.div>
            )}
          </AnimatePresence>
        </section>
      )}

      {/* Run history */}
      <section className="mt-5 rounded-lg border border-line-subtle bg-raised p-5">
        <h2 className="font-display text-lg tracking-tight">Recent Runs</h2>
        <ul className="mt-4 divide-y divide-line-subtle border-t border-line-subtle">
          {(runs.data?.runs ?? []).filter(r => r.state !== "failed").map((run) => (
            <li key={run.id} className="flex flex-wrap items-center gap-3 py-3 text-sm">
              <button 
                type="button" 
                onClick={() => setIngestId(run.id)}
                className="font-medium text-fg hover:underline cursor-pointer"
              >
                {run.filename}
              </button>
              {run.doc_kind && (
                <span className="rounded-full bg-elevated px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-fg-muted border border-line-subtle">
                  {run.doc_kind.replace(/_/g, " ")}
                </span>
              )}
              <span className={cn(
                "ml-auto rounded-full border px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide",
                run.state === "approved" ? "bg-ok/10 border-ok/20 text-ok" :
                run.state === "preview" ? "bg-warn/10 border-warn/20 text-warn" :
                run.state === "failed" ? "bg-risk/10 border-risk/20 text-risk" :
                "bg-elevated border-line-strong text-fg-muted"
              )}>
                {run.state === "preview" ? "Pending Review" : run.state}
              </span>
            </li>
          ))}
          {(runs.data?.count ?? 0) === 0 && (
            <li className="py-3 text-sm text-fg-muted">Nothing has been uploaded yet.</li>
          )}
        </ul>
      </section>
    </div>
  )
}
