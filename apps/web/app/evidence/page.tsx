"use client"

import type { ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { FileWarning, Lock } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { DeonticBadge } from "@/components/badges"
import { PageHeader } from "@/components/page-header"
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
    <div className="flex h-full">
      {/* Left: evidence mapping */}
      <div className="flex min-w-0 flex-1 flex-col">
        <PageHeader
          eyebrow="Evidence & Gaps"
          title="Evidence coverage"
          description="Which obligations are backed by evidence from your firm's read-only systems, and where the gaps are. Each gap becomes a draft remediation ticket."
        />
        {/* Summary strip */}
        <section className="grid grid-cols-3 gap-px border-b border-line bg-line">
          <Stat label="Satisfied" value={em?.satisfied ?? "—"} tone="verified" />
          <Stat label="Gaps" value={em?.gaps ?? "—"} tone={em?.gaps ? "danger" : "default"} />
          <Stat label="Read-only sources" value={em?.sources.length ?? "—"} />
        </section>

        <div className="min-h-0 flex-1 overflow-auto">
          {evidence.isError ? (
            <div className="p-6 text-sm text-danger">
              Couldn&apos;t reach the backend. Make sure the API is running on
              port 8080, then refresh.
            </div>
          ) : evidence.isLoading ? (
            <div className="p-6 text-sm text-muted-foreground">
              Checking your evidence coverage…
            </div>
          ) : (
            <table className="w-full border-collapse text-sm">
              <thead className="sticky top-0 bg-surface">
                <tr className="border-b border-line text-[11px] tracking-wide text-muted-foreground uppercase">
                  <Th>Clause</Th>
                  <Th>Obligation type</Th>
                  <Th>Control</Th>
                  <Th>Evidence (read-only source)</Th>
                  <Th>Status</Th>
                </tr>
              </thead>
              <tbody>
                {(em?.obligations ?? []).map((o) => (
                  <EvidenceRow key={o.id} o={o} />
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Read-only sources footer */}
        {em && (
          <div className="flex flex-wrap items-center gap-2 border-t border-line px-6 py-2.5 text-xs">
            <span className="inline-flex items-center gap-1 text-muted-foreground">
              <Lock className="size-3" /> Connectors are read-only:
            </span>
            {em.sources.map((s) => (
              <span
                key={s.id}
                className="hairline tnum rounded bg-surface px-2 py-0.5 text-muted-foreground"
              >
                {s.source_system}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Right: draft tickets */}
      <aside className="flex w-[380px] shrink-0 flex-col border-l border-line bg-surface">
        <header className="flex items-center justify-between border-b border-line px-4 py-3">
          <span className="inline-flex items-center gap-2 text-xs tracking-wide text-muted-foreground uppercase">
            <FileWarning className="size-3.5 text-warn" />
            Draft remediation tickets
          </span>
          <span className="tnum text-xs text-muted-foreground">
            {tickets.data?.count ?? 0}
          </span>
        </header>
        <div className="min-h-0 flex-1 overflow-auto p-4">
          <p className="mb-3 text-[11px] text-muted-foreground">
            Drafted for each gap. CHANAKYA never files these into a firm system.
          </p>
          <ul className="space-y-2">
            {(tickets.data?.tickets ?? []).map((t) => (
              <li key={t.id} className="hairline rounded-md bg-background p-3 text-xs">
                <div className="flex items-center justify-between gap-2">
                  <span className="tnum text-primary">{t.clause_ref}</span>
                  <span className="rounded border border-warn/40 px-1.5 py-0.5 text-[10px] font-medium text-warn uppercase">
                    {t.state}
                  </span>
                </div>
                <p className="mt-1 font-medium text-foreground">{t.title}</p>
                <dl className="mt-1.5 space-y-0.5 text-muted-foreground">
                  <div className="flex gap-2">
                    <dt>Owner</dt>
                    <dd className="text-foreground">{t.owner}</dd>
                  </div>
                  {t.deadline && (
                    <div className="flex gap-2">
                      <dt>Deadline</dt>
                      <dd className="text-foreground" title={t.deadline}>
                        {formatDeadline(t.deadline)}
                      </dd>
                    </div>
                  )}
                </dl>
                <blockquote className="mt-2 border-l-2 border-line pl-2 text-[11px] leading-snug text-muted-foreground">
                  {t.citation}
                </blockquote>
              </li>
            ))}
          </ul>
        </div>
      </aside>
    </div>
  )
}

function EvidenceRow({ o }: { o: ObligationEvidence }) {
  return (
    <tr
      className={`border-b border-line/60 transition-colors ${
        o.satisfied ? "odd:bg-surface/40 hover:bg-surface" : "bg-danger/5"
      }`}
    >
      <td className="px-6 py-2.5 align-top">
        <span className="tnum text-primary">{o.clause_ref}</span>
        <div className="text-xs text-muted-foreground">{o.clause_heading}</div>
      </td>
      <td className="px-6 py-2.5 align-top">
        <DeonticBadge deontic={o.deontic_type} />
      </td>
      <td className="px-6 py-2.5 align-top text-muted-foreground">
        {o.controls.length ? (
          o.controls.join(", ")
        ) : (
          <span className="text-risk">No control mapped</span>
        )}
      </td>
      <td className="px-6 py-2.5 align-top">
        {o.evidence.length ? (
          <div className="space-y-0.5">
            {o.evidence.map((e) => (
              <div key={e.id} className="text-foreground">
                {e.name}{" "}
                <span className="tnum text-xs text-muted-foreground">({e.source_system})</span>
              </div>
            ))}
          </div>
        ) : (
          <span className="text-risk">No evidence yet</span>
        )}
      </td>
      <td className="px-6 py-2.5 align-top">
        {o.satisfied ? (
          <span className="rounded border border-verified/40 px-1.5 py-0.5 text-[10px] font-medium text-verified">
            Satisfied
          </span>
        ) : (
          <span
            className="rounded border border-danger/40 px-1.5 py-0.5 text-[10px] font-medium text-danger"
            title={o.gap_reason}
          >
            Gap
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
    tone === "verified" ? "text-verified" : tone === "danger" ? "text-danger" : "text-foreground"
  return (
    <div className="bg-background px-5 py-3">
      <div className="text-[11px] tracking-wide text-muted-foreground uppercase">{label}</div>
      <div className={`tnum mt-1 text-2xl ${color}`}>{value}</div>
    </div>
  )
}

function Th({ children }: { children: ReactNode }) {
  return <th className="px-6 py-2 text-left font-medium">{children}</th>
}
