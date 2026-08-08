"use client"

import * as React from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  Ban,
  ChevronRight,
  CircleCheck,
  CircleX,
  Cpu,
  MinusCircle,
  Play,
  CheckCircle2,
  Code2,
  ShieldCheck,
} from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { Button } from "@workspace/ui/components/button"
import { EmptyState } from "@/components/empty-state"
import {
  compilePolicy,
  evaluatePolicy,
  getFirmState,
  getObligation,
  getPolicy,
  listPolicies,
  setPolicyStage,
  type ObligationDetail,
  type PolicyCandidate,
  type PolicyEvalResult,
  type PolicyStage,
} from "@/lib/api"

const STAGES: PolicyStage[] = ["audit", "soft", "hard"]

const STAGE_WORD: Record<PolicyStage, string> = {
  audit: "Audit",
  soft: "Soft",
  hard: "Hard",
}

const STAGE_EXPLAINER: Record<PolicyStage, string> = {
  audit: "Observing only. Records results but changes nothing.",
  soft: "Warns on breach, but does not block operations.",
  hard: "Blocks non-compliant actions.",
}

const OP_WORD: Record<string, string> = {
  ">=": "at least",
  ">": "more than",
  "<=": "at most",
  "<": "fewer than",
  "==": "exactly",
}

function trimNum(n: number): string {
  return Number.isInteger(n) ? String(n) : String(Number(n.toFixed(2)))
}

function rupees(v: number): string {
  if (v >= 1e7) return `₹${trimNum(v / 1e7)} crore`
  if (v >= 1e5) return `₹${trimNum(v / 1e5)} lakh`
  return `₹${v.toLocaleString("en-IN")}`
}

interface Threshold {
  metric?: string
  operator?: string
  value?: number
  unit?: string
  kind?: string
}

function thresholdPhrase(t: Threshold): string | null {
  if (!t.metric || t.value == null) return null
  const op = OP_WORD[t.operator ?? ">="] ?? t.operator ?? ""
  switch (t.metric) {
    case "clients":
      return t.operator === ">="
        ? `advises ${t.value.toLocaleString("en-IN")} or more clients`
        : `advises ${op} ${t.value.toLocaleString("en-IN")} clients`
    case "annual_fees":
      return `charges ${op} ${rupees(t.value)} in fees in a financial year`
    case "retention_period":
      return `keeps records for ${op} ${t.value} ${t.unit ?? "years"}`
    default:
      return `has ${t.metric.replace(/_/g, " ")} ${op} ${t.value} ${t.unit ?? ""}`.trim()
  }
}

interface PlainRule {
  appliesWhen: string
  thenMust: string
  source: string
}

function plainRule(o: ObligationDetail): PlainRule {
  const t = (o.threshold ?? {}) as Threshold
  const phrase = thresholdPhrase(t)
  const isTrigger = t.kind === "trigger" && phrase

  const appliesWhen = isTrigger
    ? `the firm ${phrase}.`
    : "every investment adviser, regardless of size."

  let action = ""
  const m = /\b(must not|must|shall not|shall|may)\b/i.exec(o.source_sentence ?? "")
  if (m) {
    action = (o.source_sentence ?? "")
      .slice(m.index + m[0].length)
      .replace(/\s+/g, " ")
      .replace(/[.\s]+$/, "")
      .trim()
  }
  const thenMust = action
    ? `${action}.`
    : `comply with clause ${o.clause_ref} - ${o.clause_heading}.`

  return {
    appliesWhen,
    thenMust,
    source: `SEBI IA Master Circular, clause ${o.clause_ref}`,
  }
}

function plainReason(result: PolicyEvalResult): string {
  const raw = result.denies?.[0]
  if (raw) {
    const cleaned = raw
      .replace(/SEBI\/\S+/g, "this obligation")
      .replace(/\b[0-9a-f]{8,}\b/g, "")
      .replace(/\s+/g, " ")
      .trim()
    if (/attest/i.test(cleaned))
      return "this rule applies to your firm, but the firm has not yet attested that it is satisfied."
    if (cleaned) return cleaned.charAt(0).toLowerCase() + cleaned.slice(1)
  }
  return "this rule applies to your firm but is not yet attested as satisfied."
}

export default function PolicyPage() {
  const { asOf } = useAsOf()
  const qc = useQueryClient()
  const [selected, setSelected] = React.useState<string | null>(null)

  const policies = useQuery({
    queryKey: ["policies", asOf],
    queryFn: ({ signal }) => listPolicies(asOf, signal),
  })
  const candidates = policies.data?.candidates ?? []

  React.useEffect(() => {
    if (!selected && candidates.length) setSelected(candidates[0]!.obligation_id)
  }, [candidates, selected])

  return (
    <div className="flex flex-col lg:flex-row h-full bg-background">
      {/* Left: approved obligations list */}
      <div className="w-full lg:w-[320px] shrink-0 overflow-auto lg:border-r border-b lg:border-b-0 border-line bg-surface/60">
        <div className="border-b border-line px-5 py-4 bg-surface">
          <div className="eyebrow mb-1">Automated Enforcement</div>
          <h1 className="font-display text-xl font-bold">Policy Rules</h1>
          <p className="mt-1 text-xs text-text-dim">
            Signed obligations compiled into deterministic OPA / Rego code checks.
          </p>
        </div>
        {candidates.length === 0 && !policies.isLoading && (
          <div className="p-6 text-center text-xs text-text-dim">
            No approved obligations yet. Approve obligations in the Review Queue to generate policies.
          </div>
        )}
        <ul className="divide-y divide-line/60">
          {candidates.map((c) => (
            <li key={c.obligation_id}>
              <button
                onClick={() => setSelected(c.obligation_id)}
                className={`flex w-full items-center gap-3 px-5 py-3.5 text-left transition-all ${
                  selected === c.obligation_id
                    ? "bg-cream-200/90 font-semibold shadow-2xs border-l-4 border-foreground"
                    : "hover:bg-surface"
                }`}
              >
                <div className="min-w-0 flex-1 space-y-0.5">
                  <div className="tnum text-xs font-bold text-foreground">
                    Clause {c.clause_ref}
                  </div>
                  <div className="truncate text-xs text-text-dim">
                    {c.clause_heading}
                  </div>
                </div>
                {c.compiled && c.stage ? (
                  <StageChip stage={c.stage} />
                ) : (
                  <span className="text-[10px] text-text-dim italic">not compiled</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      </div>

      {/* Right: policy detail */}
      <div className="min-w-0 flex-1 overflow-auto">
        {selected ? (
          <PolicyDetail
            key={selected}
            candidate={candidates.find((c) => c.obligation_id === selected)}
            obligationId={selected}
            asOf={asOf}
            onChanged={() => qc.invalidateQueries({ queryKey: ["policies"] })}
          />
        ) : (
          <EmptyState
            icon="sparkles"
            title="Select a Policy"
            description="Select an approved obligation from the left sidebar to inspect and run its automated check."
          />
        )}
      </div>
    </div>
  )
}

function PolicyDetail({
  candidate,
  obligationId,
  asOf,
  onChanged,
}: {
  candidate?: PolicyCandidate
  obligationId: string
  asOf: string
  onChanged: () => void
}) {
  const qc = useQueryClient()
  const policy = useQuery({
    queryKey: ["policy", obligationId],
    queryFn: ({ signal }) => getPolicy(obligationId, signal).catch(() => null),
    retry: false,
  })
  const obligation = useQuery({
    queryKey: ["obligation", obligationId],
    queryFn: ({ signal }) => getObligation(obligationId, signal),
  })
  const firmState = useQuery({
    queryKey: ["firm-state", asOf],
    queryFn: ({ signal }) => getFirmState(asOf, signal),
  })

  const [inputText, setInputText] = React.useState("")
  const [result, setResult] = React.useState<PolicyEvalResult | null>(null)
  React.useEffect(() => {
    if (firmState.data && !inputText) {
      setInputText(JSON.stringify(firmState.data, null, 2))
    }
  }, [firmState.data, inputText])

  const compile = useMutation({
    mutationFn: () => compilePolicy(obligationId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["policy", obligationId] })
      onChanged()
    },
  })
  const stageM = useMutation({
    mutationFn: (stage: PolicyStage) => setPolicyStage(obligationId, stage),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["policy", obligationId] })
      onChanged()
    },
  })
  const evaluate = useMutation({
    mutationFn: () =>
      evaluatePolicy({ obligation_id: obligationId, input: JSON.parse(inputText) }),
    onSuccess: (r) => setResult(r),
  })

  const compiled = policy.data?.policy
  const stage = compiled?.stage
  const rule = obligation.data ? plainRule(obligation.data) : null

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-8">
      <div>
        <div className="eyebrow mb-1">Clause {candidate?.clause_ref} Policy Rule</div>
        <h2 className="font-display text-2xl font-bold tracking-tight text-foreground">
          Rule: Clause {candidate?.clause_ref} - {candidate?.clause_heading}
        </h2>
      </div>

      {!compiled ? (
        <div className="rounded-lg border border-line bg-surface p-6 space-y-4 shadow-elev-1">
          <p className="text-sm text-text-dim leading-relaxed">
            This signed obligation has not been compiled into an automated check yet. Compiling it converts regulatory logic into deterministic Open Policy Agent (OPA) code.
          </p>
          <Button
            variant="default"
            isLoading={compile.isPending}
            loadingText="Compiling to Rego OPA Code…"
            onClick={() => compile.mutate()}
          >
            <Cpu className="size-4" />
            <span>Compile to Automated Check</span>
          </Button>
        </div>
      ) : (
        <>
          {/* Plain English Rule Card */}
          {rule && (
            <div className="rounded-lg border border-line bg-surface p-6 shadow-elev-1 space-y-4">
              <RuleLine label="Applies When">{rule.appliesWhen}</RuleLine>
              <RuleLine label={obligationLabel(candidate?.deontic_type)}>
                {rule.thenMust}
              </RuleLine>
              {obligation.data?.source_sentence && (
                <div className="mt-4 border-t border-line pt-4 space-y-1">
                  <div className="eyebrow">Exact Regulation Source</div>
                  <blockquote className="border-l-2 border-cream-200 pl-3 text-xs leading-relaxed text-text-dim italic">
                    &quot;{obligation.data.source_sentence}&quot;
                  </blockquote>
                </div>
              )}
              <div className="text-xs font-mono text-text-dim">{rule.source}</div>
            </div>
          )}

          {/* Enforcement Mode Selector */}
          {stage && (
            <ModeControl
              stage={stage}
              onSet={(s) => stageM.mutate(s)}
              pending={stageM.isPending}
            />
          )}

          {/* Evaluation Action */}
          <div className="space-y-4">
            <Button
              variant="default"
              size="lg"
              isLoading={evaluate.isPending}
              loadingText="Running Policy Engine Check…"
              onClick={() => evaluate.mutate()}
              className="shadow-elev-1"
            >
              <Play className="size-4" />
              <span>Evaluate Rule Against Firm State</span>
            </Button>

            {evaluate.isError && (
              <p className="text-xs text-risk font-semibold">
                Evaluation failed - please verify firm-state JSON formatting.
              </p>
            )}

            {result && <VerdictBanner result={result} />}
          </div>

          {/* Technical Details for Auditors */}
          <TechnicalDetail
            rego={compiled.rego}
            obligationId={obligationId}
            inputText={inputText}
            onInput={setInputText}
            trace={result?.trace}
          />
        </>
      )}
    </div>
  )
}

function RuleLine({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex gap-4 items-start">
      <div className="eyebrow w-32 shrink-0 pt-0.5">{label}</div>
      <div className="text-sm font-semibold leading-relaxed text-foreground">{children}</div>
    </div>
  )
}

function obligationLabel(d?: string): string {
  if (d === "MUST_NOT") return "Then it must not"
  if (d === "MAY") return "Then it may"
  return "Then it must"
}

function VerdictBanner({ result }: { result: PolicyEvalResult }) {
  if (!result.applicable) {
    return (
      <Banner
        tone="neutral"
        icon={<MinusCircle className="size-5" />}
        title="Does Not Apply"
        body="Firm parameters fall below threshold limits for this rule."
      />
    )
  }
  if (result.compliant) {
    return (
      <Banner
        tone="ok"
        icon={<CircleCheck className="size-5 text-ok" />}
        title="Compliant & Satisfied"
        body="This obligation passes all policy rules against active firm state."
      />
    )
  }
  return (
    <Banner
      tone="risk"
      icon={<CircleX className="size-5 text-risk" />}
      title="Non-Compliant Breach Detected"
      body={plainReason(result)}
      extra={
        result.blocked ? (
          <span className="inline-flex items-center gap-1 rounded-full border border-risk bg-risk/10 px-2 py-0.5 text-[10px] font-bold text-risk uppercase">
            <Ban className="size-3" /> Hard Enforced Block
          </span>
        ) : undefined
      }
    />
  )
}

function Banner({
  tone,
  icon,
  title,
  body,
  extra,
}: {
  tone: "ok" | "risk" | "neutral"
  icon: React.ReactNode
  title: string
  body: string
  extra?: React.ReactNode
}) {
  const styles: Record<string, string> = {
    ok: "border-ok/40 bg-ok/10 text-ok",
    risk: "border-risk/40 bg-risk/10 text-risk",
    neutral: "border-line bg-cream-200/50 text-text-dim",
  }
  return (
    <div className={`rounded-lg border p-5 shadow-xs ${styles[tone]}`}>
      <div className="flex items-center gap-3">
        {icon}
        <span className="font-display text-lg font-bold">{title}</span>
        {extra}
      </div>
      <p className="mt-1.5 text-xs leading-relaxed text-foreground font-medium">{body}</p>
    </div>
  )
}

function ModeControl({
  stage,
  onSet,
  pending,
}: {
  stage: PolicyStage
  onSet: (s: PolicyStage) => void
  pending: boolean
}) {
  return (
    <div className="rounded-lg border border-line bg-surface p-5 space-y-2 shadow-xs">
      <div className="flex flex-wrap items-center gap-3">
        <span className="eyebrow">Enforcement Stage</span>
        <div className="inline-flex rounded-full border border-line p-1 bg-background">
          {STAGES.map((s) => (
            <button
              key={s}
              disabled={pending || s === stage}
              onClick={() => onSet(s)}
              className={`rounded-full px-4 py-1 text-xs font-semibold transition-all ${
                s === stage
                  ? "bg-ink text-on-ink shadow-xs"
                  : "text-text-dim hover:text-foreground disabled:opacity-50"
              }`}
            >
              {STAGE_WORD[s]}
            </button>
          ))}
        </div>
        <span className="text-xs font-semibold text-foreground">{STAGE_EXPLAINER[stage]}</span>
      </div>
      <p className="text-[11px] text-text-dim">
        Rules initialize in Audit mode and require human compliance authorization before promoting to Hard Enforcement.
      </p>
    </div>
  )
}

function TechnicalDetail({
  rego,
  obligationId,
  inputText,
  onInput,
  trace,
}: {
  rego: string
  obligationId: string
  inputText: string
  onInput: (v: string) => void
  trace?: string
}) {
  return (
    <details className="group rounded-lg border border-line bg-surface shadow-xs">
      <summary className="flex cursor-pointer list-none items-center justify-between px-5 py-4 text-xs font-bold text-foreground">
        <div className="flex items-center gap-2">
          <Code2 className="size-4 text-text-dim" />
          <span>Technical Rego / OPA Code & Audit Trace</span>
        </div>
        <ChevronRight className="size-4 text-text-dim transition-transform group-open:rotate-90" />
      </summary>
      <div className="space-y-4 border-t border-line p-5">
        <div>
          <div className="eyebrow mb-1.5">Compiled Rego Policy Code</div>
          <pre className="max-h-72 overflow-auto rounded-xl border border-line bg-ink p-4 text-xs leading-relaxed text-on-ink font-mono">
            <code>{rego}</code>
          </pre>
        </div>

        <div>
          <div className="eyebrow mb-1.5">Firm State JSON Input</div>
          <textarea
            value={inputText}
            onChange={(e) => onInput(e.target.value)}
            rows={8}
            className="w-full resize-none rounded-xl border border-line bg-ink p-4 text-xs leading-relaxed text-on-ink font-mono outline-none focus:border-on-ink/40"
          />
        </div>

        {trace && (
          <div>
            <div className="eyebrow mb-1.5">Evaluation Trace Log</div>
            <pre className="max-h-64 overflow-auto rounded-xl border border-line bg-cream-200/50 p-4 text-[11px] font-mono leading-relaxed text-text-dim">
              <code>{trace}</code>
            </pre>
          </div>
        )}
      </div>
    </details>
  )
}

function StageChip({ stage }: { stage: PolicyStage }) {
  const color =
    stage === "hard"
      ? "border-risk/50 bg-risk/10 text-risk"
      : stage === "soft"
        ? "border-warn/50 bg-warn/10 text-warn"
        : "border-line bg-cream-200 text-text-dim"
  return (
    <span className={`shrink-0 rounded-full border px-2.5 py-0.5 text-[10px] font-bold uppercase ${color}`}>
      {STAGE_WORD[stage]}
    </span>
  )
}
