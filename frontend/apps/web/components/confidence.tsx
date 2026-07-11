import { confidenceHelp } from "@/lib/format"

/**
 * ConfidenceMeter renders the AI's extraction confidence as a percentage + a
 * small bar (teal at/above the review threshold, amber below), with a tooltip
 * explaining what it means and that low-confidence items are routed to review.
 */
export function ConfidenceMeter({ value }: { value: number }) {
  const pct = Math.round(value * 100)
  const tone = pct >= 75 ? "bg-verified" : "bg-warn"
  return (
    <span
      title={confidenceHelp(pct)}
      className="inline-flex items-center gap-1.5 align-middle"
    >
      <span className="tnum text-xs text-foreground">{pct}%</span>
      <span className="relative h-1.5 w-10 overflow-hidden rounded-full bg-surface-2">
        <span
          className={`absolute inset-y-0 left-0 rounded-full ${tone}`}
          style={{ width: `${pct}%` }}
        />
      </span>
    </span>
  )
}
