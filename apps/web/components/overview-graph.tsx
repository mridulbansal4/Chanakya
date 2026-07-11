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
import type {
  DeonticType,
  GraphPayload,
  ObligationStatus,
} from "@/lib/api"

const OVERVIEW_LEGEND = [
  { color: "var(--text-dim)", label: "Clause (from the regulation)" },
  { color: "var(--ink)", label: "Obligation (subject + type)" },
  { color: "var(--ok)", label: "Approved" },
  { color: "var(--warn)", label: "Needs review / pending" },
  { color: "var(--risk)", label: "Rejected" },
  { color: "var(--ink)", label: "Clause → its obligations", line: true },
]

const STATUS_DOT: Record<ObligationStatus, string> = {
  approved: "var(--ok)",
  needs_review: "var(--warn)",
  pending: "var(--text-dim)",
  rejected: "var(--risk)",
}

interface CardData {
  kind: "clause" | "obligation"
  // searchable fields (GraphSearch reads label/sublabel/ref)
  label: string
  sublabel?: string
  ref?: string
  // obligation-only
  deontic?: DeonticType
  status?: ObligationStatus
}

// One editorial node: surface + hairline border + ink label. Obligation nodes
// carry a real subject, a status dot, and a Required/Prohibited/Permitted chip —
// never a bare "MUST".
function OverviewNode({ data }: NodeProps) {
  const d = data as unknown as CardData
  if (d.kind === "clause") {
    return (
      <div className="rounded-xl border border-line bg-surface px-3 py-2 text-xs shadow-[var(--shadow-card)]">
        <Handle type="target" position={Position.Left} className="!bg-line" />
        <div className="flex items-baseline gap-2">
          <span className="tnum text-foreground">{d.ref}</span>
          <span title={d.sublabel} className="max-w-[150px] truncate text-text-dim">
            {d.sublabel}
          </span>
        </div>
        <Handle type="source" position={Position.Right} className="!bg-line" />
      </div>
    )
  }
  return (
    <div className="flex items-center gap-2 rounded-xl border border-line bg-surface px-3 py-2 text-xs shadow-[var(--shadow-card)]">
      <Handle type="target" position={Position.Left} className="!bg-line" />
      <span
        className="inline-block size-2 shrink-0 rounded-full"
        style={{ background: STATUS_DOT[d.status ?? "pending"] }}
      />
      <span title={d.label} className="max-w-[150px] truncate font-medium text-foreground">
        {d.label}
      </span>
      {d.deontic && <DeonticBadge deontic={d.deontic} />}
      <Handle type="source" position={Position.Right} className="!bg-line" />
    </div>
  )
}

const nodeTypes = { ov: OverviewNode }

const W_CLAUSE = 200
const W_OBL = 250
const NODE_H = 44

// Auto-layout (dagre 'layered', left → right) — never hand-placed.
function layout(payload: GraphPayload): { nodes: Node[]; edges: Edge[] } {
  const g = new Dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: "LR", nodesep: 22, ranksep: 90, marginx: 24, marginy: 24 })

  const clauseHeading = new Map<string, string>()
  for (const n of payload.nodes) {
    if (n.type === "clause") clauseHeading.set(n.id, n.sublabel ?? n.label)
  }
  // Each obligation's subject is its parent clause's heading.
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
    return {
      id: n.id,
      type: "ov",
      position: { x: p.x - p.width / 2, y: p.y - p.height / 2 },
      data: data.get(n.id) as unknown as Record<string, unknown>,
      draggable: false,
    }
  })

  const edges: Edge[] = payload.edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: "smoothstep",
    style: {
      // ink for the obligation links, text-dim for the clause tree — both are
      // high-contrast and clearly visible on the light cream canvas.
      stroke: e.kind === "clause_obligation" ? "var(--ink)" : "var(--text-dim)",
      strokeWidth: e.kind === "clause_obligation" ? 1.75 : 1.5,
    },
  }))
  return { nodes, edges }
}

export function OverviewGraph({ payload }: { payload: GraphPayload }) {
  const { nodes, edges } = React.useMemo(() => layout(payload), [payload])
  return (
    <div className="h-full w-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        proOptions={{ hideAttribution: true }}
        minZoom={0.2}
        nodesDraggable={false}
        nodesConnectable={false}
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="var(--line)" />
        <Controls showInteractive={false} className="!border-line" />
        <GraphLegend items={OVERVIEW_LEGEND} />
        <GraphSearch placeholder="Find a clause…" />
      </ReactFlow>
    </div>
  )
}
