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
 * GraphLegend overlays a high-contrast minimalist key on the graph.
 */
export function GraphLegend({ items }: { items: LegendItem[] }) {
  return (
    <Panel position="top-left" className="!m-0 !mt-16 !ml-6">
      <div className="w-56 rounded-2xl border border-line bg-surface/95 px-4 py-3 text-xs text-foreground shadow-2xl backdrop-blur-2xl space-y-2">
        <div className="text-[10px] font-mono font-bold tracking-widest text-muted-foreground uppercase">
          Graph Legend
        </div>
        <ul className="space-y-1.5">
          {items.map((it) => (
            <li key={it.label} className="flex items-center gap-2.5">
              {it.line ? (
                <span
                  className="inline-block h-0 w-4"
                  style={{
                    borderTop: `2px ${it.dashed ? "dashed" : "solid"} ${it.color}`,
                  }}
                />
              ) : (
                <span
                  className="inline-block size-3 rounded-md shadow-xs"
                  style={{ background: it.color, border: `1px solid ${it.color}` }}
                />
              )}
              <span className="font-medium text-slate-200 text-xs">{it.label}</span>
            </li>
          ))}
        </ul>
      </div>
    </Panel>
  )
}
