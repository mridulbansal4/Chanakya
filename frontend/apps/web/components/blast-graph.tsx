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
  { color: "#3B82F6", label: "Amended / directly affected" },
  { color: "#F59E0B", label: "Related match (semantic)" },
  { color: "#10B981", label: "Control" },
  { color: "#94A3B8", label: "Evidence" },
  { color: "#F59E0B", label: "Semantic link", line: true, dashed: true },
]

const COL_GAP = 280
const ROW_GAP = 82

function nodeStyle(kind: string): { color: string; tag: string } {
  switch (kind) {
    case "amended":
      return { color: "#3B82F6", tag: "AMENDED" }
    case "direct":
      return { color: "#3B82F6", tag: "DIRECT" }
    case "semantic":
      return { color: "#F59E0B", tag: "SEMANTIC" }
    case "control":
      return { color: "#10B981", tag: "CONTROL" }
    case "evidence":
      return { color: "#94A3B8", tag: "EVIDENCE" }
    default:
      return { color: "#64748B", tag: kind }
  }
}

function BlastNodeCard({ data, selected }: NodeProps) {
  const d = data as unknown as BlastNode
  const { color, tag } = nodeStyle(d.kind)
  const reduce = useReducedMotion()

  return (
    <motion.div
      initial={reduce ? false : { opacity: 0, scale: 0.88, y: 6 }}
      animate={{ opacity: 1, scale: 1, y: 0 }}
      transition={
        reduce
          ? { duration: 0 }
          : { delay: d.layer * 0.25, duration: 0.35, ease: [0.16, 1, 0.3, 1] }
      }
      className={`rounded-2xl border px-4 py-3 text-xs shadow-xl transition-all duration-200 hover:-translate-y-0.5 ${
        d.kind === "amended"
          ? "border-blue-500 bg-[#12141D] text-white ring-2 ring-blue-500/40 shadow-blue-500/20"
          : selected
          ? "border-blue-500 bg-[#12141D] ring-2 ring-blue-500/40"
          : d.kind === "semantic"
          ? "border-amber-500/40 bg-[#12141D] hover:border-amber-400"
          : d.kind === "control"
          ? "border-emerald-500/40 bg-[#12141D] hover:border-emerald-400"
          : "border-white/10 bg-[#12141D] hover:border-blue-400/50"
      }`}
    >
      <Handle type="target" position={Position.Left} className="!bg-[#64748B] !w-1.5 !h-1.5 !border-none" />
      <div className="flex items-center gap-2.5">
        <span
          className="inline-block size-2.5 shrink-0 rounded-full"
          style={{ background: color }}
        />
        <span
          title={d.label}
          className="tnum max-w-[180px] truncate font-bold text-white"
        >
          {d.label}
        </span>
        {d.ref && d.type !== "obligation" ? null : (
          <span className="text-slate-400 text-xs">
            {d.sublabel}
          </span>
        )}
      </div>
      <div className="mt-1.5 flex items-center gap-2 pl-4 text-[10px] text-slate-400 font-mono">
        <span className="font-bold tracking-wider" style={{ color }}>
          {tag}
        </span>
        {typeof d.similarity === "number" && d.kind === "semantic" && (
          <span className="tnum font-medium text-slate-300">
            · {Math.round(d.similarity * 100)}% related
          </span>
        )}
      </div>
      <Handle type="source" position={Position.Right} className="!bg-[#64748B] !w-1.5 !h-1.5 !border-none" />
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
        draggable: true,
      })
    })
  }
  const edges: Edge[] = payload.edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: "smoothstep",
    animated: !reduce,
    style: {
      stroke:
        e.kind === "semantic"
          ? "#F59E0B"
          : e.kind === "control_evidence"
            ? "#64748B"
            : "#3B82F6",
      strokeWidth: 2,
      strokeDasharray: e.kind === "semantic" ? "5 4" : undefined,
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
    <div key={runKey} className="h-full w-full relative bg-[#090A0F]">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        proOptions={{ hideAttribution: true }}
        minZoom={0.2}
        nodesDraggable={true}
        nodesConnectable={false}
      >
        <Background variant={BackgroundVariant.Dots} gap={24} size={1.2} color="#1E2235" />
        <Controls showInteractive={false} className="!border-white/10 !shadow-2xl" />
        <GraphLegend items={BLAST_LEGEND} />
      </ReactFlow>
    </div>
  )
}
