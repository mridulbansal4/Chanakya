"use client"

import * as React from "react"
import { Moon, Sun } from "lucide-react"
import { useTheme } from "next-themes"

/**
 * Dark/light switch for the app chrome.
 *
 * The two glyphs are stacked and cross-faded rather than swapped, so the
 * control never reflows the header row mid-transition. Until `next-themes`
 * has read the stored preference on the client a same-size placeholder holds
 * the slot — rendering the wrong icon first and correcting it is a visible
 * flicker on every page load.
 */
export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme()
  const [mounted, setMounted] = React.useState(false)

  React.useEffect(() => {
    setMounted(true)
  }, [])

  if (!mounted) {
    return <div className="size-8 shrink-0" aria-hidden />
  }

  const isDark = resolvedTheme === "dark"
  const label = isDark ? "Switch to light theme" : "Switch to dark theme"

  return (
    <button
      type="button"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      aria-label={label}
      title={label}
      className="relative grid size-8 shrink-0 place-items-center overflow-hidden rounded-full border border-line-subtle bg-elevated text-fg-muted transition-colors duration-[120ms] hover:border-line hover:text-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
    >
      <span
        aria-hidden
        className="absolute inset-0 grid place-items-center transition-all duration-[260ms] ease-[cubic-bezier(0.2,0.8,0.2,1)]"
        style={{
          transform: isDark ? "scale(1) rotate(0deg)" : "scale(0.4) rotate(-90deg)",
          opacity: isDark ? 1 : 0,
        }}
      >
        <Moon className="size-4" />
      </span>
      <span
        aria-hidden
        className="absolute inset-0 grid place-items-center transition-all duration-[260ms] ease-[cubic-bezier(0.2,0.8,0.2,1)]"
        style={{
          transform: isDark ? "scale(0.4) rotate(90deg)" : "scale(1) rotate(0deg)",
          opacity: isDark ? 0 : 1,
        }}
      >
        <Sun className="size-4" />
      </span>
    </button>
  )
}
