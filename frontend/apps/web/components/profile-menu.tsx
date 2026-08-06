"use client"

import * as React from "react"
import Link from "next/link"
import { CheckCircle2, ChevronRight, PlayCircle, Building2 } from "lucide-react"
import { cn } from "@workspace/ui/lib/utils"

const OFFICER = {
  name: "Priya Menon",
  role: "Compliance Officer",
  firm: "Acme Investment Advisers",
}

const CONNECTORS = [
  {
    name: "Jira",
    status: "Active",
    logo: "/logos/jira.svg",
  },
  {
    name: "Outlook",
    status: "Active",
    logo: "/logos/outlook.svg",
  },
  {
    name: "Gmail",
    status: "Active",
    logo: "/logos/gmail.svg",
  },
  {
    name: "Slack",
    status: "Active",
    logo: "/logos/slack.svg",
  },
  {
    name: "Microsoft Teams",
    status: "Active",
    logo: "/logos/teams.svg",
  },
  {
    name: "GitHub",
    status: "Active",
    logo: "/logos/github.svg",
    invertInDark: true,
  },
]

const OPERATIONS = [
  {
    id: 1,
    title: "Update Jira Epic #482",
    connector: "Jira",
    action: "Update Task",
    logo: "/logos/jira.svg",
  },
  {
    id: 2,
    title: "Send Client Notice",
    connector: "Gmail",
    action: "Send Mail",
    logo: "/logos/gmail.svg",
  },
  {
    id: 3,
    title: "Alert Compliance Team",
    connector: "Slack",
    action: "Send Message",
    logo: "/logos/slack.svg",
  }
]

export function ProfileMenu() {
  const [open, setOpen] = React.useState(false)
  const [executing, setExecuting] = React.useState<number | null>(null)
  const [completed, setCompleted] = React.useState<number[]>([])
  const ref = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener("mousedown", handler)
    return () => document.removeEventListener("mousedown", handler)
  }, [])

  const handleExecute = (id: number) => {
    setExecuting(id)
    setTimeout(() => {
      setExecuting(null)
      setCompleted(prev => [...prev, id])
    }, 1500)
  }

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        title={`${OFFICER.name} · ${OFFICER.role} · ${OFFICER.firm}`}
        aria-label={`Signed in as ${OFFICER.name}`}
        className={cn(
          "hidden shiny-cta size-8 shrink-0 place-items-center rounded-full !p-0 font-medium text-xs text-fg md:grid transition-all",
          open && "ring-2 ring-accent ring-offset-2 ring-offset-canvas"
        )}
      >
        <span>{OFFICER.name.split(" ").map((w) => w[0]).join("")}</span>
      </button>
      {open && (
        <div className="hairline absolute right-0 z-40 mt-2 w-[340px] rounded-xl bg-overlay p-4 shadow-elev-3 backdrop-blur-xl border border-line-subtle animate-in fade-in zoom-in-95 duration-200 overflow-hidden flex flex-col max-h-[85vh]">
          <div className="mb-4 shrink-0">
            <p className="font-semibold text-sm text-fg">{OFFICER.name}</p>
            <p className="text-xs text-fg-muted">{OFFICER.role}</p>
            <p className="text-xs text-fg-muted">{OFFICER.firm}</p>
          </div>
          
          <Link 
            href="/company" 
            onClick={() => setOpen(false)}
            className="flex items-center justify-between p-2.5 mb-2 shrink-0 rounded-lg bg-accent/10 text-accent hover:bg-accent/20 transition-colors border border-accent/20 font-medium text-sm group"
          >
            <div className="flex items-center gap-2">
              <Building2 className="size-4" />
              Corporate Profile
            </div>
            <ChevronRight className="size-4 opacity-50 group-hover:opacity-100 group-hover:translate-x-0.5 transition-all" />
          </Link>

          <div className="h-px w-full bg-line-subtle my-2 shrink-0" />
          
          <div className="flex-1 overflow-y-auto min-h-0 pr-2 pb-2 space-y-5 no-scrollbar">
            <div className="space-y-1">
              <h4 className="text-[11px] font-semibold text-fg-muted mb-3 tracking-wide uppercase">Actionable Dispatches</h4>
              <div className="space-y-2">
                {OPERATIONS.map((op) => {
                  const isDone = completed.includes(op.id)
                  const isExec = executing === op.id
                  return (
                    <div key={op.id} className="flex flex-col gap-2 p-2.5 rounded-lg border border-line-subtle bg-canvas/50">
                      <div className="flex items-center gap-2.5">
                        <img src={op.logo} alt={op.connector} className="size-4 object-contain brightness-90" />
                        <span className="text-xs font-medium text-fg flex-1">{op.title}</span>
                      </div>
                      <div className="flex justify-end">
                        <button
                          onClick={() => !isDone && !isExec && handleExecute(op.id)}
                          disabled={isDone || isExec}
                          className={cn(
                            "inline-flex items-center gap-1.5 text-[11px] font-medium px-2.5 py-1 rounded-md transition-colors",
                            isDone ? "bg-emerald-500/10 text-emerald-500 cursor-default" :
                            isExec ? "bg-accent/20 text-accent cursor-wait animate-pulse" :
                            "bg-elevated hover:bg-line-strong text-fg-muted hover:text-fg border border-line-subtle"
                          )}
                        >
                          {isDone ? (
                            <><CheckCircle2 className="size-3" /> Dispatched</>
                          ) : isExec ? (
                            <><div className="size-3 rounded-full border-2 border-accent border-r-transparent animate-spin" /> Dispatching...</>
                          ) : (
                            <><PlayCircle className="size-3" /> {op.action}</>
                          )}
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>

            <div className="h-px w-full bg-line-subtle" />

            <div className="space-y-1">
              <h4 className="text-[11px] font-semibold text-fg-muted mb-2 tracking-wide uppercase">Active Connectors (6)</h4>
              <p className="text-[11px] text-fg-muted mb-3 leading-relaxed">
                Workflows are actively dispatched to these integrations.
              </p>
              <div className="grid grid-cols-2 gap-2">
                {CONNECTORS.map((c) => (
                  <div key={c.name} className="flex items-center gap-2.5 p-2 rounded-lg bg-canvas/40 border border-line-subtle/50">
                    <div className="flex size-6 shrink-0 items-center justify-center rounded bg-white/5">
                      <img src={c.logo} alt={c.name} className={cn("size-3.5 object-contain", (c as any).invertInDark && "dark:invert")} />
                    </div>
                    <div className="flex flex-col min-w-0">
                      <span className="text-[11px] font-medium text-fg truncate">{c.name}</span>
                      <span className="text-[9px] text-emerald-500 flex items-center gap-1">
                        <span className="size-1.5 rounded-full bg-emerald-500" /> Active
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
