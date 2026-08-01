import * as React from "react"
import { cn } from "@workspace/ui/lib/utils"
import { AiLoader } from "@/components/ui/ai-loader"

/**
 * Loading placeholders.
 *
 * Each skeleton mirrors the geometry of the content it replaces, so the
 * layout does not shift when real data arrives. That is the entire point -
 * a skeleton that is the wrong size is worse than a spinner, because it
 * promises a shape and then breaks it.
 *
 * The pulse is a low-amplitude opacity cycle rather than a sweeping
 * gradient shimmer; a shimmer animates a large painted area continuously
 * and, on a screen with twenty of them, is measurable on the main thread.
 */
export function Skeleton({
  className = "",
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      aria-hidden
      className={cn("animate-pulse rounded bg-elevated", className)}
      {...props}
    />
  )
}

/** Wraps a loading region so assistive tech announces the wait. */
export function LoadingRegion({
  label,
  children,
  className,
}: {
  label: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <div role="status" aria-busy="true" aria-live="polite" className={className}>
      <span className="sr-only">{label}</span>
      {children}
    </div>
  )
}

export function SkeletonRows({ rows = 6, cols = 5 }: { rows?: number; cols?: number }) {
  return (
    <LoadingRegion label="Loading rows" className="space-y-3 p-6">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-4">
          {Array.from({ length: cols }).map((_, j) => (
            <Skeleton key={j} className="h-8 flex-1" />
          ))}
        </div>
      ))}
    </LoadingRegion>
  )
}

export function CardSkeleton({ className }: { className?: string }) {
  return (
    <div className={cn("surface space-y-4 p-5", className)}>
      <div className="flex items-center justify-between">
        <Skeleton className="h-3.5 w-28" />
        <Skeleton className="h-5 w-16 rounded-full" />
      </div>
      <Skeleton className="h-5 w-3/4" />
      <Skeleton className="h-3.5 w-full" />
      <Skeleton className="h-3.5 w-2/3" />
      <div className="flex justify-end pt-1">
        <Skeleton className="h-9 w-24 rounded-md" />
      </div>
    </div>
  )
}

/**
 * Matches the KPI cell geometry in the posture strip exactly - same
 * padding, same label height, same 2rem metric height - so the strip does
 * not reflow when the numbers land.
 */
export function MetricSkeleton() {
  return (
    <div className="bg-raised px-6 py-5">
      <Skeleton className="h-3 w-28" />
      <Skeleton className="mt-3.5 h-8 w-14" />
      <Skeleton className="mt-3 h-3 w-36" />
    </div>
  )
}

export function GraphSkeleton() {
  return (
    <LoadingRegion
      label="Loading graph"
      className="relative flex h-full w-full flex-col items-center justify-center overflow-hidden bg-sunken p-8"
    >
      <AiLoader text="Generating Graph" />
    </LoadingRegion>
  )
}

export function PageSkeleton() {
  return (
    <LoadingRegion label="Loading page" className="space-y-8 p-8">
      <div className="space-y-2.5">
        <Skeleton className="h-3 w-32" />
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-4 w-96" />
      </div>
      <div className="grid grid-cols-1 gap-px bg-line-subtle md:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <MetricSkeleton key={i} />
        ))}
      </div>
      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <CardSkeleton key={i} />
        ))}
      </div>
    </LoadingRegion>
  )
}
