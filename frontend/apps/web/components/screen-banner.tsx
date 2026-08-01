"use client"

import * as React from "react"
import { Info, X } from "lucide-react"

/**
 * A dismissible one-line explanation of what a screen is for.
 *
 * Rendered as `null` until the dismissed state is read from localStorage,
 * which avoids a flash of the banner on every navigation for users who have
 * already dismissed it.
 *
 * It is deliberately not tinted. This is orientation copy, not a warning -
 * giving it an amber or blue wash would put it in the same visual class as
 * the alerts that actually need acting on.
 */
export function ScreenBanner({
  id,
  children,
}: {
  id: string
  children: React.ReactNode
}) {
  const key = `chanakya.banner.${id}`
  const [dismissed, setDismissed] = React.useState(true)

  React.useEffect(() => {
    setDismissed(window.localStorage.getItem(key) === "1")
  }, [key])

  if (dismissed) return null
  return (
    <div className="flex shrink-0 items-start gap-2.5 border-b border-line-subtle bg-raised px-6 py-2.5">
      <Info className="mt-0.5 size-3.5 shrink-0 text-fg-subtle" aria-hidden />
      <p className="flex-1 text-body-sm text-fg-muted">{children}</p>
      <button
        type="button"
        onClick={() => {
          window.localStorage.setItem(key, "1")
          setDismissed(true)
        }}
        /* -m-1.5 p-1.5 grows the hit area to 32px without changing layout. */
        className="-m-1.5 shrink-0 rounded p-1.5 text-fg-subtle transition-colors duration-[120ms] hover:bg-elevated hover:text-fg"
        aria-label="Dismiss this explanation"
      >
        <X className="size-3.5" aria-hidden />
      </button>
    </div>
  )
}
