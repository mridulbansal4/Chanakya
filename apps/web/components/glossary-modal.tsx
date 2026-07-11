"use client"

import { X } from "lucide-react"

import { useDialog } from "@/components/use-dialog"

const TERMS: Array<{ term: string; def: string }> = [
  {
    term: "Obligation type (Required / Prohibited / Permitted)",
    def: "What the rule demands. Required = MUST do it, Prohibited = MUST NOT do it, Permitted = MAY (optional). The MUST / MUST NOT tags are the underlying legal-modal codes.",
  },
  {
    term: "AI confidence",
    def: "How sure the AI is that it read the clause correctly. Anything below 75% is automatically routed to you for review rather than trusted.",
  },
  {
    term: "Citation / source sentence",
    def: "The exact sentence in the regulation that an obligation came from. Every obligation must carry one, so nothing is invented.",
  },
  {
    term: "Blast Radius",
    def: "A preview of everything a proposed regulation change would affect — the obligations, controls, and evidence downstream — so you can see the impact before accepting it.",
  },
  {
    term: "Evidence gap",
    def: "An obligation that has no supporting evidence from your firm's systems. Each gap is turned into a draft remediation ticket.",
  },
  {
    term: "Sign-off (Ed25519 signature)",
    def: "Your cryptographic approval of an obligation. It proves a human approved this exact wording; if the obligation is later altered, the signature stops verifying.",
  },
  {
    term: "Policy (Rego / OPA)",
    def: "An automated, deterministic compliance check compiled from a signed obligation. It evaluates your firm's data and returns a pass/fail with a full trace.",
  },
  {
    term: "Staged enforcement (audit → soft → hard)",
    def: "How strictly a policy acts. 'Audit' only records the result, 'soft' warns, 'hard' blocks. Policies start at audit — nothing is blocked before you promote it.",
  },
  {
    term: "As-of date (bi-temporal)",
    def: "Every screen can reconstruct the compliance state as it was on any past date, not just today — essential for an audit. Set it with the date control (top right).",
  },
  {
    term: "Regulator Feed",
    def: "A machine-readable export of your obligations, with the source text and sign-off behind each, that a regulator's own systems (SupTech) can consume directly.",
  },
]

export function GlossaryModal({ onClose }: { onClose: () => void }) {
  const dialogRef = useDialog<HTMLDivElement>(onClose)
  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-background/70 p-4">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Glossary — in plain English"
        tabIndex={-1}
        className="hairline flex max-h-[85vh] w-[560px] max-w-full flex-col overflow-hidden rounded-lg bg-surface outline-none"
      >
        <header className="flex items-center justify-between border-b border-line px-5 py-3">
          <h2 className="font-display text-lg">Glossary — in plain English</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground"
            aria-label="Close"
          >
            <X className="size-4" />
          </button>
        </header>
        <dl className="min-h-0 flex-1 space-y-4 overflow-auto p-5 text-sm">
          {TERMS.map((t) => (
            <div key={t.term}>
              <dt className="font-medium text-foreground">{t.term}</dt>
              <dd className="mt-0.5 leading-relaxed text-muted-foreground">{t.def}</dd>
            </div>
          ))}
        </dl>
      </div>
    </div>
  )
}
