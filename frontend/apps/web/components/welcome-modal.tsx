"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import {
  ClipboardCheck,
  FileSearch,
  ScrollText,
  ShieldCheck,
  X,
  ArrowRight,
} from "lucide-react"

import { useDialog } from "@/components/use-dialog"
import { Button } from "@workspace/ui/components/button"

interface Step {
  icon: React.ReactNode
  title: string
  body: React.ReactNode
  cta?: { label: string; href: string }
}

const STEPS: Step[] = [
  {
    icon: <ScrollText className="size-6 text-primary" />,
    title: "Welcome to CHANAKYA SupTech OS",
    body: (
      <>
        CHANAKYA converts complex SEBI circulars into a live, fully auditable compliance operating system.
        The AI proposes obligations with source citations — <strong>you</strong>{" "}
        approve them with cryptographic signatures, and a deterministic engine enforces them.
      </>
    ),
  },
  {
    icon: <ClipboardCheck className="size-6 text-warn" />,
    title: "Your Daily Action Hub: Review Queue",
    body: (
      <>
        Extracted and low-confidence obligations wait here for human review. Approve,
        correct, or reject each obligation before automated policies activate.
      </>
    ),
    cta: { label: "Open Review Queue", href: "/review" },
  },
  {
    icon: <FileSearch className="size-6 text-warn" />,
    title: "Regulation Change Simulation: Blast Radius",
    body: (
      <>
        Preview the exact downstream impact of regulatory circular amendments — mapping affected obligations, controls, and evidence sources before enforcing changes.
      </>
    ),
    cta: { label: "Explore Blast Radius Simulator", href: "/amendments" },
  },
  {
    icon: <ShieldCheck className="size-6 text-ok" />,
    title: "Complete Lineage Audit Trail",
    body: (
      <>
        Trace every obligation from <strong>Review → Cryptographic Ed25519 Sign-off → OPA Policy Code → Audit Trail → Machine-Readable Regulator Feed</strong>.
      </>
    ),
    cta: { label: "Launch Overview Dashboard", href: "/" },
  },
]

export function WelcomeModal({ onClose }: { onClose: () => void }) {
  const router = useRouter()
  const [step, setStep] = React.useState(0)
  const s = STEPS[step]!
  const last = step === STEPS.length - 1

  const go = (href: string) => {
    onClose()
    router.push(href)
  }
  const dialogRef = useDialog<HTMLDivElement>(onClose)

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 backdrop-blur-sm p-4">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Getting started with CHANAKYA"
        tabIndex={-1}
        className="w-[540px] max-w-full overflow-hidden rounded-2xl border border-line bg-surface shadow-2xl outline-none"
      >
        <header className="flex items-center justify-between border-b border-line px-6 py-4 bg-surface">
          <span className="eyebrow">
            Getting Started · Step {step + 1} of {STEPS.length}
          </span>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1 text-text-dim hover:text-foreground hover:bg-cream-200/60 transition-colors"
            aria-label="Skip"
          >
            <X className="size-5" />
          </button>
        </header>

        <div className="p-6 space-y-4">
          <div className="flex size-12 items-center justify-center rounded-2xl bg-cream-200/80 shadow-inner">
            {s.icon}
          </div>
          <h2 className="font-display text-2xl font-bold tracking-tight text-foreground">{s.title}</h2>
          <p className="text-xs leading-relaxed text-text-dim">{s.body}</p>
          {s.cta && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => go(s.cta!.href)}
              className="mt-2"
            >
              <span>{s.cta.label}</span>
              <ArrowRight className="size-3.5" />
            </Button>
          )}

          <div className="mt-6 flex items-center gap-2 pt-2">
            {STEPS.map((_, i) => (
              <span
                key={i}
                className={`h-1.5 rounded-full transition-all ${
                  i === step ? "w-6 bg-foreground" : "w-1.5 bg-line"
                }`}
              />
            ))}
          </div>
        </div>

        <footer className="flex items-center justify-between border-t border-line px-6 py-4 bg-surface">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => (step === 0 ? onClose() : setStep(step - 1))}
          >
            {step === 0 ? "Skip Intro" : "Back"}
          </Button>
          <Button
            variant="default"
            size="default"
            onClick={() => (last ? onClose() : setStep(step + 1))}
          >
            {last ? "Start Using CHANAKYA" : "Next"}
          </Button>
        </footer>
      </div>
    </div>
  )
}
