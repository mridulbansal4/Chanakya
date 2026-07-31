"use client"

import * as React from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { motion, useReducedMotion } from "framer-motion"
import {
  ArrowRight,
  CheckCircle2,
  ChevronRight,
  ClipboardCheck,
  FileWarning,
  PenLine,
  Activity,
} from "lucide-react"

import type { Posture } from "@/lib/api"

/**
 * Executive Mission Control KPI Header
 * Designed for Chief Compliance Officers at enterprise financial institutions.
 * High density, machined graphite aesthetics, zero decorative bloat.
 */

export interface ExecutiveKpiHeaderProps {
  posture?: Posture
  isLoading?: boolean
}

// Inline Noise Data URL for machined graphite depth
const NOISE_BG =
  "data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)' opacity='0.035'/%3E%3C/svg%3E"

export function ExecutiveKpiHeader({ posture, isLoading }: ExecutiveKpiHeaderProps) {
  const router = useRouter()
  const prefersReducedMotion = useReducedMotion()

  // Dynamic UTC Time for Top Metadata Ticker
  const [utcTime, setUtcTime] = React.useState<string>("")
  React.useEffect(() => {
    const updateTime = () => {
      const now = new Date()
      setUtcTime(now.toISOString().substring(11, 19) + " UTC")
    }
    updateTime()
    const interval = setInterval(updateTime, 1000)
    return () => clearInterval(interval)
  }, [])

  // Keyboard shortcut listener for Ctrl+R or ⌘R -> Navigate to Execute Review
  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "r") {
        e.preventDefault()
        router.push("/review")
      }
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [router])

  if (isLoading) {
    return <ExecutiveKpiSkeleton />
  }

  const activeObligations = posture?.obligations_in_force ?? 10
  const awaitingApproval = posture?.pending_signoffs ?? 9
  const needsReview = posture?.needs_review ?? 5
  const evidenceGaps = posture?.gaps ?? 5
  const propagationLatency = "1.2s"

  return (
    <section
      role="region"
      aria-label="Executive Mission Control"
      className="relative w-full border-b border-white/10 bg-[#0B0D13] text-foreground shadow-2xl selection:bg-blue-600/30 overflow-hidden"
      style={{
        backgroundImage: `url("${NOISE_BG}")`,
      }}
    >
      {/* Top Subtle Light Reflection Line */}
      <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/20 to-transparent pointer-events-none" />

      {/* 1. TOP METADATA TICKER STRIP */}
      <div className="flex h-8 items-center justify-between border-b border-white/10 px-6 text-[11px] font-mono tracking-wider uppercase text-slate-400 bg-white/[0.015]">
        <div className="flex items-center gap-3">
          <span className="font-semibold text-slate-200">CHANAKYA</span>
          <span className="text-slate-600">//</span>
          <span className="text-slate-400">REGULATORY OS</span>
          <span className="text-slate-600">|</span>
          <div className="flex items-center gap-1.5 text-emerald-400 font-medium">
            <span className="relative flex h-1.5 w-1.5">
              {!prefersReducedMotion && (
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
              )}
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500" />
            </span>
            <span>LIVE</span>
          </div>
        </div>

        <div className="flex items-center gap-4">
          <span className="hidden sm:inline text-slate-400">
            ENGINE: <span className="text-slate-300">ONLINE</span>
          </span>
          <span className="text-slate-600 hidden sm:inline">|</span>
          <span className="tnum font-mono text-slate-300">
            {utcTime || "18:42:09 UTC"}
          </span>
        </div>
      </div>

      {/* 2. PROPORTIONAL MISSION CONTROL METRIC STRIP (Rounded Cards with Spacing) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:flex gap-3 p-3.5">
        
        {/* METRIC 1: ACTIVE OBLIGATIONS (Proportional Weight 28%) */}
        <MetricColumn
          className="lg:w-[28%]"
          label="ACTIVE OBLIGATIONS"
          value={activeObligations}
          subtext="100% Tracking Active"
          tone="emerald"
          visualization={<MicroSegmentBar count={activeObligations} total={10} />}
        />

        {/* METRIC 2: AWAITING APPROVAL (Proportional Weight 22%) */}
        <MetricColumn
          className="lg:w-[22%]"
          label="AWAITING APPROVAL"
          value={awaitingApproval}
          subtext={awaitingApproval > 0 ? "Pending Officer Sign-off" : "All Sign-offs Current"}
          tone={awaitingApproval > 0 ? "amber" : "emerald"}
          visualization={<MicroMatrixDots count={awaitingApproval} total={10} />}
        />

        {/* METRIC 3: NEEDS REVIEW (Proportional Weight 20%) */}
        <MetricColumn
          className="lg:w-[20%]"
          label="NEEDS REVIEW"
          value={needsReview}
          subtext={needsReview > 0 ? "Low AI Confidence" : "High Confidence"}
          tone={needsReview > 0 ? "amber" : "emerald"}
          visualization={<MicroSparkline active={needsReview > 0} />}
        />

        {/* METRIC 4: EVIDENCE GAPS (Proportional Weight 18%) */}
        <MetricColumn
          className="lg:w-[18%]"
          label="EVIDENCE GAPS"
          value={evidenceGaps === 0 ? "0" : evidenceGaps}
          subtext={evidenceGaps === 0 ? "All Evidence Verified" : `${evidenceGaps} Open Tickets`}
          tone={evidenceGaps > 0 ? "crimson" : "emerald"}
          visualization={<MicroCompletionRing gaps={evidenceGaps} />}
        />

        {/* METRIC 5: PROPAGATION (Proportional Weight 12%) */}
        <MetricColumn
          className="lg:w-[12%]"
          label="PROPAGATION"
          value={propagationLatency}
          subtext="Diff → Blast Radius"
          tone="cyan"
          visualization={<MicroHeartbeatWave />}
        />
      </div>

      {/* 3. OPERATIONS QUEUE & COMMAND BAR */}
      <OperationsQueue posture={posture} />
    </section>
  )
}

/* ==========================================================================
   METRIC COLUMN COMPONENT
   Strict 4-Level Typography & Micro-Interactions
   ========================================================================== */

interface MetricColumnProps {
  className?: string
  label: string
  value: string | number
  subtext: string
  tone: "emerald" | "amber" | "crimson" | "cyan"
  visualization: React.ReactNode
}

const TONE_CLASSES: Record<MetricColumnProps["tone"], { text: string; dot: string }> = {
  emerald: { text: "text-emerald-400", dot: "bg-emerald-500" },
  amber: { text: "text-amber-400", dot: "bg-amber-400" },
  crimson: { text: "text-red-400", dot: "bg-red-500" },
  cyan: { text: "text-cyan-400", dot: "bg-cyan-400" },
}

function MetricColumn({
  className = "",
  label,
  value,
  subtext,
  tone,
  visualization,
}: MetricColumnProps) {
  const toneStyle = TONE_CLASSES[tone]

  return (
    <div
      className={`group relative flex flex-col justify-between p-4.5 rounded-xl border border-white/10 bg-[#12141E] transition-all duration-200 ease-out hover:border-white/25 hover:bg-[#181B28] hover:-translate-y-[1px] hover:shadow-lg hover:shadow-black/40 overflow-hidden cursor-default ${className}`}
    >
      {/* Top Border Illumination on Hover */}
      <div className="absolute inset-x-0 top-0 h-[2px] bg-gradient-to-r from-transparent via-white/25 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-200" />

      {/* Level 1: Label (11px Mono) */}
      <div className="flex items-center justify-between gap-2">
        <span className="text-[11px] font-mono tracking-widest uppercase text-slate-400/90 font-medium">
          {label}
        </span>
        <span className={`h-1.5 w-1.5 rounded-full ${toneStyle.dot} opacity-80`} />
      </div>

      {/* Level 3: Big Number (42px Display) + Micro Viz */}
      <div className="mt-3 flex items-baseline justify-between gap-3">
        <motion.span
          key={String(value)}
          initial={{ opacity: 0.8, y: 1 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.18, ease: "easeOut" }}
          className={`text-[42px] font-display font-semibold tracking-tight leading-none tnum group-hover:text-white ${toneStyle.text}`}
        >
          {value}
        </motion.span>

        {/* Unique Micro Visualization */}
        <div className="shrink-0 self-center">{visualization}</div>
      </div>

      {/* Level 4: Description (12px Sans) */}
      <div className="mt-2.5 flex items-center justify-between text-[12px] text-slate-400 font-sans">
        <span className="truncate">{subtext}</span>
      </div>
    </div>
  )
}

/* ==========================================================================
   OPERATIONS QUEUE & COMMAND BAR
   Structured list rows, no rounded pills, enterprise command trigger
   ========================================================================== */

function OperationsQueue({ posture }: { posture?: Posture }) {
  const needsReview = posture?.needs_review ?? 5
  const pending = posture?.pending_signoffs ?? 9
  const gaps = posture?.gaps ?? 5
  const allClear = needsReview === 0 && pending === 0 && gaps === 0

  return (
    <div className="flex flex-col border-t border-white/10 bg-[#08090F] lg:flex-row lg:items-center lg:justify-between px-6 py-3 gap-3">
      {/* QUEUE TITLE & ITEMS */}
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
        <div className="flex items-center gap-2 text-[11px] font-mono tracking-widest uppercase text-slate-400 font-medium shrink-0">
          <Activity className="size-3.5 text-blue-400" />
          <span>OPERATIONS QUEUE</span>
        </div>

        {allClear ? (
          <div className="flex items-center gap-2 text-[12px] text-emerald-400 font-medium bg-emerald-950/20 px-3 py-1 border border-emerald-500/20">
            <CheckCircle2 className="size-3.5" />
            <span>Nothing Requires Attention — Platform In Full Compliance</span>
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-1.5 sm:gap-2 text-[12px]">
            {needsReview > 0 && (
              <QueueItem
                href="/review"
                icon={<ClipboardCheck className="size-3.5 text-amber-400" />}
                count={needsReview}
                label="Review Requests"
              />
            )}
            {pending > 0 && (
              <QueueItem
                href="/review"
                icon={<PenLine className="size-3.5 text-amber-400" />}
                count={pending}
                label="Pending Sign-offs"
              />
            )}
            {gaps > 0 && (
              <QueueItem
                href="/evidence"
                icon={<FileWarning className="size-3.5 text-red-400" />}
                count={gaps}
                label="Missing Evidence"
                isAlert
              />
            )}
          </div>
        )}
      </div>

      {/* COMMAND ACTION BUTTON WITH SHORTCUT */}
      <div className="shrink-0 flex items-center gap-2">
        <Link
          href="/review"
          className="group relative inline-flex items-center gap-2.5 bg-[#161822] hover:bg-[#1E202E] text-slate-200 hover:text-white px-4 py-2 border border-white/15 text-[12px] font-mono tracking-wide uppercase transition-all duration-200 ease-out hover:border-white/30 hover:shadow-[0_0_15px_rgba(255,255,255,0.06)] active:translate-y-0.5"
        >
          <span>Run Daily Review</span>
          <ArrowRight className="size-3.5 transition-transform duration-200 group-hover:translate-x-1 text-slate-400 group-hover:text-white" />
          <kbd className="hidden sm:inline-block ml-1 rounded bg-white/10 px-1.5 py-0.5 text-[10px] font-mono text-slate-300 border border-white/10">
            Ctrl+R
          </kbd>
        </Link>
      </div>
    </div>
  )
}

function QueueItem({
  href,
  icon,
  count,
  label,
  isAlert = false,
}: {
  href: string
  icon: React.ReactNode
  count: number
  label: string
  isAlert?: boolean
}) {
  return (
    <Link
      href={href}
      className={`group flex items-center gap-2 px-3 py-1 bg-white/[0.02] hover:bg-white/[0.06] border border-white/10 hover:border-white/25 text-slate-300 hover:text-white transition-all duration-150 ease-out ${
        isAlert ? "hover:border-red-500/40" : ""
      }`}
    >
      <span className="shrink-0">{icon}</span>
      <span className="font-mono font-bold text-white tnum text-[12px]">{count}</span>
      <span className="text-slate-300 font-sans text-[12px]">{label}</span>
      <ChevronRight className="size-3 text-slate-500 group-hover:text-slate-300 group-hover:translate-x-0.5 transition-all duration-150" />
    </Link>
  )
}

/* ==========================================================================
   MICRO VISUALIZATIONS (MEMOIZED)
   1. Segment Bar (Obligations)
   2. Matrix Dots (Approvals)
   3. Sparkline (Needs Review)
   4. Completion Ring (Evidence)
   5. Heartbeat Wave (Propagation)
   ========================================================================== */

const MicroSegmentBar = React.memo(function MicroSegmentBar({
  count = 10,
  total = 10,
}: {
  count?: number
  total?: number
}) {
  const prefersReducedMotion = useReducedMotion()
  return (
    <div className="flex items-center gap-1" aria-hidden="true">
      {Array.from({ length: total }).map((_, i) => (
        <motion.div
          key={i}
          initial={prefersReducedMotion ? false : { opacity: 0.3, scaleY: 0.6 }}
          animate={{ opacity: 1, scaleY: 1 }}
          transition={{ duration: 0.15, delay: i * 0.02 }}
          className={`h-4 w-1.5 rounded-[1px] ${
            i < count ? "bg-emerald-500/80 group-hover:bg-emerald-400" : "bg-white/10"
          }`}
        />
      ))}
    </div>
  )
})

const MicroMatrixDots = React.memo(function MicroMatrixDots({
  count = 9,
  total = 10,
}: {
  count?: number
  total?: number
}) {
  return (
    <div className="grid grid-cols-5 gap-1 w-[38px]" aria-hidden="true">
      {Array.from({ length: total }).map((_, i) => (
        <div
          key={i}
          className={`size-1.5 rounded-full ${
            i < count ? "bg-amber-400 group-hover:bg-amber-300" : "bg-white/15"
          }`}
        />
      ))}
    </div>
  )
})

const MicroSparkline = React.memo(function MicroSparkline({
  active = true,
}: {
  active?: boolean
}) {
  const prefersReducedMotion = useReducedMotion()
  return (
    <svg
      width="56"
      height="22"
      viewBox="0 0 56 22"
      fill="none"
      className="overflow-visible"
      aria-hidden="true"
    >
      <defs>
        <linearGradient id="sparklineGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#F59E0B" stopOpacity="0.25" />
          <stop offset="100%" stopColor="#F59E0B" stopOpacity="0.0" />
        </linearGradient>
      </defs>
      <path
        d="M 2 18 L 12 12 L 22 15 L 32 6 L 42 11 L 54 3 M 54 3 L 54 20 L 2 20 Z"
        fill="url(#sparklineGrad)"
      />
      <motion.path
        d="M 2 18 L 12 12 L 22 15 L 32 6 L 42 11 L 54 3"
        stroke={active ? "#F59E0B" : "#10B981"}
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
        initial={prefersReducedMotion ? false : { pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: 0.6, ease: "easeOut" }}
      />
      <circle cx="54" cy="3" r="2" fill="#F59E0B" className="animate-pulse" />
    </svg>
  )
})

const MicroCompletionRing = React.memo(function MicroCompletionRing({
  gaps = 0,
}: {
  gaps?: number
}) {
  const prefersReducedMotion = useReducedMotion()
  const size = 26
  const strokeWidth = 2.5
  const center = size / 2
  const radius = center - strokeWidth
  const circumference = 2 * Math.PI * radius
  const gapPercentage = gaps > 0 ? Math.min(gaps * 15, 75) : 0
  const dashOffset = (gapPercentage / 100) * circumference

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      className="-rotate-90"
      aria-hidden="true"
    >
      <circle
        cx={center}
        cy={center}
        r={radius}
        stroke="rgba(255, 255, 255, 0.12)"
        strokeWidth={strokeWidth}
        fill="none"
      />
      <motion.circle
        cx={center}
        cy={center}
        r={radius}
        stroke={gaps > 0 ? "#EF4444" : "#10B981"}
        strokeWidth={strokeWidth}
        strokeDasharray={circumference}
        strokeDashoffset={dashOffset}
        strokeLinecap="round"
        fill="none"
        initial={prefersReducedMotion ? false : { strokeDashoffset: circumference }}
        animate={{ strokeDashoffset: dashOffset }}
        transition={{ duration: 0.5, ease: "easeOut" }}
      />
    </svg>
  )
})

const MicroHeartbeatWave = React.memo(function MicroHeartbeatWave() {
  const prefersReducedMotion = useReducedMotion()
  return (
    <svg
      width="48"
      height="20"
      viewBox="0 0 48 20"
      fill="none"
      className="overflow-visible"
      aria-hidden="true"
    >
      <motion.path
        d="M 0 10 L 12 10 L 16 3 L 20 17 L 24 6 L 28 12 L 32 10 L 48 10"
        stroke="#22D3EE"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        initial={prefersReducedMotion ? false : { pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: 0.7, ease: "easeOut" }}
      />
      <circle cx="44" cy="10" r="2" fill="#22D3EE" className="animate-ping opacity-75" />
      <circle cx="44" cy="10" r="1.5" fill="#06B6D4" />
    </svg>
  )
})

/* ==========================================================================
   EXECUTIVE KPI SKELETON
   Zero layout shift loader that matches the exact geometry
   ========================================================================== */

export function ExecutiveKpiSkeleton() {
  return (
    <div className="w-full border-b border-white/10 bg-[#0B0D13] animate-pulse">
      {/* Top Metadata Strip Skeleton */}
      <div className="h-8 border-b border-white/10 px-6 flex items-center justify-between bg-white/[0.01]">
        <div className="h-3 w-48 bg-white/10 rounded-sm" />
        <div className="h-3 w-32 bg-white/10 rounded-sm" />
      </div>

      {/* Metrics Strip Skeleton */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:flex gap-3 p-3.5">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="p-4.5 rounded-xl border border-white/10 bg-[#12141E] lg:flex-1 flex flex-col justify-between">
            <div className="flex items-center justify-between">
              <div className="h-3 w-24 bg-white/10 rounded-sm" />
              <div className="h-1.5 w-1.5 rounded-full bg-white/20" />
            </div>
            <div className="my-3 flex items-baseline justify-between">
              <div className="h-9 w-16 bg-white/15 rounded-sm" />
              <div className="h-4 w-12 bg-white/10 rounded-sm" />
            </div>
            <div className="h-3 w-32 bg-white/10 rounded-sm" />
          </div>
        ))}
      </div>

      {/* Operations Queue Skeleton */}
      <div className="h-12 border-t border-white/10 px-6 flex items-center justify-between bg-[#08090F]">
        <div className="h-4 w-64 bg-white/10 rounded-sm" />
        <div className="h-8 w-36 bg-white/15 rounded-sm" />
      </div>
    </div>
  )
}
