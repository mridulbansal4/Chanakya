/**
 * Graph canvas theme.
 *
 * React Flow needs real colour strings for SVG strokes, pattern fills and
 * legend swatches, which is how hardcoded hex values got into the graph
 * components in the first place. These are `var(--token)` references rather
 * than literals: they resolve in the DOM exactly like any other CSS value,
 * so the graph stays bound to the same palette as the rest of the app and
 * cannot drift from it.
 */

export const GRAPH = {
  /** The canvas is a sunken well — a viewport into data, not another card. */
  canvas: "var(--sunken)",
  dots: "var(--line-subtle)",

  /** Edge strokes. Provenance is the meaningful link and gets the accent. */
  edgeProvenance: "var(--accent)",
  edgeDefault: "var(--line-strong)",

  handle: "var(--fg-faint)",
} as const

/** Node status colours, matching the StatusDot vocabulary in badges.tsx. */
export const GRAPH_STATUS = {
  approved: "var(--ok)",
  needs_review: "var(--warn)",
  pending: "var(--fg-faint)",
  rejected: "var(--risk)",
} as const

/**
 * Legend entries for the overview graph.
 *
 * Shared rather than redeclared per screen, because a legend that disagrees
 * with the thing it explains is worse than no legend at all.
 */
export const OVERVIEW_LEGEND = [
  { color: "var(--fg-subtle)", label: "Clause — regulation source" },
  { color: "var(--fg)", label: "Obligation — deontic duty" },
  { color: "var(--ok)", label: "Approved" },
  { color: "var(--warn)", label: "Needs review or pending" },
  { color: "var(--risk)", label: "Rejected" },
  { color: "var(--accent)", label: "Provenance link", line: true },
] as const

/**
 * Blast radius.
 *
 * `semantic` is amber because a semantic match is a *suggestion* that needs
 * human judgement — the same meaning amber carries everywhere else in the
 * app. It is not amber merely to look different from the direct hits.
 */
export const BLAST_KIND = {
  amended: { color: "var(--accent)", tag: "Amended" },
  direct: { color: "var(--accent)", tag: "Direct" },
  semantic: { color: "var(--warn)", tag: "Semantic" },
  control: { color: "var(--ok)", tag: "Control" },
  evidence: { color: "var(--fg-subtle)", tag: "Evidence" },
} as const

export function blastKind(kind: string): { color: string; tag: string } {
  return (
    BLAST_KIND[kind as keyof typeof BLAST_KIND] ?? {
      color: "var(--fg-faint)",
      tag: kind,
    }
  )
}

export const BLAST_LEGEND = [
  { color: "var(--accent)", label: "Amended or directly affected" },
  { color: "var(--warn)", label: "Semantic match — needs judgement" },
  { color: "var(--ok)", label: "Control" },
  { color: "var(--fg-subtle)", label: "Evidence" },
  { color: "var(--warn)", label: "Semantic link", line: true, dashed: true },
] as const

/** Lineage node types, coloured by the role each plays in the chain. */
export const LINEAGE_TYPE: Record<string, string> = {
  clause: "var(--fg-subtle)",
  obligation: "var(--accent)",
  control: "var(--ok)",
  evidence: "var(--fg-subtle)",
  signoff: "var(--ok)",
  policy: "var(--warn)",
}

export function lineageDot(type: string): string {
  return LINEAGE_TYPE[type] ?? "var(--fg-subtle)"
}
