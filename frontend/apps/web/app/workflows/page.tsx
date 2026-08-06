"use client"

import * as React from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"

import { useAsOf } from "@/components/as-of-provider"
import { PageHeader } from "@/components/page-header"
import {
  approveWorkflow,
  getWorkflowTasks,
  listWorkflows,
  type WorkflowTask,
} from "@/lib/api"

/** The reviewer identity recorded on approval. Auth is a stated non-goal. */
const REVIEWER = "Priya Menon"
const MIN_NOTE = 20

function TaskRow({ task, byId }: { task: WorkflowTask; byId: Map<string, WorkflowTask> }) {
  const deps = (task.depends_on ?? [])
    .map((id) => byId.get(id)?.title)
    .filter(Boolean) as string[]

  return (
    <li className="rounded-md border border-line-subtle px-3 py-2.5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="text-sm text-fg">
          <span className="tnum mr-2 text-fg-muted">{task.ordinal}</span>
          {task.title}
        </span>
        <span className="rounded bg-elevated px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-fg-muted">
          {task.state}
        </span>
      </div>
      <p className="mt-1 text-xs text-fg-muted">{task.detail}</p>
      <div className="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
        {task.owner_unresolved ? (
          // Flagged, never fabricated: assigning an arbitrary employee to fill a
          // column would put a real person's name against unagreed work.
          <span className="text-warn">
            Unassigned - no head for {task.owner_role}
          </span>
        ) : (
          <span className="text-fg">
            {task.owner_name} <span className="text-fg-muted">({task.owner_role})</span>
          </span>
        )}
        {task.deadline && (
          <span className="tnum text-fg-muted">due {task.deadline.slice(0, 10)}</span>
        )}
        {deps.length > 0 && (
          <span className="text-fg-muted">after: {deps.join(", ")}</span>
        )}
      </div>
    </li>
  )
}

export default function WorkflowsPage() {
  const { asOf } = useAsOf()
  const queryClient = useQueryClient()
  const [selected, setSelected] = React.useState<string>("")
  const [note, setNote] = React.useState("")
  const [error, setError] = React.useState<string | null>(null)
  const [approving, setApproving] = React.useState(false)

  const list = useQuery({
    queryKey: ["workflows", asOf],
    queryFn: ({ signal }) => listWorkflows(asOf, signal),
  })

  const detail = useQuery({
    queryKey: ["workflow", selected, asOf],
    queryFn: ({ signal }) => getWorkflowTasks(selected, asOf, signal),
    enabled: selected !== "",
  })

  async function onApprove() {
    if (!selected) return
    setApproving(true)
    setError(null)
    try {
      await approveWorkflow(selected, REVIEWER, note)
      setNote("")
      await queryClient.invalidateQueries({ queryKey: ["workflows"] })
      await queryClient.invalidateQueries({ queryKey: ["workflow", selected] })
    } catch (cause) {
      setError((cause as Error).message)
    } finally {
      setApproving(false)
    }
  }

  const workflows = list.data?.workflows ?? []
  const tasks = detail.data?.tasks ?? []
  const byId = new Map(tasks.map((t) => [t.id, t]))

  return (
    <div className="mx-auto w-full max-w-6xl px-6 py-6">
      <PageHeader
        eyebrow="Operational response"
        title="Generated workflows"
        description="Each approved obligation's act selects a reviewed template from a closed verb vocabulary; the template's tasks are assigned to real people from your org chart. Everything stays draft."
      />

      {list.data?.dispatch_note && (
        <p className="mt-4 rounded-md border border-line-strong bg-elevated px-3 py-2 text-sm text-fg-muted">
          {list.data.dispatch_note}
        </p>
      )}

      <div className="mt-5 grid gap-5 lg:grid-cols-[minmax(0,20rem)_1fr]">
        {/* Workflow list */}
        <section className="rounded-lg border border-line-subtle bg-raised p-4">
          <div className="flex items-baseline justify-between">
            <h2 className="font-display text-base tracking-tight">
              {list.data?.count ?? 0} workflows
            </h2>
            <span className="tnum text-xs text-fg-muted">{list.data?.draft ?? 0} draft</span>
          </div>
          <ul className="mt-3 space-y-1">
            {workflows.map((w) => (
              <li key={w.id}>
                <button
                  type="button"
                  onClick={() => setSelected(w.id)}
                  className={
                    "w-full rounded-md border px-3 py-2 text-left text-sm transition-colors " +
                    (selected === w.id
                      ? "border-line-strong bg-elevated text-fg"
                      : "border-transparent text-fg-muted hover:border-line-subtle hover:text-fg")
                  }
                >
                  <span className="block">{w.title}</span>
                  <span className="mt-0.5 flex flex-wrap items-center gap-2 text-xs text-fg-muted">
                    <span className="rounded bg-elevated px-1 py-0.5">{w.verb}</span>
                    <span className="tnum">{w.task_count} tasks</span>
                    {w.unresolved_owners > 0 && (
                      <span className="text-warn">{w.unresolved_owners} unassigned</span>
                    )}
                    {w.state === "approved" && <span className="text-ok">approved</span>}
                  </span>
                </button>
              </li>
            ))}
            {!list.isLoading && workflows.length === 0 && (
              <li className="py-2 text-sm text-fg-muted">
                No workflows yet. They are generated from approved obligations.
              </li>
            )}
          </ul>
        </section>

        {/* Task DAG */}
        <section className="rounded-lg border border-line-subtle bg-raised p-5">
          {!detail.data && (
            <p className="text-sm text-fg-muted">Select a workflow to see its task DAG.</p>
          )}
          {detail.data && (
            <>
              <div className="flex flex-wrap items-baseline justify-between gap-3">
                <h2 className="font-display text-lg tracking-tight">{detail.data.title}</h2>
                <span className="tnum text-xs text-fg-muted">SLA {detail.data.sla}</span>
              </div>
              <p className="mt-1 text-sm text-fg-muted">{detail.data.rationale}</p>

              {detail.data.unresolved_owners > 0 && (
                <p className="mt-3 rounded-md border border-warn/40 bg-warn/10 px-3 py-2 text-xs text-warn">
                  {detail.data.unresolved_owners} task(s) could not be assigned: the owning
                  department has no head. They are left unassigned rather than given to an
                  arbitrary person.
                </p>
              )}

              <ul className="mt-4 space-y-2">
                {tasks.map((t) => (
                  <TaskRow key={t.id} task={t} byId={byId} />
                ))}
              </ul>

              {detail.data.approval ? (
                <p className="mt-5 rounded-md border border-ok/40 bg-ok/10 px-3 py-2 text-sm text-ok">
                  Approved by {detail.data.approval.approver} on{" "}
                  {detail.data.approval.decided_at.slice(0, 10)}. Nothing was dispatched.
                </p>
              ) : (
                <div className="mt-5 rounded-md border border-line-strong bg-elevated p-4">
                  <h3 className="text-sm font-medium text-fg">Approve this workflow</h3>
                  <p className="mt-1 text-xs text-fg-muted">
                    Approval records that you accepted the plan. It sends nothing: tasks stay
                    draft, and a person performs the work in the firm&apos;s own systems.
                  </p>
                  <textarea
                    value={note}
                    onChange={(e) => setNote(e.target.value)}
                    rows={2}
                    placeholder="Why is this plan being accepted? (minimum 20 characters)"
                    className="mt-3 w-full rounded-md border border-line-subtle bg-canvas px-3 py-2 text-sm text-fg placeholder:text-fg-muted/60"
                  />
                  <div className="mt-3 flex items-center gap-3">
                    <button
                      type="button"
                      onClick={onApprove}
                      disabled={approving || note.trim().length < MIN_NOTE}
                      className="rounded-md border border-ok/50 bg-ok/15 px-3 py-1.5 text-sm text-ok disabled:opacity-40"
                    >
                      {approving ? "Recording..." : `Approve as ${REVIEWER}`}
                    </button>
                    <span className="tnum text-xs text-fg-muted">
                      {note.trim().length}/{MIN_NOTE}
                    </span>
                  </div>
                </div>
              )}

              {error && (
                <p className="mt-3 rounded-md border border-risk/40 bg-risk/10 px-3 py-2 text-sm text-risk">
                  {error}
                </p>
              )}
            </>
          )}
        </section>
      </div>
    </div>
  )
}
