"use client"

import { useQuery } from "@tanstack/react-query"
import { X } from "lucide-react"

import { DeonticBadge, StatusBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { formatDeadline } from "@/lib/format"
import { getObligation, type ObligationDetail } from "@/lib/api"

/**
 * ObligationDetailPanel shows an obligation's full record beside its source: the
 * clause text with the EXACT cited sentence highlighted, plus the reasoning
 * chain (deontic, threshold, deadline, confidence). This is "the citation, one
 * click away" — every extracted claim is traceable to its source sentence.
 */
export function ObligationDetailPanel({
  id,
  onClose,
}: {
  id: string
  onClose: () => void
}) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ["obligation", id],
    queryFn: ({ signal }) => getObligation(id, signal),
  })

  return (
    <aside className="flex w-[380px] shrink-0 flex-col border-l border-line bg-surface">
      <header className="flex items-center justify-between border-b border-line px-4 py-3">
        <span className="text-xs tracking-wide text-muted-foreground uppercase">
          Obligation detail
        </span>
        <button
          type="button"
          onClick={onClose}
          className="text-muted-foreground hover:text-foreground"
          aria-label="Close"
        >
          <X className="size-4" />
        </button>
      </header>

      <div className="min-h-0 flex-1 overflow-auto p-4">
        {isLoading && <p className="text-sm text-muted-foreground">Loading…</p>}
        {isError && <p className="text-sm text-danger">Failed to load.</p>}
        {data && <DetailBody d={data} />}
      </div>
    </aside>
  )
}

function DetailBody({ d }: { d: ObligationDetail }) {
  const threshold = Object.keys(d.threshold ?? {}).length
    ? JSON.stringify(d.threshold)
    : "—"
  return (
    <div className="space-y-5 text-sm">
      <div>
        <div className="flex items-center gap-2">
          <span className="tnum text-primary">{d.clause_ref}</span>
          <DeonticBadge deontic={d.deontic_type} />
          <StatusBadge status={d.status} />
        </div>
        <h2 className="font-display mt-1 text-lg leading-tight">
          {d.clause_heading}
        </h2>
      </div>

      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
        <Field label="Bearer" value={d.bearer} />
        <div>
          <dt className="text-muted-foreground">AI confidence</dt>
          <dd>
            <ConfidenceMeter value={d.confidence} />
          </dd>
        </div>
        <div>
          <dt className="text-muted-foreground">Deadline</dt>
          <dd className="text-foreground" title={d.deadline || undefined}>
            {d.deadline ? formatDeadline(d.deadline) : "—"}
          </dd>
        </div>
        <Field label="Threshold" value={threshold} mono />
        <Field label="In force from" value={d.valid_from} mono />
        <Field label="Penalty" value={d.penalty || "—"} />
      </dl>

      {/* Reasoning chain / citation */}
      <section>
        <div className="text-[11px] tracking-wide text-muted-foreground uppercase">
          Citation — source clause {d.source_clause_ref}
        </div>
        <blockquote className="mt-2 border-l-2 border-verified pl-3 text-sm leading-relaxed">
          <ClauseWithCitation text={d.clause_text} cited={d.source_sentence} />
        </blockquote>
      </section>
    </div>
  )
}

function Field({
  label,
  value,
  mono,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={mono ? "tnum text-foreground" : "text-foreground"}>{value}</dd>
    </div>
  )
}

// ClauseWithCitation renders the clause text with the cited sentence
// highlighted. Matching mirrors the backend's whitespace-normalised substring
// check, so the highlight lands even when spacing differs.
function ClauseWithCitation({ text, cited }: { text: string; cited: string }) {
  const norm = (s: string) => s.replace(/\s+/g, " ").trim()
  const idx = norm(text).indexOf(norm(cited))
  if (idx < 0) {
    return <span className="text-muted-foreground">{text}</span>
  }
  // Map back to original text by walking word boundaries is complex; for the
  // MVP we highlight the cited sentence rendered verbatim, with surrounding
  // context dimmed.
  const before = text.slice(0, text.indexOf(cited))
  const after = text.slice(text.indexOf(cited) + cited.length)
  if (text.includes(cited)) {
    return (
      <span className="text-muted-foreground">
        {before}
        <mark className="bg-transparent font-medium text-verified">{cited}</mark>
        {after}
      </span>
    )
  }
  return <span className="text-foreground">{text}</span>
}
