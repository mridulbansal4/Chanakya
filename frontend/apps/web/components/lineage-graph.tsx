"use client"

import * as React from "react"
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  Panel,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"

import type { Lineage, LineageNode, LineageNodeType } from "@/lib/api"

// The six lineage layers, left → right. Sign-off and policy are distinct
// columns so the trail reads Clause → Obligation → Control → Evidence →
// Sign-off → Policy.
const COLS: LineageNodeType[] = [
  "clause",
  "obligation",
  "control",
  "evidence",
  "signoff",
  "policy",
]
const LAYER: Record<LineageNodeType, number> = {
  clause: 0,
  obligation: 1,
  control: 2,
  evidence: 3,
  signoff: 4,
  policy: 5,
}

const COL_GAP = 250
const ROW_GAP = 80

// Small type dot — the only colour on an otherwise uniform surface node.
function typeDot(t: LineageNodeType): string {
  switch (t) {
    case "control":
    case "signoff":
      return "var(--ok)"
    case "policy":
      return "var(--warn)"
    case "obligation":
      return "var(--ink)"
    default:
      return "var(--text-dim)"
  }
}

interface CardData extends LineageNode {
  dim: boolean
  focused: boolean
}

// Editorial node: white surface, hairline border, ink label, small type dot.
// Labels wrap (no truncation) with a title tooltip for the full text.
function LineageNodeCard({ data }: NodeProps) {
  const d = data as unknown as CardData
  return (
    <div
      className="w-[184px] rounded-xl border bg-surface px-3 py-2 text-xs transition-opacity"
      style={{
        borderColor: d.focused ? "var(--ink)" : "var(--line)",
        boxShadow: "var(--shadow-card)",
        opacity: d.dim ? 0.15 : 1,
      }}
    >
      <Handle type="target" position={Position.Left} className="!bg-line" />
      <div className="flex items-center gap-1.5">
        <span
          className="inline-block size-2 shrink-0 rounded-full"
          style={{ background: typeDot(d.type) }}
        />
        <span className="tnum font-medium text-foreground" title={d.label}>
          {d.ref ?? d.label}
        </span>
      </div>
      {d.sublabel && (
        <div
          title={d.sublabel}
          className="mt-0.5 line-clamp-2 leading-snug text-text-dim"
        >
          {d.sublabel}
        </div>
      )}
      <Handle type="source" position={Position.Right} className="!bg-line" />
    </div>
  )
}

const nodeTypes = { lineage: LineageNodeCard }

/** Layered layout with a barycenter crossing-reduction pass (no external dep). */
function layout(lin: Lineage): Node[] {
  const cols: LineageNode[][] = COLS.map(() => [])
  for (const n of lin.nodes) cols[LAYER[n.type] ?? 0]!.push(n)

  // Undirected adjacency, so ordering considers both neighbour directions.
  const adj = new Map<string, string[]>()
  const link = (a: string, b: string) => {
    if (!adj.has(a)) adj.set(a, [])
    adj.get(a)!.push(b)
  }
  for (const e of lin.edges) {
    link(e.source, e.target)
    link(e.target, e.source)
  }

  const idx = new Map<string, number>()
  const reindex = () =>
    cols.forEach((col) => col.forEach((n, i) => idx.set(n.id, i)))
  reindex()

  const barycenter = (n: LineageNode): number => {
    const neigh = (adj.get(n.id) ?? []).filter((id) => idx.has(id))
    if (!neigh.length) return idx.get(n.id) ?? 0
    return neigh.reduce((s, id) => s + (idx.get(id) ?? 0), 0) / neigh.length
  }
  const sweep = (order: number[]) => {
    for (const L of order) {
      cols[L]!.sort((a, b) => barycenter(a) - barycenter(b))
      reindex()
    }
  }
  for (let i = 0; i < 4; i++) {
    sweep([1, 2, 3, 4, 5])
    sweep([4, 3, 2, 1, 0])
  }

  const nodes: Node[] = []
  cols.forEach((col, L) => {
    const offset = ((col.length - 1) * ROW_GAP) / 2
    col.forEach((n, i) => {
      nodes.push({
        id: n.id,
        type: "lineage",
        position: { x: L * COL_GAP, y: i * ROW_GAP - offset },
        data: n as unknown as Record<string, unknown>,
        draggable: false,
      })
    })
  })
  return nodes
}

export function LineageGraph({ lineage }: { lineage: Lineage }) {
  const [focus, setFocus] = React.useState<string | null>(null)

  const baseNodes = React.useMemo(() => layout(lineage), [lineage])

  // Directed adjacency: out = source→targets (downstream), in = target→sources
  // (upstream). A lineage chain is the node's ancestors + descendants — NOT the
  // whole connected component — so sibling obligations don't light up.
  const { outAdj, inAdj } = React.useMemo(() => {
    const out = new Map<string, string[]>()
    const inc = new Map<string, string[]>()
    const push = (m: Map<string, string[]>, k: string, v: string) => {
      if (!m.has(k)) m.set(k, [])
      m.get(k)!.push(v)
    }
    for (const e of lineage.edges) {
      push(out, e.source, e.target)
      push(inc, e.target, e.source)
    }
    return { outAdj: out, inAdj: inc }
  }, [lineage])

  // The focused node's lineage chain: walk descendants (out) and ancestors (in).
  const chain = React.useMemo(() => {
    if (!focus) return null
    const seen = new Set<string>([focus])
    const walk = (adj: Map<string, string[]>) => {
      const stack = [focus]
      while (stack.length) {
        const cur = stack.pop()!
        for (const nb of adj.get(cur) ?? []) {
          if (!seen.has(nb)) {
            seen.add(nb)
            stack.push(nb)
          }
        }
      }
    }
    walk(outAdj)
    walk(inAdj)
    return seen
  }, [focus, outAdj, inAdj])

  const nodes = React.useMemo(
    () =>
      baseNodes.map((n) => ({
        ...n,
        data: {
          ...n.data,
          dim: chain ? !chain.has(n.id) : false,
          focused: focus === n.id,
        },
      })),
    [baseNodes, chain, focus],
  )

  const edges: Edge[] = React.useMemo(
    () =>
      lineage.edges.map((e) => {
        const onChain = chain ? chain.has(e.source) && chain.has(e.target) : true
        return {
          id: e.id,
          source: e.source,
          target: e.target,
          type: "smoothstep",
          style: {
            stroke: onChain && chain ? "var(--ink)" : "var(--text-dim)",
            strokeWidth: onChain && chain ? 2 : 1.25,
            opacity: chain && !onChain ? 0.12 : 1,
          },
        }
      }),
    [lineage, chain],
  )

  return (
    <div className="h-full w-full">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.14 }}
        proOptions={{ hideAttribution: true }}
        minZoom={0.15}
        nodesDraggable={false}
        nodesConnectable={false}
        onNodeClick={(_, n) => setFocus((f) => (f === n.id ? null : n.id))}
        onPaneClick={() => setFocus(null)}
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="var(--line)" />
        <Controls showInteractive={false} className="!border-line" />
        {focus ? (
          <Panel position="top-right" className="!m-2">
            <button
              type="button"
              onClick={() => setFocus(null)}
              className="rounded-2xl border border-white/60 bg-white/35 px-3 py-1.5 text-xs font-medium text-foreground shadow-[0_8px_30px_rgba(20,20,20,0.12)] backdrop-blur-xl backdrop-saturate-150 hover:bg-white/55"
            >
              Clear focus
            </button>
          </Panel>
        ) : (
          <Panel position="top-right" className="!m-2">
            <span className="rounded-2xl border border-white/60 bg-white/35 px-2.5 py-1.5 text-[11px] text-text-dim shadow-[0_8px_30px_rgba(20,20,20,0.12)] backdrop-blur-xl backdrop-saturate-150">
              Click any node to trace its full lineage chain
            </span>
          </Panel>
        )}
      </ReactFlow>
    </div>
  )
}
