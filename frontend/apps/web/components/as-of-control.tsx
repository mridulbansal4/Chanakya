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
      className="shiny-cta !py-1 !px-3 !text-xs font-medium !rounded-full shadow-sm"
    >
      <span>
        <CalendarClock className="size-3.5 text-blue-400 shrink-0" aria-hidden />
        <input
          type="date"
          value={asOf}
          max={today}
          onChange={(e) => setAsOf(e.target.value || today)}
          className="tnum bg-transparent text-white outline-none [color-scheme:dark] cursor-pointer"
        />
        {!isToday && (
          <button
            type="button"
            onClick={() => setAsOf(today)}
            className="tnum text-blue-400 hover:underline font-semibold ml-1"
          >
            today
          </button>
        )}
      </span>
    </label>
  )
}
