"use client"

import * as React from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Check, PenLine, ShieldCheck, X } from "lucide-react"

import { DeonticBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { useDialog } from "@/components/use-dialog"
import { Button } from "@workspace/ui/components/button"
import { formatDeadline } from "@/lib/format"
import {
  postSignoff,
  type DeonticType,
  type Obligation,
  type SignoffCorrections,
} from "@/lib/api"

const MIN_JUSTIFICATION = 20
const DEONTICS: DeonticType[] = ["MUST", "MUST_NOT", "MAY"]

type Step = 1 | 2 | 3 | 4

export function SignoffModal({
  obligation,
  onClose,
}: {
  obligation: Obligation
  onClose: () => void
}) {
  const qc = useQueryClient()
  const [step, setStep] = React.useState<Step>(1)
  const [action, setAction] = React.useState<"approve" | "reject">("approve")
  const [signedBy, setSignedBy] = React.useState("")
  const [justification, setJustification] = React.useState("")
  const [correct, setCorrect] = React.useState(false)
  const [deontic, setDeontic] = React.useState<DeonticType>(obligation.deontic_type)
  const [deadline, setDeadline] = React.useState(obligation.deadline)

  const mutation = useMutation({
    mutationFn: () => {
      let corrections: SignoffCorrections | undefined
      if (action === "approve" && correct) {
        corrections = {}
        if (deontic !== obligation.deontic_type) corrections.deontic_type = deontic
        if (deadline !== obligation.deadline) corrections.deadline = deadline
        if (Object.keys(corrections).length === 0) corrections = undefined
      }
      return postSignoff({
        obligation_id: obligation.id,
        action,
        signed_by: signedBy.trim(),
        justification: justification.trim(),
        corrections,
      })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["review-queue"] })
      qc.invalidateQueries({ queryKey: ["posture"] })
      qc.invalidateQueries({ queryKey: ["obligations"] })
      qc.invalidateQueries({ queryKey: ["graph"] })
      setStep(4)
    },
  })

  const justificationValid = justification.trim().length >= MIN_JUSTIFICATION
  const signerValid = signedBy.trim().length > 0
  const dialogRef = useDialog<HTMLDivElement>(onClose)

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-scrim backdrop-blur-sm p-4">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={`Sign off clause ${obligation.clause_ref}`}
        tabIndex={-1}
        className="flex max-h-[88vh] w-full max-w-[580px] mx-4 sm:mx-auto flex-col overflow-hidden rounded-lg border border-line bg-surface shadow-elev-3 outline-none"
      >
        <header className="flex items-center justify-between border-b border-line px-6 py-4 bg-surface">
          <div className="flex items-center gap-2.5">
            <ShieldCheck className="size-5 text-ok" />
            <span className="font-display text-base font-bold text-foreground">
              Sign-Off: Clause {obligation.clause_ref}
            </span>
          </div>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-text-dim hover:text-foreground hover:bg-cream-200/60 transition-colors"
            aria-label="Close"
          >
            <X className="size-5" />
          </button>
        </header>

        {/* Step indicator */}
        {step < 4 && (
          <div className="flex items-center gap-3 border-b border-line bg-cream/40 px-6 py-2.5 text-xs text-text-dim font-medium">
            {["Review Source", "Decision & Justification", "Cryptographic Signing"].map((label, i) => (
              <React.Fragment key={label}>
                <span className={step === i + 1 ? "tnum font-bold text-foreground" : "tnum"}>
                  {i + 1}. {label}
                </span>
                {i < 2 && <span className="text-line">→</span>}
              </React.Fragment>
            ))}
          </div>
        )}

        <div className="min-h-0 flex-1 overflow-auto p-6 space-y-4">
          {step === 1 && <StepReview obligation={obligation} />}

          {step === 2 && (
            <div className="space-y-4 text-sm">
              <div className="flex gap-3">
                <DecisionButton
                  active={action === "approve"}
                  onClick={() => setAction("approve")}
                  tone="verified"
                >
                  Approve Extraction
                </DecisionButton>
                <DecisionButton
                  active={action === "reject"}
                  onClick={() => setAction("reject")}
                  tone="danger"
                >
                  Reject Extraction
                </DecisionButton>
              </div>

              {action === "approve" && (
                <div className="rounded-xl border border-line p-3 bg-cream/30">
                  <label className="flex items-center gap-2 text-xs font-semibold text-foreground cursor-pointer">
                    <input
                      type="checkbox"
                      checked={correct}
                      onChange={(e) => setCorrect(e.target.checked)}
                      className="rounded border-line"
                    />
                    Correct extracted fields before signing
                  </label>
                  {correct && (
                    <div className="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-3">
                      <Labeled label="Deontic Duty">
                        <select
                          value={deontic}
                          onChange={(e) => setDeontic(e.target.value as DeonticType)}
                          className="w-full rounded-xl border border-line bg-background px-3 py-1.5 text-xs"
                        >
                          {DEONTICS.map((d) => (
                            <option key={d} value={d}>{d}</option>
                          ))}
                        </select>
                      </Labeled>
                      <Labeled label="Deadline">
                        <input
                          value={deadline}
                          onChange={(e) => setDeadline(e.target.value)}
                          placeholder="e.g. P30D"
                          className="w-full rounded-xl border border-line bg-background px-3 py-1.5 text-xs font-mono"
                        />
                      </Labeled>
                    </div>
                  )}
                </div>
              )}

              <Labeled label="Compliance Officer Name & Role">
                <input
                  value={signedBy}
                  onChange={(e) => setSignedBy(e.target.value)}
                  placeholder="e.g. Priya Menon (Chief Compliance Officer)"
                  className="w-full rounded-xl border border-line bg-background px-3 py-2 text-xs font-medium"
                />
              </Labeled>

              <Labeled label={`Justification (Required, min ${MIN_JUSTIFICATION} characters)`}>
                <textarea
                  value={justification}
                  onChange={(e) => setJustification(e.target.value)}
                  rows={4}
                  placeholder="Provide substantive reasoning for this decision. This statement is cryptographically bound into the audit feed."
                  className="w-full resize-none rounded-xl border border-line bg-background p-3 text-xs leading-relaxed"
                />
                <div className="tnum mt-1 text-right text-[11px] font-mono text-text-dim">
                  {justification.trim().length}/{MIN_JUSTIFICATION}
                </div>
              </Labeled>
            </div>
          )}

          {step === 3 && (
            <div className="space-y-4 text-sm">
              <p className="text-text-dim leading-relaxed">
                {action === "approve" ? (
                  <>
                    You are about to <span className="font-bold text-ok">approve</span> and produce an Ed25519 cryptographic signature for this regulatory obligation.
                  </>
                ) : (
                  <>
                    You are about to <span className="font-bold text-risk">reject</span> this AI extraction. This decision will be logged in the audit trail.
                  </>
                )}
              </p>
              <div className="rounded-lg border border-line bg-cream/30 p-4 space-y-2">
                <Summary label="Obligation" value={`Clause ${obligation.clause_ref} - ${obligation.clause_heading}`} />
                <Summary label="Signer" value={signedBy} />
                <Summary label="Justification" value={justification} />
              </div>
              {mutation.isError && (
                <p className="text-xs font-semibold text-risk">Sign-off request failed. Please try again.</p>
              )}
            </div>
          )}

          {step === 4 && mutation.data && (
            <div className="space-y-4 text-sm">
              <div className="flex items-center gap-2.5 text-ok font-bold text-base">
                <Check className="size-6 text-ok" />
                <span>
                  {mutation.data.signoff.action === "approve"
                    ? "Cryptographically Signed & Approved"
                    : "Rejection Successfully Recorded"}
                </span>
              </div>
              {mutation.data.signoff.action === "approve" && (
                <div className="rounded-lg border border-ok/30 bg-ok/5 p-4 space-y-2">
                  <Summary label="Signature Verification" value={mutation.data.verified ? "✓ Ed25519 Signature Verified" : "-"} />
                  <Mono label="Obligation Hash (sha256)" value={mutation.data.signoff.obligation_hash} />
                  <Mono label="Ed25519 Signature" value={mutation.data.signoff.signature ?? ""} />
                  <Mono label="Public Key" value={mutation.data.signoff.public_key ?? ""} />
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer nav */}
        <footer className="flex items-center justify-between border-t border-line px-6 py-4 bg-surface">
          {step === 4 ? (
            <Button variant="default" onClick={onClose} className="ml-auto">
              Done
            </Button>
          ) : (
            <>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => (step === 1 ? onClose() : setStep((step - 1) as Step))}
              >
                {step === 1 ? "Cancel" : "Back"}
              </Button>
              {step < 3 ? (
                <Button
                  variant="default"
                  size="sm"
                  disabled={step === 2 && (!justificationValid || !signerValid)}
                  onClick={() => setStep((step + 1) as Step)}
                >
                  Continue
                </Button>
              ) : (
                <Button
                  variant="default"
                  size="default"
                  isLoading={mutation.isPending}
                  loadingText="Signing with Ed25519…"
                  onClick={() => mutation.mutate()}
                >
                  <PenLine className="size-4" />
                  <span>
                    {action === "approve" ? "Sign with Ed25519" : "Record Rejection"}
                  </span>
                </Button>
              )}
            </>
          )}
        </footer>
      </div>
    </div>
  )
}

function StepReview({ obligation }: { obligation: Obligation }) {
  return (
    <div className="space-y-4 text-sm">
      <div className="flex items-center gap-2">
        <span className="tnum font-bold text-primary bg-cream-200 px-2 py-0.5 rounded text-xs">
          Clause {obligation.clause_ref}
        </span>
        <DeonticBadge deontic={obligation.deontic_type} />
        <ConfidenceMeter value={obligation.confidence} />
      </div>
      <h3 className="font-display text-lg font-bold">{obligation.clause_heading}</h3>
      <div>
        <div className="eyebrow mb-1">Exact Regulatory Sentence</div>
        <blockquote className="border-l-2 border-ok pl-3 text-xs leading-relaxed text-text-dim italic bg-cream/30 py-2 pr-3 rounded-r-lg">
          &quot;{obligation.source_sentence}&quot;
        </blockquote>
      </div>
      <dl className="grid grid-cols-2 gap-3 text-xs border-t border-line pt-3">
        <Summary label="Bearer" value={obligation.bearer} />
        <Summary
          label="Deadline"
          value={obligation.deadline ? formatDeadline(obligation.deadline) : "None"}
        />
      </dl>
    </div>
  )
}

function DecisionButton({
  active,
  onClick,
  tone,
  children,
}: {
  active: boolean
  onClick: () => void
  tone: "verified" | "danger"
  children: React.ReactNode
}) {
  const color =
    tone === "verified"
      ? "border-ok bg-ok/10 text-ok font-bold shadow-xs"
      : "border-risk bg-risk/10 text-risk font-bold shadow-xs"
  return (
    <button
      onClick={onClick}
      className={`flex-1 rounded-xl border p-3 text-xs transition-all ${
        active ? color : "border-line bg-background text-text-dim hover:text-foreground"
      }`}
    >
      {children}
    </button>
  )
}

function Labeled({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="eyebrow">{label}</span>
      <div className="mt-1">{children}</div>
    </label>
  )
}

function Summary({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="eyebrow">{label}</div>
      <div className="text-xs font-semibold text-foreground mt-0.5">{value}</div>
    </div>
  )
}

function Mono({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="eyebrow">{label}</div>
      <div className="tnum break-all text-xs font-mono font-semibold text-ok mt-0.5">{value}</div>
    </div>
  )
}
