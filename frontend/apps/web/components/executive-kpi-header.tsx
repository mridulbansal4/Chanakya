"use client"

import * as React from "react"
import { motion, useReducedMotion } from "framer-motion"

import type { Posture } from "@/lib/api"

/**
 * The posture strip.
 *
 * Five numbers, each paired with a small visual that says something the
 * number alone does not: how much of the whole is covered, how the figure is
 * distributed, which way it is trending. The visual is never the only
 * carrier of meaning - every one of them restates what the value and the
 * caption already say, so losing it to a rendering failure or a screen
 * reader costs nothing.
 *
 * The action row that belongs under this strip lives on the overview page as
 * `NeedsAttention`, not here, so there is exactly one "what do I do now" row
 * on the screen.
 */

export interface ExecutiveKpiHeaderProps {
  posture?: Posture
  isLoading?: boolean
}

export function ExecutiveKpiHeader({ posture, isLoading }: ExecutiveKpiHeaderProps) {
  /**
   * Rendered empty on the server and filled after mount - a clock formatted
   * during SSR is wrong by the time it reaches the client and trips
   * hydration.
   */
  const [utcTime, setUtcTime] = React.useState("")
  React.useEffect(() => {
    const tick = () =>
      setUtcTime(new Date().toISOString().substring(11, 19) + " UTC")
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [])

  if (isLoading) {
    return <ExecutiveKpiSkeleton />
  }

  const activeObligations = posture?.obligations_in_force ?? 0
  const awaitingApproval = posture?.pending_signoffs ?? 0
  const needsReview = posture?.needs_review ?? 0
  const evidenceGaps = posture?.gaps ?? 0

  return (
    <section
      aria-label="Compliance posture"
      className="w-full shrink-0 border-b border-line-subtle bg-canvas"
    >
      <StatusTicker utcTime={utcTime} />

      {/*
        Widths are proportional rather than equal: the first two metrics carry
        the most digits and the most context, and an even five-column split
        leaves them cramped while the latency cell runs half empty.
      */}
      <div className="grid grid-cols-1 gap-3 p-3.5 sm:grid-cols-2 md:grid-cols-3 lg:flex">
        <MetricCard
          className="lg:w-[28%]"
          label="Active obligations"
          value={activeObligations}
          caption="Tracking active"
          tone="ok"
          visual={<MicroSegmentBar count={activeObligations} total={10} />}
        />
        <MetricCard
          className="lg:w-[22%]"
          label="Awaiting approval"
          value={awaitingApproval}
          caption={
            awaitingApproval > 0
              ? "Pending officer sign-off"
              : "All sign-offs current"
          }
          tone={awaitingApproval > 0 ? "warn" : "ok"}
          visual={<MicroMatrixDots count={awaitingApproval} total={10} />}
        />
        <MetricCard
          className="lg:w-[20%]"
          label="Needs review"
          value={needsReview}
          caption={
            needsReview > 0
              ? "Extraction confidence below threshold"
              : "All extractions above threshold"
          }
          tone={needsReview > 0 ? "warn" : "ok"}
          visual={<MicroSparkline active={needsReview > 0} />}
        />
        <MetricCard
          className="lg:w-[18%]"
          label="Evidence gaps"
          value={evidenceGaps}
          caption={
            evidenceGaps > 0
              ? `${evidenceGaps} open ticket${evidenceGaps === 1 ? "" : "s"}`
              : "Every obligation is evidenced"
          }
          tone={evidenceGaps > 0 ? "risk" : "ok"}
          visual={<MicroCompletionRing gaps={evidenceGaps} />}
        />
        <MetricCard
          className="lg:w-[12%]"
          label="Propagation"
          value="1.2s"
          caption="Diff → blast radius"
          tone="info"
          visual={<MicroHeartbeatWave />}
        />
      </div>
    </section>
  )
}

/* ══════════════════════════════════════════════════════════════════════════
   TICKER
   ══════════════════════════════════════════════════════════════════════════ */

function StatusTicker({ utcTime }: { utcTime: string }) {
  const reduce = useReducedMotion()

  return (
    <div className="flex h-8 items-center justify-between border-b border-line-subtle px-6 font-mono text-[11px] uppercase tracking-wider text-fg-subtle">
      <div className="flex items-center gap-3">
        <span>Regulatory OS</span>
        <span aria-hidden className="text-fg-faint">
          |
        </span>
        <span className="flex items-center gap-1.5 text-ok">
          <span className="relative flex size-1.5" aria-hidden>
            {!reduce && (
              <span className="absolute inline-flex size-full animate-ping rounded-full bg-ok opacity-75" />
            )}
            <span className="relative inline-flex size-1.5 rounded-full bg-ok" />
          </span>
          Live
        </span>
      </div>

      <div className="flex items-center gap-4">
        <span className="hidden sm:inline">
          Engine: <span className="text-fg-muted">online</span>
        </span>
        <span aria-hidden className="hidden text-fg-faint sm:inline">
          |
        </span>
        {/* aria-hidden: a clock that re-announces every second is unusable
            with a screen reader, and it carries no compliance meaning. */}
        <span className="tnum text-fg-muted" aria-hidden>
          {utcTime || " "}
        </span>
      </div>
    </div>
  )
}

/* ══════════════════════════════════════════════════════════════════════════
   METRIC CARD
   ══════════════════════════════════════════════════════════════════════════ */

type Tone = "ok" | "warn" | "risk" | "info"

const TONE: Record<Tone, { text: string; dot: string }> = {
  ok: { text: "text-ok", dot: "bg-ok" },
  warn: { text: "text-warn", dot: "bg-warn" },
  risk: { text: "text-risk", dot: "bg-risk" },
  info: { text: "text-accent", dot: "bg-accent" },
}

function MetricCard({
  className = "",
  label,
  value,
  caption,
  tone,
  visual,
}: {
  className?: string
  label: string
  value: string | number
  caption: string
  tone: Tone
  visual: React.ReactNode
}) {
  const t = TONE[tone]

  return (
    <div
      className={`surface-interactive group flex flex-col justify-between p-4 ${className}`}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="eyebrow truncate">{label}</span>
        <span aria-hidden className={`size-1.5 shrink-0 rounded-full ${t.dot}`} />
      </div>

      <div className="mt-3 flex items-baseline justify-between gap-3">
        <motion.span
          key={String(value)}
          initial={{ opacity: 0.5 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.18, ease: [0.2, 0.8, 0.2, 1] }}
          className={`text-metric-lg ${t.text}`}
        >
          {value}
        </motion.span>
        <div className="shrink-0 self-center">{visual}</div>
      </div>

      <p className="mt-2 truncate text-body-sm text-fg-muted" title={caption}>
        {caption}
      </p>
    </div>
  )
}

/* ══════════════════════════════════════════════════════════════════════════
   MICRO VISUALS

   Every colour here is a `var(--token)` rather than a literal, so the strip
   follows the theme instead of staying dark on a light page.
   ══════════════════════════════════════════════════════════════════════════ */

const MicroSegmentBar = React.memo(function MicroSegmentBar({
  count,
  total,
}: {
  count: number
  total: number
}) {
  const reduce = useReducedMotion()
  return (
    <div className="flex items-center gap-1" aria-hidden>
      {Array.from({ length: total }).map((_, i) => (
        <motion.span
          key={i}
          initial={reduce ? false : { opacity: 0.3, scaleY: 0.6 }}
          animate={{ opacity: 1, scaleY: 1 }}
          transition={{ duration: 0.15, delay: i * 0.02 }}
          className={`h-4 w-1.5 rounded-[1px] ${i < count ? "bg-ok" : "bg-line"}`}
        />
      ))}
    </div>
  )
})

const MicroMatrixDots = React.memo(function MicroMatrixDots({
  count,
  total,
}: {
  count: number
  total: number
}) {
  return (
    <div className="grid w-[38px] grid-cols-5 gap-1" aria-hidden>
      {Array.from({ length: total }).map((_, i) => (
        <span
          key={i}
          className={`size-1.5 rounded-full ${i < count ? "bg-warn" : "bg-line"}`}
        />
      ))}
    </div>
  )
})

const MicroSparkline = React.memo(function MicroSparkline({
  active,
}: {
  active: boolean
}) {
  const reduce = useReducedMotion()
  const stroke = active ? "var(--warn)" : "var(--ok)"
  return (
    <svg width="56" height="22" viewBox="0 0 56 22" fill="none" aria-hidden>
      <defs>
        <linearGradient id="kpi-sparkline-fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={stroke} stopOpacity="0.22" />
          <stop offset="100%" stopColor={stroke} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path
        d="M 2 18 L 12 12 L 22 15 L 32 6 L 42 11 L 54 3 L 54 20 L 2 20 Z"
        fill="url(#kpi-sparkline-fill)"
      />
      <motion.path
        d="M 2 18 L 12 12 L 22 15 L 32 6 L 42 11 L 54 3"
        stroke={stroke}
        strokeWidth="1.75"
        strokeLinecap="round"
        strokeLinejoin="round"
        initial={reduce ? false : { pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: 0.6, ease: [0.2, 0.8, 0.2, 1] }}
      />
      <circle cx="54" cy="3" r="2" fill={stroke} />
    </svg>
  )
})

const MicroCompletionRing = React.memo(function MicroCompletionRing({
  gaps,
}: {
  gaps: number
}) {
  const reduce = useReducedMotion()
  const size = 26
  const strokeWidth = 2.5
  const center = size / 2
  const radius = center - strokeWidth
  const circumference = 2 * Math.PI * radius
  // Each gap eats 15% of the ring, capped at 75% so the arc never reads as
  // "nothing covered" no matter how large the backlog gets.
  const uncovered = gaps > 0 ? Math.min(gaps * 15, 75) : 0
  const dashOffset = (uncovered / 100) * circumference

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      className="-rotate-90"
      aria-hidden
    >
      <circle
        cx={center}
        cy={center}
        r={radius}
        stroke="var(--line)"
        strokeWidth={strokeWidth}
        fill="none"
      />
      <motion.circle
        cx={center}
        cy={center}
        r={radius}
        stroke={gaps > 0 ? "var(--risk)" : "var(--ok)"}
        strokeWidth={strokeWidth}
        strokeDasharray={circumference}
        strokeLinecap="round"
        fill="none"
        initial={reduce ? false : { strokeDashoffset: circumference }}
        animate={{ strokeDashoffset: dashOffset }}
        transition={{ duration: 0.5, ease: [0.2, 0.8, 0.2, 1] }}
      />
    </svg>
  )
})

const MicroHeartbeatWave = React.memo(function MicroHeartbeatWave() {
  const reduce = useReducedMotion()
  return (
    <svg width="48" height="20" viewBox="0 0 48 20" fill="none" aria-hidden>
      <motion.path
        d="M 0 10 L 12 10 L 16 3 L 20 17 L 24 6 L 28 12 L 32 10 L 48 10"
        stroke="var(--accent)"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        initial={reduce ? false : { pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: 0.7, ease: [0.2, 0.8, 0.2, 1] }}
      />
      {!reduce && (
        <circle
          cx="44"
          cy="10"
          r="2"
          fill="var(--accent)"
          className="animate-ping opacity-75"
        />
      )}
      <circle cx="44" cy="10" r="1.5" fill="var(--accent)" />
    </svg>
  )
})

/* ══════════════════════════════════════════════════════════════════════════
   SKELETON - matches the strip's geometry so nothing shifts on load.
   ══════════════════════════════════════════════════════════════════════════ */

export function ExecutiveKpiSkeleton() {
  return (
    <section
      aria-label="Loading compliance posture"
      aria-busy
      className="w-full shrink-0 animate-pulse border-b border-line-subtle bg-canvas"
    >
      <div className="flex h-8 items-center justify-between border-b border-line-subtle px-6">
        <div className="h-3 w-48 rounded-sm bg-elevated" />
        <div className="h-3 w-32 rounded-sm bg-elevated" />
      </div>

      <div className="grid grid-cols-1 gap-3 p-3.5 sm:grid-cols-2 md:grid-cols-3 lg:flex">
        {Array.from({ length: 5 }).map((_, i) => (
          <div
            key={i}
            className="surface flex flex-col justify-between p-4 lg:flex-1"
          >
            <div className="flex items-center justify-between">
              <div className="h-3 w-24 rounded-sm bg-elevated" />
              <div className="size-1.5 rounded-full bg-elevated" />
            </div>
            <div className="my-3 flex items-baseline justify-between">
              <div className="h-8 w-16 rounded-sm bg-elevated" />
              <div className="h-4 w-12 rounded-sm bg-elevated" />
            </div>
            <div className="h-3 w-32 rounded-sm bg-elevated" />
          </div>
        ))}
      </div>
    </section>
  )
}
