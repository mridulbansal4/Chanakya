"use client"

import * as React from "react"
import { useMutation, useQuery } from "@tanstack/react-query"
import { useCompletion } from "@ai-sdk/react"
import { MarkdownRenderer } from "@/components/ui/markdown-renderer"
import { Zap, AlertTriangle, Layers, Sparkles, Loader2, Download } from "lucide-react"
import jsPDF from "jspdf"
import html2canvas from "html2canvas"

import { useAsOf } from "@/components/as-of-provider"
import { BlastGraph } from "@/components/blast-graph"
import { Button } from "@workspace/ui/components/button"
import { GraphSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import {
  computeBlastRadius,
  listClauses,
  type BlastChange,
  type BlastRadius,
  type Clause,
} from "@/lib/api"

const CATEGORY_COLOR: Record<BlastChange["category"], string> = {
  obligation: "text-warn bg-warn/10 border-warn/20",
  control: "text-ok bg-ok/10 border-ok/20",
  evidence: "text-text-dim bg-cream-200 border-line",
}

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`

function blastSummary(b: BlastRadius): string {
  const s = b.summary
  const needReview = b.nodes.filter(
    (n) => n.type === "obligation" && n.status && n.status !== "approved",
  ).length
  let msg = `This amendment impacts ${plural(s.obligations, "obligation")}, ${plural(
    s.controls,
    "control",
  )}, and ${plural(s.evidence, "evidence source")}.`
  if (needReview > 0) {
    msg += ` ${needReview} obligation${needReview === 1 ? "" : "s"} require compliance officer review.`
  }
  return msg
}

export default function AmendmentsPage() {
  const { asOf } = useAsOf()
  const clauses = useQuery({
    queryKey: ["clauses", asOf],
    queryFn: ({ signal }) => listClauses(asOf, signal),
  })

  const [ref, setRef] = React.useState<string>("")
  const [text, setText] = React.useState<string>("")
  const [runKey, setRunKey] = React.useState(0)

  const clauseList = clauses.data?.clauses ?? []
  React.useEffect(() => {
    if (!clauseList.length) return
    if (clauseList.some((c) => c.clause_ref === ref)) return
    const first = clauseList.find((c) => c.clause_ref === "4.1") ?? clauseList[0]
    setRef(first!.clause_ref)
    setText(first!.text)
  }, [clauseList, ref])

  const selectClause = (c: Clause) => {
    setRef(c.clause_ref)
    setText(c.text)
  }

  const blast = useMutation({
    mutationFn: () =>
      computeBlastRadius({ clause_ref: ref, amended_text: text, as_of: asOf }),
    onSuccess: () => setRunKey((k) => k + 1),
  })

  const { completion, complete, isLoading: isAIThinking } = useCompletion({
    api: "/api/ai/stream",
    streamProtocol: "text",
  })

  const handleGenerateReport = () => {
    if (!blast.data) return
    const context = JSON.stringify(blast.data.changes)
    complete(`Based on the following blast radius changes caused by a SEBI regulation amendment, generate a high-level executive strategic impact report (max 4 bullets) predicting risk level, departments affected, and key takeaways:\n\n${context}`)
  }

  const exportToPDF = async () => {
    const reportElement = document.getElementById("pdf-wrapper")
    if (!reportElement) return

    const originalBackground = reportElement.style.background
    
    const header = document.createElement("div")
    header.id = "pdf-temp-header"
    header.innerHTML = `
      <div style="padding: 32px 40px; background: white; color: black; font-family: sans-serif; border-bottom: 1px solid #e5e7eb;">
        <div style="display: flex; justify-content: space-between; align-items: flex-start;">
          <div>
            <h1 style="margin: 0; font-size: 24px; font-weight: 800; color: #111827; letter-spacing: -0.5px;">FIRM NAME: CHANAKYA COMPLIANCE</h1>
            <p style="margin: 8px 0 0; color: #4b5563; font-size: 14px; font-weight: 500;">GSTIN: 22AAAAA0000A1Z5</p>
          </div>
          <div style="text-align: right;">
            <p style="margin: 0; color: #374151; font-size: 14px; font-weight: 600;">Date: ${new Date().toLocaleDateString()}</p>
            <p style="margin: 4px 0 0; color: #6b7280; font-size: 14px;">Strategic Impact Report</p>
          </div>
        </div>
      </div>
    `
    reportElement.insertBefore(header, reportElement.firstChild)
    reportElement.style.background = "white"

    const isDark = document.documentElement.classList.contains("dark")
    if (isDark) document.documentElement.classList.remove("dark")

    try {
      const canvas = await html2canvas(reportElement, {
        scale: 2,
        backgroundColor: "#ffffff",
        logging: false,
      })
      
      const imgData = canvas.toDataURL("image/png")
      const pdf = new jsPDF({
        orientation: "portrait",
        unit: "mm",
        format: "a4",
      })
      
      const pdfWidth = pdf.internal.pageSize.getWidth()
      const pdfHeight = (canvas.height * pdfWidth) / canvas.width
      
      pdf.addImage(imgData, "PNG", 0, 0, pdfWidth, pdfHeight)
      pdf.save("Blast_Radius_Assessment_Report.pdf")
    } finally {
      reportElement.removeChild(header)
      reportElement.style.background = originalBackground
      if (isDark) document.documentElement.classList.add("dark")
    }
  }

  return (
    <div className="flex flex-col h-full bg-background" id="pdf-wrapper">
      <div className="flex items-center px-5 py-4 border-b border-line bg-surface">
        <h1 className="text-lg font-semibold text-foreground">Blast Radius Assessment</h1>
        <Button variant="outline" size="sm" onClick={exportToPDF} className="ml-auto">
          <Download className="mr-2 size-4" /> Export Report
        </Button>
      </div>

      <div className="flex flex-col lg:flex-row flex-1 min-h-0">
        {/* Left: amendment editor + change list */}
        <div className="flex w-full lg:w-[420px] shrink-0 flex-col lg:border-r border-b lg:border-b-0 border-line bg-surface/60">
          <div className="space-y-4 border-b border-line p-5 bg-surface">
            <div>
              <div className="eyebrow mb-1">Target Regulation Clause</div>
              <select
                value={ref}
                onChange={(e) => {
                  const c = clauseList.find((x) => x.clause_ref === e.target.value)
                  if (c) selectClause(c)
                }}
                className="w-full rounded-xl border border-line bg-background px-3 py-2 text-xs font-semibold text-foreground outline-none focus:border-foreground/40 transition-colors"
              >
                {clauseList.map((c) => (
                  <option key={c.id} value={c.clause_ref}>
                    Clause {c.clause_ref} - {c.heading}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <div className="flex justify-between items-center mb-1">
                <span className="eyebrow">Amended Text Draft</span>
                <span className="text-[10px] font-mono text-text-dim">
                  {text.length} chars
                </span>
              </div>
              <textarea
                value={text}
                onChange={(e) => setText(e.target.value)}
                rows={8}
                placeholder="Enter proposed regulatory text amendment…"
                className="w-full resize-none rounded-xl border border-line bg-background p-3 text-xs leading-relaxed text-foreground placeholder:text-text-dim outline-none focus:border-foreground/40 transition-colors font-mono"
              />
            </div>

            <Button
              variant="default"
              size="lg"
              isLoading={blast.isPending}
              loadingText="Simulating Propagation…"
              disabled={!ref || !text.trim()}
              onClick={() => blast.mutate()}
              className="w-full shadow-elev-1"
            >
              <Zap className="size-4 text-warn" />
              <span>Compute Blast Radius</span>
            </Button>
          </div>

          {/* Change Impact List */}
          <div className="min-h-0 flex-1 overflow-auto p-5 space-y-3">
            {blast.data ? (
              <div className="space-y-3">
                <div className="rounded-xl border border-warn/40 bg-warn/10 p-3.5 text-xs leading-relaxed text-foreground font-medium shadow-2xs">
                  {blastSummary(blast.data)}
                </div>

                {!completion && !isAIThinking && (
                  <Button variant="secondary" onClick={handleGenerateReport} className="w-full text-xs bg-accent/10 text-accent hover:bg-accent/20 border-accent/20">
                    <Sparkles className="size-3 mr-1" /> Generate Executive AI Impact Report
                  </Button>
                )}
                
                {(completion || isAIThinking) && (
                  <div className="rounded-xl border border-accent/30 bg-accent/5 p-4 shadow-sm text-xs text-foreground">
                    <div className="flex items-center gap-1.5 font-bold text-accent mb-2">
                      <Sparkles className="size-3.5" /> AI Strategic Impact Prediction
                    </div>
                    {isAIThinking && !completion ? (
                      <div className="flex items-center gap-2 text-text-dim">
                        <Loader2 className="size-3 animate-spin" /> Analyzing blast radius impact...
                      </div>
                    ) : (
                      <MarkdownRenderer content={completion} />
                    )}
                  </div>
                )}

                <div className="eyebrow mt-4">Impact Breakdown ({blast.data.changes.length})</div>
                <ul className="space-y-2">
                  {blast.data.changes.map((c, i) => (
                    <li
                      key={i}
                      className="rounded-xl border border-line bg-surface p-3 text-xs space-y-1 shadow-2xs transition-all hover:border-foreground/30"
                    >
                      <span
                        className={`inline-block rounded-md border px-2 py-0.5 text-[10px] font-bold uppercase ${
                          CATEGORY_COLOR[c.category]
                        }`}
                      >
                        {c.category}
                      </span>
                      <p className="text-foreground leading-relaxed font-medium">{c.detail}</p>
                    </li>
                  ))}
                </ul>
              </div>
            ) : (
              <div className="p-8 text-center text-xs text-text-dim border border-dashed border-line rounded-lg">
                <Layers className="size-8 mx-auto mb-2 text-text-dim/60" />
                Select a clause above, modify its text, and click &quot;Compute Blast Radius&quot; to calculate downstream impact.
              </div>
            )}
            {blast.isError && (
              <p className="text-xs text-risk font-semibold">Failed to compute blast radius simulation.</p>
            )}
          </div>
        </div>

        {/* Right: animated blast-radius graph */}
        <div className="relative flex-1 bg-cream/40">
          {blast.isPending ? (
            <div className="h-full p-8">
              <GraphSkeleton />
            </div>
          ) : blast.data ? (
            <BlastGraph payload={blast.data} runKey={runKey} />
          ) : (
            <EmptyState
              icon="sparkles"
              title="Blast Radius Visualizer"
              description="The interactive downstream impact graph will automatically layout here after you compute the amendment."
            />
          )}
        </div>
      </div>
    </div>
  )
}
