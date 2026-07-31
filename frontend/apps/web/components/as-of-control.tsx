"use client"

import { CalendarClock } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"

/**
 * AsOfControl is the deliberate "as-of date" selector that appears on every
 * data view. Numbers are mono/tabular. Changing it re-queries every bound view.
 */
export function AsOfControl() {
  const { asOf, setAsOf, today } = useAsOf()
  const isToday = asOf === today
  return (
    <label
      title="Reconstruct the compliance state as of this date"
      className="inline-flex items-center gap-2 rounded-lg border border-white/15 bg-white/10 px-3.5 py-2 text-sm text-white font-medium shadow-inner hover:border-white/30 transition-all"
    >
      <CalendarClock className="size-4 text-slate-300" aria-hidden />
      <input
        type="date"
        value={asOf}
        max={today}
        onChange={(e) => setAsOf(e.target.value || today)}
        className="tnum bg-transparent text-on-ink outline-none [color-scheme:dark]"
      />
      {!isToday && (
        <button
          type="button"
          onClick={() => setAsOf(today)}
          className="tnum text-lavender hover:underline"
        >
          today
        </button>
      )}
    </label>
  )
}
