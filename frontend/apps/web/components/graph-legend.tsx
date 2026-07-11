"use client"

import { Panel } from "@xyflow/react"

export interface LegendItem {
  color: string
  label: string
  /** render as a dashed line swatch (for edge meanings) instead of a node swatch */
  line?: boolean
  dashed?: boolean
}

/**
 * GraphLegend overlays a compact key on a React Flow canvas so a non-engineer
 * can decode node colours/types and edge meanings without help.
 */
export function GraphLegend({ items }: { items: LegendItem[] }) {
  return (
    <Panel position="top-left" className="!m-2">
      <div className="rounded-2xl border border-white/60 bg-white/35 px-3 py-2.5 text-[10px] shadow-[0_8px_30px_rgba(20,20,20,0.12)] backdrop-blur-xl backdrop-saturate-150">
        <div className="mb-1.5 tracking-wide text-text-dim uppercase">Legend</div>
        <ul className="space-y-1">
          {items.map((it) => (
            <li key={it.label} className="flex items-center gap-1.5">
              {it.line ? (
                <span
                  className="inline-block h-0 w-3.5"
                  style={{
                    borderTop: `2px ${it.dashed ? "dashed" : "solid"} ${it.color}`,
                  }}
                />
              ) : (
                <span
                  className="inline-block h-2.5 w-2.5 rounded-sm"
                  style={{ background: it.color + "22", border: `1px solid ${it.color}` }}
                />
              )}
              <span className="text-muted-foreground">{it.label}</span>
            </li>
          ))}
        </ul>
      </div>
    </Panel>
  )
}
