import type { DeonticType, ObligationStatus } from "@/lib/api"
import { DEONTIC_META, STATUS_LABEL } from "@/lib/format"

/**
 * Obligation type is a category, not a state, so it is not coloured.
 * Only Prohibited carries the risk colour, because a prohibition is a hard
 * boundary rather than a classification.
 */
const DEONTIC_STYLE: Record<DeonticType, string> = {
  MUST: "border-line text-fg",
  MUST_NOT: "border-risk-line bg-risk-weak text-risk",
  MAY: "border-line-subtle text-fg-muted",
}

export function DeonticBadge({ deontic }: { deontic: DeonticType }) {
  const meta = DEONTIC_META[deontic]
  return (
    <span
      title={`${meta.label} (${meta.code}) — ${meta.help}`}
      className={`inline-flex shrink-0 items-center rounded border px-2 py-0.5 text-label-md ${DEONTIC_STYLE[deontic]}`}
    >
      {meta.label}
    </span>
  )
}

/**
 * Status is the one dimension colour is allowed to encode. Even here the
 * label is always rendered alongside the colour — a badge that is only a
 * coloured pill is unreadable to anyone with a colour vision deficiency,
 * and unreadable in a printed audit pack.
 */
const STATUS_STYLE: Record<ObligationStatus, string> = {
  pending: "border-warn-line bg-warn-weak text-warn",
  needs_review: "border-warn-line bg-warn-weak text-warn",
  approved: "border-ok-line bg-ok-weak text-ok",
  rejected: "border-risk-line bg-risk-weak text-risk",
}

export function StatusBadge({ status }: { status: ObligationStatus }) {
  return (
    <span
      className={`inline-flex shrink-0 items-center rounded border px-2 py-0.5 text-label-md ${STATUS_STYLE[status]}`}
    >
      {STATUS_LABEL[status]}
    </span>
  )
}

/**
 * The compact status marker used in dense lists where a full badge would
 * not fit.
 *
 * Colour alone is never the carrier: `approved` is a filled disc,
 * `needs_review` and `pending` are ringed, and `rejected` is filled with a
 * strike. Shape survives greyscale, colour blindness, and a fax machine —
 * which, for a regulatory audit pack, is not a hypothetical.
 */
const STATUS_DOT: Record<ObligationStatus, string> = {
  approved: "bg-ok border-ok",
  needs_review: "border-warn bg-warn-weak",
  pending: "border-fg-faint bg-transparent",
  rejected: "bg-risk border-risk",
}

export function StatusDot({ status }: { status: ObligationStatus }) {
  return (
    <span
      role="img"
      aria-label={STATUS_LABEL[status]}
      title={STATUS_LABEL[status]}
      className={`inline-block size-2.5 shrink-0 rounded-full border-[1.5px] ${STATUS_DOT[status]}`}
    />
  )
}
