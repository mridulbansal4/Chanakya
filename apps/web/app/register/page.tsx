"use client"

import * as React from "react"
import { useQuery } from "@tanstack/react-query"
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"

import { useAsOf } from "@/components/as-of-provider"
import { ObligationDetailPanel } from "@/components/obligation-detail"
import { DeonticBadge, StatusBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { PageHeader } from "@/components/page-header"
import { SkeletonRows } from "@/components/skeleton"
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
    cell: (c) => <span className="tnum text-primary">{c.getValue<string>()}</span>,
  },
  {
    accessorKey: "deontic_type",
    header: "Obligation type",
    cell: (c) => <DeonticBadge deontic={c.getValue<DeonticType>()} />,
  },
  { accessorKey: "bearer", header: "Bearer" },
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
        <span title={raw} className="text-foreground">
          {formatDeadline(raw)}
        </span>
      ) : (
        <span className="text-muted-foreground">No deadline</span>
      )
    },
  },
  {
    accessorKey: "clause_heading",
    header: "Subject",
    cell: (c) => (
      <span className="text-muted-foreground">{c.getValue<string>()}</span>
    ),
  },
]

export default function RegisterPage() {
  const { asOf } = useAsOf()
  const [deontic, setDeontic] = React.useState<DeonticType | "">("")
  const [status, setStatus] = React.useState<ObligationStatus | "">("")
  const [selected, setSelected] = React.useState<string | null>(null)

  const query = useQuery({
    queryKey: ["obligations", asOf, deontic, status],
    queryFn: ({ signal }) =>
      listObligations({ asOf, deontic, status }, signal),
  })

  const data = React.useMemo(() => query.data?.obligations ?? [], [query.data])
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <div className="flex h-full">
      <div className="flex min-w-0 flex-1 flex-col">
        <PageHeader
          eyebrow="Register"
          title="Obligation register"
          description="Every obligation CHANAKYA extracted from the regulation, with its exact source. Click a row to inspect the citation."
        />
        {/* Filter bar */}
        <div className="flex items-center gap-2 border-b border-line px-6 py-2.5 text-sm">
          <Select value={deontic} onChange={(v) => setDeontic(v as DeonticType | "")} options={DEONTIC_OPTIONS} />
          <Select value={status} onChange={(v) => setStatus(v as ObligationStatus | "")} options={STATUS_OPTIONS} />
          <span className="tnum ml-auto text-xs text-muted-foreground">
            {query.data?.count ?? 0} obligations
          </span>
        </div>

        {/* Table */}
        <div className="min-h-0 flex-1 overflow-auto">
          {query.isError ? (
            <div className="p-6 text-sm text-risk">
              Couldn&apos;t reach the backend. Make sure the API is running on
              port 8080, then refresh.
            </div>
          ) : query.isLoading ? (
            <SkeletonRows rows={8} />
          ) : (
            <table className="w-full border-collapse text-sm">
              <thead className="sticky top-0 bg-surface">
                {table.getHeaderGroups().map((hg) => (
                  <tr key={hg.id} className="border-b border-line">
                    {hg.headers.map((h) => (
                      <th
                        key={h.id}
                        className="px-6 py-2 text-left text-[11px] font-medium tracking-wide text-muted-foreground uppercase"
                      >
                        {flexRender(h.column.columnDef.header, h.getContext())}
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
                      className={`cursor-pointer border-b border-line/60 transition-colors ${
                        active
                          ? "bg-surface-2"
                          : "odd:bg-surface/40 hover:bg-surface"
                      }`}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <td key={cell.id} className="px-6 py-2.5 align-top">
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      ))}
                    </tr>
                  )
                })}
                {!query.isLoading && data.length === 0 && (
                  <tr>
                    <td colSpan={columns.length} className="px-6 py-10 text-center text-sm text-muted-foreground">
                      {deontic || status
                        ? "No obligations match these filters. Try clearing them."
                        : `No obligations were in force as of ${asOf}. Pick a later date (top right).`}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* Detail panel */}
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
      className="hairline rounded-md bg-surface px-2.5 py-1.5 text-xs text-foreground outline-none [color-scheme:light]"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  )
}
