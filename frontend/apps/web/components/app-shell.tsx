"use client"

import * as React from "react"
import type { ReactNode } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { useQuery } from "@tanstack/react-query"
import { motion } from "framer-motion"

import { AsOfControl } from "@/components/as-of-control"
import { HelpMenu } from "@/components/help-menu"
import { ScreenBanner } from "@/components/screen-banner"
import { WelcomeModal } from "@/components/welcome-modal"
import { GlossaryModal } from "@/components/glossary-modal"
import { CommandPalette } from "@/components/command-palette"
import { PageTransition } from "@/components/page-transition"
import { useAsOf } from "@/components/as-of-provider"
import { getReviewQueue } from "@/lib/api"
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

  return (
    <div className="flex h-svh flex-col overflow-hidden bg-background text-foreground">
      <header className="flex h-16 shrink-0 items-center justify-between border-b border-white/10 bg-[#090A0F]/95 px-4 lg:px-6 text-white shadow-xl backdrop-blur-xl z-30 gap-2">
        {/* Brand wordmark - Clean CHANAKYA Logo without SUPTECH badge */}
        <div className="flex items-center gap-2 shrink-0">
          <Link href="/" className="flex items-center gap-2.5 group">
            <img src="/logo.svg" alt="CHANAKYA Logo" className="h-7 w-auto transition-transform group-hover:scale-105" />
            <span className="font-display text-2xl lg:text-[26px] leading-none tracking-tight text-white font-extrabold">
              CHANAKYA
            </span>
          </Link>
        </div>

        {/* Navigation Tabs - Perfectly fitted */}
        <nav className="flex min-w-0 flex-1 items-center justify-center gap-1 text-xs lg:text-[13px] overflow-x-auto no-scrollbar py-1 px-1">
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
                  "relative inline-flex items-center gap-1.5 rounded-full px-2.5 py-1.5 text-xs lg:text-[13px] font-semibold transition-all duration-150 whitespace-nowrap shrink-0",
                  active
                    ? "text-white"
                    : "text-slate-300 hover:text-white hover:bg-white/10"
                )}
              >
                {active && (
                  <motion.div
                    layoutId="activeNavTab"
                    className="absolute inset-0 rounded-full bg-blue-600/40 border border-blue-400/60 shadow-[0_0_15px_rgba(59,130,246,0.3)]"
                    transition={{ type: "spring", stiffness: 380, damping: 30 }}
                  />
                )}
                <span className="relative z-10">{item.label}</span>
                {badge != null && (
                  <span className="relative z-10 tnum inline-flex min-w-4 items-center justify-center rounded-full bg-amber-400 px-1.5 py-0.2 text-[10px] font-extrabold text-black shadow-sm">
                    {badge}
                  </span>
                )}
              </Link>
            )
          })}
        </nav>

        {/* Action Controls */}
        <div className="flex items-center justify-end gap-2 shrink-0">
          <CommandPalette />
          <AsOfControl />
          <HelpMenu
            onTour={() => setWelcomeOpen(true)}
            onGlossary={() => setGlossaryOpen(true)}
          />
          <div
            title={`${OFFICER.name} · ${OFFICER.role} · ${OFFICER.firm}`}
            className="hidden size-8 place-items-center rounded-full border border-white/20 bg-white/10 md:grid shadow-inner cursor-pointer hover:border-white/50 transition-all"
          >
            <span className="grid size-7 place-items-center rounded-full bg-white/15 text-[11px] font-bold text-white">
              {OFFICER.name
                .split(" ")
                .map((w) => w[0])
                .join("")}
            </span>
          </div>
        </div>
      </header>

      {banner && <ScreenBanner id={banner.id}>{banner.text}</ScreenBanner>}

      <main className="min-h-0 flex-1 overflow-y-auto bg-background">
        <PageTransition key={pathname}>{children}</PageTransition>
      </main>

      {welcomeOpen && <WelcomeModal onClose={closeWelcome} />}
      {glossaryOpen && <GlossaryModal onClose={() => setGlossaryOpen(false)} />}
    </div>
  )
}
