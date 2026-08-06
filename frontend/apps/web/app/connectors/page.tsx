"use client"

import { useQuery } from "@tanstack/react-query"

import { PageHeader } from "@/components/page-header"
import { listConnectors } from "@/lib/api"

/**
 * The connector registry.
 *
 * This list IS the safety story, so it is rendered in full rather than
 * summarised: every adapter, its mode, its READ-only scopes, and read_only:true
 * on each one. A reviewer should be able to see the guarantee, not be told it.
 */
export default function ConnectorsPage() {
  const connectors = useQuery({
    queryKey: ["connectors"],
    queryFn: ({ signal }) => listConnectors(signal),
  })

  const list = connectors.data?.connectors ?? []
  const allReadOnly =
    list.length > 0 && connectors.data?.read_only_count === connectors.data?.count

  return (
    <div className="mx-auto w-full max-w-6xl px-6 py-6">
      <PageHeader
        eyebrow="Evidence sources"
        title="Connector registry"
        description="Every system CHANAKYA can read evidence from. None of them can be written to - the Connector interface has exactly one data method, and no write method exists to implement."
      />

      {connectors.data && (
        <div
          className={
            "mt-4 rounded-md border px-4 py-3 " +
            (allReadOnly ? "border-ok/40 bg-ok/10" : "border-risk/40 bg-risk/10")
          }
        >
          <p className={"text-sm " + (allReadOnly ? "text-ok" : "text-risk")}>
            <span className="tnum font-semibold">
              {connectors.data.read_only_count}/{connectors.data.count}
            </span>{" "}
            connectors are read-only.
          </p>
          <p className="mt-1 text-xs text-fg-muted">{connectors.data.guarantee}</p>
        </div>
      )}

      <div className="mt-5 overflow-x-auto rounded-lg border border-line-subtle">
        <table className="w-full min-w-[52rem] text-left text-sm">
          <thead className="bg-elevated text-xs uppercase tracking-wide text-fg-muted">
            <tr>
              <th className="px-4 py-2 font-medium">Connector</th>
              <th className="px-4 py-2 font-medium">Mode</th>
              <th className="px-4 py-2 font-medium">Read only</th>
              <th className="px-4 py-2 font-medium">Scopes</th>
              <th className="px-4 py-2 font-medium">Rate limit</th>
              <th className="px-4 py-2 font-medium">Health</th>
            </tr>
          </thead>
          <tbody>
            {list.map((c) => (
              <tr key={c.id} className="border-t border-line-subtle align-top">
                <td className="px-4 py-2.5">
                  <div className="text-fg">{c.vendor}</div>
                  <div className="tnum text-xs text-fg-muted">{c.kind}</div>
                  <p className="mt-1 max-w-md text-xs text-fg-muted">{c.description}</p>
                </td>
                <td className="px-4 py-2.5">
                  <span className="rounded bg-elevated px-1.5 py-0.5 text-xs text-fg-muted">
                    {c.mode}
                  </span>
                </td>
                <td className="px-4 py-2.5">
                  <span
                    className={
                      "rounded px-1.5 py-0.5 text-xs " +
                      (c.read_only ? "bg-ok/15 text-ok" : "bg-risk/15 text-risk")
                    }
                  >
                    {String(c.read_only)}
                  </span>
                </td>
                <td className="px-4 py-2.5">
                  <div className="flex flex-wrap gap-1">
                    {c.scopes.map((s) => (
                      <span
                        key={s}
                        className="tnum rounded bg-elevated px-1.5 py-0.5 text-[11px] text-fg-muted"
                      >
                        {s}
                      </span>
                    ))}
                  </div>
                </td>
                <td className="tnum px-4 py-2.5 text-xs text-fg-muted">
                  {c.rate_limit.requests}/{c.rate_limit.per}
                </td>
                <td className="px-4 py-2.5">
                  <span
                    className={
                      "text-xs " + (c.health.ok === "ok" ? "text-ok" : "text-warn")
                    }
                  >
                    {c.health.ok}
                  </span>
                  <p className="mt-0.5 max-w-[16rem] text-[11px] text-fg-muted">
                    {c.health.detail}
                  </p>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {connectors.isLoading && (
        <p className="mt-4 text-sm text-fg-muted">Loading the registry...</p>
      )}

      {connectors.isError && (
        <div className="mt-4 rounded-md border border-risk/40 bg-risk/10 p-4 text-sm text-risk">
          <p className="font-medium">Failed to load connectors</p>
          <p className="mt-1 text-xs text-fg-muted">
            {(connectors.error as Error)?.message || "Could not connect to the CHANAKYA API server."}
          </p>
          <button
            onClick={() => connectors.refetch()}
            className="mt-3 rounded bg-risk/20 px-3 py-1 text-xs font-medium text-risk hover:bg-risk/30 cursor-pointer"
          >
            Retry
          </button>
        </div>
      )}
    </div>
  )
}
