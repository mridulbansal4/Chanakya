"use client"

import * as React from "react"
import { useQuery } from "@tanstack/react-query"
import { ShieldAlert, ShieldCheck, CheckCircle2, ArrowRight } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { DeonticBadge, StatusBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { SignoffModal } from "@/components/signoff-modal"
import { Button } from "@workspace/ui/components/button"
import { CardSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import { durationDays, formatDeadline } from "@/lib/format"
import { getReviewQueue, type Obligation } from "@/lib/api"

export default function ReviewPage() {
  const { asOf } = useAsOf()
  const [selected, setSelected] = React.useState<Obligation | null>(null)

  const queue = useQuery({
    queryKey: ["review-queue", asOf],
    queryFn: ({ signal }) => getReviewQueue(asOf, signal),
  })

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
            Obligations extracted by CHANAKYA requiring human review and cryptographic sign-off before automated policy enforcement. Priority sorted by AI confidence and deadline.
          </p>
        </div>
        <div className="tnum rounded-full bg-warn/15 border border-warn/30 px-3 py-1 text-xs font-bold text-warn">
          {items.length} Awaiting Action
        </div>
      </div>

      {queue.isLoading && (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      )}

      {queue.isError && (
        <EmptyState
          icon="alert"
          title="Review Queue Unavailable"
          description="Could not connect to the backend server. Make sure API is running on port 8080."
          primaryAction={{ label: "Retry", onClick: () => queue.refetch() }}
        />
      )}

      {items.length === 0 && !queue.isLoading && !queue.isError && (
        <EmptyState
          icon="sparkles"
          title="Inbox Zero — All Caught Up"
          description={`Every obligation in force as of ${asOf} has been reviewed, approved, and signed off.`}
        />
      )}

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

      {selected && (
        <SignoffModal obligation={selected} onClose={() => setSelected(null)} />
      )}
    </div>
  )
}
