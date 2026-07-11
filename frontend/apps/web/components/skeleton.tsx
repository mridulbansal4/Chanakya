/**
 * Skeleton is the loading placeholder — a soft cream shimmer. The global
 * prefers-reduced-motion rule disables the pulse for motion-sensitive users.
 */
export function Skeleton({ className = "" }: { className?: string }) {
  return (
    <div
      aria-hidden
      className={`animate-pulse rounded bg-cream-200 ${className}`}
    />
  )
}

/** A few skeleton rows for a loading table/list. */
export function SkeletonRows({ rows = 6 }: { rows?: number }) {
  return (
    <div className="space-y-2 p-6">
      {Array.from({ length: rows }).map((_, i) => (
        <Skeleton key={i} className="h-9 w-full" />
      ))}
    </div>
  )
}
