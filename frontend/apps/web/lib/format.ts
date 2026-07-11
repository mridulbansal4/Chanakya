// Plain-language formatting helpers for the compliance-officer persona.
// The raw machine value stays available (e.g. in a tooltip) at every call site.

import type { DeonticType, ObligationStatus } from "@/lib/api"

/**
 * humanizeDuration turns an ISO-8601 duration into plain English:
 * "P30D" → "30 days", "P7D" → "7 days", "P5Y" → "5 years", "P1Y6M" → "1 year, 6 months".
 * Returns the input unchanged if it isn't a recognizable duration.
 */
export function humanizeDuration(iso: string): string {
  if (!iso) return ""
  const m = /^P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)W)?(?:(\d+)D)?$/.exec(iso.trim())
  if (!m || m.slice(1).every((x) => x === undefined)) return iso
  const parts: string[] = []
  const push = (n: string | undefined, unit: string) => {
    if (n) {
      const v = Number(n)
      parts.push(`${v} ${unit}${v === 1 ? "" : "s"}`)
    }
  }
  push(m[1], "year")
  push(m[2], "month")
  push(m[3], "week")
  push(m[4], "day")
  return parts.join(", ")
}

/** formatDeadline renders a deadline duration as "within 30 days". */
export function formatDeadline(iso: string): string {
  const h = humanizeDuration(iso)
  return h && h !== iso ? `within ${h}` : h
}

/**
 * durationDays approximates an ISO-8601 duration in days (for urgency sorting).
 * An empty/unparseable value is treated as least urgent (Infinity).
 */
export function durationDays(iso: string): number {
  const m = /^P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)W)?(?:(\d+)D)?$/.exec((iso || "").trim())
  if (!m) return Number.POSITIVE_INFINITY
  const days =
    Number(m[1] || 0) * 365 +
    Number(m[2] || 0) * 30 +
    Number(m[3] || 0) * 7 +
    Number(m[4] || 0)
  return days || Number.POSITIVE_INFINITY
}

/** Plain-language meaning of each deontic type, with the raw code for tooltips. */
export const DEONTIC_META: Record<
  DeonticType,
  { label: string; code: string; help: string }
> = {
  MUST: { label: "Required", code: "MUST", help: "Required — the firm must do this." },
  MUST_NOT: {
    label: "Prohibited",
    code: "MUST NOT",
    help: "Prohibited — the firm must not do this.",
  },
  MAY: { label: "Permitted", code: "MAY", help: "Permitted — optional for the firm." },
}

/** Plain-language label for each review status. */
export const STATUS_LABEL: Record<ObligationStatus, string> = {
  pending: "Pending",
  needs_review: "Needs review",
  approved: "Approved",
  rejected: "Rejected",
}

/** The review threshold below which extractions are routed to a human. */
export const REVIEW_THRESHOLD_PCT = 75

export function confidenceHelp(pct: number): string {
  return `AI confidence: ${pct}%. Items below ${REVIEW_THRESHOLD_PCT}% confidence are routed to you for review.`
}
