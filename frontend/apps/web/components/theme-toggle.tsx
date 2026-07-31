"use client"

import * as React from "react"
import { Moon, Sun } from "lucide-react"
import { useTheme } from "next-themes"

export function ThemeToggle() {
  const { theme, setTheme } = useTheme()
  const [mounted, setMounted] = React.useState(false)

  React.useEffect(() => {
    setMounted(true)
  }, [])

  if (!mounted) {
    return <div className="size-8" />
  }

  const isDark = theme === "dark"

  return (
    <button
      onClick={() => setTheme(isDark ? "light" : "dark")}
      className="relative size-8 rounded-full border border-white/20 bg-white/10 flex items-center justify-center text-white shadow-inner transition-all hover:border-white/50 hover:bg-white/15 overflow-hidden focus:outline-none focus-visible:ring-2 focus-visible:ring-white/50"
      aria-label="Toggle theme"
    >
      <div 
        className="absolute inset-0 flex items-center justify-center transition-all duration-500 ease-[cubic-bezier(0.34,1.56,0.64,1)]"
        style={{
          transform: isDark ? 'scale(1) rotate(0deg)' : 'scale(0.3) rotate(-90deg)',
          opacity: isDark ? 1 : 0
        }}
      >
        <Moon className="size-4" strokeWidth={2.5} />
      </div>
      <div 
        className="absolute inset-0 flex items-center justify-center transition-all duration-500 ease-[cubic-bezier(0.34,1.56,0.64,1)]"
        style={{
          transform: !isDark ? 'scale(1) rotate(0deg)' : 'scale(0.3) rotate(90deg)',
          opacity: !isDark ? 1 : 0
        }}
      >
        <Sun className="size-4" strokeWidth={2.5} />
      </div>
    </button>
  )
}
