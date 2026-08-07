"use client"

import * as React from "react"
import type { ReactNode } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import { motion, useReducedMotion } from "framer-motion"
import { toast } from "sonner"

import { AsOfControl } from "@/components/as-of-control"
import { HelpMenu } from "@/components/help-menu"
import { ProfileMenu } from "@/components/profile-menu"
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@workspace/ui/components/dropdown-menu"
import { ChevronDown } from "lucide-react"
const OFFICER = {
  name: "Priya Menon",
  role: "Compliance Officer",
  firm: "Acme Investment Advisers",
}

const PRIMARY_NAV = [
  { href: "/", label: "Overview", hint: "Your compliance dashboard at a glance." },
  { href: "/ingest", label: "Regulatory Intake", hint: "Upload a SEBI circular and watch the pipeline run - approval required." },
  { href: "/review", label: "Review Queue", hint: "Obligations awaiting your approval - your daily inbox." },
  { href: "/workflows", label: "Workflows", hint: "Draft task DAGs generated from approved obligations, with named owners." },
  { href: "/evidence", label: "Evidence & Gaps", hint: "Which obligations are backed by evidence, and where the gaps are." },
]

const SECONDARY_NAV = [
  { href: "/register", label: "Regulatory Register", hint: "Every obligation extracted from the regulation, with its source." },
  { href: "/regulatory-feed", label: "Regulatory Feed", hint: "Every circular in the corpus, how it arrived, and what an amendment changed." },
  { href: "/amendments", label: "Blast Radius", hint: "See everything a regulation change affects." },
  { href: "/policy", label: "Policy", hint: "Turn approved obligations into automated compliance checks." },
  { href: "/audit", label: "Audit", hint: "Reconstruct the full compliance trail as of any date." },
  { href: "/feed", label: "Feed", hint: "The machine-readable feed a regulator's systems can consume." },
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
  
  React.useEffect(() => {
    const timer = setTimeout(() => {
      toast("🚨 SEBI Alert", { 
        description: "New circular regarding Mutual Funds released. Blast radius analysis is ready.", 
        action: { label: "View", onClick: () => console.log("Viewed") } 
      })
    }, 5000)
    return () => clearTimeout(timer)
  }, [])

  const closeWelcome = () => {
    window.localStorage.setItem("chanakya.welcomed", "1")
    setWelcomeOpen(false)
  }

  const banner = bannerFor(pathname)
  const pending = reviewCount.data ?? 0

  return (
    <div className="flex h-svh flex-col overflow-hidden bg-canvas text-fg">
      {/*
        The header uses bg-sunken/90 backdrop-blur-md with high-contrast text and icons
        matching both dark and light modes.
      */}
      <header className="z-30 relative flex h-14 shrink-0 items-center gap-6 border-b border-line-subtle bg-sunken/90 backdrop-blur-md px-4 lg:px-6 shadow-elev-2">
        {/* Animated rotating/glowing bottom border accent line */}
        <div className="absolute inset-x-0 bottom-0 h-[1.5px] bg-gradient-to-r from-transparent via-accent to-transparent opacity-80 animate-pulse pointer-events-none" />

        <Link
          href="/"
          className="flex shrink-0 items-center gap-2.5 rounded group transition-opacity hover:opacity-90"
          aria-label="CHANAKYA - go to overview"
        >
          {/* Dark mode logo */}
          <img
            src="/logo-dark.png"
            alt="CHANAKYA"
            className="hidden dark:block h-7 w-auto object-contain"
          />
          {/* Light mode logo */}
          <img
            src="/logo-light.png"
            alt="CHANAKYA"
            className="block dark:hidden h-7 w-auto object-contain"
          />
          <span className="text-headline-sm tracking-tight font-bold text-fg">CHANAKYA</span>
        </Link>

        {/*
          Nine destinations with active indicator and badges
        */}
        <nav
          aria-label="Primary"
          className="no-scrollbar flex min-w-0 flex-1 items-center justify-center gap-1 overflow-x-auto"
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
                  "relative inline-flex h-8.5 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full px-3.5 text-[13px] font-medium tracking-tight border",
                  "transition-all duration-200 ease-out",
                  active
                    ? "text-fg font-semibold border-line-strong dark:border-white/25 shadow-xs"
                    : "text-fg-muted border-transparent hover:text-fg hover:bg-elevated hover:border-line-subtle",
                )}
              >
                {active && (
                  <motion.span
                    // Shared layoutId lets the indicator travel between tabs
                    layoutId={reduce ? undefined : "nav-active"}
                    aria-hidden
                    className="absolute inset-0 rounded-full bg-raised dark:bg-[#1A1D28] border border-line-strong dark:border-white/25 shadow-sm"
                    transition={SPRING_LAYOUT}
                  />
                )}
                <WaveText
                  className="relative z-10 text-[13px]"
                  text={item.label}
                />
                {badge != null && (
                  <span
                    className="relative z-10 tnum inline-flex min-w-[17px] h-4.5 items-center justify-center rounded-full bg-warn px-1.5 text-[10px] font-bold text-[var(--warn-foreground)] shadow-elev-1"
                    aria-label={`${badge} awaiting review`}
                  >
                    {badge}
                  </span>
                )}
              </Link>
            )
          })}
        </nav>

        <div className="flex shrink-0 items-center gap-3">
          <DropdownMenu>
            <DropdownMenuTrigger className="inline-flex h-8 items-center justify-center rounded-md px-3 text-xs font-semibold uppercase tracking-wider text-fg-muted bg-surface/50 border border-line-subtle transition-all hover:bg-elevated hover:text-fg hover:border-line-strong outline-none focus:ring-2 focus:ring-accent/20">
              Tools & Reports
              <ChevronDown className="ml-1.5 h-3.5 w-3.5 opacity-70" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48">
              {SECONDARY_NAV.map((item) => (
                <DropdownMenuItem key={item.href} render={<Link href={item.href} title={item.hint} className="w-full cursor-pointer" />}>
                  {item.label}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>

          <CommandPalette />
          <AsOfControl />
          <ThemeToggle />
          <HelpMenu
            onTour={() => setWelcomeOpen(true)}
            onGlossary={() => setGlossaryOpen(true)}
          />
          <ProfileMenu />
        </div>
      </header>

      {banner && <ScreenBanner id={banner.id}>{banner.text}</ScreenBanner>}

      <main id="main-content" className="min-h-0 flex-1 overflow-y-auto bg-canvas">
        <PageTransition key={pathname}>{children}</PageTransition>
      </main>

      {welcomeOpen && <WelcomeModal onClose={closeWelcome} />}
      {glossaryOpen && <GlossaryModal onClose={() => setGlossaryOpen(false)} />}
    </div>
  )
}
