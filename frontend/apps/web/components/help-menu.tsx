"use client"

import * as React from "react"
import { HelpCircle } from "lucide-react"

/**
 * HelpMenu is the "?" control in the top bar: re-open the welcome tour or the
 * glossary at any time.
 */
export function HelpMenu({
  onTour,
  onGlossary,
}: {
  onTour: () => void
  onGlossary: () => void
}) {
  const [open, setOpen] = React.useState(false)
  const ref = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener("mousedown", handler)
    return () => document.removeEventListener("mousedown", handler)
  }, [])

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        title="Help & guide"
        aria-label="Help"
        className="grid size-8 place-items-center rounded-md border border-line-dark bg-ink-800 text-on-ink-dim hover:text-on-ink"
      >
        <HelpCircle className="size-4" />
      </button>
      {open && (
        <div className="hairline absolute right-0 z-40 mt-1 w-44 rounded-md bg-surface p-1 text-sm">
          <button
            type="button"
            onClick={() => {
              setOpen(false)
              onTour()
            }}
            className="w-full rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-2"
          >
            Take the tour
          </button>
          <button
            type="button"
            onClick={() => {
              setOpen(false)
              onGlossary()
            }}
            className="w-full rounded px-2.5 py-1.5 text-left text-foreground hover:bg-surface-2"
          >
            Glossary of terms
          </button>
        </div>
      )}
    </div>
  )
}
