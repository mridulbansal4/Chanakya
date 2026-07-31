"use client"

import * as React from "react"
import { motion } from "framer-motion"

import { DeonticBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { STATUS_LABEL } from "@/lib/format"
import type { Clause, Obligation, ObligationStatus } from "@/lib/api"

const STATUS_DOT: Record<ObligationStatus, string> = {
  approved: "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.6)]",
  needs_review: "bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.6)]",
  pending: "bg-slate-500",
  rejected: "bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.6)]",
}

function StatusDot({ status }: { status: ObligationStatus }) {
  return (
    <span
      title={STATUS_LABEL[status]}
      className={`inline-block size-2.5 shrink-0 rounded-full ${STATUS_DOT[status]}`}
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
    <div className="h-full overflow-hidden p-6 pt-14 bg-background">
      <div className="grid h-full auto-rows-fr grid-cols-1 gap-5 md:grid-cols-2">
        {sections.map((sec, idx) => (
          <motion.section
            key={sec.ref}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, delay: idx * 0.05 }}
            className="card-3d flex min-h-0 flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-2xl hover:border-ring/40"
          >
            <header className="flex shrink-0 items-center justify-between border-b border-line bg-foreground/5 px-6 py-4">
              <div className="flex items-center gap-3 min-w-0">
                <span className="tnum rounded-md bg-foreground/10 px-2.5 py-0.5 text-xs font-bold text-foreground font-mono border border-line">
                  §{sec.ref}
                </span>
                <h3 className="truncate font-display text-base font-bold text-foreground">
                  {sec.heading}
                </h3>
              </div>
              <div className="flex shrink-0 items-center gap-3 text-xs text-muted-foreground font-mono">
                {SUMMARY_ORDER.filter((s) => sec.counts[s]).map((s) => (
                  <span key={s} className="inline-flex items-center gap-1.5 font-semibold">
                    <StatusDot status={s} />
                    <span className="tnum font-extrabold text-foreground">{sec.counts[s]}</span>
                  </span>
                ))}
                <span className="tnum text-muted-foreground font-medium">· {sec.obligations.length} total</span>
              </div>
            </header>

            <ul className="min-h-0 flex-1 overflow-y-auto divide-y divide-line">
              {sec.obligations.map((o) => (
                <li
                  key={o.id}
                  className="flex items-center gap-3.5 px-6 py-3.5 transition-colors hover:bg-surface-2"
                >
                  <StatusDot status={o.status} />
                  <span className="tnum font-bold text-xs text-primary font-mono w-10 shrink-0">
                    {o.clause_ref}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-xs font-semibold text-foreground">
                    {o.clause_heading}
                  </span>
                  <DeonticBadge deontic={o.deontic_type} />
                  <div className="hidden shrink-0 lg:block">
                    <ConfidenceMeter value={o.confidence} />
                  </div>
                </li>
              ))}
            </ul>
          </motion.section>
        ))}
      </div>
    </div>
  )
}
