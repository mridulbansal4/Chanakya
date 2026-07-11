"use client"

import * as React from "react"
import { motion, useReducedMotion } from "framer-motion"
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"

import { GraphLegend } from "@/components/graph-legend"
import type { BlastNode, BlastRadius } from "@/lib/api"

const BLAST_LEGEND = [
  { color: "var(--ink)", label: "Amended / directly affected" },
  { color: "var(--warn)", label: "Related match (semantic)" },
  { color: "var(--ok)", label: "Control" },
  { color: "var(--text-dim)", label: "Evidence" },
  { color: "var(--warn)", label: "Semantic link", line: true, dashed: true },
]

const COL_GAP = 250
const ROW_GAP = 74

// Small type dot + tag by kind. Semantic obligations are amber (pulled in by
// similarity, not a direct structural link) — the interesting propagation.
function nodeStyle(kind: string): { color: string; tag: string } {
  switch (kind) {
    case "amended":
      return { color: "var(--ink)", tag: "AMENDED" }
    case "direct":
      return { color: "var(--ink)", tag: "direct" }
    case "semantic":
      return { color: "var(--warn)", tag: "semantic" }
    case "control":
      return { color: "var(--ok)", tag: "control" }
    case "evidence":
      return { color: "var(--text-dim)", tag: "evidence" }
    default:
      return { color: "var(--line)", tag: kind }
  }
}

// BlastNodeCard animates in with a delay proportional to its layer, so the
// impact visibly *propagates* clause → obligation → control → evidence. Motion
// here communicates causation, per the design system.
function BlastNodeCard({ data }: NodeProps) {
  const d = data as unknown as BlastNode
  const { color, tag } = nodeStyle(d.kind)
  const reduce = useReducedMotion()
  return (
    <motion.div
      initial={reduce ? false : { opacity: 0, scale: 0.9, y: 4 }}
      animate={{ opacity: 1, scale: 1, y: 0 }}
      transition={
        reduce
          ? { duration: 0 }
          : { delay: d.layer * 0.35, duration: 0.35, ease: "easeOut" }
      }
      className="rounded-xl border px-3 py-2 text-xs"
      style={{
        borderColor: d.kind === "amended" ? "var(--ink)" : "var(--line)",
        background: "var(--surface)",
        boxShadow: "var(--shadow-card)",
      }}
    >
      <Handle type="target" position={Position.Left} className="!bg-line" />
      <div className="flex items-center gap-1.5">
        <span
          className="inline-block size-2 shrink-0 rounded-full"
          style={{ background: color }}
        />
        <span title={d.label} className="tnum max-w-[150px] truncate font-medium text-foreground">
          {d.label}
        </span>
        {d.ref && d.type !== "obligation" ? null : (
          <span className="text-text-dim">{d.sublabel}</span>
        )}
      </div>
      <div className="mt-0.5 flex items-center gap-1.5 pl-3.5 text-[10px] text-text-dim">
        <span style={{ color }}>{tag}</span>
        {typeof d.similarity === "number" && d.kind === "semantic" && (
          <span
            className="tnum"
            title="Estimated how closely related this obligation is to the amended clause. Higher = more likely to be affected."
          >
            · {Math.round(d.similarity * 100)}% related
          </span>
        )}
      </div>
      <Handle type="source" position={Position.Right} className="!bg-line" />
    </motion.div>
  )
}

const nodeTypes = { blast: BlastNodeCard }

function layout(
  payload: BlastRadius,
  reduce: boolean,
): { nodes: Node[]; edges: Edge[] } {
  const byLayer: Record<number, BlastNode[]> = {}
  for (const n of payload.nodes) {
    ;(byLayer[n.layer] ??= []).push(n)
  }
  const nodes: Node[] = []
  for (const [layerStr, group] of Object.entries(byLayer)) {
    const layer = Number(layerStr)
    const offset = ((group.length - 1) * ROW_GAP) / 2
    group.forEach((n, i) => {
      nodes.push({
        id: n.id,
        type: "blast",
        position: { x: layer * COL_GAP, y: i * ROW_GAP - offset + 260 },
        data: n as unknown as Record<string, unknown>,
      })
    })
  }
  const edges: Edge[] = payload.edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: "smoothstep",
    animated: !reduce, // moving dashes = the amendment propagating downstream
    style: {
      stroke:
        e.kind === "semantic"
          ? "var(--warn)"
          : e.kind === "control_evidence"
            ? "var(--text-dim)"
            : "var(--ink)",
      strokeWidth: 1.5,
      strokeDasharray: e.kind === "semantic" ? "4 3" : undefined,
    },
  }))
  return { nodes, edges }
}

export function BlastGraph({
  payload,
  runKey,
}: {
  payload: BlastRadius
  runKey: number
}) {
  const reduce = useReducedMotion() ?? false
  const { nodes, edges } = React.useMemo(
    () => layout(payload, reduce),
    [payload, reduce],
  )
  return (
    // key forces a remount per computation so the propagation animation replays.
    <div key={runKey} className="h-full w-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        proOptions={{ hideAttribution: true }}
        minZoom={0.2}
        nodesDraggable={false}
        nodesConnectable={false}
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="var(--line)" />
        <Controls showInteractive={false} className="!border-line" />
        <GraphLegend items={BLAST_LEGEND} />
      </ReactFlow>
    </div>
  )
}
