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

import { GRAPH, lineageDot } from "@/lib/graph-theme"
import type { Lineage, LineageNode, LineageNodeType } from "@/lib/api"

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

const COL_GAP = 280
const ROW_GAP = 86

const typeDot = lineageDot

interface CardData extends LineageNode {
  dim: boolean
  focused: boolean
}

function LineageNodeCard({ data }: NodeProps) {
  const d = data as unknown as CardData
  return (
    <div
      className={`w-[200px] rounded-2xl border bg-raised px-4 py-3 text-xs shadow-elev-2 transition-all duration-200 hover:-translate-y-0.5 ${
        d.focused
          ? "border-accent ring-2 ring-accent/40 opacity-100 scale-105"
          : d.dim
          ? "border-line-subtle opacity-20"
          : "border-line-subtle hover:border-accent-line opacity-100"
      }`}
    >
      <Handle type="target" position={Position.Left} className="!w-1.5 !h-1.5 !border-none" style={{ background: GRAPH.handle }} />
      <div className="flex items-center gap-2">
        <span
          className="inline-block size-2.5 shrink-0 rounded-full"
          style={{ background: typeDot(d.type) }}
        />
        <span className="tnum truncate font-bold text-fg" title={d.label}>
          {d.ref ?? d.label}
        </span>
      </div>
      {d.sublabel && (
        <div
          title={d.sublabel}
          className="mt-1 line-clamp-2 text-[11px] leading-snug text-fg-muted"
        >
          {d.sublabel}
        </div>
      )}
      <Handle type="source" position={Position.Right} className="!w-1.5 !h-1.5 !border-none" style={{ background: GRAPH.handle }} />
    </div>
  )
}

const nodeTypes = { lineage: LineageNodeCard }

function layout(lin: Lineage): Node[] {
  const cols: LineageNode[][] = COLS.map(() => [])
  for (const n of lin.nodes) cols[LAYER[n.type] ?? 0]!.push(n)

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
        draggable: true,
      })
    })
  })
  return nodes
}

export function LineageGraph({ lineage }: { lineage: Lineage }) {
  const [focus, setFocus] = React.useState<string | null>(null)

  const baseNodes = React.useMemo(() => layout(lineage), [lineage])

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
          animated: onChain && !!chain,
          style: {
            stroke: onChain && chain ? GRAPH.edgeProvenance : GRAPH.edgeDefault,
            strokeWidth: onChain && chain ? 2.5 : 1.25,
            opacity: chain && !onChain ? 0.15 : 1,
          },
        }
      }),
    [lineage, chain],
  )

  return (
    <div className="h-full w-full relative" style={{ background: GRAPH.canvas }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.14 }}
        proOptions={{ hideAttribution: true }}
        minZoom={0.15}
        nodesDraggable={true}
        nodesConnectable={false}
        onNodeClick={(_, n) => setFocus((f) => (f === n.id ? null : n.id))}
        onPaneClick={() => setFocus(null)}
      >
        <Background variant={BackgroundVariant.Dots} gap={24} size={1.2} color={GRAPH.dots} />
        <Controls showInteractive={false} className="!border-line-subtle !shadow-elev-2" />
        {focus ? (
          <Panel position="top-right" className="!m-3">
            <button
              type="button"
              onClick={() => setFocus(null)}
              className="rounded-xl border border-accent-line bg-accent-solid px-4 py-2 text-xs font-bold text-accent-on shadow-elev-1 transition-colors hover:bg-accent-hover"
            >
              Clear focus
            </button>
          </Panel>
        ) : (
          <Panel position="top-right" className="!m-3">
            <span className="rounded-xl border border-line-subtle bg-overlay/90 px-4 py-2 text-xs font-medium text-fg-muted shadow-elev-2 backdrop-blur-2xl">
              Click any node to trace its full lineage chain
            </span>
          </Panel>
        )}
      </ReactFlow>
    </div>
  )
}
