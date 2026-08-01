"use client"

import * as React from "react"
import type { ReactNode } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import { motion, useReducedMotion } from "framer-motion"

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
import { RandomLetterSwap } from "@/components/ui/random-letter-swap"
import { SPRING_LAYOUT } from "@/lib/motion"
import { cn } from "@workspace/ui/lib/utils"

const OFFICER = {
  name: "Priya Menon",
  role: "Compliance Officer",
  firm: "Acme Investment Advisers",
}

const NAV = [
  { href: "/", label: "Overview", hint: "Your compliance dashboard at a glance." },
  { href: "/regulatory-feed", label: "Regulatory Feed", hint: "New SEBI circulars, detected and processed end to end." },
  { href: "/register", label: "Register", hint: "Every obligation extracted from the regulation, with its source." },
  { href: "/amendments", label: "Blast Radius", hint: "See everything a regulation change affects." },
  { href: "/evidence", label: "Evidence & Gaps", hint: "Which obligations are backed by evidence, and where the gaps are." },
  { href: "/review", label: "Review Queue", hint: "Obligations awaiting your approval — your daily inbox." },
  { href: "/policy", label: "Policy", hint: "Turn approved obligations into automated compliance checks." },
  { href: "/audit", label: "Audit", hint: "Reconstruct the full compliance trail as of any date." },
  { href: "/feed", label: "Feed", hint: "The machine-readable feed a regulator's systems can consume." },
]

const BANNER: Record<string, ReactNode> = {
  "/": "Your compliance posture at a glance. Start with what needs your attention, then explore the live obligation graph below.",
  "/regulatory-feed": "CHANAKYA monitors SEBI for new circulars. Process the new MITC amendment to see the full lifecycle — diff, obligations, blast radius, workflows, approval, evidence, and an audit pack.",
  "/register": "Every obligation CHANAKYA extracted from the regulation, with the exact source text behind each one. Click a row to see its citation.",
  "/amendments": "Preview what a regulation change would affect before you accept it. Pick a clause, edit its text, and compute the impact.",
  "/evidence": "Which obligations are backed by evidence from your firm's systems, and where the gaps are. Each gap becomes a draft remediation ticket.",
  "/review": "These obligations need your judgement before CHANAKYA can act on them. Approve, correct, or reject each one.",
  "/policy": "Turn a signed obligation into an automated compliance check, then test it against your firm's data. Enforcement stays in 'audit' until you promote it.",
  "/audit": "Reconstruct the full compliance trail — clause to obligation to sign-off — as of any date. Change the date (top right) to travel through time.",
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
  const closeWelcome = () => {
    window.localStorage.setItem("chanakya.welcomed", "1")
    setWelcomeOpen(false)
  }

  const banner = bannerFor(pathname)
  const pending = reviewCount.data ?? 0

  return (
    <div className="flex h-svh flex-col overflow-hidden bg-canvas text-fg">
      {/*
        The chrome sits on --sunken, one step below the content canvas, so
        the app surface reads as sitting *in* a frame. It is solid rather
        than translucent: a blurred bar over a scrolling data table is
        visual noise in an instrument, and it costs a compositing layer on
        every scroll frame.
      */}
      <header className="z-30 relative flex h-14 shrink-0 items-center gap-4 border-b border-line-subtle bg-sunken/90 backdrop-blur-md px-4 lg:px-5 shadow-elev-2">
        {/* Animated rotating/glowing bottom border accent line */}
        <div className="absolute inset-x-0 bottom-0 h-[1.5px] bg-gradient-to-r from-transparent via-accent to-transparent opacity-80 animate-pulse pointer-events-none" />

        <Link
          href="/"
          className="flex shrink-0 items-center gap-2.5 rounded group transition-opacity hover:opacity-90"
          aria-label="CHANAKYA — go to overview"
        >
          <img src="/logo.svg" alt="" aria-hidden className="h-5 w-auto" />
          <span className="text-headline-sm tracking-tight font-semibold text-fg">CHANAKYA</span>
        </Link>

        {/*
          Nine destinations is a lot for one row, but these are the product's
          operating surfaces and a compliance officer moves between them
          constantly — burying them in a menu would cost more than it saves.
          The command palette (⌘K) is the fast path for anyone who prefers it.
        */}
        <nav
          aria-label="Primary"
          className="no-scrollbar flex min-w-0 flex-1 items-center justify-center gap-0.5 overflow-x-auto"
        >
          {NAV.map((item) => {
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
                  "relative inline-flex h-8.5 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full px-3 text-[13px] font-medium tracking-tight",
                  "transition-all duration-200 ease-out",
                  active
                    ? "text-fg font-semibold"
                    : "text-fg-muted hover:text-fg hover:bg-elevated",
                )}
              >
                {active && (
                  <motion.span
                    // Shared layoutId lets the indicator travel between tabs
                    layoutId={reduce ? undefined : "nav-active"}
                    aria-hidden
                    className="shiny-cta absolute inset-0 rounded-full !p-0 shadow-elev-1"
                    transition={SPRING_LAYOUT}
                  />
                )}
                <RandomLetterSwap
                  className="relative z-10 text-[13px]"
                  label={item.label}
                  staggerDuration={0.04}
                  transition={{ duration: 0.8, type: "spring" }}
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

        <div className="flex shrink-0 items-center gap-2.5">
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

      {banner && <ScreenBanner id={banner.id}>{banner.text}</ScreenBanner>}

      <main id="main-content" className="min-h-0 flex-1 overflow-y-auto bg-canvas">
        <PageTransition key={pathname}>{children}</PageTransition>
      </main>

      {welcomeOpen && <WelcomeModal onClose={closeWelcome} />}
      {glossaryOpen && <GlossaryModal onClose={() => setGlossaryOpen(false)} />}
    </div>
  )
}
