"use client"

import * as React from "react"
import { useQuery } from "@tanstack/react-query"
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type SortingState,
} from "@tanstack/react-table"
import { ArrowUpDown, ArrowUp, ArrowDown, Search, Filter } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { ObligationDetailPanel } from "@/components/obligation-detail"
import { DeonticBadge, StatusBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { PageHeader } from "@/components/page-header"
import { SkeletonRows } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import { formatDeadline } from "@/lib/format"
import {
  listObligations,
  type DeonticType,
  type Obligation,
  type ObligationStatus,
} from "@/lib/api"

const DEONTIC_OPTIONS: Array<{ value: DeonticType | ""; label: string }> = [
  { value: "", label: "All types" },
  { value: "MUST", label: "Required (MUST)" },
  { value: "MUST_NOT", label: "Prohibited (MUST NOT)" },
  { value: "MAY", label: "Permitted (MAY)" },
]

const STATUS_OPTIONS: Array<{ value: ObligationStatus | ""; label: string }> = [
  { value: "", label: "All statuses" },
  { value: "pending", label: "Pending" },
  { value: "needs_review", label: "Needs review" },
  { value: "approved", label: "Approved" },
  { value: "rejected", label: "Rejected" },
]

const columns: ColumnDef<Obligation>[] = [
  {
    accessorKey: "clause_ref",
    header: "Clause",
    cell: (c) => <span className="tnum font-semibold text-primary">{c.getValue<string>()}</span>,
  },
  {
    accessorKey: "deontic_type",
    header: "Obligation type",
    cell: (c) => <DeonticBadge deontic={c.getValue<DeonticType>()} />,
  },
  {
    accessorKey: "bearer",
    header: "Bearer",
    cell: (c) => <span className="font-medium text-foreground">{c.getValue<string>()}</span>,
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: (c) => <StatusBadge status={c.getValue<ObligationStatus>()} />,
  },
  {
    accessorKey: "confidence",
    header: "AI confidence",
    cell: (c) => <ConfidenceMeter value={c.getValue<number>()} />,
  },
  {
    accessorKey: "deadline",
    header: "Deadline",
    cell: (c) => {
      const raw = c.getValue<string>()
      return raw ? (
        <span title={raw} className="tnum font-medium text-foreground">
          {formatDeadline(raw)}
        </span>
      ) : (
        <span className="text-muted-foreground italic text-xs">No deadline</span>
      )
    },
  },
  {
    accessorKey: "clause_heading",
    header: "Subject",
    cell: (c) => (
      <span className="text-text-dim line-clamp-1 max-w-xs">{c.getValue<string>()}</span>
    ),
  },
]

export default function RegisterPage() {
  const { asOf } = useAsOf()
  const [deontic, setDeontic] = React.useState<DeonticType | "">("")
  const [status, setStatus] = React.useState<ObligationStatus | "">("")
  const [search, setSearch] = React.useState("")
  const [selected, setSelected] = React.useState<string | null>(null)
  const [sorting, setSorting] = React.useState<SortingState>([])

  const query = useQuery({
    queryKey: ["obligations", asOf, deontic, status],
    queryFn: ({ signal }) => listObligations({ asOf, deontic, status }, signal),
  })

  const rawData = React.useMemo(() => query.data?.obligations ?? [], [query.data])

  const filteredData = React.useMemo(() => {
    if (!search.trim()) return rawData
    const q = search.toLowerCase()
    return rawData.filter(
      (o) =>
        o.clause_ref.toLowerCase().includes(q) ||
        o.clause_heading.toLowerCase().includes(q) ||
        o.bearer.toLowerCase().includes(q) ||
        o.source_sentence.toLowerCase().includes(q)
    )
  }, [rawData, search])

  const table = useReactTable({
    data: filteredData,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })

  return (
    <div className="flex h-full">
      <div className="flex min-w-0 flex-1 flex-col">
        <PageHeader
          eyebrow="Register"
          title="Obligation Register"
          description="Every obligation CHANAKYA extracted from the regulation with complete source provenance and AI extraction confidence scores."
        />

        {/* Enhanced Filter Bar */}
        <div className="flex flex-wrap items-center gap-3 border-b border-line bg-surface/80 backdrop-blur-md px-7 py-3 text-sm z-10">
          <div className="relative flex-1 max-w-xs">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-text-dim" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search clause, subject, sentence…"
              className="w-full rounded-xl border border-line bg-background pl-9 pr-3 py-1.5 text-xs text-foreground placeholder:text-text-dim outline-none focus:border-foreground/40 transition-colors"
            />
          </div>

          <div className="flex items-center gap-2">
            <Filter className="size-3.5 text-text-dim" />
            <Select value={deontic} onChange={(v) => setDeontic(v as DeonticType | "")} options={DEONTIC_OPTIONS} />
            <Select value={status} onChange={(v) => setStatus(v as ObligationStatus | "")} options={STATUS_OPTIONS} />
          </div>

          <span className="tnum ml-auto text-xs font-mono text-text-dim">
            {filteredData.length} of {query.data?.count ?? 0} obligations
          </span>
        </div>

        {/* Table Container */}
        <div className="min-h-0 flex-1 overflow-auto">
          {query.isError ? (
            <EmptyState
              icon="alert"
              title="Backend Unreachable"
              description="Could not connect to the CHANAKYA API server. Make sure the backend is running on port 8080."
              primaryAction={{
                label: "Retry Connection",
                onClick: () => query.refetch(),
              }}
            />
          ) : query.isLoading ? (
            <SkeletonRows rows={10} cols={7} />
          ) : filteredData.length === 0 ? (
            <EmptyState
              icon="search"
              title="No Obligations Found"
              description={
                search || deontic || status
                  ? "No obligations match your current search and filter criteria."
                  : `No obligations were in force as of ${asOf}.`
              }
              primaryAction={
                search || deontic || status
                  ? {
                      label: "Clear Filters",
                      onClick: () => {
                        setSearch("")
                        setDeontic("")
                        setStatus("")
                      },
                    }
                  : undefined
              }
            />
          ) : (
            <table className="w-full border-collapse text-sm">
              <thead className="sticky top-0 bg-surface/95 backdrop-blur-md shadow-xs z-10">
                {table.getHeaderGroups().map((hg) => (
                  <tr key={hg.id} className="border-b border-line">
                    {hg.headers.map((h) => (
                      <th
                        key={h.id}
                        onClick={h.column.getToggleSortingHandler()}
                        className="px-7 py-3 text-left text-xs font-semibold tracking-wider text-muted-foreground uppercase cursor-pointer select-none hover:text-foreground transition-colors"
                      >
                        <div className="flex items-center gap-1.5">
                          {flexRender(h.column.columnDef.header, h.getContext())}
                          {h.column.getIsSorted() === "asc" ? (
                            <ArrowUp className="size-3.5 text-primary" />
                          ) : h.column.getIsSorted() === "desc" ? (
                            <ArrowDown className="size-3.5 text-primary" />
                          ) : (
                            <ArrowUpDown className="size-3.5 text-text-dim/40 group-hover:text-foreground" />
                          )}
                        </div>
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody>
                {table.getRowModel().rows.map((row) => {
                  const active = row.original.id === selected
                  return (
                    <tr
                      key={row.id}
                      onClick={() => setSelected(row.original.id)}
                      className={`cursor-pointer border-b border-line/50 transition-all duration-150 ${
                        active
                          ? "bg-cream-200/80 shadow-xs border-foreground/20 font-medium"
                          : "odd:bg-surface/30 hover:bg-cream-200/50"
                      }`}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <td key={cell.id} className="px-7 py-3.5 align-middle">
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      ))}
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Detail Panel */}
      {selected && (
        <ObligationDetailPanel
          id={selected}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  )
}

function Select({
  value,
  onChange,
  options,
}: {
  value: string
  onChange: (v: string) => void
  options: Array<{ value: string; label: string }>
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="rounded-xl border border-line bg-background px-3 py-1.5 text-xs text-foreground outline-none focus:border-foreground/40 transition-colors"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  )
}
