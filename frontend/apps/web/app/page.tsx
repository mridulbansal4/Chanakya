"use client"

import * as React from "react"
import type { ReactNode } from "react"
import Link from "next/link"
import { useQuery } from "@tanstack/react-query"
import { motion } from "framer-motion"
import {
  ArrowRight,
  CheckCircle2,
  ClipboardCheck,
  FileWarning,
  ListTree,
  Network,
  PenLine,
} from "lucide-react"

import { ExecutiveKpiHeader } from "@/components/executive-kpi-header"
import { useAsOf } from "@/components/as-of-provider"
import { OverviewHierarchy } from "@/components/overview-hierarchy"
import { OverviewGraph } from "@/components/overview-graph"
import { GraphSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import { SPRING_LAYOUT } from "@/lib/motion"
import { cn } from "@workspace/ui/lib/utils"
import {
  getGraph,
  getPosture,
  listClauses,
  listObligations,
  type Posture,
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

  const [view, setView] = React.useState<OverviewView>("graph")
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
    <div className="flex h-full flex-col bg-canvas">
      <ExecutiveKpiHeader posture={posture.data} isLoading={posture.isLoading} />

      <NeedsAttention posture={posture.data} />

      <div className="relative min-h-0 flex-1 overflow-hidden bg-sunken">
        {hasObligations && (
          <div className="absolute left-6 top-4 z-20">
            <ViewToggle value={view} onChange={changeView} />
          </div>
        )}

        {obligations.isLoading && <GraphSkeleton />}

        {obligations.isError && (
          <EmptyState
            icon="alert"
            title="Cannot reach the CHANAKYA API"
            description="The backend on port 8080 did not respond. Start the service, then retry."
            primaryAction={{
              label: "Retry",
              onClick: () => obligations.refetch(),
            }}
          />
        )}

        {obligations.data && !hasObligations && (
          <EmptyState
            icon="inbox"
            title="No obligations in force"
            description={`Nothing is active as of ${asOf}. Choose a date after 2024-05-15 to see active compliance items.`}
          />
        )}

        {hasObligations && view === "list" && (
          <div className="h-full overflow-y-auto">
            <OverviewHierarchy
              obligations={obligations.data!.obligations}
              clauses={clauses.data?.clauses ?? []}
            />
          </div>
        )}

        {hasObligations && view === "graph" && (
          <div className="h-full">
            {graph.isLoading && <GraphSkeleton />}
            {graph.isError && (
              <EmptyState
                icon="alert"
                title="Could not build the obligation graph"
                description="The graph structure failed to compile. Retry, or switch back to the hierarchy view."
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

/**
 * Segmented control. The selected background is a single shared element
 * that slides between segments, so the movement itself communicates which
 * way you switched.
 */
function ViewToggle({
  value,
  onChange,
}: {
  value: OverviewView
  onChange: (v: OverviewView) => void
}) {
  const opts: Array<{ v: OverviewView; label: string; icon: ReactNode }> = [
    { v: "list", label: "Hierarchy", icon: <ListTree className="size-3.5" aria-hidden /> },
    { v: "graph", label: "Graph", icon: <Network className="size-3.5" aria-hidden /> },
  ]
  return (
    <div
      role="group"
      aria-label="Obligation view"
      className="inline-flex items-center gap-0.5 rounded-md border border-line-subtle bg-overlay p-1 shadow-elev-2"
    >
      {opts.map((o) => {
        const selected = value === o.v
        return (
          <button
            key={o.v}
            type="button"
            onClick={() => onChange(o.v)}
            aria-pressed={selected}
            className={cn(
              "relative inline-flex h-8 items-center gap-1.5 rounded px-3 text-label-lg",
              "transition-colors duration-[120ms] ease-[cubic-bezier(0.2,0.8,0.2,1)]",
              selected ? "text-fg" : "text-fg-subtle hover:text-fg",
            )}
          >
            {selected && (
              <motion.span
                layoutId="view-toggle"
                aria-hidden
                className="absolute inset-0 rounded bg-elevated"
                transition={SPRING_LAYOUT}
              />
            )}
            <span className="relative">{o.icon}</span>
            <span className="relative">{o.label}</span>
          </button>
        )
      })}
    </div>
  )
}

/**
 * The action bar. This is the one row on the screen that answers "what do I
 * do now", so it carries the only high-emphasis button in the layout.
 *
 * Chips render only when their count is non-zero — a row of greyed-out
 * zeroes is noise that pushes the real signal off to the side.
 */
function NeedsAttention({ posture }: { posture?: Posture }) {
  const needsReview = posture?.needs_review ?? 0
  const gaps = posture?.gaps ?? 0
  const pending = posture?.pending_signoffs ?? 0
  const allClear =
    posture != null && needsReview === 0 && gaps === 0 && pending === 0

  return (
    <section
      aria-label="Needs attention"
      className="flex shrink-0 flex-wrap items-center gap-3 border-b border-line-subtle bg-canvas px-6 py-3"
    >
      <p className="eyebrow shrink-0">Needs attention</p>

      {allClear ? (
        <p className="inline-flex items-center gap-2 text-body-sm text-ok">
          <CheckCircle2 className="size-4" aria-hidden />
          Nothing pending — no reviews, sign-offs or evidence gaps today.
        </p>
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          <ActionChip
            href="/review"
            icon={<ClipboardCheck className="size-3.5" aria-hidden />}
            count={needsReview}
            label="need review"
            tone="warn"
          />
          <ActionChip
            href="/review"
            icon={<PenLine className="size-3.5" aria-hidden />}
            count={pending}
            label="awaiting sign-off"
            tone="warn"
          />
          <ActionChip
            href="/evidence"
            icon={<FileWarning className="size-3.5" aria-hidden />}
            count={gaps}
            label="evidence gaps"
            tone="risk"
          />
        </div>
      )}

      <Link
        href="/review"
        className="ml-auto inline-flex h-9 shrink-0 items-center gap-2 rounded-md bg-accent-solid px-4 text-label-lg text-accent-on shadow-elev-1 transition-colors duration-[120ms] hover:bg-accent-hover"
      >
        Start daily review
        <ArrowRight className="size-4" aria-hidden />
      </Link>
    </section>
  )
}

const CHIP_TONE = {
  warn: "border-warn-line bg-warn-weak text-warn hover:border-warn",
  risk: "border-risk-line bg-risk-weak text-risk hover:border-risk",
} as const

function ActionChip({
  href,
  icon,
  count,
  label,
  tone,
}: {
  href: string
  icon: ReactNode
  count: number
  label: string
  tone: keyof typeof CHIP_TONE
}) {
  if (count === 0) return null
  return (
    <Link
      href={href}
      className={cn(
        "inline-flex h-8 items-center gap-2 rounded-md border px-3 text-label-lg",
        "transition-colors duration-[120ms] ease-[cubic-bezier(0.2,0.8,0.2,1)]",
        CHIP_TONE[tone],
      )}
    >
      {icon}
      <span className="tnum font-semibold">{count}</span>
      <span className="text-fg-muted">{label}</span>
    </Link>
  )
}
