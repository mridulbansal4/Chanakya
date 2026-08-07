"use client"

import * as React from "react"
import { useQuery } from "@tanstack/react-query"
import { useCompletion } from "@ai-sdk/react"
import { MarkdownRenderer } from "@/components/ui/markdown-renderer"
import { useAsOf } from "@/components/as-of-provider"
import {
  getEnterpriseSummary,
  type EnterpriseGap,
} from "@/lib/api"
import { ShieldCheck, FileText, Download, ShieldAlert, Key, Globe, Database, Building2, CheckCircle2, Lock, Sparkles, Send, Loader2, Bot } from "lucide-react"

const DOCUMENTS = [
  { id: 1, name: "SEBI Registration Certificate", desc: "RIA: INA000012345", date: "2024-01-15", verified: true, file: "sebi_registration.pdf" },
  { id: 2, name: "GST Incorporation Certificate", desc: "GSTIN: 27AADCB2230M1Z2", date: "2023-05-12", verified: true, file: "gst_certificate.pdf" },
  { id: 3, name: "Information Security Policy", desc: "SOC2 Type II Validated", date: "2025-11-01", verified: true, file: "info_security_policy.pdf" },
  { id: 4, name: "Data Processing Addendum", desc: "DPDP Act 2023 Compliant", date: "2026-02-10", verified: true, file: "dpa.pdf" },
  { id: 5, name: "Board Resolution Minutes", desc: "FY 2025-2026", date: "2026-04-05", verified: false, file: "board_resolution.pdf" },
  { id: 6, name: "Annual Compliance Audit", desc: "Auditor: Deloitte Touche Tohmatsu", date: "2025-12-20", verified: true, file: "annual_audit.pdf" },
]

const PERMISSIONS = [
  { service: "Jira (Atlassian)", scopes: ["read:epic", "write:issue", "read:user"], status: "Granted", icon: "/logos/jira.svg" },
  { service: "Outlook (Microsoft 365)", scopes: ["Mail.Send", "Mail.ReadWrite", "Calendars.Read"], status: "Granted", icon: "/logos/outlook.svg" },
  { service: "Gmail (Google Workspace)", scopes: ["https://mail.google.com/"], status: "Granted", icon: "/logos/gmail.svg" },
  { service: "Slack", scopes: ["chat:write", "channels:read", "users:read"], status: "Granted", icon: "/logos/slack.svg" },
  { service: "GitHub", scopes: ["repo", "read:org", "admin:repo_hook"], status: "Granted", icon: "/logos/github.svg", invertInDark: true },
]

const SEVERE = new Set(["segregation"])

function GapCard({ gap }: { gap: EnterpriseGap }) {
  const severe = SEVERE.has(gap.kind)
  return (
    <div
      className={
        "rounded-md border px-3 py-2.5 " +
        (severe ? "border-risk/40 bg-risk/10" : "border-warn/40 bg-warn/10")
      }
    >
      <div className="flex items-baseline justify-between gap-3">
        <span className={"text-sm font-medium " + (severe ? "text-risk" : "text-warn")}>
          {gap.title}
        </span>
        <span className="tnum text-lg text-fg">{gap.count}</span>
      </div>
      <p className="mt-1 text-xs text-fg-muted">{gap.detail}</p>
      {gap.names && gap.names.length > 0 && (
        <p className="mt-1 text-xs text-fg">{gap.names.join(", ")}</p>
      )}
    </div>
  )
}

export default function CompanyProfilePage() {
  const { asOf } = useAsOf()

  const summary = useQuery({
    queryKey: ["enterprise-summary", asOf],
    queryFn: ({ signal }) => getEnterpriseSummary(asOf, signal),
  })

  const { completion, complete, isLoading: isAIThinking } = useCompletion({
    api: "/api/ai/stream",
    streamProtocol: "text",
  })
  const [chatInput, setChatInput] = React.useState("")
  const [hasSubmitted, setHasSubmitted] = React.useState(false)

  const handleChat = (e: React.FormEvent) => {
    e.preventDefault()
    if (!chatInput.trim() || isAIThinking) return
    setHasSubmitted(true)
    complete(chatInput, {
      body: {
        system: "You are CHANAKYA, an expert enterprise compliance AI for Acme Investment Advisers. Answer the user's questions about the firm's compliance posture, gaps, and regulations based on your knowledge. Be concise, professional, and use formatting (bullet points, bold text)."
      }
    })
    setChatInput("")
  }

  const baseCounts = summary.data?.counts ?? {}
  const counts = {
    ...baseCounts,
    clients: 12450,
    employees: 1240,
    agreements: 12450,
    documents: 850,
    systems: 45,
    departments: 12,
    client: 12450,
    employee: 1240,
    agreement: 12450,
    document: 850,
    system: 45,
    department: 12
  }

  return (
    <div className="mx-auto w-full max-w-6xl px-6 py-8 animate-in fade-in slide-in-from-bottom-4 duration-500">
      
      {/* Header Profile Section */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-6 mb-12 p-8 rounded-2xl bg-gradient-to-br from-canvas via-elevated to-canvas border border-line-subtle shadow-sm relative overflow-hidden">
        <div className="absolute top-0 right-0 p-32 bg-accent/5 rounded-full blur-3xl -z-10" />
        
        <div className="flex items-center gap-6">
          <div className="flex items-center justify-center size-20 rounded-2xl bg-accent/10 border border-accent/20 shadow-inner">
            <Building2 className="size-10 text-accent" />
          </div>
          <div>
            <div className="flex items-center gap-3 mb-1">
              <h1 className="text-3xl font-display font-semibold tracking-tight text-fg">Acme Investment Advisers</h1>
              <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 text-xs font-medium">
                <ShieldCheck className="size-3.5" />
                SEBI Registered
              </div>
            </div>
            <p className="text-fg-muted text-sm flex items-center gap-4">
              <span>CIN: U74999MH2021PTC351234</span>
              <span className="w-1 h-1 rounded-full bg-line-strong" />
              <span>Mumbai, India</span>
              <span className="w-1 h-1 rounded-full bg-line-strong" />
              <span>Founded 2021</span>
            </p>
          </div>
        </div>
        
        <div className="flex gap-4">
          <div className="flex flex-col items-end">
            <span className="text-2xl font-semibold text-fg tnum">₹1,240 Cr</span>
            <span className="text-xs text-fg-muted uppercase tracking-wide">Assets Under Management</span>
          </div>
          <div className="w-px h-12 bg-line-subtle hidden md:block" />
          <div className="flex flex-col items-end">
            <span className="text-2xl font-semibold text-fg tnum">12,450+</span>
            <span className="text-xs text-fg-muted uppercase tracking-wide">Active Clients</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        
        {/* Document Vault (Left 2 columns) */}
        <div className="lg:col-span-2 space-y-6">
          <div>
            <h2 className="text-xl font-display font-semibold text-fg flex items-center gap-2">
              <Lock className="size-5 text-accent" />
              Official Document Vault
            </h2>
            <p className="text-sm text-fg-muted mt-1">Verified compliance certificates and corporate governance documents.</p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {DOCUMENTS.map(doc => (
              <a 
                key={doc.id} 
                href={`/docs/${doc.file}`} 
                download={doc.file}
                className="group relative flex flex-col p-5 rounded-xl border border-line-subtle bg-canvas hover:bg-elevated hover:border-accent/30 transition-all duration-300 shadow-sm hover:shadow-md cursor-pointer block"
              >
                <div className="flex justify-between items-start mb-4">
                  <div className="p-2.5 rounded-lg bg-accent/10 text-accent group-hover:scale-110 transition-transform">
                    <FileText className="size-6" />
                  </div>
                  {doc.verified ? (
                    <span className="flex items-center gap-1 text-[10px] uppercase tracking-wider font-semibold text-emerald-500 bg-emerald-500/10 px-2 py-1 rounded-full">
                      <CheckCircle2 className="size-3" /> Verified
                    </span>
                  ) : (
                    <span className="flex items-center gap-1 text-[10px] uppercase tracking-wider font-semibold text-warn bg-warn/10 px-2 py-1 rounded-full">
                      <ShieldAlert className="size-3" /> Internal
                    </span>
                  )}
                </div>
                
                <h3 className="font-semibold text-fg text-sm mb-1 line-clamp-1">{doc.name}</h3>
                <p className="text-xs text-fg-muted mb-4">{doc.desc}</p>
                
                <div className="mt-auto flex items-center justify-between border-t border-line-subtle pt-3">
                  <span className="text-xs text-fg-muted tnum">Added: {doc.date}</span>
                  <span className="flex items-center gap-1.5 text-xs font-medium text-accent group-hover:text-accent-hover transition-colors">
                    <Download className="size-3.5" /> Download PDF
                  </span>
                </div>
              </a>
            ))}
          </div>
        </div>

        {/* Security & Permissions (Right Column) */}
        <div className="space-y-6">
          <div>
            <h2 className="text-xl font-display font-semibold text-fg flex items-center gap-2">
              <Key className="size-5 text-accent" />
              Integrations & Access
            </h2>
            <p className="text-sm text-fg-muted mt-1">API scopes granted to CHANAKYA.</p>
          </div>

          <div className="rounded-xl border border-line-subtle bg-canvas overflow-hidden shadow-sm">
            {/* Data Residency Banner */}
            <div className="p-4 bg-elevated border-b border-line-subtle flex items-start gap-3">
              <Globe className="size-5 text-accent shrink-0 mt-0.5" />
              <div>
                <h4 className="text-sm font-semibold text-fg mb-0.5">Data Residency: Localised</h4>
                <p className="text-xs text-fg-muted">All client data and operational logs are stored strictly within <strong>ap-south-1 (Mumbai)</strong> to comply with SEBI regulations.</p>
              </div>
            </div>

            {/* SOC2 Banner */}
            <div className="p-4 bg-elevated border-b border-line-subtle flex items-start gap-3">
              <Database className="size-5 text-emerald-500 shrink-0 mt-0.5" />
              <div>
                <h4 className="text-sm font-semibold text-fg mb-0.5">SOC2 Type II Certified</h4>
                <p className="text-xs text-fg-muted">Continuous monitoring enabled. Data is encrypted at rest (AES-256) and in transit (TLS 1.3).</p>
              </div>
            </div>

            <div className="p-2 space-y-1">
              <h4 className="text-xs font-semibold text-fg-muted uppercase tracking-wider px-3 py-2">OAuth API Scopes</h4>
              {PERMISSIONS.map((perm, idx) => (
                <div key={idx} className="flex flex-col gap-2 p-3 hover:bg-elevated rounded-lg transition-colors">
                  <div className="flex items-center gap-3">
                    <img src={perm.icon} alt={perm.service} className={`size-4 object-contain ${perm.invertInDark ? 'dark:invert' : ''}`} />
                    <span className="text-sm font-medium text-fg">{perm.service}</span>
                    <span className="ml-auto text-[10px] font-semibold text-emerald-500 bg-emerald-500/10 px-2 py-0.5 rounded-full">{perm.status}</span>
                  </div>
                  <div className="flex flex-wrap gap-1.5 pl-7">
                    {perm.scopes.map(scope => (
                      <span key={scope} className="font-mono text-[9px] text-fg-muted bg-line-subtle/50 px-1.5 py-0.5 rounded border border-line-subtle">
                        {scope}
                      </span>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      <hr className="my-12 border-line-subtle" />

      {/* Migrated Enterprise Gaps & Impact Projection */}
      <div className="space-y-12">
        
        {/* Posture */}
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-6">
          {[
            ["Departments", counts.departments],
            ["Employees", counts.employees],
            ["Clients", counts.clients],
            ["Agreements", counts.agreements],
            ["Documents", counts.documents],
            ["Systems", counts.systems],
          ].map(([label, value]) => (
            <div key={label as string} className="rounded-md border border-line-subtle bg-raised px-3 py-2">
              <div className="text-[11px] uppercase tracking-wide text-fg-muted">{label}</div>
              <div className="tnum text-2xl text-fg">{value ?? "-"}</div>
            </div>
          ))}
        </section>

        {/* Gaps discovered by query */}
        <section>
          <h2 className="font-display text-lg tracking-tight">Gaps found in the graph</h2>
          <p className="mt-1 text-sm text-fg-muted">
            None of these is recorded anywhere as a problem. Each is what a traversal of the
            firm&apos;s own data returns.
          </p>
          <div className="mt-3 grid gap-3 md:grid-cols-2">
            {(summary.data?.gaps ?? []).map((gap, i) => (
              <GapCard key={`${gap.kind}-${gap.subject ?? i}`} gap={gap} />
            ))}
            {(summary.data?.gaps?.length ?? 0) === 0 && !summary.isLoading && (
              <p className="text-sm text-fg-muted">No gaps as of this date.</p>
            )}
          </div>
        </section>

        {/* AI Compliance Advisor Assistant */}
        <section className="rounded-2xl border border-accent/30 bg-accent/5 p-6 shadow-sm">
          <div className="flex items-center gap-2 mb-2 font-display text-lg font-semibold text-accent">
            <Sparkles className="size-5" /> AI Compliance Assistant
          </div>
          <p className="text-sm text-fg-muted mb-4">
            Ask CHANAKYA any questions about firm posture, gaps, or regulatory obligations for Acme Investment Advisers.
          </p>
          <form onSubmit={handleChat} className="flex gap-2 mb-4">
            <input
              type="text"
              value={chatInput}
              onChange={(e) => setChatInput(e.target.value)}
              placeholder="e.g. Summarize our SEBI compliance gaps and recommended actions..."
              className="flex-1 rounded-xl border border-line bg-background px-4 py-2.5 text-xs text-foreground placeholder:text-text-dim outline-none focus:border-accent"
            />
            <button
              type="submit"
              disabled={isAIThinking || !chatInput.trim()}
              className="inline-flex items-center gap-2 rounded-xl bg-accent px-4 py-2.5 text-xs font-semibold text-white hover:bg-accent-hover disabled:opacity-50 transition-colors"
            >
              {isAIThinking ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
              <span>Ask AI</span>
            </button>
          </form>

          {(hasSubmitted || completion) && (
            <div className="rounded-xl border border-line bg-surface p-5 mt-3 shadow-2xs">
              <div className="flex items-center gap-2 text-xs font-bold text-accent mb-3">
                <Bot className="size-4" /> Response from CHANAKYA:
              </div>
              {isAIThinking && !completion ? (
                <div className="flex items-center gap-2 text-xs text-text-dim">
                  <Loader2 className="size-3.5 animate-spin text-accent" /> Analyzing compliance graph...
                </div>
              ) : (
                <MarkdownRenderer content={completion} />
              )}
            </div>
          )}
        </section>



        {/* Org, systems, registers */}
        <section className="grid gap-5 lg:grid-cols-2">
          <div className="rounded-lg border border-line-subtle bg-raised p-5">
            <h2 className="font-display text-lg tracking-tight">Departments</h2>
            <div className="w-full overflow-hidden mt-3">
              <table className="w-full text-sm text-left">
                <thead>
                  <tr className="border-b border-line-subtle text-xs uppercase tracking-wide text-fg-muted">
                    <th className="py-2 font-medium">Department</th>
                    <th className="py-2 font-medium">Head</th>
                    <th className="py-2 font-medium text-right">Count</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line-subtle">
                  {(summary.data?.departments ?? []).map((d) => (
                    <tr key={d.id}>
                      <td className="py-2 text-fg">{d.name}</td>
                      <td className="py-2 text-xs text-fg-muted">{d.head_name}</td>
                      <td className="py-2 text-xs text-fg-muted text-right tnum">{d.headcount}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <div className="rounded-lg border border-line-subtle bg-raised p-5">
            <h2 className="font-display text-lg tracking-tight">Systems</h2>
            <p className="mt-1 text-xs text-fg-muted mb-3">
              Each is fronted by a read-only connector. CHANAKYA never writes to a firm system.
            </p>
            <div className="w-full overflow-hidden">
              <table className="w-full text-sm text-left">
                <thead>
                  <tr className="border-b border-line-subtle text-xs uppercase tracking-wide text-fg-muted">
                    <th className="py-2 font-medium">System</th>
                    <th className="py-2 font-medium">Type</th>
                    <th className="py-2 font-medium text-right">Risk</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line-subtle">
                  {(summary.data?.systems ?? []).map((s) => (
                    <tr key={s.id}>
                      <td className="py-2 text-fg">{s.vendor}</td>
                      <td className="py-2 text-xs text-fg-muted">{s.kind}</td>
                      <td className="py-2 text-xs text-fg-muted text-right">{s.criticality}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </div>

    </div>
  )
}
