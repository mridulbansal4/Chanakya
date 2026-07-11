"use client"

import * as React from "react"

import { DeonticBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { STATUS_LABEL } from "@/lib/format"
import type { Clause, Obligation, ObligationStatus } from "@/lib/api"

/**
 * OverviewHierarchy answers "what obligations exist and what is their status" as
 * four clause-section boxes laid out in a grid that fits in a single view — no
 * page scrolling. A box with many rows scrolls internally so all four stay
 * visible at once.
 */

// Status → editorial dot colour. Colour signals STATE only.
const STATUS_DOT: Record<ObligationStatus, string> = {
  approved: "bg-ok",
  needs_review: "bg-warn",
  pending: "bg-text-dim",
  rejected: "bg-risk",
}

function StatusDot({ status }: { status: ObligationStatus }) {
  return (
    <span
      title={STATUS_LABEL[status]}
      className={`inline-block size-2 shrink-0 rounded-full ${STATUS_DOT[status]}`}
      aria-hidden
    />
  )
}

interface Section {
  ref: string
  heading: string
  obligations: Obligation[]
  counts: Partial<Record<ObligationStatus, number>>
}

/** Top-level section number of a clause ref: "4.2" → "4", "1" → "1". */
function sectionOf(ref: string): string {
  return (ref.split(".")[0] ?? ref).trim()
}

function buildSections(obligations: Obligation[], clauses: Clause[]): Section[] {
  const headingByRef = new Map(clauses.map((c) => [c.clause_ref, c.heading]))
  const bySection = new Map<string, Obligation[]>()
  for (const o of obligations) {
    const s = sectionOf(o.clause_ref)
    const arr = bySection.get(s) ?? []
    arr.push(o)
    bySection.set(s, arr)
  }
  return [...bySection.entries()]
    .sort((a, b) => a[0].localeCompare(b[0], undefined, { numeric: true }))
    .map(([ref, obls]) => {
      const counts: Partial<Record<ObligationStatus, number>> = {}
      for (const o of obls) counts[o.status] = (counts[o.status] ?? 0) + 1
      const sorted = [...obls].sort((a, b) =>
        a.clause_ref.localeCompare(b.clause_ref, undefined, { numeric: true }),
      )
      return {
        ref,
        heading: headingByRef.get(ref) ?? `Section ${ref}`,
        obligations: sorted,
        counts,
      }
    })
}

// Summary chips read attention-first.
const SUMMARY_ORDER: ObligationStatus[] = [
  "needs_review",
  "pending",
  "approved",
  "rejected",
]

export function OverviewHierarchy({
  obligations,
  clauses,
}: {
  obligations: Obligation[]
  clauses: Clause[]
}) {
  const sections = React.useMemo(
    () => buildSections(obligations, clauses),
    [obligations, clauses],
  )

  return (
    <div className="h-full overflow-hidden p-3 pt-12">
      <div className="grid h-full auto-rows-fr grid-cols-1 gap-3 md:grid-cols-2">
        {sections.map((sec) => (
          <section
            key={sec.ref}
            className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-line bg-surface shadow-[var(--shadow-card)]"
          >
            <header className="flex shrink-0 items-center gap-2 border-b border-line px-4 py-2.5">
              <span className="tnum text-xs text-text-dim">§{sec.ref}</span>
              <span className="truncate font-display text-base leading-none">
                {sec.heading}
              </span>
              <span className="ml-auto flex shrink-0 items-center gap-2 text-[11px] text-text-dim">
                {SUMMARY_ORDER.filter((s) => sec.counts[s]).map((s) => (
                  <span key={s} className="inline-flex items-center gap-1">
                    <StatusDot status={s} />
                    <span className="tnum">{sec.counts[s]}</span>
                  </span>
                ))}
                <span className="tnum">· {sec.obligations.length}</span>
              </span>
            </header>

            <ul className="min-h-0 flex-1 overflow-y-auto">
              {sec.obligations.map((o) => (
                <li
                  key={o.id}
                  className="flex items-center gap-2.5 border-b border-line/60 px-4 py-2 last:border-b-0"
                >
                  <StatusDot status={o.status} />
                  <span className="tnum w-10 shrink-0 text-xs text-text-dim">
                    {o.clause_ref}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-sm text-foreground">
                    {o.clause_heading}
                  </span>
                  <DeonticBadge deontic={o.deontic_type} />
                  <div className="hidden shrink-0 lg:block">
                    <ConfidenceMeter value={o.confidence} />
                  </div>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  )
}
