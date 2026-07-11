"use client"

import * as React from "react"
import { Panel, useReactFlow } from "@xyflow/react"
import { Search } from "lucide-react"

/**
 * GraphSearch lets a user jump to a node by clause number or heading text on a
 * large graph. It centers the viewport on the first match. Must be rendered as
 * a child of <ReactFlow>.
 */
export function GraphSearch({ placeholder = "Find a clause…" }: { placeholder?: string }) {
  const rf = useReactFlow()
  const [q, setQ] = React.useState("")
  const [miss, setMiss] = React.useState(false)

  const find = () => {
    const term = q.trim().toLowerCase()
    if (!term) return
    const match = rf.getNodes().find((n) => {
      const d = n.data as Record<string, unknown>
      return [d.label, d.sublabel, d.ref]
        .map((v) => String(v ?? "").toLowerCase())
        .some((s) => s.includes(term))
    })
    if (match) {
      setMiss(false)
      rf.setCenter(match.position.x + 70, match.position.y + 20, {
        zoom: 1.4,
        duration: 500,
      })
    } else {
      setMiss(true)
    }
  }

  return (
    <Panel position="top-right" className="!m-2">
      <div
        className={`flex items-center gap-1.5 rounded-2xl border bg-white/35 px-2.5 py-1.5 shadow-[0_8px_30px_rgba(20,20,20,0.12)] backdrop-blur-xl backdrop-saturate-150 ${
          miss ? "border-danger/60" : "border-white/60"
        }`}
      >
        <Search className="size-3 text-text-dim" aria-hidden />
        <input
          value={q}
          onChange={(e) => {
            setQ(e.target.value)
            setMiss(false)
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") find()
          }}
          placeholder={placeholder}
          className="w-36 bg-transparent text-xs text-foreground outline-none placeholder:text-muted-foreground"
        />
      </div>
    </Panel>
  )
}
