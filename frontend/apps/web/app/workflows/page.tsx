"use client"

import * as React from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useCompletion } from "@ai-sdk/react"
import { MarkdownRenderer } from "@/components/ui/markdown-renderer"
import { Sparkles, Loader2, FileText, Server, Send, Database } from "lucide-react"

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

const highlightJson = (jsonString: string) => {
  const parts = jsonString.split(/("(?:[^"\\]|\\.)*"\s*:|".*?"|\btrue\b|\bfalse\b|\bnull\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/)
  return parts.map((part, i) => {
    if (part.match(/^"(?:[^"\\]|\\.)*"\s*:$/)) {
      return <span key={i} className="text-[#79c0ff]">{part}</span>
    } else if (part.match(/^".*?"$/)) {
      return <span key={i} className="text-[#a5d6ff]">{part}</span>
    } else if (part.match(/^(true|false|null|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)$/)) {
      return <span key={i} className="text-[#ff7b72]">{part}</span>
    }
    return <span key={i} className="text-[#c9d1d9]">{part}</span>
  })
}

function TaskRow({ task, byId }: { task: WorkflowTask; byId: Map<string, WorkflowTask> }) {
  const deps = (task.depends_on ?? [])
    .map((id) => byId.get(id)?.title)
    .filter(Boolean) as string[]

  const { completion, complete, isLoading: isAIThinking } = useCompletion({
    api: "/api/ai/stream",
    streamProtocol: "text",
  })

  const [expandedSOP, setExpandedSOP] = React.useState(false)

  const handleGenerateSOP = () => {
    if (!expandedSOP) setExpandedSOP(true)
    if (!completion && !isAIThinking) {
      complete(`Generate a detailed Standard Operating Procedure (SOP) or Jira ticket description for the following compliance task: ${task.title}. Details: ${task.detail}. Role responsible: ${task.owner_role}. Keep it extremely professional and actionable. Use markdown formatting.`)
    }
  }

  const lowerDetail = task.detail.toLowerCase()
  let connector = null
  if (lowerDetail.includes("jira") || lowerDetail.includes("ticket")) {
    connector = {
      title: "Jira Epic Creation",
      icon: Server,
      method: "POST /rest/api/2/issue",
      code: `{\n  "fields": {\n    "project": { "key": "COMP" },\n    "summary": "${task.title}",\n    "description": "Auto-generated from Chanakya",\n    "issuetype": { "name": "Epic" }\n  }\n}`,
    }
  } else if (lowerDetail.includes("email") || lowerDetail.includes("dispatch") || lowerDetail.includes("communication")) {
    connector = {
      title: "MS Graph API: SendMail",
      icon: Send,
      method: "POST /v1.0/me/sendMail",
      code: `{\n  "message": {\n    "subject": "Action Required: ${task.title}",\n    "body": {\n      "contentType": "Text",\n      "content": "Please complete your assigned task."\n    },\n    "toRecipients": [\n      { "emailAddress": { "address": "${task.owner_role.toLowerCase().replace(/ /g, '.')}@firm.com" } }\n    ]\n  }\n}`,
    }
  } else if (lowerDetail.includes("regulator") || lowerDetail.includes("submit")) {
    connector = {
      title: "SEBI e-Filing API",
      icon: Database,
      method: "POST https://efiling.sebi.gov.in/api/v1/submit",
      code: `{\n  "firm_id": "FIRM-123",\n  "report_type": "Compliance",\n  "task_ref": "${task.title}",\n  "timestamp": "${new Date().toISOString()}"\n}`,
    }
  }

  return (
    <li className="rounded-md border border-line-subtle px-3 py-2.5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="text-sm text-fg font-medium">
          <span className="tnum mr-2 text-fg-muted">{task.ordinal}</span>
          {task.title}
        </span>
        <span className="rounded bg-elevated px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-fg-muted">
          {task.state}
        </span>
      </div>
      <p className="mt-1 text-xs text-fg-muted">{task.detail}</p>
      <div className="mt-2 flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-xs">
        <div className="flex items-center gap-x-4">
          {task.owner_unresolved ? (
            <span className="text-warn">Unassigned - no head for {task.owner_role}</span>
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
        
        <button 
          onClick={handleGenerateSOP}
          className="flex items-center gap-1.5 text-accent hover:text-accent/80 font-medium transition-colors bg-accent/10 hover:bg-accent/20 px-2 py-1 rounded"
        >
          <Sparkles className="size-3" /> {completion ? "View SOP" : "Generate SOP"}
        </button>
      </div>

      {expandedSOP && (
        <div className="mt-3 p-3 bg-surface border border-line rounded-md text-xs text-foreground">
          <div className="flex items-center gap-1.5 font-bold text-accent mb-2">
            <FileText className="size-3.5" /> AI Generated SOP / Ticket
          </div>
          {isAIThinking && !completion ? (
            <div className="flex items-center gap-2 text-text-dim">
              <Loader2 className="size-3 animate-spin" /> Drafting instructions...
            </div>
          ) : (
            <MarkdownRenderer content={completion} />
          )}
        </div>
      )}

      {connector && (
        <div className="mt-4 overflow-hidden rounded-md border border-line-subtle bg-surface">
          <div className="flex items-center gap-2 border-b border-line-subtle bg-elevated px-3 py-2">
            <connector.icon className="size-3.5 text-accent" />
            <span className="text-[10px] font-semibold uppercase tracking-wider text-fg-muted">
              {connector.title}
            </span>
          </div>
          <div className="p-3 bg-[#0d1117] overflow-x-auto">
            <div className="text-xs font-mono whitespace-pre leading-relaxed">
              <span className="text-[#ff7b72]">{connector.method.split(' ')[0]}</span>{" "}
              <span className="text-[#a5d6ff]">{connector.method.split(' ')[1]}</span>
              {"\n\n"}
              {highlightJson(connector.code)}
            </div>
          </div>
        </div>
      )}
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
        description="Each approved obligation's act selects a reviewed template from a closed verb vocabulary; the template's tasks are assigned to real people from your org chart. Tasks are dispatched to external integrations."
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
                  {detail.data.approval.decided_at.slice(0, 10)}. Tasks were successfully dispatched to active connectors.
                </p>
              ) : (
                <div className="mt-5 rounded-md border border-line-strong bg-elevated p-4">
                  <h3 className="text-sm font-medium text-fg">Approve this workflow</h3>
                  <p className="mt-1 text-xs text-fg-muted">
                    Approval records your acceptance of the plan. This will automatically
                    dispatch tasks (e.g. Jira tickets, Outlook emails) to your firm&apos;s active connectors.
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
