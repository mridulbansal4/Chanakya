"use client"

import * as React from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Check, PenLine, ShieldCheck, X } from "lucide-react"

import { DeonticBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { useDialog } from "@/components/use-dialog"
import { formatDeadline } from "@/lib/format"
import {
  postSignoff,
  type DeonticType,
  type Obligation,
  type SignoffCorrections,
} from "@/lib/api"

const MIN_JUSTIFICATION = 20
const DEONTICS: DeonticType[] = ["MUST", "MUST_NOT", "MAY"]

type Step = 1 | 2 | 3 | 4 // 4 = result

/**
 * SignoffModal is the deliberate, multi-step human sign-off. Friction is a
 * feature: the reviewer must (1) review the obligation against its source
 * sentence, (2) choose approve/reject and optionally correct, (3) type a
 * substantive justification, before an Ed25519 signature is produced.
 */
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
    <div className="fixed inset-0 z-50 grid place-items-center bg-background/70 p-4">
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={`Sign off clause ${obligation.clause_ref}`}
        tabIndex={-1}
        className="hairline flex max-h-[85vh] w-[560px] flex-col overflow-hidden rounded-lg bg-surface outline-none"
      >
        <header className="flex items-center justify-between border-b border-line px-5 py-3">
          <div className="flex items-center gap-2">
            <ShieldCheck className="size-4 text-verified" />
            <span className="text-sm font-medium">Sign-off — clause {obligation.clause_ref}</span>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground" aria-label="Close">
            <X className="size-4" />
          </button>
        </header>

        {/* Step indicator */}
        {step < 4 && (
          <div className="flex items-center gap-2 border-b border-line px-5 py-2 text-[11px] text-muted-foreground">
            {["Review", "Decide", "Sign"].map((label, i) => (
              <React.Fragment key={label}>
                <span className={step === i + 1 ? "tnum text-foreground" : "tnum"}>
                  {i + 1}. {label}
                </span>
                {i < 2 && <span className="text-line">→</span>}
              </React.Fragment>
            ))}
          </div>
        )}

        <div className="min-h-0 flex-1 overflow-auto p-5">
          {step === 1 && (
            <StepReview obligation={obligation} />
          )}

          {step === 2 && (
            <div className="space-y-4 text-sm">
              <div className="flex gap-2">
                <DecisionButton
                  active={action === "approve"}
                  onClick={() => setAction("approve")}
                  tone="verified"
                >
                  Approve
                </DecisionButton>
                <DecisionButton
                  active={action === "reject"}
                  onClick={() => setAction("reject")}
                  tone="danger"
                >
                  Reject
                </DecisionButton>
              </div>

              {action === "approve" && (
                <div>
                  <label className="flex items-center gap-2 text-xs text-muted-foreground">
                    <input
                      type="checkbox"
                      checked={correct}
                      onChange={(e) => setCorrect(e.target.checked)}
                    />
                    Correct the obligation before signing
                  </label>
                  {correct && (
                    <div className="mt-2 grid grid-cols-2 gap-3">
                      <Labeled label="Deontic">
                        <select
                          value={deontic}
                          onChange={(e) => setDeontic(e.target.value as DeonticType)}
                          className="hairline w-full rounded bg-background px-2 py-1 text-sm [color-scheme:light]"
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
                          className="hairline w-full rounded bg-background px-2 py-1 text-sm"
                        />
                      </Labeled>
                    </div>
                  )}
                </div>
              )}

              <Labeled label="Signed by (reviewer)">
                <input
                  value={signedBy}
                  onChange={(e) => setSignedBy(e.target.value)}
                  placeholder="Your name and role"
                  className="hairline w-full rounded bg-background px-2.5 py-1.5 text-sm"
                />
              </Labeled>

              <Labeled label={`Justification (required, min ${MIN_JUSTIFICATION} chars)`}>
                <textarea
                  value={justification}
                  onChange={(e) => setJustification(e.target.value)}
                  rows={4}
                  placeholder="Why is this decision correct? This is recorded and signed."
                  className="hairline w-full resize-none rounded bg-background px-2.5 py-2 text-sm leading-relaxed"
                />
                <div className="tnum mt-1 text-right text-[11px] text-muted-foreground">
                  {justification.trim().length}/{MIN_JUSTIFICATION}
                </div>
              </Labeled>
            </div>
          )}

          {step === 3 && (
            <div className="space-y-3 text-sm">
              <p className="text-muted-foreground">
                {action === "approve" ? (
                  <>
                    You are about to <span className="text-verified">approve</span> and
                    cryptographically sign this obligation with an Ed25519 signature.
                  </>
                ) : (
                  <>
                    You are about to <span className="text-danger">reject</span> this
                    extraction. This is recorded with your justification.
                  </>
                )}
              </p>
              <Summary label="Obligation" value={`${obligation.clause_ref} — ${obligation.clause_heading}`} />
              <Summary label="Signed by" value={signedBy} />
              <Summary label="Justification" value={justification} />
              {mutation.isError && (
                <p className="text-danger">Sign-off failed. Please try again.</p>
              )}
            </div>
          )}

          {step === 4 && mutation.data && (
            <div className="space-y-3 text-sm">
              <div className="flex items-center gap-2 text-verified">
                <Check className="size-5" />
                <span className="font-medium">
                  {mutation.data.signoff.action === "approve"
                    ? "Approved & signed"
                    : "Rejection recorded"}
                </span>
              </div>
              {mutation.data.signoff.action === "approve" && (
                <div className="space-y-2">
                  <Summary label="Verified" value={mutation.data.verified ? "✓ signature valid" : "—"} />
                  <Mono label="Obligation hash (sha256)" value={mutation.data.signoff.obligation_hash} />
                  <Mono label="Ed25519 signature" value={mutation.data.signoff.signature ?? ""} />
                  <Mono label="Public key" value={mutation.data.signoff.public_key ?? ""} />
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer nav */}
        <footer className="flex items-center justify-between border-t border-line px-5 py-3">
          {step === 4 ? (
            <button
              onClick={onClose}
              className="ml-auto hairline rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground"
            >
              Done
            </button>
          ) : (
            <>
              <button
                onClick={() => (step === 1 ? onClose() : setStep((step - 1) as Step))}
                className="text-sm text-muted-foreground hover:text-foreground"
              >
                {step === 1 ? "Cancel" : "Back"}
              </button>
              {step < 3 ? (
                <button
                  onClick={() => setStep((step + 1) as Step)}
                  disabled={step === 2 && (!justificationValid || !signerValid)}
                  className="hairline rounded-md bg-surface-2 px-4 py-1.5 text-sm disabled:opacity-40"
                >
                  Continue
                </button>
              ) : (
                <button
                  onClick={() => mutation.mutate()}
                  disabled={mutation.isPending}
                  className="hairline inline-flex items-center gap-2 rounded-md bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground disabled:opacity-50"
                >
                  <PenLine className="size-4" />
                  {mutation.isPending
                    ? "Signing…"
                    : action === "approve"
                      ? "Sign with Ed25519"
                      : "Record rejection"}
                </button>
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
        <span className="tnum text-primary">{obligation.clause_ref}</span>
        <DeonticBadge deontic={obligation.deontic_type} />
        <ConfidenceMeter value={obligation.confidence} />
      </div>
      <h3 className="font-display text-lg leading-tight">{obligation.clause_heading}</h3>
      <div>
        <div className="text-[11px] tracking-wide text-muted-foreground uppercase">
          Reasoning chain — source sentence
        </div>
        <blockquote className="mt-1.5 border-l-2 border-verified pl-3 leading-relaxed">
          {obligation.source_sentence}
        </blockquote>
      </div>
      <dl className="grid grid-cols-2 gap-2 text-xs">
        <Summary label="Bearer" value={obligation.bearer} />
        <Summary
          label="Deadline"
          value={obligation.deadline ? formatDeadline(obligation.deadline) : "—"}
        />
        <Summary label="Threshold" value={JSON.stringify(obligation.threshold)} />
        <Summary label="Current status" value={obligation.status} />
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
  const color = tone === "verified" ? "border-verified text-verified" : "border-danger text-danger"
  return (
    <button
      onClick={onClick}
      className={`flex-1 rounded-md border px-3 py-2 text-sm font-medium transition-colors ${
        active ? color : "border-line text-muted-foreground"
      }`}
    >
      {children}
    </button>
  )
}

function Labeled({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-[11px] tracking-wide text-muted-foreground uppercase">{label}</span>
      <div className="mt-1">{children}</div>
    </label>
  )
}

function Summary({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[11px] tracking-wide text-muted-foreground uppercase">{label}</div>
      <div className="text-foreground">{value}</div>
    </div>
  )
}

function Mono({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[11px] tracking-wide text-muted-foreground uppercase">{label}</div>
      <div className="tnum break-all text-xs text-verified">{value}</div>
    </div>
  )
}
