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
} from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
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

// ---- Plain-English rule builders (from the obligation's own fields) --------

const OP_WORD: Record<string, string> = {
  ">=": "at least",
  ">": "more than",
  "<=": "at most",
  "<": "fewer than",
  "==": "exactly",
}

/** Trim a trailing ".0" from a crore/lakh figure. */
function trimNum(n: number): string {
  return Number.isInteger(n) ? String(n) : String(Number(n.toFixed(2)))
}

/** Rupees in Indian words: 30000000 → "₹3 crore", 300000 → "₹3 lakh". */
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

/** A plain phrase for a structured threshold, e.g. "300 or more clients". */
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

/** Build a plain APPLIES WHEN / THEN IT MUST from the obligation's fields. */
function plainRule(o: ObligationDetail): PlainRule {
  const t = (o.threshold ?? {}) as Threshold
  const phrase = thresholdPhrase(t)
  const isTrigger = t.kind === "trigger" && phrase

  const appliesWhen = isTrigger
    ? `the firm ${phrase}.`
    : "every investment adviser, regardless of size."

  // Pull the action straight from the regulator's sentence: the text after the
  // deontic verb is the duty itself, in plain English. The verb lives in the
  // "Then it must / must not / may" label, so it's stripped from the value here.
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
    : `comply with clause ${o.clause_ref} — ${o.clause_heading}.`

  return {
    appliesWhen,
    thenMust,
    source: `SEBI IA Master Circular, clause ${o.clause_ref}`,
  }
}

/** Rewrite an OPA deny message into a layman reason (strip IDs/paths). */
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

// ---- Page ------------------------------------------------------------------

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
    <div className="flex h-full">
      {/* Left: approved obligations, as plain clause + subject + mode */}
      <div className="w-[300px] shrink-0 overflow-auto border-r border-line">
        <div className="border-b border-line px-4 py-3">
          <div className="eyebrow mb-1">Policy</div>
          <h1 className="font-display text-xl leading-tight">Automated checks</h1>
          <p className="mt-1 text-xs text-text-dim">
            Only signed obligations become enforceable checks.
          </p>
        </div>
        {candidates.length === 0 && !policies.isLoading && (
          <p className="p-4 text-sm text-text-dim">
            No approved obligations yet. Sign one off in the Review Queue.
          </p>
        )}
        <ul>
          {candidates.map((c) => (
            <li key={c.obligation_id}>
              <button
                onClick={() => setSelected(c.obligation_id)}
                className={`flex w-full items-center gap-2 border-b border-line/60 px-4 py-3 text-left ${
                  selected === c.obligation_id ? "bg-cream-200" : "hover:bg-surface"
                }`}
              >
                <div className="min-w-0 flex-1">
                  <div className="text-sm text-foreground">Clause {c.clause_ref}</div>
                  <div className="truncate text-xs text-text-dim">
                    {c.clause_heading}
                  </div>
                </div>
                {c.compiled && c.stage ? (
                  <StageChip stage={c.stage} />
                ) : (
                  <span className="text-[10px] text-text-dim">not compiled</span>
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
          <div className="grid h-full place-items-center text-sm text-text-dim">
            Select an approved obligation.
          </div>
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
    <div className="mx-auto max-w-3xl space-y-6 p-6">
      <div>
        <div className="eyebrow mb-1">Clause {candidate?.clause_ref}</div>
        <h2 className="font-display text-2xl leading-tight tracking-tight">
          Rule from Clause {candidate?.clause_ref} — {candidate?.clause_heading}
        </h2>
      </div>

      {!compiled ? (
        <div className="space-y-3">
          <p className="text-sm text-text-dim">
            This signed obligation hasn&apos;t been turned into an automated check
            yet. Compiling it produces deterministic policy code that evaluates
            your firm&apos;s data.
          </p>
          <button
            onClick={() => compile.mutate()}
            disabled={compile.isPending}
            className="inline-flex items-center gap-2 rounded-full bg-ink px-4 py-2 text-sm font-medium text-on-ink disabled:opacity-50"
          >
            <Cpu className="size-4" />
            {compile.isPending ? "Compiling…" : "Compile to automated check"}
          </button>
        </div>
      ) : (
        <>
          {/* 1 — Plain-English rule card (the main content) */}
          {rule && (
            <div className="rounded-2xl border border-line bg-surface p-5 shadow-[var(--shadow-card)]">
              <RuleLine label="Applies when">{rule.appliesWhen}</RuleLine>
              <RuleLine label={obligationLabel(candidate?.deontic_type)}>
                {rule.thenMust}
              </RuleLine>
              {obligation.data?.source_sentence && (
                <div className="mt-4 border-t border-line pt-3">
                  <div className="eyebrow mb-1">In the circular&apos;s words</div>
                  <blockquote className="border-l-2 border-cream-200 pl-3 text-sm leading-relaxed text-foreground">
                    {obligation.data.source_sentence}
                  </blockquote>
                </div>
              )}
              <div className="mt-3 text-xs text-text-dim">{rule.source}</div>
            </div>
          )}

          {/* 2 — Enforcement mode + human-readable explainer */}
          {stage && (
            <ModeControl
              stage={stage}
              onSet={(s) => stageM.mutate(s)}
              pending={stageM.isPending}
            />
          )}

          {/* 3 — Evaluate → plain verdict */}
          <div>
            <button
              onClick={() => evaluate.mutate()}
              disabled={evaluate.isPending}
              className="inline-flex items-center gap-2 rounded-full bg-ink px-4 py-2 text-sm font-medium text-on-ink disabled:opacity-50"
            >
              <Play className="size-4" />
              {evaluate.isPending
                ? "Checking…"
                : "Check this rule against your firm"}
            </button>
            {evaluate.isError && (
              <p className="mt-2 text-xs text-risk">
                Couldn&apos;t evaluate — the firm-state input isn&apos;t valid JSON.
              </p>
            )}
            {result && (
              <div className="mt-3">
                <VerdictBanner result={result} />
              </div>
            )}
          </div>

          {/* 4 — Technical proof, collapsed by default */}
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
    <div className="flex gap-4 py-1.5">
      <div className="eyebrow w-28 shrink-0 pt-0.5">{label}</div>
      <div className="text-[15px] leading-relaxed text-foreground">{children}</div>
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
        title="Does not apply"
        body="The firm is below the threshold for this rule, so nothing needs to be done."
      />
    )
  }
  if (result.compliant) {
    return (
      <Banner
        tone="ok"
        icon={<CircleCheck className="size-5" />}
        title="Compliant"
        body="This obligation is attested as met."
      />
    )
  }
  return (
    <Banner
      tone="risk"
      icon={<CircleX className="size-5" />}
      title="Not compliant"
      body={plainReason(result)}
      extra={
        result.blocked ? (
          <span className="inline-flex items-center gap-1 rounded border border-risk px-1.5 py-0.5 text-[10px] font-medium text-risk uppercase">
            <Ban className="size-3" /> Blocked
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
    ok: "border-ok/40 bg-ok/5 text-ok",
    risk: "border-risk/40 bg-risk/5 text-risk",
    neutral: "border-line bg-cream-200/40 text-text-dim",
  }
  return (
    <div className={`rounded-xl border p-4 ${styles[tone]}`}>
      <div className="flex items-center gap-2">
        {icon}
        <span className="font-display text-lg leading-none">{title}</span>
        {extra}
      </div>
      <p className="mt-1.5 text-sm leading-relaxed text-foreground">{body}</p>
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
    <div className="rounded-xl border border-line bg-surface p-4">
      <div className="flex flex-wrap items-center gap-3">
        <span className="eyebrow">Enforcement</span>
        <div className="inline-flex rounded-full border border-line p-0.5">
          {STAGES.map((s) => (
            <button
              key={s}
              disabled={pending || s === stage}
              onClick={() => onSet(s)}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                s === stage
                  ? "bg-ink text-on-ink"
                  : "text-text-dim hover:text-foreground disabled:opacity-50"
              }`}
            >
              {STAGE_WORD[s]}
            </button>
          ))}
        </div>
        <span className="text-sm text-foreground">{STAGE_EXPLAINER[stage]}</span>
      </div>
      <p className="mt-2 text-xs text-text-dim">
        New rules start in Audit and are promoted by a human — they can never
        block operations automatically.
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
    <details className="group rounded-xl border border-line bg-surface">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-4 py-3 text-sm font-medium text-foreground">
        <ChevronRight className="size-4 text-text-dim transition-transform group-open:rotate-90" />
        View technical detail (for auditors)
      </summary>
      <div className="space-y-4 border-t border-line px-4 py-4">
        <p className="text-xs text-text-dim">
          This rule runs as deterministic open-source policy code (OPA/Rego) — the
          same input always produces the same result. Expand to inspect.
        </p>

        <div>
          <div className="eyebrow mb-1.5">Compiled Rego policy</div>
          <pre className="max-h-72 overflow-auto rounded-lg border border-line bg-cream-200/40 p-3 text-xs leading-relaxed text-foreground">
            <code className="tnum">{rego}</code>
          </pre>
        </div>

        <div>
          <div className="eyebrow mb-1.5">Firm-state input (read-only facts)</div>
          <textarea
            value={inputText}
            onChange={(e) => onInput(e.target.value)}
            rows={10}
            spellCheck={false}
            className="tnum w-full resize-none rounded-lg border border-line bg-cream-200/40 p-3 text-xs leading-relaxed [color-scheme:light]"
          />
        </div>

        {trace && (
          <div>
            <div className="eyebrow mb-1.5">OPA evaluation trace</div>
            <pre className="max-h-64 overflow-auto rounded-lg border border-line bg-cream-200/40 p-3 text-[11px] leading-snug text-text-dim">
              <code className="tnum">{trace}</code>
            </pre>
          </div>
        )}

        <div>
          <div className="eyebrow mb-1">Obligation id</div>
          <div className="tnum break-all text-[11px] text-text-dim">{obligationId}</div>
        </div>
      </div>
    </details>
  )
}

function StageChip({ stage }: { stage: PolicyStage }) {
  const color =
    stage === "hard"
      ? "border-risk/50 text-risk"
      : stage === "soft"
        ? "border-warn/50 text-warn"
        : "border-line text-text-dim"
  return (
    <span className={`shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-medium ${color}`}>
      {STAGE_WORD[stage]}
    </span>
  )
}
