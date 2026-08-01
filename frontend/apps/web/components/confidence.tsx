import { confidenceHelp } from "@/lib/format"

/** Below this, extraction is routed to human review. */
const REVIEW_THRESHOLD = 75

/**
 * Extraction confidence as a percentage plus a bar.
 *
 * The percentage is always rendered, not just the bar: colour and length
 * both encode the same thing, and neither survives a printed audit pack on
 * its own. The bar is amber below the review threshold - the same meaning
 * amber carries everywhere else, i.e. "a person needs to look at this".
 */
export function ConfidenceMeter({ value }: { value: number }) {
  const pct = Math.round(value * 100)
  const belowThreshold = pct < REVIEW_THRESHOLD

  return (
    <span
      title={confidenceHelp(pct)}
      role="meter"
      aria-valuenow={pct}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={`Extraction confidence ${pct} percent${
        belowThreshold ? ", below review threshold" : ""
      }`}
      className="inline-flex items-center gap-2 align-middle"
    >
      <span
        className={`tnum text-label-lg ${belowThreshold ? "text-warn" : "text-fg-muted"}`}
      >
        {pct}%
      </span>
      <span className="relative h-1 w-14 overflow-hidden rounded-full bg-elevated">
        <span
          className={`absolute inset-y-0 left-0 rounded-full ${
            belowThreshold ? "bg-warn" : "bg-ok"
          }`}
          style={{ width: `${pct}%` }}
        />
      </span>
    </span>
  )
}
