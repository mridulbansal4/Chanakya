"use client"

import * as React from "react"
import { motion, useReducedMotion } from "framer-motion"
import { Check } from "lucide-react"

// Subtle, enterprise-grade motion primitives for the amendment simulation.
// Everything honours prefers-reduced-motion (jumps straight to the end state).

/** Fade + slight rise on mount. */
export function Reveal({
  children,
  delay = 0,
  y = 8,
  className,
}: {
  children: React.ReactNode
  delay?: number
  y?: number
  className?: string
}) {
  const reduce = useReducedMotion()
  return (
    <motion.div
      className={className}
      initial={reduce ? false : { opacity: 0, y }}
      animate={{ opacity: 1, y: 0 }}
      transition={reduce ? { duration: 0 } : { duration: 0.35, delay, ease: "easeOut" }}
    >
      {children}
    </motion.div>
  )
}

/** Count an integer up to `target` once `active`. */
export function useCountUp(target: number, active = true, ms = 900): number {
  const reduce = useReducedMotion()
  const [v, setV] = React.useState(0)
  React.useEffect(() => {
    if (!active) return
    if (reduce) {
      setV(target)
      return
    }
    let raf = 0
    const start = performance.now()
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / ms)
      const eased = 1 - Math.pow(1 - t, 3)
      setV(Math.round(target * eased))
      if (t < 1) raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [target, active, reduce, ms])
  return v
}

/**
 * Reveal `count` items one after another on a timer; returns how many are shown
 * and fires `onDone` once when all are visible. Used for the pipeline, the
 * execution log, evidence, and workflow lists.
 */
export function useSequence(
  count: number,
  {
    intervalMs = 650,
    active = true,
    onDone,
  }: { intervalMs?: number; active?: boolean; onDone?: () => void } = {},
): number {
  const reduce = useReducedMotion()
  const [shown, setShown] = React.useState(0)
  const done = React.useRef(false)

  React.useEffect(() => {
    if (!active) return
    done.current = false
    if (reduce) {
      setShown(count)
      return
    }
    setShown(0)
    let i = 0
    const id = setInterval(() => {
      i += 1
      setShown(i)
      if (i >= count) clearInterval(id)
    }, intervalMs)
    return () => clearInterval(id)
  }, [count, intervalMs, active, reduce])

  React.useEffect(() => {
    if (shown >= count && count > 0 && !done.current) {
      done.current = true
      onDone?.()
    }
  }, [shown, count, onDone])

  return shown
}

/** A checklist row that shows a spinner→check as it becomes "done". */
export function CheckRow({
  label,
  done,
  running,
}: {
  label: React.ReactNode
  done: boolean
  running?: boolean
}) {
  return (
    <Reveal className="flex items-center gap-3 rounded-lg border border-line bg-surface px-4 py-2.5">
      <span
        className={`grid size-5 shrink-0 place-items-center rounded-full border ${
          done ? "border-ok bg-ok text-white" : "border-line text-text-dim"
        }`}
      >
        {done ? (
          <Check className="size-3" />
        ) : running ? (
          <span className="size-2 animate-pulse rounded-full bg-warn" />
        ) : (
          <span className="size-1.5 rounded-full bg-line" />
        )}
      </span>
      <span className={`text-sm ${done ? "text-foreground" : "text-text-dim"}`}>{label}</span>
      {running && !done && (
        <span className="ml-auto text-[11px] tracking-wide text-text-dim uppercase">Running…</span>
      )}
      {done && (
        <span className="ml-auto text-[11px] tracking-wide text-ok uppercase">Done</span>
      )}
    </Reveal>
  )
}

/** The 11-stage progress rail across the top of the simulation. */
export const STAGE_LABELS = [
  "Inbox",
  "Processing",
  "Clause Diff",
  "Obligations",
  "Graph",
  "Blast Radius",
  "Workflows",
  "Approval",
  "Execution",
  "Evidence",
  "Audit Pack",
]

export function StageRail({ step }: { step: number }) {
  return (
    <div className="flex items-center gap-1 overflow-x-auto border-b border-line bg-surface px-6 py-2.5">
      {STAGE_LABELS.map((label, i) => {
        const state = i < step ? "done" : i === step ? "current" : "future"
        return (
          <React.Fragment key={label}>
            <div className="flex shrink-0 items-center gap-1.5">
              <span
                className={`grid size-4 place-items-center rounded-full text-[9px] font-medium ${
                  state === "done"
                    ? "bg-ok text-white"
                    : state === "current"
                      ? "bg-ink text-on-ink"
                      : "border border-line text-text-dim"
                }`}
              >
                {state === "done" ? <Check className="size-2.5" /> : i + 1}
              </span>
              <span
                className={`text-[11px] whitespace-nowrap ${
                  state === "current"
                    ? "font-medium text-foreground"
                    : state === "done"
                      ? "text-text-dim"
                      : "text-text-dim/60"
                }`}
              >
                {label}
              </span>
            </div>
            {i < STAGE_LABELS.length - 1 && (
              <span className={`h-px w-4 shrink-0 ${i < step ? "bg-ok" : "bg-line"}`} />
            )}
          </React.Fragment>
        )
      })}
    </div>
  )
}

/** Primary / secondary CTA buttons (editorial). */
export function PrimaryButton({
  children,
  onClick,
  disabled,
  tone = "ink",
}: {
  children: React.ReactNode
  onClick?: () => void
  disabled?: boolean
  tone?: "ink" | "ok" | "risk"
}) {
  const tones: Record<string, string> = {
    ink: "bg-ink text-on-ink",
    ok: "bg-ok text-white",
    risk: "bg-risk text-white",
  }
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-medium transition-opacity disabled:opacity-40 ${tones[tone]}`}
    >
      {children}
    </button>
  )
}

export function GhostButton({
  children,
  onClick,
}: {
  children: React.ReactNode
  onClick?: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-2 rounded-full border border-line px-4 py-2 text-sm font-medium text-foreground hover:bg-cream-200"
    >
      {children}
    </button>
  )
}

/** A section title used at the top of each simulation screen. */
export function ScreenTitle({
  eyebrow,
  title,
  description,
}: {
  eyebrow: string
  title: string
  description?: string
}) {
  return (
    <div className="mb-5">
      <div className="eyebrow mb-1">{eyebrow}</div>
      <h2 className="font-display text-2xl leading-tight tracking-tight">{title}</h2>
      {description && <p className="mt-1 max-w-2xl text-sm text-text-dim">{description}</p>}
    </div>
  )
}
