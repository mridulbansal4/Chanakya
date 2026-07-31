import * as React from "react"
import { cn } from "@workspace/ui/lib/utils"

/**
 * Skeleton loading placeholder with refined shimmer & pulse effects.
 */
export function Skeleton({ className = "", ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      aria-hidden
      className={cn("animate-pulse rounded-lg bg-cream-200/80 dark:bg-ink-800/60", className)}
      {...props}
    />
  )
}

/** A skeleton for a loading table with customizable rows and columns. */
export function SkeletonRows({ rows = 6, cols = 5 }: { rows?: number; cols?: number }) {
  return (
    <div className="space-y-3 p-6">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-4">
          {Array.from({ length: cols }).map((_, j) => (
            <Skeleton key={j} className="h-8 flex-1" />
          ))}
        </div>
      ))}
    </div>
  )
}

/** Rich skeleton card placeholder with header, content lines, and action button. */
export function CardSkeleton({ className }: { className?: string }) {
  return (
    <div className={cn("hairline rounded-2xl bg-surface p-5 space-y-4 shadow-sm", className)}>
      <div className="flex items-center justify-between">
        <Skeleton className="h-4 w-28" />
        <Skeleton className="h-6 w-16 rounded-full" />
      </div>
      <Skeleton className="h-6 w-3/4" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-2/3" />
      <div className="pt-2 flex justify-end">
        <Skeleton className="h-9 w-24 rounded-xl" />
      </div>
    </div>
  )
}

/** Bloomberg terminal style metric card skeleton. */
export function MetricSkeleton() {
  return (
    <div className="bg-surface p-5 space-y-3 border-b border-line">
      <Skeleton className="h-3 w-24" />
      <Skeleton className="h-9 w-16" />
      <Skeleton className="h-3 w-32" />
    </div>
  )
}

/** Graph canvas loading skeleton. */
export function GraphSkeleton() {
  return (
    <div className="relative h-full w-full bg-cream/40 rounded-2xl border border-line/60 p-8 flex flex-col justify-between overflow-hidden">
      <div className="flex justify-between items-center">
        <Skeleton className="h-9 w-48 rounded-xl" />
        <Skeleton className="h-9 w-32 rounded-xl" />
      </div>
      <div className="grid grid-cols-3 gap-12 my-auto place-items-center">
        <Skeleton className="h-14 w-44 rounded-xl" />
        <Skeleton className="h-14 w-44 rounded-xl" />
        <Skeleton className="h-14 w-44 rounded-xl" />
      </div>
      <div className="flex justify-between items-center">
        <Skeleton className="h-8 w-36 rounded-lg" />
        <Skeleton className="h-8 w-24 rounded-lg" />
      </div>
    </div>
  )
}

/** Full page loader skeleton. */
export function PageSkeleton() {
  return (
    <div className="p-8 space-y-8 animate-in fade-in-50">
      <div className="space-y-2">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-8 w-72" />
        <Skeleton className="h-4 w-96" />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <MetricSkeleton key={i} />
        ))}
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {Array.from({ length: 3 }).map((_, i) => (
          <CardSkeleton key={i} />
        ))}
      </div>
    </div>
  )
}

