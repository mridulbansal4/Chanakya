"use client"

import * as React from "react"
import { Panel, useReactFlow } from "@xyflow/react"
import { Search } from "lucide-react"

/**
 * GraphSearch provides instant, ultra-fast jump-to-node search on graph canvases.
 * - Enter / Live Search: Fast zoom-in to target node with glowing halo.
 * - Backspace / Empty: Guaranteed zoom-out reset to fit full graph.
 */
export function GraphSearch({ placeholder = "Find a clause or obligation…" }: { placeholder?: string }) {
  const rf = useReactFlow()
  const [q, setQ] = React.useState("")
  const [miss, setMiss] = React.useState(false)

  // Guaranteed zoom-out reset to fit the whole graph
  const resetToDefaultZoom = React.useCallback(() => {
    setMiss(false)
    rf.setNodes((nodes) => nodes.map((n) => ({ ...n, selected: false })))

    // Execute fitView after React Flow processes node unselection
    requestAnimationFrame(() => {
      setTimeout(() => {
        const allNodes = rf.getNodes()
        rf.fitView({
          nodes: allNodes,
          duration: 300,
          padding: 0.15,
        })
      }, 40)
    })
  }, [rf])

  const findNode = React.useCallback(
    (searchTerm: string) => {
      const term = searchTerm.trim().toLowerCase()
      if (!term) {
        resetToDefaultZoom()
        return
      }

      const allNodes = rf.getNodes()
      const match = allNodes.find((n) => {
        const d = (n.data as Record<string, unknown>) ?? {}
        const fields = [
          n.id,
          d.label,
          d.sublabel,
          d.ref,
          d.rawLabel,
          d.rawSublabel,
        ]
        return fields
          .map((v) => String(v ?? "").toLowerCase())
          .some((s) => s.includes(term))
      })

      if (match) {
        setMiss(false)

        // Highlight matching node with glowing selection halo
        rf.setNodes((nodes) =>
          nodes.map((n) => ({
            ...n,
            selected: n.id === match.id,
          }))
        )

        // Calculate node center coordinates
        const nodeWidth = (match.width as number) || (match.measured?.width as number) || 250
        const nodeHeight = (match.height as number) || (match.measured?.height as number) || 52
        const centerX = match.position.x + nodeWidth / 2
        const centerY = match.position.y + nodeHeight / 2

        // Fast camera glide centered on target node
        requestAnimationFrame(() => {
          rf.setCenter(centerX, centerY, {
            zoom: 1.6,
            duration: 300,
          })
        })
      } else {
        setMiss(true)
      }
    },
    [rf, resetToDefaultZoom]
  )

  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setQ(val)

    if (val.trim() === "") {
      resetToDefaultZoom()
    } else {
      findNode(val)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      findNode(q)
    } else if (e.key === "Backspace" && (q.length <= 1 || q.trim() === "")) {
      resetToDefaultZoom()
    }
  }

  return (
    <Panel position="top-right" className="!m-4">
      <div
        className={`flex items-center gap-2.5 rounded-2xl border bg-surface/95 px-4 py-2.5 shadow-2xl backdrop-blur-2xl transition-all ${
          miss
            ? "border-red-500/80 ring-2 ring-red-500/30"
            : "border-line focus-within:border-primary/80 focus-within:ring-2 focus-within:ring-primary/30"
        }`}
      >
        <Search className="size-4 text-muted-foreground shrink-0" aria-hidden />
        <input
          value={q}
          onChange={handleSearchChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className="w-56 bg-transparent text-xs font-semibold text-foreground outline-none placeholder:text-muted-foreground"
        />
      </div>
    </Panel>
  )
}
