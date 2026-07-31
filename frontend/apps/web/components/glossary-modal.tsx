"use client"

import { X, BookOpen } from "lucide-react"

import { useDialog } from "@/components/use-dialog"
import { Button } from "@workspace/ui/components/button"

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
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 backdrop-blur-sm p-4">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Glossary — in plain English"
        tabIndex={-1}
        className="flex max-h-[85vh] w-[580px] max-w-full flex-col overflow-hidden rounded-2xl border border-line bg-surface shadow-2xl outline-none"
      >
        <header className="flex items-center justify-between border-b border-line px-6 py-4 bg-surface">
          <div className="flex items-center gap-2.5">
            <BookOpen className="size-5 text-brand" />
            <h2 className="font-display text-lg font-bold">Glossary — Plain English Guide</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1 text-text-dim hover:text-foreground hover:bg-cream-200/60 transition-colors"
            aria-label="Close"
          >
            <X className="size-5" />
          </button>
        </header>
        <dl className="min-h-0 flex-1 space-y-4 overflow-auto p-6 text-sm divide-y divide-line/40">
          {TERMS.map((t) => (
            <div key={t.term} className="pt-3 first:pt-0 space-y-1">
              <dt className="font-semibold text-foreground text-xs uppercase tracking-wide font-sans">{t.term}</dt>
              <dd className="leading-relaxed text-text-dim text-xs">{t.def}</dd>
            </div>
          ))}
        </dl>
        <footer className="border-t border-line px-6 py-3 bg-surface flex justify-end">
          <Button variant="default" size="sm" onClick={onClose}>
            Close
          </Button>
        </footer>
      </div>
    </div>
  )
}
