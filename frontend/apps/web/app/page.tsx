"use client"

import * as React from "react"
import type { ReactNode } from "react"
import Link from "next/link"
import { useQuery } from "@tanstack/react-query"
import { motion } from "framer-motion"
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  ClipboardCheck,
  FileWarning,
  ListTree,
  Network,
  PenLine,
  Sparkles,
  Activity,
} from "lucide-react"

import { ExecutiveKpiHeader } from "@/components/executive-kpi-header"
import { useAsOf } from "@/components/as-of-provider"
import { OverviewHierarchy } from "@/components/overview-hierarchy"
import { OverviewGraph } from "@/components/overview-graph"
import { GraphSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import {
  getGraph,
  getPosture,
  listClauses,
  listObligations,
} from "@/lib/api"

type OverviewView = "list" | "graph"
const VIEW_KEY = "chanakya.overview.view"

export default function OverviewPage() {
  const { asOf } = useAsOf()

  const posture = useQuery({
    queryKey: ["posture", asOf],
    queryFn: ({ signal }) => getPosture(asOf, signal),
  })
  const obligations = useQuery({
    queryKey: ["obligations", asOf, "", ""],
    queryFn: ({ signal }) => listObligations({ asOf }, signal),
  })
  const clauses = useQuery({
    queryKey: ["clauses", asOf],
    queryFn: ({ signal }) => listClauses(asOf, signal),
  })

  const [view, setView] = React.useState<OverviewView>("list")
  React.useEffect(() => {
    const v = window.sessionStorage.getItem(VIEW_KEY)
    if (v === "graph" || v === "list") setView(v)
  }, [])
  const changeView = (v: OverviewView) => {
    setView(v)
    window.sessionStorage.setItem(VIEW_KEY, v)
  }

  const graph = useQuery({
    queryKey: ["graph", asOf],
    queryFn: ({ signal }) => getGraph(asOf, signal),
    enabled: view === "graph",
  })

  const hasObligations =
    !!obligations.data && obligations.data.obligations.length > 0

  return (
    <div className="flex h-full flex-col bg-[#08090E] text-foreground">
      {/* Executive Mission Control Header Bar */}
      <ExecutiveKpiHeader posture={posture.data} isLoading={posture.isLoading} />

      {/* Primary Data Surface (List or React Flow Graph) */}
      <div className="relative min-h-0 flex-1 overflow-hidden bg-[#08090E]">
        {hasObligations && (
          <div className="absolute top-4 left-6 z-20 flex items-center rounded-full bg-[#11131C] p-1 border border-white/15 shadow-2xl backdrop-blur-xl text-xs font-medium">
            <ViewToggle value={view} onChange={changeView} />
          </div>
        )}

        {obligations.isLoading && (
          <div className="h-full p-8">
            <GraphSkeleton />
          </div>
        )}

        {obligations.isError && (
          <EmptyState
            icon="alert"
            title="Backend Offline"
            description="Could not connect to CHANAKYA API port 8080. Please start the backend service."
            primaryAction={{
              label: "Retry Connection",
              onClick: () => obligations.refetch(),
            }}
          />
        )}

        {obligations.data && !hasObligations && (
          <EmptyState
            icon="inbox"
            title="No Obligations In Force"
            description={`No regulatory obligations are active as of ${asOf}. Pick a date after 2024-05-15 to inspect active compliance items.`}
          />
        )}

        {hasObligations && view === "list" && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="h-full overflow-y-auto"
          >
            <OverviewHierarchy
              obligations={obligations.data!.obligations}
              clauses={clauses.data?.clauses ?? []}
            />
          </motion.div>
        )}

        {hasObligations && view === "graph" && (
          <div className="h-full">
            {graph.isLoading && <GraphSkeleton />}
            {graph.isError && (
              <EmptyState
                icon="alert"
                title="Graph Render Failure"
                description="Unable to compile obligation DAG graph structure."
                primaryAction={{ label: "Retry", onClick: () => graph.refetch() }}
              />
            )}
            {graph.data && graph.data.nodes.length > 0 && (
              <OverviewGraph payload={graph.data} />
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function ViewToggle({
  value,
  onChange,
}: {
  value: OverviewView
  onChange: (v: OverviewView) => void
}) {
  const opts: Array<{ v: OverviewView; label: string; icon: ReactNode }> = [
    { v: "list", label: "Hierarchy List", icon: <ListTree className="size-3.5" /> },
    { v: "graph", label: "DAG Graph", icon: <Network className="size-3.5" /> },
  ]
  return (
    <>
      {opts.map((o) => (
        <button
          key={o.v}
          type="button"
          onClick={() => onChange(o.v)}
          aria-pressed={value === o.v}
          className={`relative inline-flex items-center gap-1.5 rounded-full px-4 py-1.5 transition-all text-label-md ${
            value === o.v
              ? "bg-blue-600 text-white font-bold shadow-md shadow-blue-600/30"
              : "text-slate-300 hover:text-white hover:bg-white/10"
          }`}
        >
          {o.icon}
          <span>{o.label}</span>
        </button>
      ))}
    </>
  )
}


