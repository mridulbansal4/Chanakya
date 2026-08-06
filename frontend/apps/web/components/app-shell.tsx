"use client"

import * as React from "react"
import type { ReactNode } from "react"
import { createPortal } from "react-dom"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import { motion, useReducedMotion, AnimatePresence } from "framer-motion"
import { ChevronDown, Zap, ShieldCheck, Code2, History, FileCode } from "lucide-react"

import { AsOfControl } from "@/components/as-of-control"
import { HelpMenu } from "@/components/help-menu"
import { ScreenBanner } from "@/components/screen-banner"
import { WelcomeModal } from "@/components/welcome-modal"
import { GlossaryModal } from "@/components/glossary-modal"
import { CommandPalette } from "@/components/command-palette"
import { ThemeToggle } from "@/components/theme-toggle"
import { PageTransition } from "@/components/page-transition"
import { useAsOf } from "@/components/as-of-provider"
import { getReviewQueue } from "@/lib/api"
import { WaveText } from "@/components/ui/wave-text"
import { SPRING_LAYOUT } from "@/lib/motion"
import { cn } from "@workspace/ui/lib/utils"

const OFFICER = {
  name: "Priya Menon",
  role: "Compliance Officer",
  firm: "Acme Investment Advisers",
}

const PRIMARY_NAV = [
  { href: "/", label: "Overview", hint: "Your compliance dashboard at a glance." },
  { href: "/ingest", label: "Ingest", hint: "Upload a SEBI circular and watch the pipeline run." },
  { href: "/regulatory-feed", label: "Feed", hint: "Every circular in the corpus and clause diffs." },
  { href: "/workflows", label: "Workflows", hint: "Draft task DAGs with named owners." },
  { href: "/connectors", label: "Connectors", hint: "Read-only evidence connectors." },
  { href: "/enterprise", label: "Enterprise", hint: "Your firm as queryable data." },
  { href: "/register", label: "Register", hint: "Obligations extracted from regulations." },
  { href: "/review", label: "Review Queue", hint: "Obligations awaiting human sign-off." },
]

const MORE_NAV = [
  { href: "/amendments", label: "Blast Radius", hint: "Compute impact of clause amendments.", icon: Zap },
  { href: "/evidence", label: "Evidence & Gaps", hint: "Evidence coverage and remediation tickets.", icon: ShieldCheck },
  { href: "/policy", label: "Policy", hint: "Compile signed obligations to Rego rules.", icon: Code2 },
  { href: "/audit", label: "Audit", hint: "Reconstruct full compliance trail as of any date.", icon: History },
  { href: "/feed", label: "Regulator Feed API", hint: "Machine-readable feed for regulators.", icon: FileCode },
]

const BANNER: Record<string, ReactNode> = {
  "/": "Your compliance posture at a glance. Start with what needs your attention, then explore the live obligation graph below.",
  "/ingest": "Upload a SEBI circular. CHANAKYA parses it, extracts obligations with verbatim citations, and stops - nothing enters the graph until you approve it.",
  "/regulatory-feed": "Every circular CHANAKYA holds, how each one arrived, what it supersedes, and the clause-level diff an amendment actually applied. All of it queried live - nothing here is scripted.",
  "/workflows": "Approved obligations become draft task DAGs with real named owners. Nothing is ever dispatched: CHANAKYA drafts the work, a person performs it.",
  "/connectors": "Every connector CHANAKYA can read evidence through. All read-only - not by policy, but because the interface has no write method to implement.",
  "/enterprise": "Your firm as queryable data, reconstructed as of any date. Every gap shown was found by traversing this graph - none of it is written in as a known problem.",
  "/register": "Every obligation CHANAKYA extracted from the regulation, with the exact source text behind each one. Click a row to see its citation.",
  "/amendments": "Preview what a regulation change would affect before you accept it. Pick a clause, edit its text, and compute the impact.",
  "/evidence": "Which obligations are backed by evidence from your firm's systems, and where the gaps are. Each gap becomes a draft remediation ticket.",
  "/review": "These obligations need your judgement before CHANAKYA can act on them. Approve, correct, or reject each one.",
  "/policy": "Turn a signed obligation into an automated compliance check, then test it against your firm's data. Enforcement stays in 'audit' until you promote it.",
  "/audit": "Reconstruct the full compliance trail - clause to obligation to sign-off - as of any date. Change the date (top right) to travel through time.",
  "/feed": "The machine-readable feed a regulator's systems consume, with the source text and cryptographic sign-off behind every obligation.",
}

function bannerFor(pathname: string): { id: string; text: ReactNode } | null {
  const key = pathname === "/" ? "/" : "/" + (pathname.split("/")[1] ?? "")
  const text = BANNER[key]
  return text ? { id: key, text } : null
}

export function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const { asOf } = useAsOf()
  const reduce = useReducedMotion()

  const [moreOpen, setMoreOpen] = React.useState(false)
  const [coords, setCoords] = React.useState<{ top: number; right: number } | null>(null)
  const buttonRef = React.useRef<HTMLButtonElement>(null)
  const menuRef = React.useRef<HTMLDivElement>(null)

  const toggleMore = () => {
    if (!moreOpen && buttonRef.current) {
      const rect = buttonRef.current.getBoundingClientRect()
      setCoords({
        top: rect.bottom + 6,
        right: window.innerWidth - rect.right,
      })
      setMoreOpen(true)
    } else {
      setMoreOpen(false)
    }
  }

  React.useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (
        moreOpen &&
        buttonRef.current &&
        !buttonRef.current.contains(e.target as Node) &&
        menuRef.current &&
        !menuRef.current.contains(e.target as Node)
      ) {
        setMoreOpen(false)
      }
    }
    document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [moreOpen])

  const reviewCount = useQuery({
    queryKey: ["review-queue", asOf],
    queryFn: ({ signal }) => getReviewQueue(asOf, signal),
    select: (d) => d.count,
  })

  const [welcomeOpen, setWelcomeOpen] = React.useState(false)
  const [glossaryOpen, setGlossaryOpen] = React.useState(false)
  React.useEffect(() => {
    if (window.localStorage.getItem("chanakya.welcomed") !== "1") {
      setWelcomeOpen(true)
    }
  }, [])
  const closeWelcome = () => {
    window.localStorage.setItem("chanakya.welcomed", "1")
    setWelcomeOpen(false)
  }

  const banner = bannerFor(pathname)
  const pending = reviewCount.data ?? 0
  const isMoreActive = MORE_NAV.some((item) =>
    item.href === "/" ? pathname === "/" : pathname.startsWith(item.href)
  )

  return (
    <div className="flex h-svh flex-col overflow-hidden bg-canvas text-fg">
      <header className="z-30 relative flex h-14 shrink-0 items-center justify-between border-b border-line-subtle bg-sunken/90 backdrop-blur-md px-3 lg:px-4 shadow-elev-2 gap-2">
        {/* Glowing bottom border accent */}
        <div className="absolute inset-x-0 bottom-0 h-[1.5px] bg-gradient-to-r from-transparent via-accent to-transparent opacity-80 pointer-events-none" />

        {/* Logo */}
        <Link
          href="/"
          className="flex shrink-0 items-center gap-2 rounded transition-opacity hover:opacity-90 pr-3 border-r border-line-subtle/50"
          aria-label="CHANAKYA - go to overview"
        >
          <img
            src="/logo-dark.png"
            alt="CHANAKYA"
            className="hidden dark:block h-6 w-auto object-contain"
          />
          <img
            src="/logo-light.png"
            alt="CHANAKYA"
            className="block dark:hidden h-6 w-auto object-contain"
          />
          <span className="text-sm tracking-tight font-bold text-fg">CHANAKYA</span>
        </Link>

        {/* Navigation Bar */}
        <nav
          aria-label="Primary"
          className="no-scrollbar flex min-w-0 flex-1 items-center gap-1 overflow-x-auto py-1"
        >
          {PRIMARY_NAV.map((item) => {
            const active =
              item.href === "/" ? pathname === "/" : pathname.startsWith(item.href)
            const badge = item.href === "/review" && pending > 0 ? pending : null

            return (
              <Link
                key={item.href}
                href={item.href}
                title={item.hint}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative inline-flex h-8 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full px-3 text-xs font-medium tracking-tight border transition-all duration-200",
                  active
                    ? "text-fg font-semibold border-line-strong dark:border-white/25 shadow-xs"
                    : "text-fg-muted border-transparent hover:text-fg hover:bg-elevated hover:border-line-subtle"
                )}
              >
                {active && (
                  <motion.span
                    layoutId={reduce ? undefined : "nav-active"}
                    aria-hidden
                    className="absolute inset-0 rounded-full bg-raised dark:bg-[#1A1D28] border border-line-strong dark:border-white/25 shadow-sm"
                    transition={SPRING_LAYOUT}
                  />
                )}
                <span className="relative z-10">{item.label}</span>
                {badge != null && (
                  <span
                    className="relative z-10 tnum inline-flex min-w-[16px] h-4 items-center justify-center rounded-full bg-warn px-1.5 text-[10px] font-bold text-[var(--warn-foreground)] shadow-elev-1"
                    aria-label={`${badge} awaiting review`}
                  >
                    {badge}
                  </span>
                )}
              </Link>
            )
          })}

          {/* More Tools Button (Positioned right after Review Queue) */}
          <button
            ref={buttonRef}
            type="button"
            onClick={toggleMore}
            className={cn(
              "relative inline-flex h-8 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full px-3 text-xs font-medium tracking-tight border transition-all duration-200 cursor-pointer select-none",
              isMoreActive
                ? "text-fg font-semibold border-line-strong dark:border-white/25 shadow-xs bg-raised dark:bg-[#1A1D28]"
                : "text-fg-muted border-transparent hover:text-fg hover:bg-elevated hover:border-line-subtle"
            )}
          >
            <WaveText text="More Tools" className="relative z-10" />
            <ChevronDown
              className={cn(
                "size-3.5 transition-transform duration-200 relative z-10 text-fg-muted",
                moreOpen && "rotate-180 text-fg"
              )}
            />
          </button>
        </nav>

        {/* Right-side Controls */}
        <div className="flex shrink-0 items-center gap-2">
          <CommandPalette />
          <AsOfControl />
          <ThemeToggle />
          <HelpMenu
            onTour={() => setWelcomeOpen(true)}
            onGlossary={() => setGlossaryOpen(true)}
          />
          <button
            type="button"
            title={`${OFFICER.name} · ${OFFICER.role} · ${OFFICER.firm}`}
            aria-label={`Signed in as ${OFFICER.name}, ${OFFICER.role} at ${OFFICER.firm}`}
            className="hidden shiny-cta size-8 shrink-0 place-items-center rounded-full !p-0 font-medium text-xs text-fg md:grid"
          >
            <span>{OFFICER.name.split(" ").map((w) => w[0]).join("")}</span>
          </button>
        </div>
      </header>

      {/* Dropdown Menu Portaled Directly to document.body to Prevent Any Overflow Clipping */}
      {moreOpen &&
        coords &&
        typeof window !== "undefined" &&
        createPortal(
          <AnimatePresence>
            <motion.div
              ref={menuRef}
              initial={{ opacity: 0, y: 6, scale: 0.96 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: 4, scale: 0.96 }}
              transition={{ duration: 0.15 }}
              style={{
                position: "fixed",
                top: `${coords.top}px`,
                right: `${coords.right}px`,
              }}
              className="z-[9999] w-64 rounded-xl border border-line-strong bg-raised dark:bg-[#131622] p-1.5 shadow-2xl backdrop-blur-2xl ring-1 ring-black/20 dark:ring-white/10"
            >
              <div className="px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wider text-fg-muted">
                Analysis & Tools
              </div>
              <div className="space-y-0.5 mt-0.5">
                {MORE_NAV.map((item) => {
                  const Icon = item.icon
                  const active = pathname.startsWith(item.href)
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={() => setMoreOpen(false)}
                      className={cn(
                        "flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-xs font-medium transition-colors",
                        active
                          ? "bg-accent/15 text-accent font-semibold"
                          : "text-fg-muted hover:bg-elevated hover:text-fg"
                      )}
                    >
                      <Icon className="size-4 shrink-0 text-accent/80" />
                      <div className="flex flex-col">
                        <span className="text-fg font-medium leading-tight">{item.label}</span>
                        <span className="text-[10px] text-fg-muted font-normal line-clamp-1 mt-0.5">
                          {item.hint}
                        </span>
                      </div>
                    </Link>
                  )
                })}
              </div>
            </motion.div>
          </AnimatePresence>,
          document.body
        )}

      {banner && <ScreenBanner id={banner.id}>{banner.text}</ScreenBanner>}

      <main id="main-content" className="min-h-0 flex-1 overflow-y-auto bg-canvas">
        <PageTransition key={pathname}>{children}</PageTransition>
      </main>

      {welcomeOpen && <WelcomeModal onClose={closeWelcome} />}
      {glossaryOpen && <GlossaryModal onClose={() => setGlossaryOpen(false)} />}
    </div>
  )
}
