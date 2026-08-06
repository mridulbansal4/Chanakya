"use client"

import * as React from "react"
import { useQuery } from "@tanstack/react-query"

import { useAsOf } from "@/components/as-of-provider"
import { PageHeader } from "@/components/page-header"
import {
  getEnterpriseImpact,
  getEnterpriseSummary,
  listObligations,
  type EnterpriseGap,
} from "@/lib/api"

/** Gap kinds rendered with a risk tone rather than a warning tone. */
const SEVERE = new Set(["segregation"])

function GapCard({ gap }: { gap: EnterpriseGap }) {
  const severe = SEVERE.has(gap.kind)
  return (
    <div
      className={
        "rounded-md border px-3 py-2.5 " +
        (severe ? "border-risk/40 bg-risk/10" : "border-warn/40 bg-warn/10")
      }
    >
      <div className="flex items-baseline justify-between gap-3">
        <span className={"text-sm font-medium " + (severe ? "text-risk" : "text-warn")}>
          {gap.title}
        </span>
        <span className="tnum text-lg text-fg">{gap.count}</span>
      </div>
      <p className="mt-1 text-xs text-fg-muted">{gap.detail}</p>
      {gap.names && gap.names.length > 0 && (
        <p className="mt-1 text-xs text-fg">{gap.names.join(", ")}</p>
      )}
    </div>
  )
}

export default function EnterprisePage() {
  const { asOf } = useAsOf()
  const [obligationId, setObligationId] = React.useState<string>("")

  const summary = useQuery({
    queryKey: ["enterprise-summary", asOf],
    queryFn: ({ signal }) => getEnterpriseSummary(asOf, signal),
  })

  const obligations = useQuery({
    queryKey: ["obligations", asOf],
    queryFn: ({ signal }) => listObligations({ asOf }, signal),
  })

  const impact = useQuery({
    queryKey: ["enterprise-impact", obligationId, asOf],
    queryFn: ({ signal }) => getEnterpriseImpact(obligationId, asOf, signal),
    enabled: obligationId !== "",
  })

  const counts = summary.data?.counts ?? {}

  return (
    <div className="mx-auto w-full max-w-6xl px-6 py-6">
      <PageHeader
        eyebrow="The firm"
        title={summary.data?.firm.name ?? "Enterprise"}
        description="Your firm as queryable data: departments, people, clients, agreements, documents and systems - reconstructed as of the date in the header. Every gap below was found by traversing this graph, not written into it."
      />

      {/* Posture */}
      <section className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-6">
        {[
          ["Departments", counts.departments],
          ["Employees", counts.employees],
          ["Clients", counts.clients],
          ["Agreements", counts.agreements],
          ["Documents", counts.documents],
          ["Systems", counts.systems],
        ].map(([label, value]) => (
          <div key={label as string} className="rounded-md border border-line-subtle bg-raised px-3 py-2">
            <div className="text-[11px] uppercase tracking-wide text-fg-muted">{label}</div>
            <div className="tnum text-2xl text-fg">{value ?? "-"}</div>
          </div>
        ))}
      </section>

      {/* Gaps discovered by query */}
      <section className="mt-5">
        <h2 className="font-display text-lg tracking-tight">Gaps found in the graph</h2>
        <p className="mt-1 text-sm text-fg-muted">
          None of these is recorded anywhere as a problem. Each is what a traversal of the
          firm&apos;s own data returns.
        </p>
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          {(summary.data?.gaps ?? []).map((gap, i) => (
            <GapCard key={`${gap.kind}-${gap.subject ?? i}`} gap={gap} />
          ))}
          {(summary.data?.gaps?.length ?? 0) === 0 && !summary.isLoading && (
            <p className="text-sm text-fg-muted">No gaps as of this date.</p>
          )}
        </div>
      </section>

      {/* Obligation -> firm projection */}
      <section className="mt-6 rounded-lg border border-line-subtle bg-raised p-5">
        <h2 className="font-display text-lg tracking-tight">What does an obligation touch?</h2>
        <p className="mt-1 text-sm text-fg-muted">
          Pick an obligation to project it onto the firm. Bindings are inference - each carries a
          confidence and is unconfirmed until a person confirms it.
        </p>
        <select
          value={obligationId}
          onChange={(e) => setObligationId(e.target.value)}
          className="mt-3 w-full max-w-2xl rounded-md border border-line-subtle bg-canvas px-3 py-2 text-sm text-fg"
        >
          <option value="">Select an obligation...</option>
          {(obligations.data?.obligations ?? []).map((o) => (
            <option key={o.id} value={o.id}>
              {o.clause_ref} - {o.clause_heading || o.source_sentence.slice(0, 70)}
            </option>
          ))}
        </select>

        {impact.data && (
          <div className="mt-4">
            <p className="text-sm text-fg">{impact.data.summary}</p>

            {impact.data.unbound ? (
              <p className="mt-3 rounded-md border border-warn/40 bg-warn/10 px-3 py-2 text-sm text-warn">
                Nothing in the firm currently addresses this obligation. An empty blast radius is a
                real answer, not a failed query.
              </p>
            ) : (
              <div className="mt-4 grid gap-4 lg:grid-cols-2">
                {/* Named owners, not counts */}
                {(impact.data.departments?.length ?? 0) > 0 && (
                  <div>
                    <h3 className="text-sm font-medium text-fg">Owners</h3>
                    <ul className="mt-2 space-y-1 text-sm">
                      {(impact.data.departments ?? []).map((d) => (
                        <li key={d.id} className="text-fg-muted">
                          <span className="text-fg">{d.head_name || d.name}</span>
                          {d.head_name && <span> ({d.name})</span>} - {d.reason}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                {(impact.data.controls?.length ?? 0) > 0 && (
                  <div>
                    <h3 className="text-sm font-medium text-fg">Controls</h3>
                    <ul className="mt-2 space-y-1 text-sm text-fg-muted">
                      {(impact.data.controls ?? []).map((c) => (
                        <li key={c.id}>
                          <span className="text-fg">{c.name}</span> ({c.kind})
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                {(impact.data.documents?.length ?? 0) > 0 && (
                  <div>
                    <h3 className="text-sm font-medium text-fg">Documents</h3>
                    <ul className="mt-2 space-y-1 text-sm text-fg-muted">
                      {(impact.data.documents ?? []).map((d) => (
                        <li key={d.id}>
                          <span className="text-fg">{d.title}</span> v{d.version}
                          {d.stale && <span className="ml-1 text-warn">(review overdue)</span>}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                {(impact.data.registers?.length ?? 0) > 0 && (
                  <div>
                    <h3 className="text-sm font-medium text-fg">Registers</h3>
                    <ul className="mt-2 space-y-1 text-sm text-fg-muted">
                      {(impact.data.registers ?? []).map((r) => (
                        <li key={r.id}>
                          <span className="text-fg">{r.kind}</span> - {r.row_count} rows, updated{" "}
                          {r.stale_days} days ago
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                {(impact.data.bindings?.length ?? 0) > 0 && (
                  <div className="lg:col-span-2">
                    <h3 className="text-sm font-medium text-fg">Bindings (inference)</h3>
                    <ul className="mt-2 space-y-1 text-xs">
                      {(impact.data.bindings ?? []).map((b) => (
                        <li key={`${b.target_type}-${b.target_id}`} className="flex flex-wrap gap-2 text-fg-muted">
                          <span className="tnum">{b.confidence.toFixed(2)}</span>
                          <span className="text-fg">{b.target_label || b.target_id}</span>
                          <span>({b.target_type})</span>
                          <span>- {b.rationale}</span>
                          {!b.human_confirmed && (
                            <span className="rounded bg-elevated px-1 text-[10px] uppercase tracking-wide">
                              unconfirmed
                            </span>
                          )}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            )}

            {/* Named clients - the point of the whole phase */}
            {(impact.data.clients?.length ?? 0) > 0 && (
              <div className="mt-5">
                <h3 className="text-sm font-medium text-fg">
                  Impacted clients ({impact.data.clients?.length})
                </h3>
                <p className="text-xs text-fg-muted">
                  Named, not counted - and as-of aware: set the date before the re-papering and the
                  whole book comes back.
                </p>
                <div className="mt-2 max-h-72 overflow-y-auto rounded-md border border-line-subtle">
                  <table className="w-full text-left text-xs">
                    <thead className="sticky top-0 bg-elevated text-fg-muted">
                      <tr>
                        <th className="px-3 py-1.5 font-medium">Client</th>
                        <th className="px-3 py-1.5 font-medium">Segment</th>
                        <th className="px-3 py-1.5 font-medium">Adviser</th>
                        <th className="px-3 py-1.5 font-medium">Template</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(impact.data.clients ?? []).map((c) => (
                        <tr key={c.id} className="border-t border-line-subtle">
                          <td className="px-3 py-1.5 text-fg">{c.name}</td>
                          <td className="px-3 py-1.5 text-fg-muted">{c.segment}</td>
                          <td className="px-3 py-1.5 text-fg-muted">{c.adviser_name}</td>
                          <td className="px-3 py-1.5 tnum text-fg-muted">{c.template_version || "-"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>
        )}
      </section>

      {/* Org, systems, registers */}
      <section className="mt-6 grid gap-5 lg:grid-cols-2">
        <div className="rounded-lg border border-line-subtle bg-raised p-5">
          <h2 className="font-display text-lg tracking-tight">Departments</h2>
          <ul className="mt-3 divide-y divide-line-subtle text-sm">
            {(summary.data?.departments ?? []).map((d) => (
              <li key={d.id} className="flex items-baseline justify-between gap-3 py-1.5">
                <span className="text-fg">{d.name}</span>
                <span className="text-xs text-fg-muted">{d.head_name}</span>
                <span className="tnum text-xs text-fg-muted">{d.headcount}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className="rounded-lg border border-line-subtle bg-raised p-5">
          <h2 className="font-display text-lg tracking-tight">Systems</h2>
          <p className="mt-1 text-xs text-fg-muted">
            Each is fronted by a read-only connector. CHANAKYA never writes to a firm system.
          </p>
          <ul className="mt-3 divide-y divide-line-subtle text-sm">
            {(summary.data?.systems ?? []).map((s) => (
              <li key={s.id} className="flex items-baseline justify-between gap-3 py-1.5">
                <span className="text-fg">{s.vendor}</span>
                <span className="text-xs text-fg-muted">{s.kind}</span>
                <span className="text-xs text-fg-muted">{s.criticality}</span>
              </li>
            ))}
          </ul>
        </div>
      </section>
    </div>
  )
}
