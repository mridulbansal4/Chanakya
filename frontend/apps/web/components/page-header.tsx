import type { ReactNode } from "react"

/**
 * The one page-title pattern used across every screen: an ALL-CAPS eyebrow
 * naming the domain, a serif title, and a single line of plain language
 * describing what the screen is for.
 *
 * The serif appears here and in dialog titles only. It is the app's one
 * editorial gesture - used sparingly it signals "this is a document of
 * record"; used inside tables and cards it would just be noise.
 *
 * The description is capped at ~68 characters per line. Measure matters
 * more than font choice for whether a paragraph actually gets read.
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
    <div className="flex flex-col sm:flex-row flex-wrap shrink-0 sm:items-start justify-between gap-6 border-b border-line-subtle px-7 py-6">
      <div className="min-w-0">
        <p className="eyebrow mb-2">{eyebrow}</p>
        <h1 className="text-display-sm text-fg">{title}</h1>
        {description && (
          <p className="mt-2 max-w-[68ch] text-body-md text-fg-muted">
            {description}
          </p>
        )}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  )
}
