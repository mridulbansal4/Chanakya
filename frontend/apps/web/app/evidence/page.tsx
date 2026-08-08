"use client"

import type { ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { FileWarning, Lock, ShieldAlert, CheckCircle2, Ticket } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { DeonticBadge } from "@/components/badges"
import { PageHeader } from "@/components/page-header"
import { SkeletonRows, CardSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import { formatDeadline } from "@/lib/format"
import {
  getEvidenceMap,
  getTickets,
  type ObligationEvidence,
} from "@/lib/api"

export default function EvidencePage() {
  const { asOf } = useAsOf()

  const evidence = useQuery({
    queryKey: ["evidence", asOf],
    queryFn: ({ signal }) => getEvidenceMap(asOf, signal),
  })
  const tickets = useQuery({
    queryKey: ["tickets", asOf],
    queryFn: ({ signal }) => getTickets(asOf, signal),
  })

  const em = evidence.data

  return (
    <div className="flex flex-col lg:flex-row h-full bg-background">
      {/* Left: evidence mapping */}
      <div className="flex min-w-0 flex-1 flex-col border-r border-line">
        <PageHeader
          eyebrow="Evidence & Gaps"
          title="Evidence Coverage Matrix"
          description="Real-time map linking regulatory obligations to read-only enterprise data connectors. Evidence gaps automatically generate remediation tickets."
        />

        {/* Summary strip - Rounded Cards with Spacing */}
        <section className="grid grid-cols-1 sm:grid-cols-3 gap-3 p-3.5 border-b border-line bg-surface/50">
          <Stat label="Satisfied Obligations" value={em?.satisfied ?? "-"} tone="verified" />
          <Stat label="Remediation Gaps" value={em?.gaps ?? "-"} tone={em?.gaps ? "danger" : "default"} />
          <Stat label="Connected Systems" value={em?.sources.length ?? "-"} />
        </section>

        <div className="min-h-0 flex-1 overflow-auto">
          {evidence.isError ? (
            <EmptyState
              icon="alert"
              title="Backend Connection Error"
              description="Could not load evidence mapping. Please ensure backend is running on port 8080."
              primaryAction={{ label: "Retry", onClick: () => evidence.refetch() }}
            />
          ) : evidence.isLoading ? (
            <SkeletonRows rows={8} cols={5} />
          ) : (em?.obligations ?? []).length === 0 ? (
            <EmptyState
              icon="inbox"
              title="No Evidence Requirements"
              description={`No active obligations requiring evidence as of ${asOf}.`}
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-sm">
                <thead className="sticky top-0 bg-surface/95 backdrop-blur-md shadow-xs z-10">
                  <tr className="border-b border-line text-xs font-semibold tracking-wider text-muted-foreground uppercase">
                    <Th>Clause</Th>
                    <Th>Obligation Type</Th>
                    <Th>Control Enforceable</Th>
                    <Th>Read-Only System Source</Th>
                    <Th>Status</Th>
                  </tr>
                </thead>
                <tbody>
                  {(em?.obligations ?? []).map((o) => (
                    <EvidenceRow key={o.id} o={o} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Read-only sources footer */}
        {em && (
          <div className="flex flex-wrap items-center gap-2 border-t border-line bg-surface/80 px-6 py-3 text-xs">
            <span className="inline-flex items-center gap-1.5 font-semibold text-text-dim">
              <Lock className="size-3.5 text-ok" /> Enterprise Connectors (Read-Only):
            </span>
            {em.sources.map((s) => (
              <span
                key={s.id}
                className="tnum rounded-lg border border-line bg-background px-2.5 py-1 font-mono text-xs text-foreground shadow-2xs"
              >
                {s.source_system}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Right: draft tickets */}
      <aside className="flex w-full lg:w-[400px] shrink-0 flex-col bg-surface/60 border-t lg:border-t-0 lg:border-l border-line">
        <header className="flex items-center justify-between border-b border-line px-5 py-4 bg-surface">
          <span className="inline-flex items-center gap-2 text-xs font-bold tracking-wider text-foreground uppercase">
            <Ticket className="size-4 text-warn" />
            Draft Remediation Tickets
          </span>
          <span className="tnum rounded-full bg-warn/15 border border-warn/30 px-2.5 py-0.5 text-xs font-bold text-warn">
            {tickets.data?.count ?? 0}
          </span>
        </header>

        <div className="min-h-0 flex-1 overflow-auto p-5 space-y-3">
          <p className="text-xs text-text-dim leading-relaxed">
            Automatically generated for compliance gaps. CHANAKYA prepares these draft tickets without modifying external systems.
          </p>

          {tickets.isLoading ? (
            Array.from({ length: 3 }).map((_, i) => <CardSkeleton key={i} />)
          ) : (tickets.data?.tickets ?? []).length === 0 ? (
            <div className="p-8 text-center text-xs text-text-dim border border-dashed border-line rounded-lg">
              No remediation tickets required.
            </div>
          ) : (
            <ul className="space-y-3">
              {(tickets.data?.tickets ?? []).map((t) => (
                <li
                  key={t.id}
                  className="rounded-lg border border-line bg-surface p-4 text-xs shadow-xs transition-all hover:shadow-elev-1 hover:border-foreground/30 hover:-translate-y-0.5 space-y-2"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="tnum font-bold text-primary bg-cream-200/80 px-2 py-0.5 rounded-md">
                      Clause {t.clause_ref}
                    </span>
                    <span className="rounded-full border border-warn/40 bg-warn/10 px-2 py-0.5 text-[10px] font-bold text-warn uppercase">
                      {t.state}
                    </span>
                  </div>

                  <p className="font-semibold text-foreground text-sm leading-snug">{t.title}</p>

                  <dl className="space-y-1 text-text-dim">
                    <div className="flex justify-between">
                      <dt className="text-text-dim">Owner:</dt>
                      <dd className="font-medium text-foreground">{t.owner}</dd>
                    </div>
                    {t.deadline && (
                      <div className="flex justify-between">
                        <dt className="text-text-dim">Deadline:</dt>
                        <dd className="tnum font-medium text-foreground" title={t.deadline}>
                          {formatDeadline(t.deadline)}
                        </dd>
                      </div>
                    )}
                  </dl>

                  <blockquote className="border-l-2 border-line pl-2.5 text-[11px] leading-relaxed text-text-dim italic">
                    {t.citation}
                  </blockquote>
                </li>
              ))}
            </ul>
          )}
        </div>
      </aside>
    </div>
  )
}

function EvidenceRow({ o }: { o: ObligationEvidence }) {
  return (
    <tr
      className={`border-b border-line/60 transition-all duration-150 ${
        o.satisfied ? "odd:bg-surface/30 hover:bg-cream-200/40" : "bg-risk/5 hover:bg-risk/10"
      }`}
    >
      <td className="px-6 py-3.5 align-top">
        <span className="tnum font-bold text-primary bg-cream-200/80 px-2 py-0.5 rounded-md">
          {o.clause_ref}
        </span>
        <div className="mt-1 text-xs text-text-dim line-clamp-1 max-w-xs">{o.clause_heading}</div>
      </td>
      <td className="px-6 py-3.5 align-top">
        <DeonticBadge deontic={o.deontic_type} />
      </td>
      <td className="px-6 py-3.5 align-top text-text-dim font-medium">
        {o.controls.length ? (
          o.controls.join(", ")
        ) : (
          <span className="text-risk font-semibold text-xs">No control mapped</span>
        )}
      </td>
      <td className="px-6 py-3.5 align-top">
        {o.evidence.length ? (
          <div className="space-y-1">
            {o.evidence.map((e) => (
              <div key={e.id} className="text-foreground font-medium">
                {e.name}{" "}
                <span className="tnum text-xs text-text-dim">({e.source_system})</span>
              </div>
            ))}
          </div>
        ) : (
          <span className="text-risk font-semibold text-xs">No evidence source</span>
        )}
      </td>
      <td className="px-6 py-3.5 align-top">
        {o.satisfied ? (
          <span className="inline-flex items-center gap-1 rounded-full border border-ok/40 bg-ok/10 px-2.5 py-0.5 text-xs font-bold text-ok">
            <CheckCircle2 className="size-3" /> Satisfied
          </span>
        ) : (
          <span
            className="inline-flex items-center gap-1 rounded-full border border-risk/40 bg-risk/10 px-2.5 py-0.5 text-xs font-bold text-risk"
            title={o.gap_reason}
          >
            <ShieldAlert className="size-3" /> Gap
          </span>
        )}
      </td>
    </tr>
  )
}

function Stat({
  label,
  value,
  tone = "default",
}: {
  label: string
  value: string | number
  tone?: "default" | "verified" | "danger"
}) {
  const color =
    tone === "verified" ? "text-ok" : tone === "danger" ? "text-risk" : "text-foreground"
  return (
    <div className="rounded-xl border border-line bg-surface p-4.5 transition-all duration-200 hover:border-foreground/20 hover:bg-surface-2 shadow-sm">
      <div className="eyebrow">{label}</div>
      <div className={`tnum mt-1 text-metric-md ${color}`}>{value}</div>
    </div>
  )
}

function Th({ children }: { children: ReactNode }) {
  return <th className="px-6 py-3 text-left font-semibold">{children}</th>
}
