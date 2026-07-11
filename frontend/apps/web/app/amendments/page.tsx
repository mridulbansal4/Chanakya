"use client"

import * as React from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import { Zap } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { BlastGraph } from "@/components/blast-graph"
import {
  computeBlastRadius,
  listClauses,
  type BlastChange,
  type BlastRadius,
  type Clause,
} from "@/lib/api"

const CATEGORY_COLOR: Record<BlastChange["category"], string> = {
  obligation: "text-warn",
  control: "text-verified",
  evidence: "text-muted-foreground",
}

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`

/** One-line plain-language summary of a blast-radius result. */
function blastSummary(b: BlastRadius): string {
  const s = b.summary
  const needReview = b.nodes.filter(
    (n) => n.type === "obligation" && n.status && n.status !== "approved",
  ).length
  let msg = `This change affects ${plural(s.obligations, "obligation")}, ${plural(
    s.controls,
    "control",
  )}, and ${plural(s.evidence, "evidence source")}.`
  if (needReview > 0) {
    msg += ` ${needReview} obligation${needReview === 1 ? "" : "s"} still need${
      needReview === 1 ? "s" : ""
    } your review.`
  }
  return msg
}

export default function AmendmentsPage() {
  const { asOf } = useAsOf()
  const clauses = useQuery({
    queryKey: ["clauses", asOf],
    queryFn: ({ signal }) => listClauses(asOf, signal),
  })

  const [ref, setRef] = React.useState<string>("")
  const [text, setText] = React.useState<string>("")
  const [runKey, setRunKey] = React.useState(0)

  // Prefill on load, and re-select when the current clause is no longer in the
  // list (e.g. the as-of date changed to before it was in force) so a stale
  // clause_ref is never submitted.
  const clauseList = clauses.data?.clauses ?? []
  React.useEffect(() => {
    if (!clauseList.length) return
    if (clauseList.some((c) => c.clause_ref === ref)) return
    const first = clauseList.find((c) => c.clause_ref === "4.1") ?? clauseList[0]
    setRef(first!.clause_ref)
    setText(first!.text)
  }, [clauseList, ref])

  const selectClause = (c: Clause) => {
    setRef(c.clause_ref)
    setText(c.text)
  }

  const blast = useMutation({
    mutationFn: () =>
      computeBlastRadius({ clause_ref: ref, amended_text: text, as_of: asOf }),
    onSuccess: () => setRunKey((k) => k + 1),
  })

  return (
    <div className="flex h-full">
      {/* Left: amendment editor + change list */}
      <div className="flex w-[380px] shrink-0 flex-col border-r border-line">
        <div className="space-y-3 border-b border-line p-4">
          <div>
            <label className="text-[11px] tracking-wide text-muted-foreground uppercase">
              Amend clause
            </label>
            <select
              value={ref}
              onChange={(e) => {
                const c = clauseList.find((x) => x.clause_ref === e.target.value)
                if (c) selectClause(c)
              }}
              className="hairline mt-1 w-full rounded-md bg-surface px-2.5 py-1.5 text-sm text-foreground outline-none [color-scheme:light]"
            >
              {clauseList.map((c) => (
                <option key={c.id} value={c.clause_ref}>
                  {c.clause_ref} — {c.heading}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="text-[11px] tracking-wide text-muted-foreground uppercase">
              Amended text
            </label>
            <textarea
              value={text}
              onChange={(e) => setText(e.target.value)}
              rows={7}
              className="hairline mt-1 w-full resize-none rounded-md bg-surface px-2.5 py-2 text-sm leading-relaxed text-foreground outline-none"
            />
          </div>
          <button
            type="button"
            onClick={() => blast.mutate()}
            disabled={!ref || !text.trim() || blast.isPending}
            className="hairline inline-flex w-full items-center justify-center gap-2 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
          >
            <Zap className="size-4" />
            {blast.isPending ? "Computing…" : "Compute blast radius"}
          </button>
        </div>

        {/* Change list */}
        <div className="min-h-0 flex-1 overflow-auto p-4">
          {blast.data ? (
            <div className="space-y-3">
              <div className="rounded-md border border-warn/40 bg-warn/5 p-3 text-xs leading-relaxed text-foreground">
                {blastSummary(blast.data)}
              </div>
              <ul className="space-y-2">
                {blast.data.changes.map((c, i) => (
                  <li
                    key={i}
                    className="hairline rounded-md bg-surface px-3 py-2 text-xs"
                  >
                    <span
                      className={`text-[10px] tracking-wide uppercase ${CATEGORY_COLOR[c.category]}`}
                    >
                      {c.category}
                    </span>
                    <p className="mt-0.5 text-foreground">{c.detail}</p>
                  </li>
                ))}
              </ul>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              Edit a clause and compute the blast radius to see exactly which
              obligations, controls and evidence are affected.
            </p>
          )}
          {blast.isError && (
            <p className="text-sm text-danger">Failed to compute blast radius.</p>
          )}
        </div>
      </div>

      {/* Right: animated blast-radius graph */}
      <div className="relative flex-1">
        {blast.data ? (
          <BlastGraph payload={blast.data} runKey={runKey} />
        ) : (
          <div className="absolute inset-0 grid place-items-center text-sm text-muted-foreground">
            The blast-radius graph appears here.
          </div>
        )}
      </div>
    </div>
  )
}
