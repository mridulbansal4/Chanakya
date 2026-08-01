"use client"

import * as React from "react"
import Dagre from "@dagrejs/dagre"
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

import { DeonticBadge } from "@/components/badges"
import { GraphLegend } from "@/components/graph-legend"
import { GraphSearch } from "@/components/graph-search"
import { GRAPH, GRAPH_STATUS, OVERVIEW_LEGEND } from "@/lib/graph-theme"
import type { DeonticType, GraphPayload, ObligationStatus } from "@/lib/api"

const STATUS_DOT: Record<ObligationStatus, string> = GRAPH_STATUS

interface CardData {
  kind: "clause" | "obligation"
  label: string
  sublabel?: string
  ref?: string
  rawLabel?: string
  rawSublabel?: string
  deontic?: DeonticType
  status?: ObligationStatus
}

function OverviewNode({ data, selected }: NodeProps) {
  const d = data as unknown as CardData
  const isApproved = d.status === "approved"
  const isReview = d.status === "needs_review"
  const isRisk = d.status === "rejected"

  if (d.kind === "clause") {
    return (
      <div
        className={`rounded-2xl border px-4 py-3 text-xs shadow-2xl transition-all duration-300 hover:-translate-y-0.5 ${
          selected
            ? "border-accent bg-accent-weak ring-4 ring-accent/60 scale-105 z-50"
            : "border-line-subtle bg-raised hover:border-accent-line"
        }`}
      >
        <Handle type="target" position={Position.Left} className="!w-1.5 !h-1.5 !border-none" style={{ background: GRAPH.handle }} />
        <div className="flex items-center gap-2.5">
          <span className="tnum rounded-md border border-line bg-elevated px-2 py-0.5 font-mono text-xs font-bold text-fg">
            Clause {d.ref}
          </span>
          <span title={d.sublabel} className="max-w-[180px] truncate text-xs font-semibold text-fg-muted">
            {d.sublabel}
          </span>
        </div>
        <Handle type="source" position={Position.Right} className="!w-1.5 !h-1.5 !border-none" style={{ background: GRAPH.handle }} />
      </div>
    )
  }

  return (
    <div
      className={`flex items-center gap-3 rounded-2xl border px-4 py-3 text-xs shadow-2xl transition-all duration-300 hover:-translate-y-0.5 ${
        selected
          ? "border-accent bg-accent-weak ring-4 ring-accent/60 scale-105 z-50"
          : isApproved
          ? "border-ok-line bg-raised hover:border-ok"
          : isReview
          ? "border-warn-line bg-raised hover:border-warn"
          : isRisk
          ? "border-risk-line bg-raised hover:border-risk"
          : "border-line-subtle bg-raised hover:border-accent-line"
      }`}
    >
      <Handle type="target" position={Position.Left} className="!w-1.5 !h-1.5 !border-none" style={{ background: GRAPH.handle }} />
      <span
        className="inline-block size-2.5 shrink-0 rounded-full"
        style={{ background: STATUS_DOT[d.status ?? "pending"] }}
      />
      <span title={d.label} className="max-w-[200px] truncate text-xs font-bold text-fg">
        {d.label}
      </span>
      {d.deontic && <DeonticBadge deontic={d.deontic} />}
      <Handle type="source" position={Position.Right} className="!w-1.5 !h-1.5 !border-none" style={{ background: GRAPH.handle }} />
    </div>
  )
}

const nodeTypes = { ov: OverviewNode }

const W_CLAUSE = 230
const W_OBL = 280
const NODE_H = 52

function layout(payload: GraphPayload): { nodes: Node[]; edges: Edge[] } {
  const g = new Dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: "LR", nodesep: 24, ranksep: 100, marginx: 32, marginy: 32 })

  const clauseHeading = new Map<string, string>()
  for (const n of payload.nodes) {
    if (n.type === "clause") clauseHeading.set(n.id, n.sublabel ?? n.label)
  }
  const parentClause = new Map<string, string>()
  for (const e of payload.edges) {
    if (e.kind === "clause_obligation") parentClause.set(e.target, e.source)
  }

  const data = new Map<string, CardData>()
  for (const n of payload.nodes) {
    if (n.type === "clause") {
      data.set(n.id, {
        kind: "clause",
        label: n.ref ?? n.label,
        sublabel: n.sublabel ?? "",
        ref: n.ref,
        rawLabel: n.label,
        rawSublabel: n.sublabel,
      })
      g.setNode(n.id, { width: W_CLAUSE, height: NODE_H })
    } else {
      const parent = parentClause.get(n.id)
      const subject = (parent && clauseHeading.get(parent)) || n.sublabel || n.label
      data.set(n.id, {
        kind: "obligation",
        label: subject,
        sublabel: subject,
        ref: n.ref,
        rawLabel: n.label,
        rawSublabel: n.sublabel,
        deontic: n.deontic ?? "MUST",
        status: n.status ?? "pending",
      })
      g.setNode(n.id, { width: W_OBL, height: NODE_H })
    }
  }
  for (const e of payload.edges) g.setEdge(e.source, e.target)

  Dagre.layout(g)

  const nodes: Node[] = payload.nodes.map((n) => {
    const p = g.node(n.id)
    const width = n.type === "clause" ? W_CLAUSE : W_OBL
    return {
      id: n.id,
      type: "ov",
      position: { x: p.x - p.width / 2, y: p.y - p.height / 2 },
      width,
      height: NODE_H,
      data: data.get(n.id) as unknown as Record<string, unknown>,
      draggable: true,
    }
  })

  const edges: Edge[] = payload.edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: "smoothstep",
    animated: true,
    style: {
      stroke: e.kind === "clause_obligation" ? GRAPH.edgeProvenance : GRAPH.edgeDefault,
      strokeWidth: 2,
    },
  }))
  return { nodes, edges }
}

export function OverviewGraph({ payload }: { payload: GraphPayload }) {
  const { nodes, edges } = React.useMemo(() => layout(payload), [payload])
  return (
    <div className="h-full w-full relative" style={{ background: GRAPH.canvas }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        proOptions={{ hideAttribution: true }}
        minZoom={0.2}
        nodesDraggable={true}
        nodesConnectable={false}
      >
        <Background variant={BackgroundVariant.Dots} gap={24} size={1.2} color={GRAPH.dots} />
        <Controls showInteractive={false} className="!border-line-subtle !shadow-elev-2" />
        <GraphLegend items={OVERVIEW_LEGEND} />
        <GraphSearch placeholder="Find a clause or obligation…" />
      </ReactFlow>
    </div>
  )
}
