"use client"

import { useQuery } from "@tanstack/react-query"
import { ShieldCheck, Network, HelpCircle } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { LineageGraph } from "@/components/lineage-graph"
import { GraphSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import { getLineage } from "@/lib/api"

const ORDER = ["clause", "obligation", "control", "evidence", "signoff", "policy"] as const

const COLUMN: Record<string, string> = {
  clause: "Source Clause",
  obligation: "Extracted Obligation",
  control: "Mapped Control",
  evidence: "System Evidence",
  signoff: "Officer Sign-Off",
  policy: "Enforceable Policy",
}

export default function AuditPage() {
  const { asOf } = useAsOf()
  const lineage = useQuery({
    queryKey: ["lineage", asOf],
    queryFn: ({ signal }) => getLineage(asOf, signal),
  })
  const counts = lineage.data?.counts ?? {}
  const total = Object.values(counts).reduce((a, b) => a + b, 0)

  return (
    <div className="flex h-full flex-col bg-background">
      {/* Title + caption header */}
      <div className="border-b border-line bg-surface px-7 py-5 shadow-2xs">
        <div className="eyebrow mb-1">Audit Trail &amp; Provenance</div>
        <h1 className="font-display text-2xl font-bold tracking-tight">
          Compliance Lineage Graph
        </h1>
        <p className="mt-1 max-w-3xl text-xs text-text-dim leading-relaxed">
          End-to-end audit reconstruction tracing every obligation from source circular clause through controls, evidence, human approval, and compiled policy enforcement as of{" "}
          <span className="tnum font-bold text-foreground font-mono bg-cream-200/80 px-2 py-0.5 rounded-md">
            {asOf}
          </span>.
        </p>
      </div>

      {lineage.data && total > 0 && ((counts.signoff ?? 0) === 0 || (counts.policy ?? 0) === 0) && (
        <div className="border-b border-line bg-warn/10 px-7 py-2.5 text-xs text-foreground font-medium flex items-center gap-2">
          <HelpCircle className="size-4 text-warn shrink-0" />
          <span>
            No sign-offs or policies exist yet as of this date. Approve obligations in the{" "}
            <a href="/review" className="font-bold text-foreground underline">
              Review Queue
            </a>{" "}
            to populate the full lineage trail.
          </span>
        </div>
      )}

      {/* Persistent column headers - Rounded Cards with Spacing */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-3 p-3.5 border-b border-line bg-surface/50">
        {ORDER.map((t) => (
          <div
            key={t}
            className="rounded-xl border border-line bg-surface p-3.5 text-center transition-all duration-200 hover:border-foreground/20 hover:bg-surface-2 shadow-sm"
          >
            <div className="eyebrow">{COLUMN[t]}</div>
            <div className="tnum font-display mt-1 text-base font-bold text-foreground">
              {counts[t] ?? 0}
            </div>
          </div>
        ))}
      </div>

      <div className="relative flex-1 bg-cream/40">
        {lineage.isLoading && (
          <div className="h-full p-8">
            <GraphSkeleton />
          </div>
        )}

        {lineage.isError && (
          <EmptyState
            icon="alert"
            title="Backend Unreachable"
            description="Could not load lineage audit trail from backend port 8080."
            primaryAction={{ label: "Retry", onClick: () => lineage.refetch() }}
          />
        )}

        {total === 0 && lineage.data && (
          <EmptyState
            icon="inbox"
            title="No Lineage Recorded"
            description={`No compliance lineage elements in force as of ${asOf}. Select a date after 2024-05-15.`}
          />
        )}

        {lineage.data && total > 0 && <LineageGraph lineage={lineage.data} />}
      </div>
    </div>
  )
}
