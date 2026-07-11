"use client"

import { useQuery } from "@tanstack/react-query"

import { getHealth, type HealthResponse } from "@/lib/api"

type Posture = "up" | "degraded" | "down" | "checking"

function posture(
  data: HealthResponse | undefined,
  isError: boolean,
  isLoading: boolean,
): Posture {
  if (isLoading && !data) return "checking"
  if (isError || !data) return "down"
  if (data.status === "ok" && data.checks.database.ok) return "up"
  return "degraded"
}

const LABEL: Record<Posture, string> = {
  up: "Backend online",
  degraded: "Backend degraded",
  down: "Backend offline",
  checking: "Checking…",
}

const DOT: Record<Posture, string> = {
  up: "bg-verified",
  degraded: "bg-warn",
  down: "bg-danger",
  checking: "bg-muted-foreground",
}

/**
 * HealthIndicator polls GET /health every 5s and renders a hairline status
 * pill: teal = online, amber = degraded (backend up, DB failing), red =
 * offline, dim = checking. The version is shown in tabular mono.
 */
export function HealthIndicator() {
  const { data, isError, isLoading } = useQuery({
    queryKey: ["health"],
    queryFn: ({ signal }) => getHealth(signal),
    refetchInterval: 5_000,
    retry: false,
  })

  const p = posture(data, isError, isLoading)

  return (
    <div className="hairline inline-flex items-center gap-2 rounded-md bg-surface px-3 py-1.5 text-xs">
      <span className="relative flex h-2 w-2">
        <span className={`relative inline-flex h-2 w-2 rounded-full ${DOT[p]}`} />
      </span>
      <span className="text-foreground">{LABEL[p]}</span>
      {data?.version && (
        <span className="tnum text-muted-foreground">v{data.version}</span>
      )}
    </div>
  )
}
