"use client"

import * as React from "react"
import { motion, useReducedMotion } from "framer-motion"

import { DeonticBadge, StatusDot } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { staggerDelay, DUR_STANDARD, EASE_OUT } from "@/lib/motion"
import type { Clause, Obligation, ObligationStatus } from "@/lib/api"

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

/** Ordered by urgency, so the count that needs action is read first. */
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
  const reduce = useReducedMotion()
  const sections = React.useMemo(
    () => buildSections(obligations, clauses),
    [obligations, clauses],
  )

  return (
    <div className="p-6 pt-16">
      <div className="grid auto-rows-fr grid-cols-1 gap-4 xl:grid-cols-2">
        {sections.map((sec, idx) => (
          <motion.section
            key={sec.ref}
            initial={reduce ? false : { opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{
              duration: DUR_STANDARD,
              ease: EASE_OUT,
              // Capped so a long register does not make the last card wait.
              delay: staggerDelay(idx),
            }}
            className="surface-interactive flex min-h-0 flex-col overflow-hidden"
          >
            <header className="flex shrink-0 items-center justify-between gap-4 border-b border-line-subtle px-5 py-3.5">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className="tnum shrink-0 rounded border border-line bg-elevated px-1.5 py-0.5 font-mono text-[11px] text-fg-muted">
                  §{sec.ref}
                </span>
                <h3 className="truncate text-title-lg text-fg">{sec.heading}</h3>
              </div>
              <div className="flex shrink-0 items-center gap-3">
                {SUMMARY_ORDER.filter((s) => sec.counts[s]).map((s) => (
                  <span key={s} className="inline-flex items-center gap-1.5">
                    <StatusDot status={s} />
                    <span className="tnum text-label-lg text-fg">{sec.counts[s]}</span>
                  </span>
                ))}
                <span className="tnum text-label-md text-fg-subtle">
                  {sec.obligations.length} total
                </span>
              </div>
            </header>

            <ul className="min-h-0 flex-1 divide-y divide-line-subtle overflow-y-auto">
              {sec.obligations.map((o) => (
                <li
                  key={o.id}
                  className="flex items-center gap-3 px-5 py-2.5 transition-colors duration-[120ms] hover:bg-elevated"
                >
                  <StatusDot status={o.status} />
                  {/* Clause refs are monospaced and fixed-width so they form a
                      scannable column rather than a ragged left edge. */}
                  <span className="tnum w-11 shrink-0 font-mono text-[11px] text-fg-subtle">
                    {o.clause_ref}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-body-md text-fg">
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
