"use client"

import { Panel } from "@xyflow/react"

export interface LegendItem {
  color: string
  label: string
  /** Render as a line swatch, for edge meanings rather than node meanings. */
  line?: boolean
  dashed?: boolean
}

/**
 * The key to the graph's colour vocabulary.
 *
 * It sits directly on the canvas as an opaque panel rather than a blurred
 * one - a legend read against moving nodes underneath is harder to use, and
 * backdrop blur over a pannable canvas repaints on every frame of the pan.
 */
export function GraphLegend({ items }: { items: readonly LegendItem[] }) {
  return (
    <Panel position="top-left" className="!m-0 !ml-6 !mt-[4.5rem]">
      <div className="w-56 rounded-md border border-line-subtle bg-overlay px-3.5 py-3 shadow-elev-2">
        <p className="eyebrow">Legend</p>
        <ul className="mt-2.5 space-y-1.5">
          {items.map((it) => (
            <li key={it.label} className="flex items-center gap-2.5">
              {it.line ? (
                <span
                  aria-hidden
                  className="inline-block h-0 w-4 shrink-0"
                  style={{
                    borderTop: `2px ${it.dashed ? "dashed" : "solid"} ${it.color}`,
                  }}
                />
              ) : (
                <span
                  aria-hidden
                  className="inline-block size-2.5 shrink-0 rounded-sm"
                  style={{ background: it.color }}
                />
              )}
              <span className="text-body-sm text-fg-muted">{it.label}</span>
            </li>
          ))}
        </ul>
      </div>
    </Panel>
  )
}
