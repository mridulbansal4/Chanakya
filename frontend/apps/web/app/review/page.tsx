"use client"

import * as React from "react"
import { useQuery } from "@tanstack/react-query"

import { useAsOf } from "@/components/as-of-provider"
import { DeonticBadge, StatusBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { SignoffModal } from "@/components/signoff-modal"
import { durationDays } from "@/lib/format"
import { getReviewQueue, type Obligation } from "@/lib/api"

export default function ReviewPage() {
  const { asOf } = useAsOf()
  const [selected, setSelected] = React.useState<Obligation | null>(null)

  const queue = useQuery({
    queryKey: ["review-queue", asOf],
    queryFn: ({ signal }) => getReviewQueue(asOf, signal),
  })

  // Prioritise: lowest AI confidence first, then the nearest deadline — the
  // items most likely to be wrong and most time-sensitive rise to the top.
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
    <div className="mx-auto max-w-4xl px-6 py-6">
      <div className="mb-4 flex items-baseline justify-between">
        <div>
          <h1 className="font-display text-2xl">Review Queue — your inbox</h1>
          <p className="text-sm text-muted-foreground">
            These obligations need your judgement before CHANAKYA can act on
            them. Highest priority (least confident, nearest deadline) first.
          </p>
        </div>
        <span className="tnum text-sm text-muted-foreground">{items.length} awaiting</span>
      </div>

      {queue.isLoading && (
        <p className="hairline rounded-md bg-surface p-6 text-center text-sm text-muted-foreground">
          Loading your review queue…
        </p>
      )}

      {queue.isError && (
        <p className="hairline rounded-md bg-surface p-6 text-center text-sm text-danger">
          Couldn&apos;t reach the backend. Make sure the API is running on port
          8080, then refresh.
        </p>
      )}

      {items.length === 0 && !queue.isLoading && !queue.isError && (
        <p className="hairline rounded-md bg-surface p-6 text-center text-sm text-verified">
          All caught up — every in-force obligation has been reviewed and signed
          off.
        </p>
      )}

      <ul className="space-y-4">
        {items.map((o) => (
          <li key={o.id} className="hairline rounded-md bg-surface p-5">
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="tnum text-primary">{o.clause_ref}</span>
                  <DeonticBadge deontic={o.deontic_type} />
                  <StatusBadge status={o.status} />
                  <ConfidenceMeter value={o.confidence} />
                </div>
                <div className="mt-1.5 text-base font-medium">{o.clause_heading}</div>
                <blockquote className="mt-1.5 border-l-2 border-line pl-3 text-sm leading-relaxed text-muted-foreground">
                  {o.source_sentence}
                </blockquote>
              </div>
              <button
                onClick={() => setSelected(o)}
                className="hairline shrink-0 rounded-md bg-surface-2 px-4 py-2.5 text-base font-medium hover:bg-accent"
              >
                Review &amp; sign
              </button>
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
