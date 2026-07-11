"use client"

import { useQuery } from "@tanstack/react-query"

import { useAsOf } from "@/components/as-of-provider"
import { LineageGraph } from "@/components/lineage-graph"
import { getLineage } from "@/lib/api"

// The lineage columns, in flow order.
const ORDER = ["clause", "obligation", "control", "evidence", "signoff", "policy"] as const

// Singular column headers (the eyebrow style uppercases them).
const COLUMN: Record<string, string> = {
  clause: "Clause",
  obligation: "Obligation",
  control: "Control",
  evidence: "Evidence",
  signoff: "Sign-off",
  policy: "Policy",
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
    <div className="flex h-full flex-col">
      {/* Title + caption */}
      <div className="border-b border-line px-6 py-4">
        <div className="eyebrow mb-1">Audit</div>
        <h1 className="font-display text-2xl leading-tight tracking-tight">
          Compliance lineage
        </h1>
        <p className="mt-1 max-w-3xl text-sm text-text-dim">
          Every obligation traced from its source clause through control,
          evidence, human sign-off, and enforced policy — as of{" "}
          <span className="tnum text-foreground">{asOf}</span>.
        </p>
      </div>

      {lineage.data && total > 0 && ((counts.signoff ?? 0) === 0 || (counts.policy ?? 0) === 0) && (
        <p className="border-b border-line bg-surface px-6 py-2 text-xs text-text-dim">
          No sign-offs or policies yet as of this date — approve obligations in the{" "}
          <a href="/review" className="font-medium text-foreground underline">
            Review Queue
          </a>{" "}
          to populate the audit trail.
        </p>
      )}

      {/* Persistent column headers, aligned to the layered graph below. */}
      <div className="grid grid-cols-6 border-b border-line bg-surface">
        {ORDER.map((t) => (
          <div
            key={t}
            className="border-r border-line/60 px-4 py-2.5 text-center last:border-r-0"
          >
            <div className="eyebrow">{COLUMN[t]}</div>
            <div className="tnum mt-0.5 text-sm text-foreground">{counts[t] ?? 0}</div>
          </div>
        ))}
      </div>

      <div className="relative flex-1">
        {lineage.isError && (
          <div className="absolute inset-0 grid place-items-center text-sm text-risk">
            Backend unreachable — is the API running on :8080?
          </div>
        )}
        {total === 0 && lineage.data && (
          <div className="absolute inset-0 grid place-items-center text-center text-sm text-text-dim">
            <div>
              Nothing in force as of {asOf}.
              <br />
              <span className="text-xs">
                Pick a date after the circular&apos;s 2024-05-15 issue.
              </span>
            </div>
          </div>
        )}
        {lineage.data && total > 0 && <LineageGraph lineage={lineage.data} />}
      </div>
    </div>
  )
}
