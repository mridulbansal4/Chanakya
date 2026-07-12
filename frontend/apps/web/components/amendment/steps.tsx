"use client"

import * as React from "react"
import { motion } from "framer-motion"
import {
  AlertTriangle,
  ArrowDown,
  ArrowRight,
  BadgeCheck,
  CheckCircle2,
  Download,
  FileText,
  Inbox,
  RotateCcw,
  ShieldCheck,
  Sparkles,
} from "lucide-react"

import { ConfidenceMeter } from "@/components/confidence"
import {
  AUDIT_LINEAGE,
  BLAST_COUNTERS,
  BLAST_FLOW,
  CIRCULARS,
  CLAUSE_DIFFS,
  DOCUMENTS,
  EVIDENCE_ITEMS,
  EXECUTION_STEPS,
  MITC_DATE,
  MITC_REF,
  PIPELINE_STAGES,
  REVIEW,
  SIM_OBLIGATIONS,
  WORKFLOWS,
  buildAuditPack,
} from "@/lib/amendment-sim"
import {
  CheckRow,
  GhostButton,
  PrimaryButton,
  Reveal,
  ScreenTitle,
  useCountUp,
  useSequence,
} from "@/components/amendment/kit"

const APPROVER = "Priya Menon · Compliance Officer"

// A small "new / modified" pill.
function KindBadge({ kind }: { kind: "new" | "added" | "modified" }) {
  const isNew = kind === "new" || kind === "added"
  return (
    <span
      className={`rounded border px-1.5 py-0.5 text-[10px] font-medium uppercase ${
        isNew ? "border-lavender bg-lavender/15 text-foreground" : "border-warn/50 text-warn"
      }`}
    >
      {isNew ? "New" : "Modified"}
    </span>
  )
}

/* ── Screen 1 · Regulatory inbox ─────────────────────────────────────────── */
export function Inbox_({ onProcess }: { onProcess: () => void }) {
  return (
    <div className="mx-auto max-w-3xl">
      <ScreenTitle
        eyebrow="Regulatory Feed"
        title="Recent circulars"
        description="CHANAKYA continuously monitors SEBI. Prior circulars are already processed — one new circular needs attention."
      />
      <div className="overflow-hidden rounded-xl border border-line bg-surface">
        {CIRCULARS.map((c, i) => {
          const isNew = c.status === "New"
          return (
            <Reveal key={c.id} delay={i * 0.08}>
              <div
                className={`flex items-center gap-3 border-b border-line/60 px-4 py-3 last:border-b-0 ${
                  isNew ? "bg-warn/5" : ""
                }`}
              >
                <span
                  className={`grid size-8 shrink-0 place-items-center rounded-md ${
                    isNew ? "bg-warn/15 text-warn" : "bg-cream-200 text-text-dim"
                  }`}
                >
                  {isNew ? <Sparkles className="size-4" /> : <FileText className="size-4" />}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    {isNew && (
                      <span className="rounded bg-warn px-1.5 py-0.5 text-[9px] font-semibold tracking-wide text-white uppercase">
                        New
                      </span>
                    )}
                    <span className="truncate text-sm font-medium text-foreground">{c.title}</span>
                  </div>
                  {c.ref && <div className="tnum mt-0.5 text-[11px] text-text-dim">{c.ref}</div>}
                </div>
                <div className="shrink-0 text-right">
                  <div className="tnum text-xs text-text-dim">{c.date}</div>
                  <div
                    className={`text-[11px] font-medium ${isNew ? "text-warn" : "text-ok"}`}
                  >
                    {c.status}
                  </div>
                </div>
                {isNew && (
                  <PrimaryButton onClick={onProcess}>
                    Review &amp; Process <ArrowRight className="size-4" />
                  </PrimaryButton>
                )}
              </div>
            </Reveal>
          )
        })}
      </div>
      <p className="mt-3 flex items-center gap-1.5 text-xs text-text-dim">
        <Inbox className="size-3.5" /> No upload needed — the circular is fetched automatically.
      </p>
    </div>
  )
}

/* ── Screen 2 · Circular processing pipeline ─────────────────────────────── */
export function Processing({ onNext }: { onNext: () => void }) {
  const [done, setDone] = React.useState(false)
  const shown = useSequence(PIPELINE_STAGES.length, {
    intervalMs: 720,
    onDone: () => setDone(true),
  })
  return (
    <div className="mx-auto max-w-2xl">
      <ScreenTitle
        eyebrow="Processing"
        title="Processing the MITC circular"
        description={MITC_REF}
      />
      <div className="space-y-2">
        {PIPELINE_STAGES.map((stage, i) => (
          <CheckRow
            key={stage}
            label={stage}
            done={i < shown}
            running={i === shown && !done}
          />
        ))}
      </div>
      {done && (
        <Reveal className="mt-5 flex justify-end">
          <PrimaryButton onClick={onNext}>
            View clause diff <ArrowRight className="size-4" />
          </PrimaryButton>
        </Reveal>
      )}
    </div>
  )
}

/* ── Screen 3 · Clause diff ──────────────────────────────────────────────── */
export function ClauseDiff({ onNext }: { onNext: () => void }) {
  return (
    <div className="mx-auto max-w-3xl">
      <ScreenTitle
        eyebrow="Clause Diff"
        title="What changed"
        description="Comparing the firm's current regulation against the new MITC circular, clause by clause."
      />
      <div className="space-y-3">
        {CLAUSE_DIFFS.map((d, i) => (
          <Reveal key={d.id} delay={i * 0.1}>
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="mb-2 flex items-center gap-2">
                <KindBadge kind={d.kind} />
                <span className="text-sm font-medium text-foreground">{d.title}</span>
              </div>
              <div className="space-y-1.5">
                <div className="rounded-md border border-risk/30 bg-risk/5 px-3 py-2 text-xs leading-relaxed text-foreground">
                  <span className="mr-1.5 text-[10px] font-semibold text-risk uppercase">Before</span>
                  {d.before}
                </div>
                <div className="flex justify-center text-text-dim">
                  <ArrowDown className="size-3.5" />
                </div>
                <div className="rounded-md border border-ok/30 bg-ok/5 px-3 py-2 text-xs leading-relaxed text-foreground">
                  <span className="mr-1.5 text-[10px] font-semibold text-ok uppercase">After</span>
                  {d.after}
                </div>
              </div>
            </div>
          </Reveal>
        ))}
      </div>
      <div className="mt-5 flex justify-end">
        <PrimaryButton onClick={onNext}>
          Extract obligations <ArrowRight className="size-4" />
        </PrimaryButton>
      </div>
    </div>
  )
}

/* ── Screen 4 · Obligation extraction ────────────────────────────────────── */
export function Obligations({ onNext }: { onNext: () => void }) {
  const modified = SIM_OBLIGATIONS.filter((o) => o.kind === "modified").length
  const added = SIM_OBLIGATIONS.filter((o) => o.kind === "new").length
  return (
    <div className="mx-auto max-w-3xl">
      <ScreenTitle
        eyebrow="Obligations"
        title="Structured obligations extracted"
        description="The circular's legal text, converted into typed, cited obligations — pending human review."
      />
      <div className="mb-4 grid grid-cols-3 gap-3">
        <Stat label="Modified" value={`${modified}`} />
        <Stat label="Added" value={`${added}`} tone="lavender" />
        <Stat label="Avg. confidence" value="97%" tone="ok" />
      </div>
      <div className="space-y-2.5">
        {SIM_OBLIGATIONS.map((o, i) => (
          <Reveal key={o.ref} delay={i * 0.1}>
            <div className="rounded-xl border border-line bg-surface p-4">
              <div className="flex flex-wrap items-center gap-2">
                <span className="tnum text-xs text-text-dim">{o.ref}</span>
                <KindBadge kind={o.kind} />
                <span className="ml-auto">
                  <ConfidenceMeter value={o.confidence} />
                </span>
              </div>
              <p className="mt-1.5 text-sm text-foreground">{o.summary}</p>
              <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-text-dim">
                <span className="tnum">Source: {o.source}</span>
                <span className="tnum">Citation: {o.citation}</span>
                <span className="rounded border border-warn/40 px-1.5 py-0.5 font-medium text-warn">
                  {o.status}
                </span>
              </div>
            </div>
          </Reveal>
        ))}
      </div>
      <div className="mt-5 flex justify-end">
        <PrimaryButton onClick={onNext}>
          Update obligation graph <ArrowRight className="size-4" />
        </PrimaryButton>
      </div>
    </div>
  )
}

const GRAPH_ROWS = [
  { obl: "3.1 · Agreement incorporates MITC", control: "Agreement Control", kind: "modified" as const, ctrl: "affected" as const },
  { obl: "3.4 · Retain acknowledgement", control: "Records Control", kind: "modified" as const, ctrl: "affected" as const },
  { obl: "5.2 · Inform existing clients", control: "Notification Workflow", kind: "modified" as const, ctrl: "new" as const },
  { obl: "MITC-1 · Provide MITC + acknowledge", control: "Client Comms Control", kind: "new" as const, ctrl: "new" as const },
]

/* ── Screen 5 · Obligation graph update ──────────────────────────────────── */
export function GraphUpdate({ onNext }: { onNext: () => void }) {
  return (
    <div className="mx-auto max-w-4xl">
      <ScreenTitle
        eyebrow="Graph"
        title="Obligation graph updated"
        description="The new obligation joins the existing graph; affected controls are highlighted."
      />
      <div className="rounded-xl border border-line bg-surface p-5">
        <div className="mb-3 grid grid-cols-[1fr_auto_1.4fr_auto_1.4fr] items-center gap-2 text-[10px] tracking-wide text-text-dim uppercase">
          <span>Regulation</span>
          <span />
          <span>Obligation</span>
          <span />
          <span>Control</span>
        </div>
        <div className="space-y-2">
          {GRAPH_ROWS.map((r, i) => (
            <Reveal key={r.obl} delay={0.15 + i * 0.18}>
              <div className="grid grid-cols-[1fr_auto_1.4fr_auto_1.4fr] items-center gap-2">
                <div className="rounded-lg border border-line bg-cream-200/40 px-2.5 py-2 text-[11px] text-text-dim">
                  {i === 0 ? "MITC Circular" : "↳"}
                </div>
                <ArrowRight className="size-3.5 text-text-dim" />
                <GraphNode label={r.obl} kind={r.kind} />
                <ArrowRight className="size-3.5 text-text-dim" />
                <GraphNode label={r.control} control={r.ctrl} />
              </div>
            </Reveal>
          ))}
        </div>
      </div>
      <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-text-dim">
        <span className="inline-flex items-center gap-1.5"><Dot c="warn" /> 3 modified</span>
        <span className="inline-flex items-center gap-1.5"><Dot c="lavender" /> 1 added</span>
        <span className="inline-flex items-center gap-1.5"><Dot c="warn" /> 2 existing controls updated</span>
      </div>
      <div className="mt-5 flex justify-end">
        <PrimaryButton onClick={onNext}>
          Compute blast radius <ArrowRight className="size-4" />
        </PrimaryButton>
      </div>
    </div>
  )
}

function GraphNode({
  label,
  kind,
  control,
}: {
  label: string
  kind?: "new" | "modified"
  control?: "affected" | "new"
}) {
  const affected = control === "affected" || kind === "modified"
  const isNew = control === "new" || kind === "new"
  const border = affected ? "border-warn" : isNew ? "border-lavender" : "border-line"
  const dot = affected ? "warn" : isNew ? "lavender" : "text-dim"
  return (
    <motion.div
      initial={false}
      className={`flex items-center gap-2 rounded-lg border bg-surface px-2.5 py-2 text-xs text-foreground shadow-[var(--shadow-card)] ${border}`}
    >
      <Dot c={dot as "warn" | "lavender" | "text-dim"} />
      <span className="truncate">{label}</span>
    </motion.div>
  )
}

function Dot({ c }: { c: "warn" | "lavender" | "text-dim" | "ok" }) {
  const bg = { warn: "bg-warn", lavender: "bg-lavender", "text-dim": "bg-text-dim", ok: "bg-ok" }[c]
  return <span className={`inline-block size-2 shrink-0 rounded-full ${bg}`} />
}

/* ── Screen 6 · Blast radius (hero) ──────────────────────────────────────── */
export function BlastRadius({ onNext }: { onNext: () => void }) {
  const shown = useSequence(BLAST_FLOW.length, { intervalMs: 340 })
  const [countersOn, setCountersOn] = React.useState(false)
  React.useEffect(() => {
    if (shown >= BLAST_FLOW.length) {
      const t = setTimeout(() => setCountersOn(true), 200)
      return () => clearTimeout(t)
    }
  }, [shown])
  return (
    <div className="mx-auto max-w-4xl">
      <ScreenTitle
        eyebrow="Blast Radius"
        title="Operational impact of the amendment"
        description="One clause change propagates across the firm — obligations, agreements, clients, evidence, and the audit trail."
      />
      <div className="grid gap-6 md:grid-cols-[auto_1fr]">
        {/* Vertical propagation */}
        <div className="flex flex-col items-center">
          {BLAST_FLOW.map((label, i) => (
            <React.Fragment key={label}>
              {i > 0 && (
                <motion.div
                  initial={{ scaleY: 0 }}
                  animate={{ scaleY: i < shown ? 1 : 0 }}
                  transition={{ duration: 0.2 }}
                  className="h-4 w-px origin-top bg-ink"
                />
              )}
              <motion.div
                initial={{ opacity: 0, scale: 0.9 }}
                animate={i <= shown ? { opacity: 1, scale: 1 } : { opacity: 0.15, scale: 0.95 }}
                transition={{ duration: 0.28, ease: "easeOut" }}
                className={`w-56 rounded-lg border px-3 py-2 text-center text-xs font-medium shadow-[var(--shadow-card)] ${
                  i === 0
                    ? "border-ink bg-ink text-on-ink"
                    : "border-line bg-surface text-foreground"
                }`}
              >
                {label}
              </motion.div>
            </React.Fragment>
          ))}
        </div>

        {/* Counters */}
        <div className="grid grid-cols-2 content-start gap-3 sm:grid-cols-1 lg:grid-cols-2">
          {BLAST_COUNTERS.map((c) => (
            <BlastCounter key={c.label} label={c.label} value={c.value} active={countersOn} />
          ))}
        </div>
      </div>
      {countersOn && (
        <Reveal className="mt-5 flex justify-end">
          <PrimaryButton onClick={onNext}>
            Generate workflows <ArrowRight className="size-4" />
          </PrimaryButton>
        </Reveal>
      )}
    </div>
  )
}

function BlastCounter({ label, value, active }: { label: string; value: number; active: boolean }) {
  const v = useCountUp(value, active, 800)
  return (
    <div className="rounded-xl border border-line bg-surface px-4 py-3">
      <div className="tnum text-3xl text-foreground">{v}</div>
      <div className="text-[11px] tracking-wide text-text-dim uppercase">{label}</div>
    </div>
  )
}

/* ── Screen 7 · Operational workflow generator ───────────────────────────── */
export function Workflows({ onNext }: { onNext: () => void }) {
  const [done, setDone] = React.useState(false)
  const shown = useSequence(WORKFLOWS.length, { intervalMs: 500, onDone: () => setDone(true) })
  const prio: Record<string, string> = {
    Critical: "border-risk/50 text-risk",
    High: "border-warn/50 text-warn",
    Normal: "border-line text-text-dim",
  }
  return (
    <div className="mx-auto max-w-3xl">
      <ScreenTitle
        eyebrow="Workflows"
        title="Operational tasks generated"
        description="CHANAKYA turns the obligation into concrete, owned, deadlined tasks — automatically."
      />
      <div className="space-y-2.5">
        {WORKFLOWS.slice(0, shown).map((w, i) => (
          <Reveal key={w.id} delay={0} className="">
            <div className="flex items-center gap-3 rounded-xl border border-line bg-surface p-3.5">
              <span className="grid size-6 shrink-0 place-items-center rounded-full bg-cream-200 text-[11px] font-medium text-text-dim">
                {i + 1}
              </span>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-foreground">{w.title}</div>
                <div className="mt-0.5 flex flex-wrap gap-x-4 text-[11px] text-text-dim">
                  <span>Owner: {w.owner}</span>
                  <span>Deadline: {w.deadline}</span>
                </div>
              </div>
              <span className={`shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-medium uppercase ${prio[w.priority]}`}>
                {w.priority}
              </span>
            </div>
          </Reveal>
        ))}
      </div>
      {done && (
        <Reveal className="mt-5 flex justify-end">
          <PrimaryButton onClick={onNext}>
            Send for human approval <ArrowRight className="size-4" />
          </PrimaryButton>
        </Reveal>
      )}
    </div>
  )
}

/* ── Screen 8 · Human review (governance gate) ───────────────────────────── */
export function HumanReview({ onApprove }: { onApprove: () => void }) {
  const [note, setNote] = React.useState<string | null>(null)
  return (
    <div className="mx-auto max-w-2xl">
      <ScreenTitle
        eyebrow="Approval"
        title="Human review & approval"
        description="Nothing is enforced automatically. A compliance officer reviews the AI's interpretation before any action runs."
      />
      <div className="space-y-3 rounded-xl border border-line bg-surface p-5">
        <Field label="Source clause">{REVIEW.sourceClause}</Field>
        <Chain />
        <Field label="AI interpretation">{REVIEW.aiInterpretation}</Field>
        <Chain />
        <Field label="Suggested operational action">{REVIEW.suggestedAction}</Field>
        <div>
          <div className="eyebrow mb-1.5">Affected systems</div>
          <div className="flex flex-wrap gap-1.5">
            {REVIEW.affectedSystems.map((s) => (
              <span key={s} className="rounded-full border border-line bg-cream-200/50 px-2.5 py-0.5 text-[11px] text-foreground">
                {s}
              </span>
            ))}
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-x-5 gap-y-1 border-t border-line pt-3 text-xs text-text-dim">
          <span className="inline-flex items-center gap-1.5">
            Confidence <ConfidenceMeter value={REVIEW.confidence} />
          </span>
          <span className="tnum">Citation: {REVIEW.citation}</span>
        </div>
      </div>

      {note && (
        <Reveal className="mt-3 rounded-lg border border-warn/40 bg-warn/5 px-3 py-2 text-xs text-foreground">
          {note}
        </Reveal>
      )}

      <div className="mt-5 flex flex-wrap items-center justify-end gap-2">
        <GhostButton onClick={() => setNote("Change request noted — routed back to the compliance team (demo).")}>
          Request changes
        </GhostButton>
        <GhostButton onClick={() => setNote("Rejected — the amendment would not be enforced. Approve to continue the demo.")}>
          Reject
        </GhostButton>
        <PrimaryButton tone="ok" onClick={onApprove}>
          <BadgeCheck className="size-4" /> Approve
        </PrimaryButton>
      </div>
      <p className="mt-2 text-right text-[11px] text-text-dim">Enforcement never runs before approval.</p>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="eyebrow mb-1">{label}</div>
      <p className="text-sm leading-relaxed text-foreground">{children}</p>
    </div>
  )
}
function Chain() {
  return (
    <div className="flex justify-center text-text-dim">
      <ArrowDown className="size-3.5" />
    </div>
  )
}

/* ── Screen 9 · Execution ────────────────────────────────────────────────── */
export function Execution({ onNext }: { onNext: () => void }) {
  const [done, setDone] = React.useState(false)
  const shown = useSequence(EXECUTION_STEPS.length, { intervalMs: 600, onDone: () => setDone(true) })
  return (
    <div className="mx-auto max-w-2xl">
      <ScreenTitle
        eyebrow="Execution"
        title="Approved — executing the workflow"
        description="Each operational task runs and reports back in order."
      />
      <div className="space-y-2">
        {EXECUTION_STEPS.map((s, i) => (
          <CheckRow key={s} label={s} done={i < shown} running={i === shown && !done} />
        ))}
      </div>
      {done && (
        <Reveal className="mt-5 flex justify-end">
          <PrimaryButton onClick={onNext}>
            Collect evidence <ArrowRight className="size-4" />
          </PrimaryButton>
        </Reveal>
      )}
    </div>
  )
}

/* ── Screen 10 · Evidence collection ─────────────────────────────────────── */
export function Evidence({ onNext }: { onNext: () => void }) {
  const [done, setDone] = React.useState(false)
  const shown = useSequence(EVIDENCE_ITEMS.length, { intervalMs: 450, onDone: () => setDone(true) })
  return (
    <div className="mx-auto max-w-2xl">
      <ScreenTitle
        eyebrow="Evidence"
        title="Evidence collected & linked"
        description="Every action produces evidence, automatically attached to the obligation it satisfies."
      />
      <div className="grid gap-2.5 sm:grid-cols-2">
        {EVIDENCE_ITEMS.slice(0, shown).map((e) => (
          <Reveal key={e.label}>
            <div className="flex items-center gap-3 rounded-xl border border-ok/30 bg-ok/5 p-3.5">
              <CheckCircle2 className="size-5 shrink-0 text-ok" />
              <div className="min-w-0">
                <div className="text-sm font-medium text-foreground">{e.label}</div>
                <div className="tnum text-xs text-text-dim">{e.value}</div>
              </div>
            </div>
          </Reveal>
        ))}
      </div>
      {done && (
        <Reveal className="mt-5 flex justify-end">
          <PrimaryButton onClick={onNext}>
            Assemble audit pack <ArrowRight className="size-4" />
          </PrimaryButton>
        </Reveal>
      )}
    </div>
  )
}

/* ── Screen 11 · Audit pack ──────────────────────────────────────────────── */
export function AuditPack({ onRestart }: { onRestart: () => void }) {
  const download = () => {
    const data = buildAuditPack(APPROVER)
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = "chanakya-mitc-audit-pack.json"
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  }
  return (
    <div className="mx-auto max-w-3xl">
      <ScreenTitle
        eyebrow="Audit Pack"
        title="Audit-ready — compliance restored"
        description="Complete, cited lineage from the SEBI circular to the final evidence bundle — assembled with no manual tracing."
      />

      <Reveal className="mb-4 flex flex-wrap items-center gap-3 rounded-xl border border-ok/40 bg-ok/5 px-4 py-3">
        <ShieldCheck className="size-6 text-ok" />
        <div>
          <div className="font-display text-lg leading-none text-foreground">Compliant</div>
          <div className="text-xs text-text-dim">MITC amendment · {MITC_DATE} · Reg 19(3)</div>
        </div>
        <span className="ml-auto rounded-full border border-ok/50 px-2.5 py-1 text-xs font-medium text-ok">
          Audit status: Ready
        </span>
      </Reveal>

      {/* Lineage chain */}
      <div className="flex flex-wrap items-center gap-2 rounded-xl border border-line bg-surface p-4">
        {AUDIT_LINEAGE.map((n, i) => (
          <React.Fragment key={n}>
            <Reveal delay={i * 0.09}>
              <span className="rounded-lg border border-line bg-cream-200/40 px-2.5 py-1.5 text-xs text-foreground">
                {n}
              </span>
            </Reveal>
            {i < AUDIT_LINEAGE.length - 1 && <ArrowRight className="size-3.5 text-text-dim" />}
          </React.Fragment>
        ))}
      </div>

      {/* Evidence bundle — the assembled document set */}
      <div className="mt-5">
        <div className="eyebrow mb-2">Evidence bundle · {DOCUMENTS.length} documents</div>
        <div className="grid gap-2 sm:grid-cols-2">
          {DOCUMENTS.map((d, i) => (
            <Reveal
              key={d.name}
              delay={i * 0.04}
              className="flex items-center gap-2.5 rounded-lg border border-line bg-surface px-3 py-2 text-sm"
            >
              <FileText className="size-4 shrink-0 text-text-dim" />
              <span className="tnum truncate text-foreground">{d.name}</span>
            </Reveal>
          ))}
        </div>
      </div>

      <div className="mt-5 flex flex-wrap items-center justify-between gap-3">
        <span className="inline-flex items-center gap-1.5 text-xs text-text-dim">
          <AlertTriangle className="size-3.5 text-ok" /> Signed off by {APPROVER}
        </span>
        <div className="flex gap-2">
          <GhostButton onClick={onRestart}>
            <RotateCcw className="size-4" /> Restart demo
          </GhostButton>
          <PrimaryButton onClick={download}>
            <Download className="size-4" /> Download Audit Pack
          </PrimaryButton>
        </div>
      </div>
    </div>
  )
}

/* Shared small stat card. */
function Stat({
  label,
  value,
  tone = "default",
}: {
  label: string
  value: string
  tone?: "default" | "ok" | "lavender"
}) {
  const color = tone === "ok" ? "text-ok" : "text-foreground"
  return (
    <div className="rounded-xl border border-line bg-surface px-4 py-3">
      <div className={`tnum text-2xl ${color}`}>{value}</div>
      <div className="text-[11px] tracking-wide text-text-dim uppercase">{label}</div>
    </div>
  )
}
