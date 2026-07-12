"use client"

import * as React from "react"
import type { ReactNode } from "react"
import Link from "next/link"
import { useQuery } from "@tanstack/react-query"
import {
  AlertTriangle,
  ArrowRight,
  ClipboardCheck,
  FileWarning,
  ListTree,
  Network,
  PenLine,
} from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { OverviewHierarchy } from "@/components/overview-hierarchy"
import { OverviewGraph } from "@/components/overview-graph"
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
  tone?: "default" | "warn" | "verified" | "danger"
  /** Small-text empty state shown instead of a big figure (no data yet). */
  empty?: string
}

function postureMetrics(p?: Posture): Metric[] {
  return [
    { label: "Obligations in force", value: p?.obligations_in_force ?? "—" },
    { label: "Pending sign-off", value: p?.pending_signoffs ?? "—", tone: "warn" },
    { label: "Needs review", value: p?.needs_review ?? "—", tone: "warn" },
    { label: "Gaps", value: p?.gaps ?? "—", tone: p?.gaps ? "danger" : "default" },
    { label: "Median propagation", value: "", empty: "Not enough history yet" },
  ]
}

const TONE: Record<NonNullable<Metric["tone"]>, string> = {
  default: "text-foreground",
  warn: "text-warn",
  verified: "text-verified",
  danger: "text-danger",
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

  // List / Graph view, remembered for the session.
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
    <div className="flex h-full flex-col">
      {/* Thin posture strip */}
      <section className="grid grid-cols-2 gap-px border-b border-line bg-line sm:grid-cols-3 lg:grid-cols-5">
        {postureMetrics(posture.data).map((m) => (
          <div key={m.label} className="bg-surface px-6 py-4">
            <div className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
              {m.label}
            </div>
            {m.empty ? (
              <div className="mt-2 text-sm text-muted-foreground">{m.empty}</div>
            ) : (
              <div className={`tnum mt-1.5 text-4xl ${TONE[m.tone ?? "default"]}`}>
                {m.value}
              </div>
            )}
          </div>
        ))}
      </section>

      {/* Needs your attention — the compliance officer's next actions */}
      <NeedsAttention posture={posture.data} />

      {/* The obligation view — scannable List or auto-laid-out Graph, with a
          small floating toggle on the side (no separate banded section). */}
      <div className="relative min-h-0 flex-1 overflow-hidden">
        {hasObligations && (
          <div className="absolute top-2.5 right-3 z-20">
            <ViewToggle value={view} onChange={changeView} />
          </div>
        )}
        {obligations.isLoading && (
          <div className="grid h-full place-items-center text-sm text-text-dim">
            Loading your obligations…
          </div>
        )}
        {obligations.isError && (
          <div className="grid h-full place-items-center text-center text-sm text-risk">
            <div>
              Couldn&apos;t reach the backend.
              <br />
              <span className="text-xs text-text-dim">
                Make sure the API is running on port 8080, then refresh.
              </span>
            </div>
          </div>
        )}
        {obligations.data && !hasObligations && (
          <div className="grid h-full place-items-center text-center text-sm text-text-dim">
            <div>
              No obligations are in force as of this date.
              <br />
              <span className="text-xs">
                Use the date control (top right) to pick a date after the
                circular took effect.
              </span>
            </div>
          </div>
        )}
        {hasObligations && view === "list" && (
          <OverviewHierarchy
            obligations={obligations.data!.obligations}
            clauses={clauses.data?.clauses ?? []}
          />
        )}
        {hasObligations && view === "graph" && (
          <>
            {graph.isLoading && (
              <div className="grid h-full place-items-center text-sm text-text-dim">
                Laying out the graph…
              </div>
            )}
            {graph.isError && (
              <div className="grid h-full place-items-center text-sm text-risk">
                Couldn&apos;t load the graph — is the API running on :8080?
              </div>
            )}
            {graph.data && graph.data.nodes.length > 0 && (
              // top padding keeps the graph's search/legend clear of the toggle
              <div className="h-full pt-11">
                <OverviewGraph payload={graph.data} />
              </div>
            )}
          </>
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
    { v: "list", label: "List", icon: <ListTree className="size-4" /> },
    { v: "graph", label: "Graph", icon: <Network className="size-4" /> },
  ]
  return (
    <div className="inline-flex rounded-full border border-line bg-surface p-1 text-sm shadow-[var(--shadow-card)]">
      {opts.map((o) => (
        <button
          key={o.v}
          type="button"
          onClick={() => onChange(o.v)}
          aria-pressed={value === o.v}
          className={`inline-flex items-center gap-2 rounded-full px-4 py-1.5 font-semibold transition-colors ${
            value === o.v
              ? "bg-ink text-on-ink"
              : "text-text-dim hover:text-foreground"
          }`}
        >
          {o.icon}
          {o.label}
        </button>
      ))}
    </div>
  )
}

function NeedsAttention({ posture }: { posture?: Posture }) {
  const needsReview = posture?.needs_review ?? 0
  const gaps = posture?.gaps ?? 0
  const pending = posture?.pending_signoffs ?? 0
  const allClear = posture != null && needsReview === 0 && gaps === 0 && pending === 0

  return (
    <section className="flex flex-wrap items-center gap-3 border-b border-line px-7 py-4">
      <span className="eyebrow">Needs your attention</span>

      {allClear ? (
        <span className="text-sm text-verified">All clear — nothing needs action today.</span>
      ) : (
        <>
          <ActionChip
            href="/review"
            icon={<ClipboardCheck className="size-3.5" />}
            count={needsReview}
            label="need review"
            tone="warn"
          />
          <ActionChip
            href="/review"
            icon={<PenLine className="size-3.5" />}
            count={pending}
            label="awaiting sign-off"
            tone="warn"
          />
          <ActionChip
            href="/evidence"
            icon={<FileWarning className="size-3.5" />}
            count={gaps}
            label="evidence gaps"
            tone="danger"
          />
        </>
      )}

      <Link
        href="/review"
        className="hairline ml-auto inline-flex items-center gap-2 rounded-md bg-primary px-5 py-2.5 text-base font-semibold text-primary-foreground"
      >
        Start review <ArrowRight className="size-4" />
      </Link>
    </section>
  )
}

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
  tone: "warn" | "danger"
}) {
  if (count === 0) return null
  const color = tone === "danger" ? "text-danger" : "text-warn"
  return (
    <Link
      href={href}
      className="hairline inline-flex items-center gap-2 rounded-md bg-surface px-3.5 py-2 text-sm hover:bg-surface-2"
    >
      <span className={color}>{icon}</span>
      <span className="tnum text-foreground">{count}</span>
      <span className="text-muted-foreground">{label}</span>
      {tone === "danger" && count > 0 && (
        <AlertTriangle className="size-3 text-danger" aria-hidden />
      )}
    </Link>
  )
}
