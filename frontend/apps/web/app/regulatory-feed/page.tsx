"use client"

import * as React from "react"
import { useQuery } from "@tanstack/react-query"
import Link from "next/link"

import { useAsOf } from "@/components/as-of-provider"
import { PageHeader } from "@/components/page-header"
import { getRegulatoryFeed, type ClauseLineageChange } from "@/lib/api"

/**
 * The regulatory corpus, driven entirely by the API.
 *
 * This screen previously rendered a scripted client-side simulation with the
 * MITC circular's contents hardcoded in lib/amendment-sim.ts. That file is gone.
 * Everything below is a query result: which circulars CHANAKYA holds, how each
 * one arrived, what it supersedes, and - for an amendment - the actual
 * clause-level diff recorded when a human approved it.
 */

const RELATION_TONE: Record<string, string> = {
  supersedes: "border-risk/40 bg-risk/10 text-risk",
  amends: "border-warn/40 bg-warn/10 text-warn",
  references: "border-line-strong bg-elevated text-fg-muted",
}

const CHANGE_TONE: Record<string, string> = {
  modified: "bg-warn/15 text-warn",
  added: "bg-ok/15 text-ok",
  deleted: "bg-risk/15 text-risk",
  unchanged: "bg-elevated text-fg-muted",
}

function ChangeRow({ change }: { change: ClauseLineageChange }) {
  return (
    <li className="rounded-md border border-line-subtle px-3 py-2">
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span
          className={
            "rounded px-1.5 py-0.5 uppercase tracking-wide " +
            (CHANGE_TONE[change.relation] ?? "bg-elevated text-fg-muted")
          }
        >
          {change.relation}
        </span>
        <span className="tnum text-fg">{change.clause_ref}</span>
        {change.score > 0 && (
          <span className="tnum text-fg-muted">score {change.score.toFixed(3)}</span>
        )}
      </div>
      {(change.old_text || change.new_text) && (
        <div className="mt-2 grid gap-2 md:grid-cols-2">
          <div className="rounded border border-risk/30 bg-risk/5 p-2">
            <div className="text-[10px] uppercase tracking-wide text-fg-muted">Before</div>
            <p className="mt-1 text-xs text-fg">{change.old_text || "(no prior version)"}</p>
          </div>
          <div className="rounded border border-ok/30 bg-ok/5 p-2">
            <div className="text-[10px] uppercase tracking-wide text-fg-muted">After</div>
            <p className="mt-1 text-xs text-fg">{change.new_text || "(retired)"}</p>
          </div>
        </div>
      )}
    </li>
  )
}

export default function RegulatoryFeedPage() {
  const { asOf } = useAsOf()

  const feed = useQuery({
    queryKey: ["regulatory-feed", asOf],
    queryFn: ({ signal }) => getRegulatoryFeed(asOf, signal),
  })

  const circulars = feed.data?.circulars ?? []
  const runs = feed.data?.runs ?? []

  return (
    <div className="mx-auto w-full max-w-6xl px-6 py-6">
      <PageHeader
        eyebrow="Regulatory corpus"
        title="Circulars CHANAKYA holds"
        description="Every circular in the graph, how it arrived, what it supersedes, and - for an amendment - the clause-level diff that was applied when a human approved it."
      />

      {/* The honest statement about monitoring, served by the API rather than
          written into the page, so it cannot drift from what the system does. */}
      {feed.data?.monitoring_note && (
        <p className="mt-4 rounded-md border border-line-strong bg-elevated px-3 py-2 text-sm text-fg-muted">
          {feed.data.monitoring_note}{" "}
          <Link href="/ingest" className="text-fg underline underline-offset-2">
            Upload a circular
          </Link>
          .
        </p>
      )}

      {feed.isLoading && <p className="mt-6 text-sm text-fg-muted">Loading the corpus...</p>}
      {feed.isError && (
        <p className="mt-6 rounded-md border border-risk/40 bg-risk/10 px-3 py-2 text-sm text-risk">
          Could not reach the backend.
        </p>
      )}

      <section className="mt-5 space-y-4">
        {circulars.map((c) => (
          <article key={c.circular_id} className="rounded-lg border border-line-subtle bg-raised p-5">
            <div className="flex flex-wrap items-baseline justify-between gap-3">
              <div className="min-w-0">
                <h2 className="font-display text-lg tracking-tight">{c.title}</h2>
                <p className="tnum mt-0.5 text-xs text-fg-muted">{c.circular_id}</p>
              </div>
              <div className="flex flex-wrap items-center gap-2 text-xs">
                {c.doc_kind && (
                  <span className="rounded bg-elevated px-1.5 py-0.5 text-fg-muted">{c.doc_kind}</span>
                )}
                <span className="text-fg-muted">{c.regulator}</span>
                <span className="tnum text-fg-muted">{c.issued_on?.slice(0, 10)}</span>
              </div>
            </div>

            <dl className="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-xs text-fg-muted">
              <div>
                <dt className="inline">Clauses: </dt>
                <dd className="tnum inline text-fg">{c.clauses}</dd>
              </div>
              <div>
                <dt className="inline">Obligations: </dt>
                <dd className="tnum inline text-fg">{c.obligations}</dd>
              </div>
              <div>
                <dt className="inline">Source: </dt>
                <dd className="inline text-fg">{c.source}</dd>
              </div>
              {c.approved_by && (
                <div>
                  <dt className="inline">Approved by: </dt>
                  <dd className="inline text-fg">{c.approved_by}</dd>
                </div>
              )}
            </dl>

            {(c.relations?.length ?? 0) > 0 && (
              <div className="mt-3 flex flex-wrap gap-2">
                {(c.relations ?? []).map((r, i) => (
                  <span
                    key={`${r.kind}-${r.to_ref}-${i}`}
                    className={
                      "rounded-full border px-2 py-0.5 text-[11px] " +
                      (RELATION_TONE[r.kind] ?? RELATION_TONE.references)
                    }
                  >
                    {r.kind} {r.to_ref}
                  </span>
                ))}
              </div>
            )}

            {c.amendment && (
              <div className="mt-4">
                <div className="flex flex-wrap gap-3 text-xs text-fg-muted">
                  {Object.entries(c.amendment.counts).map(([kind, n]) => (
                    <span key={kind}>
                      <span className="tnum text-fg">{n}</span> {kind}
                    </span>
                  ))}
                </div>
                {(c.amendment.changes?.length ?? 0) > 0 && (
                  <ul className="mt-2 space-y-2">
                    {(c.amendment.changes ?? []).map((ch) => (
                      <ChangeRow key={`${ch.relation}-${ch.new_clause_id}-${ch.old_clause_id}`} change={ch} />
                    ))}
                  </ul>
                )}
              </div>
            )}
          </article>
        ))}

        {!feed.isLoading && circulars.length === 0 && (
          <p className="rounded-md border border-line-subtle bg-raised px-4 py-6 text-sm text-fg-muted">
            No circulars in the corpus as of this date.{" "}
            <Link href="/ingest" className="text-fg underline underline-offset-2">
              Upload one
            </Link>{" "}
            to get started.
          </p>
        )}
      </section>

      <section className="mt-6 rounded-lg border border-line-subtle bg-raised p-5">
        <h2 className="font-display text-lg tracking-tight">Ingestion history</h2>
        <p className="mt-1 text-xs text-fg-muted">
          Every upload attempt, including the ones that were discarded or failed. Job and run rows
          are never deleted - they are part of the audit trail.
        </p>
        <ul className="mt-3 divide-y divide-line-subtle">
          {runs.map((run) => (
            <li key={run.id} className="flex flex-wrap items-center gap-3 py-2 text-sm">
              <span className="text-fg">{run.filename}</span>
              <span className="rounded bg-elevated px-1.5 py-0.5 text-xs text-fg-muted">{run.state}</span>
              {run.doc_kind && <span className="text-xs text-fg-muted">{run.doc_kind}</span>}
              <span className="tnum ml-auto text-xs text-fg-muted">
                {run.created_at.slice(0, 16).replace("T", " ")}
              </span>
            </li>
          ))}
          {runs.length === 0 && (
            <li className="py-2 text-sm text-fg-muted">Nothing has been ingested yet.</li>
          )}
        </ul>
      </section>
    </div>
  )
}
