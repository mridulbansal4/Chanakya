"use client"

import * as React from "react"

const FOCUSABLE =
  'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'

/**
 * useDialog makes a modal keyboard-operable and accessible: it focuses the first
 * control on open, traps Tab within the dialog, and closes on Escape. Attach the
 * returned ref to the dialog container (also set role="dialog" aria-modal).
 */
export function useDialog<T extends HTMLElement>(onClose: () => void) {
  const ref = React.useRef<T>(null)

  React.useEffect(() => {
    const el = ref.current
    if (!el) return

    const focusables = () =>
      Array.from(el.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
        (n) => n.offsetParent !== null,
      )

    // Focus the first control (or the container) when the dialog opens.
    const first = focusables()[0]
    ;(first ?? el).focus()

    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation()
        onClose()
        return
      }
      if (e.key !== "Tab") return
      const items = focusables()
      if (items.length === 0) return
      const firstEl = items[0]!
      const lastEl = items[items.length - 1]!
      if (e.shiftKey && document.activeElement === firstEl) {
        e.preventDefault()
        lastEl.focus()
      } else if (!e.shiftKey && document.activeElement === lastEl) {
        e.preventDefault()
        firstEl.focus()
      }
    }

    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [onClose])

  return ref
}
