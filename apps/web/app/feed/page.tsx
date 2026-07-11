"use client"

import { useQuery } from "@tanstack/react-query"
import { CheckCircle2, ExternalLink, ShieldCheck } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { DeonticBadge, StatusBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { feedSchemaUrl, feedUrl, getFeed } from "@/lib/api"

export default function FeedPage() {
  const { asOf } = useAsOf()
  const feed = useQuery({
    queryKey: ["feed", asOf],
    queryFn: ({ signal }) => getFeed(asOf, signal),
  })
  const f = feed.data

  return (
    <div className="mx-auto max-w-4xl px-6 py-6">
      <div className="mb-4 flex items-start justify-between">
        <div>
          <h1 className="font-display text-2xl">Regulator Feed</h1>
          <p className="text-sm text-muted-foreground">
            Machine-readable, versioned SupTech feed of obligations with causal
            provenance. Read-only.
          </p>
        </div>
        <div className="flex gap-2 text-xs">
          <a
            href={feedUrl(asOf)}
            target="_blank"
            rel="noreferrer"
            className="hairline inline-flex items-center gap-1 rounded-md bg-surface px-2.5 py-1.5 text-foreground"
          >
            raw feed <ExternalLink className="size-3" />
          </a>
          <a
            href={feedSchemaUrl()}
            target="_blank"
            rel="noreferrer"
            className="hairline inline-flex items-center gap-1 rounded-md bg-surface px-2.5 py-1.5 text-foreground"
          >
            schema <ExternalLink className="size-3" />
          </a>
        </div>
      </div>

      {feed.isError && (
        <p className="text-sm text-danger">Backend unreachable — is the API running on :8080?</p>
      )}

      {f && (
        <>
          <div className="hairline mb-4 flex flex-wrap items-center gap-x-6 gap-y-1 rounded-md bg-surface px-4 py-3 text-xs">
            <Meta label="feed_version" value={f.feed_version} />
            <Meta label="source" value={f.source} />
            <Meta label="regulator" value={f.regulator} />
            <Meta label="obligations" value={String(f.obligations.length)} />
            <span className="ml-auto inline-flex items-center gap-1 text-verified">
              <CheckCircle2 className="size-3.5" /> validated against schema
            </span>
          </div>

          <ul className="space-y-2">
            {f.obligations.map((o) => (
              <li key={o.id} className="hairline rounded-md bg-surface p-3 text-sm">
                <div className="flex items-center gap-2">
                  <span className="tnum text-primary">{o.clause_ref}</span>
                  <DeonticBadge deontic={o.deontic_type} />
                  <StatusBadge status={o.status} />
                  <span className="ml-auto">
                    <ConfidenceMeter value={o.provenance.extractor_confidence} />
                  </span>
                </div>
                <blockquote className="mt-1.5 border-l-2 border-line pl-3 text-xs leading-relaxed text-muted-foreground">
                  {o.provenance.source_sentence}
                </blockquote>
                {o.provenance.signoff ? (
                  <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px]">
                    <span className="inline-flex items-center gap-1 text-verified">
                      <ShieldCheck className="size-3.5" />
                      signed by {o.provenance.signoff.signed_by}
                    </span>
                    <span className="tnum text-muted-foreground">
                      hash {o.provenance.signoff.obligation_hash.slice(0, 16)}…
                    </span>
                  </div>
                ) : (
                  <div className="mt-2 text-[11px] text-muted-foreground">
                    no sign-off yet — not enforceable
                  </div>
                )}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  )
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="tnum text-muted-foreground">{label}:</span>
      <span className="tnum text-foreground">{value}</span>
    </span>
  )
}
