"use client"

import * as React from "react"
import type { ReactNode } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { useQuery } from "@tanstack/react-query"

import { AsOfControl } from "@/components/as-of-control"
import { HelpMenu } from "@/components/help-menu"
import { ScreenBanner } from "@/components/screen-banner"
import { WelcomeModal } from "@/components/welcome-modal"
import { GlossaryModal } from "@/components/glossary-modal"
import { useAsOf } from "@/components/as-of-provider"
import { getReviewQueue } from "@/lib/api"
import { cn } from "@workspace/ui/lib/utils"

// Demo persona + firm, so the app reads like a real deployment.
const OFFICER = { name: "Priya Menon", role: "Compliance Officer", firm: "Acme Investment Advisers" }

const NAV = [
  { href: "/", label: "Overview", hint: "Your compliance dashboard at a glance." },
  { href: "/register", label: "Register", hint: "Every obligation extracted from the regulation, with its source." },
  { href: "/amendments", label: "Blast Radius", hint: "See everything a regulation change affects." },
  { href: "/evidence", label: "Evidence & Gaps", hint: "Which obligations are backed by evidence, and where the gaps are." },
  { href: "/review", label: "Review Queue", hint: "Obligations awaiting your approval — your daily inbox." },
  { href: "/policy", label: "Policy", hint: "Turn approved obligations into automated compliance checks." },
  { href: "/audit", label: "Audit", hint: "Reconstruct the full compliance trail as of any date." },
  { href: "/feed", label: "Feed", hint: "The machine-readable feed a regulator's systems can consume." },
]

// Plain-language purpose of each screen, shown as a dismissible banner.
const BANNER: Record<string, ReactNode> = {
  "/": "Your compliance posture at a glance. Start with what needs your attention, then explore the live obligation graph below.",
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

/**
 * AppShell is the persistent chrome: wordmark, primary nav (with a live Review
 * Queue count badge), the global as-of control, the health indicator, and a
 * per-screen context banner. It is a fixed-height flex column so full-height
 * pages (the graphs) and scrolling pages both work inside <main>.
 */
export function AppShell({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const { asOf } = useAsOf()

  const reviewCount = useQuery({
    queryKey: ["review-queue", asOf],
    queryFn: ({ signal }) => getReviewQueue(asOf, signal),
    select: (d) => d.count,
  })

  // First-run welcome tour (skippable, re-openable from the help menu).
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

  return (
    <div className="flex h-svh flex-col overflow-hidden">
      <header className="grid h-14 shrink-0 grid-cols-[1fr_auto_1fr] items-center gap-4 bg-ink px-6 text-on-ink">
        <div className="flex items-center">
          <Link
            href="/"
            className="font-display text-xl leading-none tracking-tight text-on-ink"
          >
            CHANAKYA
          </Link>
        </div>
        <nav className="flex items-center justify-center gap-1 text-sm">
            {NAV.map((item) => {
              const active =
                item.href === "/"
                  ? pathname === "/"
                  : pathname.startsWith(item.href)
              const badge =
                item.href === "/review" && (reviewCount.data ?? 0) > 0
                  ? reviewCount.data
                  : null
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  title={item.hint}
                  className={cn(
                    "inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 transition-colors",
                    active
                      ? "bg-on-ink/12 text-on-ink"
                      : "text-on-ink-dim hover:bg-on-ink/8 hover:text-on-ink",
                  )}
                >
                  {item.label}
                  {badge != null && (
                    <span className="tnum inline-flex min-w-4 items-center justify-center rounded-full bg-warn px-1 text-[10px] font-medium text-white">
                      {badge}
                    </span>
                  )}
                </Link>
              )
            })}
        </nav>
        <div className="flex items-center justify-end gap-3">
          <AsOfControl />
          <HelpMenu
            onTour={() => setWelcomeOpen(true)}
            onGlossary={() => setGlossaryOpen(true)}
          />
          <div
            title={`${OFFICER.name} · ${OFFICER.role} · ${OFFICER.firm}`}
            className="hidden size-8 place-items-center rounded-md border border-line-dark bg-ink-800 md:grid"
          >
            <span className="grid size-6 place-items-center rounded-full bg-on-ink/15 text-[11px] font-medium text-on-ink">
              {OFFICER.name
                .split(" ")
                .map((w) => w[0])
                .join("")}
            </span>
          </div>
        </div>
      </header>

      {banner && <ScreenBanner id={banner.id}>{banner.text}</ScreenBanner>}

      <main className="min-h-0 flex-1 overflow-y-auto">{children}</main>

      {welcomeOpen && <WelcomeModal onClose={closeWelcome} />}
      {glossaryOpen && <GlossaryModal onClose={() => setGlossaryOpen(false)} />}
    </div>
  )
}
