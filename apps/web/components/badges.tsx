import type { DeonticType, ObligationStatus } from "@/lib/api"
import { DEONTIC_META, STATUS_LABEL } from "@/lib/format"

// Obligation type is not a "state": Required / Permitted read neutral;
// only Prohibited carries the risk colour (prohibition = a hard boundary).
const DEONTIC_STYLE: Record<DeonticType, string> = {
  MUST: "border-line bg-surface text-foreground",
  MUST_NOT: "border-danger/40 bg-surface text-danger",
  MAY: "border-line bg-surface text-muted-foreground",
}

/**
 * DeonticBadge shows only the plain-language obligation type ("Required" /
 * "Prohibited" / "Permitted"). The machine code (MUST / MUST NOT / MAY) lives
 * in the tooltip, so the UI never shows a confusing double badge.
 */
export function DeonticBadge({ deontic }: { deontic: DeonticType }) {
  const meta = DEONTIC_META[deontic]
  return (
    <span
      title={`${meta.label} (${meta.code}) — ${meta.help}`}
      className={`inline-flex items-center rounded border px-2 py-0.5 text-[11px] font-medium ${DEONTIC_STYLE[deontic]}`}
    >
      {meta.label}
    </span>
  )
}

// Status is state — the only place colour is allowed to signal.
const STATUS_STYLE: Record<ObligationStatus, string> = {
  pending: "border-warn/40 text-warn",
  needs_review: "border-warn/40 text-warn",
  approved: "border-verified/40 text-verified",
  rejected: "border-danger/40 text-danger",
}

export function StatusBadge({ status }: { status: ObligationStatus }) {
  return (
    <span
      className={`inline-flex rounded border bg-surface px-2 py-0.5 text-[11px] font-medium ${STATUS_STYLE[status]}`}
    >
      {STATUS_LABEL[status]}
    </span>
  )
}
