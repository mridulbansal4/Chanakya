import type { ReactNode } from "react"

/**
 * PageHeader is the one editorial page-title pattern used across screens:
 * a small ALL-CAPS eyebrow, a large serif title, and a one-line description —
 * always in the same position, with optional right-aligned actions.
 */
export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow: string
  title: string
  description?: string
  actions?: ReactNode
}) {
  return (
    <div className="flex shrink-0 items-start justify-between gap-4 border-b border-line px-7 py-5">
      <div className="min-w-0">
        <div className="eyebrow mb-1.5">{eyebrow}</div>
        <h1 className="font-display text-3xl leading-tight tracking-tight">{title}</h1>
        {description && (
          <p className="mt-1.5 max-w-2xl text-[15px] text-text-dim">{description}</p>
        )}
      </div>
      {actions && <div className="shrink-0">{actions}</div>}
    </div>
  )
}
