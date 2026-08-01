"use client"

import { useQuery } from "@tanstack/react-query"
import { CheckCircle2, ExternalLink, ShieldCheck, Code2, Copy } from "lucide-react"

import { useAsOf } from "@/components/as-of-provider"
import { DeonticBadge, StatusBadge } from "@/components/badges"
import { ConfidenceMeter } from "@/components/confidence"
import { Button } from "@workspace/ui/components/button"
import { CardSkeleton } from "@/components/skeleton"
import { EmptyState } from "@/components/empty-state"
import { feedSchemaUrl, feedUrl, getFeed } from "@/lib/api"

export default function FeedPage() {
  const { asOf } = useAsOf()
  const feed = useQuery({
    queryKey: ["feed", asOf],
    queryFn: ({ signal }) => getFeed(asOf, signal),
  })
  const f = feed.data

  return (
    <div className="mx-auto max-w-5xl px-6 py-8 space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4 border-b border-line pb-6">
        <div>
          <div className="eyebrow mb-1">SupTech Data Interchange</div>
          <h1 className="font-display text-3xl font-bold tracking-tight">Regulator Feed</h1>
          <p className="mt-1 text-sm text-text-dim max-w-2xl leading-relaxed">
            Machine-readable, versioned SupTech JSON feed of extracted obligations with cryptographic provenance hashes for regulatory ingestion.
          </p>
        </div>
        <div className="flex gap-2 text-xs">
          <Button variant="outline" size="sm" onClick={() => window.open(feedUrl(asOf), "_blank")}>
            <span>Raw JSON Feed</span>
            <ExternalLink className="size-3" />
          </Button>
          <Button variant="outline" size="sm" onClick={() => window.open(feedSchemaUrl(), "_blank")}>
            <span>JSON Schema</span>
            <ExternalLink className="size-3" />
          </Button>
        </div>
      </div>

      {feed.isLoading && (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <CardSkeleton key={i} />
          ))}
        </div>
      )}

      {feed.isError && (
        <EmptyState
          icon="alert"
          title="SupTech Feed Offline"
          description="Could not connect to backend server on port 8080."
          primaryAction={{ label: "Retry", onClick: () => feed.refetch() }}
        />
      )}

      {f && (
        <>
          {/* Metadata Banner */}
          <div className="rounded-lg border border-line bg-surface p-5 shadow-xs flex flex-wrap items-center gap-x-6 gap-y-2 text-xs font-mono">
            <Meta label="feed_version" value={f.feed_version} />
            <Meta label="source" value={f.source} />
            <Meta label="regulator" value={f.regulator} />
            <Meta label="total_obligations" value={String(f.obligations.length)} />
            <span className="ml-auto inline-flex items-center gap-1.5 font-sans font-bold text-ok bg-ok/10 border border-ok/30 px-3 py-1 rounded-full">
              <CheckCircle2 className="size-3.5" /> Validated Against Schema
            </span>
          </div>

          <ul className="space-y-3">
            {f.obligations.map((o) => (
              <li
                key={o.id}
                className="rounded-lg border border-line bg-surface p-5 text-sm shadow-xs transition-all duration-200 hover:shadow-elev-1 hover:border-foreground/30 space-y-3"
              >
                <div className="flex flex-wrap items-center gap-3">
                  <span className="tnum font-bold text-primary bg-cream-200/80 px-2.5 py-1 rounded-lg text-xs">
                    Clause {o.clause_ref}
                  </span>
                  <DeonticBadge deontic={o.deontic_type} />
                  <StatusBadge status={o.status} />
                  <span className="ml-auto">
                    <ConfidenceMeter value={o.provenance.extractor_confidence} />
                  </span>
                </div>

                <blockquote className="border-l-2 border-line pl-3 text-xs leading-relaxed text-text-dim italic bg-cream/30 py-2 pr-3 rounded-r-lg">
                  &quot;{o.provenance.source_sentence}&quot;
                </blockquote>

                {o.provenance.signoff ? (
                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs font-mono pt-1">
                    <span className="inline-flex items-center gap-1.5 font-bold text-ok">
                      <ShieldCheck className="size-4" />
                      Signed by {o.provenance.signoff.signed_by}
                    </span>
                    <span className="tnum text-text-dim bg-cream-200/60 px-2 py-0.5 rounded text-[11px]">
                      hash: {o.provenance.signoff.obligation_hash.slice(0, 24)}…
                    </span>
                  </div>
                ) : (
                  <div className="text-xs text-text-dim font-mono italic">
                    Status: Unsigned — Not Enforceable
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
    <span className="inline-flex items-center gap-2">
      <span className="text-text-dim">{label}:</span>
      <span className="font-bold text-foreground">{value}</span>
    </span>
  )
}
