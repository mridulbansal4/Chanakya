"use client"

import * as React from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import Link from "next/link"
import { ShieldAlert, ShieldCheck, CheckCircle2, ArrowRight, ChevronDown, ChevronUp, FileText, Check, X, Sparkles, Loader2 } from "lucide-react"

import { useCompletion } from "@ai-sdk/react"

import { useAsOf } from "@/components/as-of-provider"
import { DeonticBadge, StatusBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { SignoffModal } from "@/components/signoff-modal"
import { Button } from "@workspace/ui/components/button"
import { CardSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import { listIngestRuns, getIngestPreview, approveIngest, getReviewQueue, type IngestRunSummary, type ProposedObligation, type Obligation } from "@/lib/api"
import { durationDays } from "@/lib/format"
import { cn } from "@workspace/ui/lib/utils"

const ObligationItem = React.memo(({
  o,
  isApproved,
  isRejected,
  onToggle
}: {
  o: any
  isApproved: boolean
  isRejected: boolean
  onToggle: (id: string, status: "approve" | "reject") => void
}) => {
  const id = o.obligation.ID
  const [isExpanded, setIsExpanded] = React.useState(false)
  const [explanation, setExplanation] = React.useState<string | null>(null)
  const { completion, complete, isLoading } = useCompletion({
    api: "/api/ai/stream",
    streamProtocol: "text",
    onFinish: (prompt, completion) => {
      setExplanation(completion)
    },
    onError: (err) => {
      setExplanation("Failed to generate explanation: " + err.message)
    }
  })

  const handleExpand = async () => {
    if (isExpanded) {
      setIsExpanded(false)
      return
    }
    
    setIsExpanded(true)
    if (!explanation && !isLoading) {
      complete(o.clause_text, {
        body: {
          system: "You are a helpful regulatory assistant. Summarize and explain this raw regulatory clause in very simple, plain English. Keep it strictly to 1 or 2 concise sentences so it's easy to understand at a glance."
        }
      })
    }
  }

  return (
    <li className={cn("rounded-lg border p-5 transition-colors", isRejected ? "bg-raised border-line opacity-60" : "bg-surface border-primary/20")}>
      <div className="flex items-start gap-4">
        <div className="flex-1 space-y-3 min-w-0">
          <div className="flex items-center gap-2.5">
            <span className="tnum font-bold text-primary bg-primary/10 px-2 py-0.5 rounded text-[10px] shrink-0">
              {o.clause_ref}
            </span>
            <DeonticBadge deontic={o.obligation.DeonticType as any} />
            <ConfidenceMeter value={o.obligation.Confidence} />
          </div>
          
          <p className="text-sm font-medium text-foreground leading-relaxed break-words">
            {o.obligation.Condition}
          </p>
          
          <div 
            className="cursor-pointer group"
            onClick={handleExpand}
          >
            <blockquote className={cn("border-l-2 border-line pl-3 text-xs italic text-text-dim break-words transition-all duration-300", !isExpanded && "line-clamp-3")}>
              "{o.clause_text}"
            </blockquote>
            {!isExpanded && (
              <div className="mt-1 text-[10px] font-medium text-accent opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1">
                <Sparkles className="size-3" /> Click to expand and explain
              </div>
            )}
            {isExpanded && (
              <div className="mt-3 rounded-md bg-accent/5 border border-accent/20 p-3 text-xs text-foreground cursor-auto" onClick={(e) => e.stopPropagation()}>
                <div className="flex items-center gap-1.5 font-medium text-accent mb-1.5">
                  <Sparkles className="size-3.5" /> AI Summary
                </div>
                {isLoading && !completion ? (
                  <div className="flex items-center gap-2 text-text-dim">
                    <Loader2 className="size-3 animate-spin" /> Generating simple explanation...
                  </div>
                ) : (
                  <p className="leading-relaxed whitespace-pre-wrap">{completion || explanation}</p>
                )}
              </div>
            )}
          </div>
        </div>

        <div className="flex shrink-0 flex-col gap-2">
          <button 
            className={cn(
              "flex items-center gap-2 rounded-md border px-3 py-1.5 text-sm font-medium transition-colors w-24",
              isApproved 
                ? "bg-ok/10 border-ok/50 text-ok" 
                : "bg-transparent border-line text-fg-muted hover:bg-raised hover:text-fg"
            )}
            onClick={() => onToggle(id, "approve")}
          >
            <Check className="size-4" /> Approve
          </button>
          <button 
            className={cn(
              "flex items-center gap-2 rounded-md border px-3 py-1.5 text-sm font-medium transition-colors w-24",
              isRejected 
                ? "bg-risk/10 border-risk/50 text-risk" 
                : "bg-transparent border-line text-fg-muted hover:bg-raised hover:text-fg"
            )}
            onClick={() => onToggle(id, "reject")}
          >
            <X className="size-4" /> Reject
          </button>
        </div>
      </div>
    </li>
  )
})
ObligationItem.displayName = "ObligationItem"

function ChangePackage({ run }: { run: IngestRunSummary }) {
  const queryClient = useQueryClient()
  const [expanded, setExpanded] = React.useState(false)
  const [approvedIds, setApprovedIds] = React.useState<Set<string>>(new Set())
  const [rejectedIds, setRejectedIds] = React.useState<Set<string>>(new Set())
  const [publishing, setPublishing] = React.useState(false)
  const [visibleCount, setVisibleCount] = React.useState(20)

  const preview = useQuery({
    queryKey: ["ingest-preview", run.id],
    queryFn: ({ signal }) => getIngestPreview(run.id, signal),
    enabled: expanded,
  })

  // Auto-approve all by default when loaded
  React.useEffect(() => {
    if (preview.data?.proposal.obligations) {
      if (approvedIds.size === 0 && rejectedIds.size === 0) {
        setApprovedIds(new Set(preview.data.proposal.obligations.map((o) => o.obligation.ID)))
      }
    }
  }, [preview.data?.proposal.obligations])

  const obligations = preview.data?.proposal.obligations ?? []
  
  const toggleStatus = React.useCallback((id: string, status: "approve" | "reject") => {
    if (status === "approve") {
      setApprovedIds((prev) => {
        const next = new Set(prev)
        next.add(id)
        return next
      })
      setRejectedIds((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    } else {
      setRejectedIds((prev) => {
        const next = new Set(prev)
        next.add(id)
        return next
      })
      setApprovedIds((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    }
  }, [])

  const handlePublish = async () => {
    if (!preview.data?.proposal.obligations) return
    setPublishing(true)
    
    // Convert approved ProposedObligation to Obligation
    const approvedObligations = preview.data.proposal.obligations
      .filter((o) => approvedIds.has(o.obligation.ID))
      .map((o) => {
        return o.obligation as any
      })

    try {
      await approveIngest(run.id, "Priya Menon", "Reviewed and published via Regulatory Change Package flow", approvedObligations)
      await queryClient.invalidateQueries({ queryKey: ["ingest-runs"] })
    } catch (err) {
      console.error(err)
      alert("Failed to publish changes.")
    } finally {
      setPublishing(false)
    }
  }

  return (
    <div className="rounded-lg border border-line bg-surface shadow-elev-1 overflow-hidden transition-all duration-200">
      {/* Header / Summary */}
      <div 
        className={cn("p-6 cursor-pointer flex flex-wrap items-center justify-between gap-4 transition-colors hover:bg-raised/50", expanded && "bg-raised/50 border-b border-line")}
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-4">
          <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <FileText className="size-5" />
          </div>
          <div>
            <h3 className="font-display text-lg font-bold text-foreground">
              {run.filename}
            </h3>
            <div className="flex items-center gap-3 mt-1 text-xs text-text-dim">
              <span>{run.doc_kind}</span>
              <span>&bull;</span>
              <span className="tnum">ID: {run.circular_id}</span>
              <span>&bull;</span>
              <span>{new Date(run.created_at).toLocaleDateString()}</span>
            </div>
          </div>
        </div>
        
        <div className="flex items-center gap-4">
          <div className="text-right">
            <div className="text-sm font-medium text-foreground">Pending Review</div>
            <div className="text-xs text-text-dim">Click to expand package</div>
          </div>
          {expanded ? <ChevronUp className="size-5 text-text-dim" /> : <ChevronDown className="size-5 text-text-dim" />}
        </div>
      </div>

      {/* Expanded Content */}
      {expanded && (
        <div className="p-6 bg-surface">
          {preview.isLoading && (
            <div className="space-y-4">
              <CardSkeleton />
              <CardSkeleton />
            </div>
          )}
          
          {preview.data && (
            <div className="space-y-6">
              <div className="flex items-center justify-between">
                <div>
                  <h4 className="font-medium text-foreground">Extracted Obligations</h4>
                  <p className="text-sm text-text-dim">
                    {obligations.length} total drafts &bull; {approvedIds.size} ready to publish &bull; {rejectedIds.size} rejected
                  </p>
                </div>
                
                <Button onClick={handlePublish} disabled={publishing || approvedIds.size === 0}>
                  {publishing ? "Publishing..." : "Publish Regulatory Change"}
                </Button>
              </div>

              {obligations.length === 0 ? (
                <div className="py-8 text-center text-sm text-text-dim">No obligations were extracted from this document.</div>
              ) : (
                <div className="space-y-4">
                  <ul className="space-y-4">
                    {obligations.slice(0, visibleCount).map((o, index) => {
                      const id = o.obligation.ID
                      const isApproved = approvedIds.has(id)
                      const isRejected = rejectedIds.has(id)
                      return (
                        <ObligationItem
                          key={`${id}-${index}`}
                          o={o}
                          isApproved={isApproved}
                          isRejected={isRejected}
                          onToggle={toggleStatus}
                        />
                      )
                    })}
                  </ul>
                  {visibleCount < obligations.length && (
                    <div className="pt-2 flex justify-center">
                      <Button variant="secondary" onClick={() => setVisibleCount(v => v + 50)}>
                        Load More Drafts ({obligations.length - visibleCount} remaining)
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default function ReviewPage() {
  const { asOf } = useAsOf()
  const [selected, setSelected] = React.useState<Obligation | null>(null)

  const runsQuery = useQuery({
    queryKey: ["ingest-runs"],
    queryFn: ({ signal }) => listIngestRuns(signal),
  })

  const queue = useQuery({
    queryKey: ["review-queue", asOf],
    queryFn: ({ signal }) => getReviewQueue(asOf, signal),
  })

  const packages = React.useMemo(() => {
    return (runsQuery.data?.runs ?? []).filter((r) => r.state === "preview")
  }, [runsQuery.data])

  const items = React.useMemo(() => {
    const list = [...(queue.data?.obligations ?? [])]
    list.sort(
      (a, b) =>
        a.confidence - b.confidence ||
        durationDays(a.deadline) - durationDays(b.deadline),
    )
    return list
  }, [queue.data])

  return (
    <div className="mx-auto max-w-5xl px-6 py-8 space-y-6">
      {/* Header */}
      <div className="flex flex-wrap items-end justify-between gap-4 border-b border-line pb-6">
        <div>
          <div className="eyebrow mb-1">Compliance Officer Inbox</div>
          <h1 className="font-display text-3xl font-bold tracking-tight">Review Queue</h1>
          <p className="mt-1 text-sm text-text-dim max-w-2xl leading-relaxed">
            Review and publish Regulatory Change Packages, or sign-off on individual obligations extracted by CHANAKYA.
          </p>
        </div>
        <div className="tnum rounded-full bg-warn/15 border border-warn/30 px-3 py-1 text-xs font-bold text-warn">
          {packages.length + items.length} Awaiting Action
        </div>
      </div>

      {(runsQuery.isLoading || queue.isLoading) && (
        <div className="space-y-4">
          {Array.from({ length: 2 }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      )}

      {packages.length === 0 && items.length === 0 && !runsQuery.isLoading && !queue.isLoading && (
        <div className="mx-auto my-12 flex max-w-2xl flex-col items-center text-center rounded-2xl border border-ok/30 bg-ok/5 p-12 shadow-sm">
          <div className="flex size-16 items-center justify-center rounded-full bg-ok/20 text-ok mb-6">
            <Sparkles className="size-8" />
          </div>
          <h2 className="text-2xl font-display font-bold text-foreground mb-3">
            Inbox Zero - All Caught Up!
          </h2>
          <p className="text-base text-text-dim max-w-[46ch] mb-8">
            Every regulatory change package has been reviewed and published, and all obligations have been signed off.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-4 w-full">
            <Link href="/amendments" tabIndex={-1}>
              <Button size="lg" className="px-8">
                Analyze Impact (Blast Radius)
              </Button>
            </Link>
            <Link href="/workflows" tabIndex={-1}>
              <Button variant="outline" size="lg" className="px-8 border-line">
                Assign Workflows
              </Button>
            </Link>
          </div>
        </div>
      )}

      {packages.length > 0 && (
        <div className="space-y-4">
          <h2 className="font-display text-lg tracking-tight mt-6">Regulatory Change Packages</h2>
          {packages.map((pkg) => (
            <ChangePackage key={pkg.id} run={pkg} />
          ))}
        </div>
      )}

      {items.length > 0 && (
        <div className="space-y-4">
          <h2 className="font-display text-lg tracking-tight mt-8">Individual Obligations Awaiting Sign-off</h2>
          <ul className="space-y-4">
            {items.map((o) => (
              <li
                key={o.id}
                className="rounded-lg border border-line bg-surface p-6 shadow-elev-1 transition-all duration-200 hover:shadow-elev-1 hover:border-foreground/30 hover:-translate-y-0.5 space-y-4"
              >
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="flex flex-wrap items-center gap-2.5">
                      <span className="tnum font-bold text-primary bg-cream-200/80 px-2.5 py-1 rounded-lg text-xs">
                        Clause {o.clause_ref}
                      </span>
                      <DeonticBadge deontic={o.deontic_type} />
                      <StatusBadge status={o.status} />
                      <ConfidenceMeter value={o.confidence} />
                      {o.confidence < 0.8 && (
                        <span className="rounded-full bg-risk/10 border border-risk/30 px-2 py-0.5 text-[10px] font-bold text-risk uppercase">
                          Low Confidence Flag
                        </span>
                      )}
                    </div>

                    <h3 className="font-display text-lg font-bold text-foreground">
                      {o.clause_heading}
                    </h3>

                    <blockquote className="border-l-2 border-line pl-4 text-xs leading-relaxed text-text-dim italic bg-cream/30 py-2 pr-3 rounded-r-lg">
                      &quot;{o.source_sentence}&quot;
                    </blockquote>
                  </div>

                  <Button
                    variant="default"
                    size="default"
                    onClick={() => setSelected(o)}
                    className="shrink-0 shadow-elev-1"
                  >
                    <span>Review &amp; Sign</span>
                    <ArrowRight className="size-4" />
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}

      {selected && (
        <SignoffModal obligation={selected} onClose={() => setSelected(null)} />
      )}
    </div>
  )
}
