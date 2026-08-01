"use client"

import * as React from "react"
import { Panel, useReactFlow } from "@xyflow/react"
import { Search } from "lucide-react"

import { cn } from "@workspace/ui/lib/utils"

/**
 * Jump-to-node search for graph canvases.
 *
 * Typing centres the camera on the first match; clearing the field returns
 * the view to fit the whole graph. The camera glide is 300ms - long enough
 * that the eye can follow the movement and understand where in the graph it
 * landed, which is the entire reason for animating a viewport change rather
 * than cutting to it.
 */
export function GraphSearch({
  placeholder = "Find a clause or obligation…",
}: {
  placeholder?: string
}) {
  const rf = useReactFlow()
  const [q, setQ] = React.useState("")
  const [miss, setMiss] = React.useState(false)

  const resetToDefaultZoom = React.useCallback(() => {
    setMiss(false)
    rf.setNodes((nodes) => nodes.map((n) => ({ ...n, selected: false })))

    // Wait for React Flow to commit the deselection before fitting, or the
    // fit is computed against stale node state.
    requestAnimationFrame(() => {
      setTimeout(() => {
        rf.fitView({ nodes: rf.getNodes(), duration: 300, padding: 0.15 })
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

      const match = rf.getNodes().find((n) => {
        const d = (n.data as Record<string, unknown>) ?? {}
        return [n.id, d.label, d.sublabel, d.ref, d.rawLabel, d.rawSublabel]
          .map((v) => String(v ?? "").toLowerCase())
          .some((s) => s.includes(term))
      })

      if (!match) {
        setMiss(true)
        return
      }

      setMiss(false)
      rf.setNodes((nodes) =>
        nodes.map((n) => ({ ...n, selected: n.id === match.id })),
      )

      const nodeWidth =
        (match.width as number) || (match.measured?.width as number) || 250
      const nodeHeight =
        (match.height as number) || (match.measured?.height as number) || 52

      requestAnimationFrame(() => {
        rf.setCenter(
          match.position.x + nodeWidth / 2,
          match.position.y + nodeHeight / 2,
          { zoom: 1.6, duration: 300 },
        )
      })
    },
    [rf, resetToDefaultZoom],
  )

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setQ(val)
    if (val.trim() === "") resetToDefaultZoom()
    else findNode(val)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") findNode(q)
    else if (e.key === "Backspace" && q.trim().length <= 1) resetToDefaultZoom()
  }

  return (
    <Panel position="top-right" className="!m-4">
      <div
        className={cn(
          "flex items-center gap-2.5 rounded-md border bg-overlay px-3 py-2 shadow-elev-2",
          "transition-colors duration-[120ms] ease-[cubic-bezier(0.2,0.8,0.2,1)]",
          miss
            ? "border-risk"
            : "border-line-subtle focus-within:border-accent-line",
        )}
      >
        <Search className="size-4 shrink-0 text-fg-subtle" aria-hidden />
        <input
          type="search"
          value={q}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          aria-label="Find a node in the graph"
          aria-invalid={miss || undefined}
          className="w-56 bg-transparent text-body-sm text-fg outline-none placeholder:text-fg-faint"
        />
      </div>
      {/* The camera move is invisible to a screen reader, so the outcome of
          the search is announced explicitly. */}
      <p role="status" aria-live="polite" className="sr-only">
        {q.trim() === "" ? "" : miss ? "No matching node" : "Node found"}
      </p>
    </Panel>
  )
}
