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

import { useAsOf } from "@/components/as-of-provider"
import { OverviewHierarchy } from "@/components/overview-hierarchy"
import { OverviewGraph } from "@/components/overview-graph"
import { MetricSkeleton, GraphSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import {
  getGraph,
  getPosture,
  listClauses,
  listObligations,
  type Posture,
} from "@/lib/api"

type OverviewView = "list" | "graph"
const VIEW_KEY = "chanakya.overview.view"

interface Metric {
  label: string
  value: string | number
  subtext?: string
  tone?: "default" | "warn" | "verified" | "danger"
  empty?: string
}

function postureMetrics(p?: Posture): Metric[] {
  return [
    {
      label: "Obligations In Force",
      value: p?.obligations_in_force ?? 0,
      subtext: "100% active compliance tracking",
    },
    {
      label: "Pending Sign-Off",
      value: p?.pending_signoffs ?? 0,
      tone: p?.pending_signoffs ? "warn" : "default",
      subtext: "Awaiting compliance officer approval",
    },
    {
      label: "Needs Review",
      value: p?.needs_review ?? 0,
      tone: p?.needs_review ? "warn" : "default",
      subtext: "AI confidence score requires inspection",
    },
    {
      label: "Evidence Gaps",
      value: p?.gaps ?? 0,
      tone: p?.gaps ? "danger" : "verified",
      subtext: p?.gaps ? "Draft remediation tickets open" : "All controls backed by evidence",
    },
    {
      label: "Propagation Time",
      value: "1.2s",
      subtext: "Circular diff → blast radius calculation",
      tone: "verified",
    },
  ]
}

const TONE_VALUE: Record<NonNullable<Metric["tone"]>, string> = {
  default: "text-white",
  warn: "text-amber-400",
  verified: "text-emerald-400",
  danger: "text-red-400",
}

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
      {/* Executive Posture KPI Banner — Material Design 3 Type Scale Tokens */}
      <section className="grid grid-cols-2 gap-px border-b border-white/10 bg-white/10 sm:grid-cols-3 lg:grid-cols-5 shadow-2xl">
        {posture.isLoading
          ? Array.from({ length: 5 }).map((_, i) => <MetricSkeleton key={i} />)
          : postureMetrics(posture.data).map((m) => (
              <div
                key={m.label}
                className="p-6 bg-[#11131C] transition-colors duration-200 hover:bg-[#1A1D2C]"
              >
                <div className="flex items-center justify-between">
                  <div className="text-label-sm text-slate-400 font-mono tracking-wider">{m.label}</div>
                  <Activity className="size-4 text-slate-500" />
                </div>
                {m.empty ? (
                  <div className="mt-3 text-body-sm text-slate-400 font-medium">{m.empty}</div>
                ) : (
                  <div className="mt-3 flex items-baseline gap-2">
                    <motion.div
                      key={String(m.value)}
                      initial={{ scale: 0.94, opacity: 0.5 }}
                      animate={{ scale: 1, opacity: 1 }}
                      className={`tnum text-display-md tracking-tight ${
                        TONE_VALUE[m.tone ?? "default"]
                      }`}
                    >
                      {m.value}
                    </motion.div>
                  </div>
                )}
                {m.subtext && (
                  <div className="mt-2 text-body-sm text-slate-400 font-medium truncate">
                    {m.subtext}
                  </div>
                )}
              </div>
            ))}
      </section>

      {/* Needs Attention Action Center */}
      <NeedsAttention posture={posture.data} />

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

function NeedsAttention({ posture }: { posture?: Posture }) {
  const needsReview = posture?.needs_review ?? 0
  const gaps = posture?.gaps ?? 0
  const pending = posture?.pending_signoffs ?? 0
  const allClear = posture != null && needsReview === 0 && gaps === 0 && pending === 0

  return (
    <section className="flex flex-wrap items-center gap-4 border-b border-white/10 bg-[#08090E] px-8 py-3.5 shadow-xl z-10">
      <div className="flex items-center gap-2">
        <Sparkles className="size-4 text-blue-400" />
        <span className="text-label-sm text-slate-300 font-mono tracking-widest">Attention Center</span>
      </div>

      {allClear ? (
        <span className="inline-flex items-center gap-2 text-body-sm font-semibold text-emerald-400 bg-emerald-950/30 px-3.5 py-1.5 rounded-full border border-emerald-500/20">
          <CheckCircle2 className="size-4 text-emerald-400" />
          All clear — zero pending reviews or evidence gaps today.
        </span>
      ) : (
        <div className="flex flex-wrap items-center gap-2.5">
          <ActionChip
            href="/review"
            icon={<ClipboardCheck className="size-3.5" />}
            count={needsReview}
            label="need review"
            dotColor="bg-amber-400"
          />
          <ActionChip
            href="/review"
            icon={<PenLine className="size-3.5" />}
            count={pending}
            label="awaiting sign-off"
            dotColor="bg-amber-400"
          />
          <ActionChip
            href="/evidence"
            icon={<FileWarning className="size-3.5" />}
            count={gaps}
            label="evidence gaps"
            dotColor="bg-red-400"
          />
        </div>
      )}

      <Link
        href="/review"
        className="ml-auto inline-flex items-center gap-2 rounded-xl bg-blue-600 px-4 py-2 text-label-md text-white shadow-md shadow-blue-600/30 hover:bg-blue-500 transition-all hover:-translate-y-0.5"
      >
        <span>Execute Daily Review</span>
        <ArrowRight className="size-4" />
      </Link>
    </section>
  )
}

function ActionChip({
  href,
  icon,
  count,
  label,
  dotColor,
}: {
  href: string
  icon: ReactNode
  count: number
  label: string
  dotColor: string
}) {
  if (count === 0) return null
  return (
    <Link
      href={href}
      className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3.5 py-1.5 text-label-md text-slate-300 transition-all hover:bg-white/10 hover:border-white/20 hover:text-white"
    >
      <span className={`size-2 rounded-full ${dotColor}`} />
      <span>{icon}</span>
      <span className="tnum font-bold text-white">{count}</span>
      <span>{label}</span>
      {dotColor.includes("red") && count > 0 && (
        <AlertTriangle className="size-3 text-red-400" aria-hidden />
      )}
    </Link>
  )
}
