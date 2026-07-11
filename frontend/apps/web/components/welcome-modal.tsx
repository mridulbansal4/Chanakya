"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import {
  ClipboardCheck,
  FileSearch,
  ScrollText,
  ShieldCheck,
  X,
} from "lucide-react"

import { useDialog } from "@/components/use-dialog"

interface Step {
  icon: React.ReactNode
  title: string
  body: React.ReactNode
  cta?: { label: string; href: string }
}

const STEPS: Step[] = [
  {
    icon: <ScrollText className="size-5 text-primary" />,
    title: "Welcome to CHANAKYA",
    body: (
      <>
        CHANAKYA turns SEBI regulations into a live, auditable compliance system.
        The AI only ever <em>proposes</em> obligations as data — <strong>you</strong>{" "}
        approve them, and a deterministic engine enforces them. Nothing is acted
        on without your sign-off.
      </>
    ),
  },
  {
    icon: <ClipboardCheck className="size-5 text-warn" />,
    title: "Your daily work: the Review Queue",
    body: (
      <>
        New and low-confidence obligations wait here for your judgement. Approve,
        correct, or reject each one. Nothing is enforced until you sign — this is
        your inbox.
      </>
    ),
    cta: { label: "Open the Review Queue", href: "/review" },
  },
  {
    icon: <FileSearch className="size-5 text-warn" />,
    title: "When a regulation changes",
    body: (
      <>
        Use <strong>Blast Radius</strong> to preview exactly what an amendment
        would affect — which obligations, controls, and evidence — <em>before</em>{" "}
        you accept it. No more guessing what a circular change touches.
      </>
    ),
    cta: { label: "See Blast Radius", href: "/amendments" },
  },
  {
    icon: <ShieldCheck className="size-5 text-verified" />,
    title: "From text to enforcement — fully traceable",
    body: (
      <>
        The flow is: <strong>Review → Sign-off</strong> (a cryptographic signature
        proving you approved this exact obligation) <strong>→ Policy</strong> (an
        automated check) <strong>→ Audit</strong> (reconstruct the trail as of any
        date) <strong>→ Regulator Feed</strong>. Every claim is cited and
        reproducible.
      </>
    ),
    cta: { label: "Go to the Overview", href: "/" },
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
    <div className="fixed inset-0 z-50 grid place-items-center bg-background/70 p-4">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Getting started with CHANAKYA"
        tabIndex={-1}
        className="hairline w-[520px] max-w-full overflow-hidden rounded-lg bg-surface outline-none"
      >
        <header className="flex items-center justify-between border-b border-line px-5 py-3">
          <span className="text-[11px] tracking-widest text-muted-foreground uppercase">
            Getting started · {step + 1} of {STEPS.length}
          </span>
          <button
            type="button"
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground"
            aria-label="Skip"
          >
            <X className="size-4" />
          </button>
        </header>

        <div className="p-6">
          <div className="flex items-center gap-2">{s.icon}</div>
          <h2 className="font-display mt-2 text-2xl leading-tight">{s.title}</h2>
          <p className="mt-3 text-sm leading-relaxed text-muted-foreground">{s.body}</p>
          {s.cta && (
            <button
              type="button"
              onClick={() => go(s.cta!.href)}
              className="mt-4 text-sm text-primary hover:underline"
            >
              {s.cta.label} →
            </button>
          )}

          <div className="mt-6 flex items-center gap-1.5">
            {STEPS.map((_, i) => (
              <span
                key={i}
                className={`h-1.5 rounded-full transition-all ${
                  i === step ? "w-5 bg-primary" : "w-1.5 bg-line"
                }`}
              />
            ))}
          </div>
        </div>

        <footer className="flex items-center justify-between border-t border-line px-5 py-3">
          <button
            type="button"
            onClick={() => (step === 0 ? onClose() : setStep(step - 1))}
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            {step === 0 ? "Skip" : "Back"}
          </button>
          <button
            type="button"
            onClick={() => (last ? onClose() : setStep(step + 1))}
            className="hairline rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground"
          >
            {last ? "Start using CHANAKYA" : "Next"}
          </button>
        </footer>
      </div>
    </div>
  )
}
