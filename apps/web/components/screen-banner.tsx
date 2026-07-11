"use client"

import * as React from "react"
import { Info, X } from "lucide-react"

/**
 * ScreenBanner is a dismissible one-line, plain-language explanation of what a
 * screen is for and what to do next. Dismissal is remembered per screen id in
 * localStorage. It renders nothing until we know the dismissed state (avoids a
 * hydration flash).
 */
export function ScreenBanner({ id, children }: { id: string; children: React.ReactNode }) {
  const key = `chanakya.banner.${id}`
  const [dismissed, setDismissed] = React.useState(true)

  React.useEffect(() => {
    setDismissed(window.localStorage.getItem(key) === "1")
  }, [key])

  if (dismissed) return null
  return (
    <div className="flex items-start gap-2 border-b border-line bg-surface px-6 py-2 text-xs text-muted-foreground">
      <Info className="mt-0.5 size-3.5 shrink-0 text-primary" aria-hidden />
      <p className="flex-1 leading-relaxed">{children}</p>
      <button
        type="button"
        onClick={() => {
          window.localStorage.setItem(key, "1")
          setDismissed(true)
        }}
        className="shrink-0 text-muted-foreground hover:text-foreground"
        aria-label="Dismiss"
      >
        <X className="size-3.5" />
      </button>
    </div>
  )
}
