"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { AnimatePresence, motion } from "framer-motion"
import {
  BookOpen,
  CheckCircle2,
  CornerDownLeft,
  FileCheck,
  FileText,
  Home,
  Network,
  Radio,
  Search,
  ShieldCheck,
  Zap,
} from "lucide-react"

import {
  TRANSITION_MICRO,
  TRANSITION_STANDARD,
  overlayVariants,
  scrimVariants,
} from "@/lib/motion"
import { cn } from "@workspace/ui/lib/utils"

interface NavItem {
  id: string
  title: string
  subtitle: string
  href: string
  icon: React.ReactNode
  group: string
}

/**
 * Icons are monochrome and inherit from the row.
 *
 * The previous version gave each destination its own hue - emerald, cyan,
 * purple, amber - which meant nine colours carrying no information. In a
 * system where amber means "needs judgement" and red means "gap", spending
 * those same hues on decoration is what makes the real signals stop
 * registering.
 */
const ITEMS: NavItem[] = [
  {
    id: "overview",
    title: "Overview",
    subtitle: "Compliance posture and the live obligation graph",
    href: "/",
    icon: <Home className="size-4" aria-hidden />,
    group: "Monitor",
  },
  {
    id: "reg-feed",
    title: "Regulatory Feed",
    subtitle: "New SEBI circulars, detected and processed",
    href: "/regulatory-feed",
    icon: <Radio className="size-4" aria-hidden />,
    group: "Monitor",
  },
  {
    id: "register",
    title: "Obligation Register",
    subtitle: "Every obligation extracted, with its source clause",
    href: "/register",
    icon: <BookOpen className="size-4" aria-hidden />,
    group: "Monitor",
  },
  {
    id: "blast",
    title: "Blast Radius",
    subtitle: "Preview what a clause amendment would affect",
    href: "/amendments",
    icon: <Zap className="size-4" aria-hidden />,
    group: "Analyse",
  },
  {
    id: "evidence",
    title: "Evidence & Gaps",
    subtitle: "Coverage from connected systems, and what is missing",
    href: "/evidence",
    icon: <FileCheck className="size-4" aria-hidden />,
    group: "Analyse",
  },
  {
    id: "audit",
    title: "Audit Trail",
    subtitle: "Reconstruct the compliance trail as of any date",
    href: "/audit",
    icon: <Network className="size-4" aria-hidden />,
    group: "Analyse",
  },
  {
    id: "review",
    title: "Review Queue",
    subtitle: "Obligations awaiting your sign-off",
    href: "/review",
    icon: <ShieldCheck className="size-4" aria-hidden />,
    group: "Act",
  },
  {
    id: "policy",
    title: "Policy Engine",
    subtitle: "Turn obligations into automated checks",
    href: "/policy",
    icon: <CheckCircle2 className="size-4" aria-hidden />,
    group: "Act",
  },
  {
    id: "feed",
    title: "Machine-Readable Feed",
    subtitle: "The feed a regulator's systems consume",
    href: "/feed",
    icon: <FileText className="size-4" aria-hidden />,
    group: "Act",
  },
]

const LISTBOX_ID = "command-palette-listbox"

export function CommandPalette() {
  const [open, setOpen] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [activeIndex, setActiveIndex] = React.useState(0)
  const router = useRouter()

  const panelRef = React.useRef<HTMLDivElement>(null)
  const inputRef = React.useRef<HTMLInputElement>(null)
  const listRef = React.useRef<HTMLDivElement>(null)
  // Remembering the trigger lets us put focus back where the user left it.
  const restoreFocusRef = React.useRef<HTMLElement | null>(null)

  const openPalette = () => {
    restoreFocusRef.current = document.activeElement as HTMLElement | null
    setOpen(true)
  }

  const closePalette = React.useCallback(() => {
    setOpen(false)
    setQuery("")
    restoreFocusRef.current?.focus()
  }, [])

  React.useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault()
        if (open) closePalette()
        else openPalette()
      }
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [open, closePalette])

  // Lock the page behind the dialog. Without this the list underneath
  // scrolls when the palette's own list reaches its end.
  React.useEffect(() => {
    if (!open) return
    const previous = document.body.style.overflow
    document.body.style.overflow = "hidden"
    return () => {
      document.body.style.overflow = previous
    }
  }, [open])

  const filtered = React.useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return ITEMS
    return ITEMS.filter(
      (item) =>
        item.title.toLowerCase().includes(q) ||
        item.subtitle.toLowerCase().includes(q) ||
        item.group.toLowerCase().includes(q),
    )
  }, [query])

  React.useEffect(() => {
    setActiveIndex(0)
  }, [query])

  // Keep the highlighted row in view during keyboard navigation - otherwise
  // arrowing past the fold moves a selection the user cannot see.
  React.useEffect(() => {
    if (!open) return
    const el = listRef.current?.querySelector<HTMLElement>(
      `[data-index="${activeIndex}"]`,
    )
    el?.scrollIntoView({ block: "nearest" })
  }, [activeIndex, open])

  const selectItem = (item: NavItem) => {
    setOpen(false)
    setQuery("")
    router.push(item.href)
  }

  const onInputKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault()
      setActiveIndex((i) => (filtered.length ? (i + 1) % filtered.length : 0))
    } else if (e.key === "ArrowUp") {
      e.preventDefault()
      setActiveIndex((i) =>
        filtered.length ? (i - 1 + filtered.length) % filtered.length : 0,
      )
    } else if (e.key === "Home") {
      e.preventDefault()
      setActiveIndex(0)
    } else if (e.key === "End") {
      e.preventDefault()
      setActiveIndex(Math.max(0, filtered.length - 1))
    } else if (e.key === "Enter") {
      const item = filtered[activeIndex]
      if (item) {
        e.preventDefault()
        selectItem(item)
      }
    } else if (e.key === "Escape") {
      e.preventDefault()
      closePalette()
    }
  }

  // Focus trap. The palette has exactly one focusable control (the input),
  // so containment is simply: any Tab returns focus to it.
  const onPanelKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Tab") {
      e.preventDefault()
      inputRef.current?.focus()
    }
  }

  let renderedGroup: string | null = null

  return (
    <>
      <button
        type="button"
        onClick={openPalette}
        className="shiny-cta !py-1 !px-3 !text-xs font-medium !rounded-full shadow-sm md:inline-flex hidden"
        aria-label="Search and navigate. Keyboard shortcut: Control or Command K"
      >
        <span>
          <Search className="size-3.5 text-accent" aria-hidden />
          <span className="text-fg">Search</span>
          <kbd className="rounded border border-line-subtle bg-elevated px-1.5 py-0.5 font-mono text-[10px] text-fg-subtle">
            ⌘K
          </kbd>
        </span>
      </button>

      <AnimatePresence>
        {open && (
          <div className="fixed inset-0 z-50 flex items-start justify-center px-4 pt-[12vh]">
            <motion.div
              variants={scrimVariants}
              initial="hidden"
              animate="visible"
              exit="exit"
              transition={TRANSITION_MICRO}
              onClick={closePalette}
              className="absolute inset-0 bg-scrim"
              aria-hidden
            />

            <motion.div
              ref={panelRef}
              role="dialog"
              aria-modal="true"
              aria-label="Command palette"
              onKeyDown={onPanelKeyDown}
              variants={overlayVariants}
              initial="hidden"
              animate="visible"
              exit="exit"
              transition={TRANSITION_STANDARD}
              className="relative w-full max-w-xl overflow-hidden rounded-lg border border-line bg-overlay shadow-elev-3"
            >
              <div className="flex items-center gap-3 border-b border-line-subtle px-4 py-3">
                <Search className="size-4 shrink-0 text-fg-subtle" aria-hidden />
                <input
                  ref={inputRef}
                  autoFocus
                  type="text"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={onInputKeyDown}
                  placeholder="Search destinations…"
                  className="w-full bg-transparent text-body-lg text-fg outline-none placeholder:text-fg-faint"
                  role="combobox"
                  aria-expanded="true"
                  aria-controls={LISTBOX_ID}
                  aria-autocomplete="list"
                  aria-activedescendant={
                    filtered[activeIndex] ? `cmd-${filtered[activeIndex].id}` : undefined
                  }
                />
                <kbd className="shrink-0 rounded border border-line bg-elevated px-1.5 py-0.5 font-mono text-[10px] text-fg-subtle">
                  Esc
                </kbd>
              </div>

              {/* Announces result counts to screen readers as the user types. */}
              <p className="sr-only" role="status" aria-live="polite">
                {filtered.length} result{filtered.length === 1 ? "" : "s"}
              </p>

              <div
                ref={listRef}
                id={LISTBOX_ID}
                role="listbox"
                aria-label="Destinations"
                className="max-h-[22rem] overflow-y-auto p-1.5"
              >
                {filtered.length === 0 ? (
                  <p className="px-4 py-10 text-center text-body-md text-fg-muted">
                    Nothing matches “{query}”.
                  </p>
                ) : (
                  filtered.map((item, idx) => {
                    const active = idx === activeIndex
                    const showGroup = item.group !== renderedGroup
                    renderedGroup = item.group
                    return (
                      <React.Fragment key={item.id}>
                        {showGroup && (
                          <p className="eyebrow px-3 pb-1.5 pt-3 first:pt-1.5">
                            {item.group}
                          </p>
                        )}
                        <div
                          id={`cmd-${item.id}`}
                          role="option"
                          aria-selected={active}
                          data-index={idx}
                          tabIndex={-1}
                          onClick={() => selectItem(item)}
                          onMouseMove={() => setActiveIndex(idx)}
                          className={cn(
                            "flex cursor-pointer items-center gap-3 rounded px-3 py-2.5",
                            "transition-colors duration-[120ms] ease-[cubic-bezier(0.2,0.8,0.2,1)]",
                            active ? "bg-elevated text-fg" : "text-fg-muted",
                          )}
                        >
                          <span
                            className={cn(
                              "shrink-0",
                              active ? "text-accent" : "text-fg-subtle",
                            )}
                          >
                            {item.icon}
                          </span>
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-title-md text-fg">
                              {item.title}
                            </span>
                            <span className="block truncate text-body-sm text-fg-muted">
                              {item.subtitle}
                            </span>
                          </span>
                          {active && (
                            <CornerDownLeft
                              className="size-3.5 shrink-0 text-fg-subtle"
                              aria-hidden
                            />
                          )}
                        </div>
                      </React.Fragment>
                    )
                  })
                )}
              </div>

              <div className="flex items-center gap-4 border-t border-line-subtle px-4 py-2 text-label-md text-fg-subtle">
                <span>↑↓ Navigate</span>
                <span>↵ Open</span>
                <span>Esc Close</span>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </>
  )
}
