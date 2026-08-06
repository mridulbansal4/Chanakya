package ingest

import (
	"fmt"
	"regexp"
	"strings"
)

// Stage 6 - cross-reference resolution.
//
// Regulations are a web of citations: "clause 3.1", "regulation 15", "Annexure
// A", "the said circular". Resolved, they are graph edges that make blast radius
// and lineage work. UNRESOLVED, they are the most important output of this
// stage: a reference the pipeline could not follow becomes a dangling_reference
// row in the review queue, never a silently dropped edge. A graph that looks
// complete but is not is worse than one that admits its gaps.

// RefKind is what a reference points at.
type RefKind string

const (
	RefClause     RefKind = "clause"
	RefRegulation RefKind = "regulation"
	RefAnnexure   RefKind = "annexure"
	RefDocument   RefKind = "document"
	RefVague      RefKind = "vague"
)

// ClauseRef is a resolved intra-document reference: an edge from one clause to
// another in the same circular.
type ClauseRef struct {
	FromClauseID string  `json:"from_clause_id"`
	ToClauseID   string  `json:"to_clause_id"`
	RawText      string  `json:"raw_text"`
	Kind         RefKind `json:"kind"`
}

// DanglingRef is a reference that could not be resolved.
type DanglingRef struct {
	ID         string  `json:"id"`
	CircularID string  `json:"circular_id"`
	ClauseID   string  `json:"clause_id"`
	RawText    string  `json:"raw_text"`
	Kind       RefKind `json:"kind"`
	Reason     string  `json:"reason"`
}

var (
	// Reuses the label vocabulary compiler.normalizeClauseRef already handles
	// ("Clause 3", "cl. 3", "§3", "reg 3"), extended with the numbering shapes
	// that appear in citations.
	reRefClause     = regexp.MustCompile(`(?i)\b(?:clause|cl\.|cl|para|paragraph|point|§)\s*(\d+(?:\.\d+)*)\b`)
	reRefRegulation = regexp.MustCompile(`(?i)\b(?:regulation|reg\.|reg)\s*(\d+(?:\.\d+)*(?:\(\d+\))?)\b`)
	reRefAnnexure   = regexp.MustCompile(`(?i)\b(annexure|annex|schedule|appendix)\s+([A-Z0-9]+)\b`)
	reRefVague      = regexp.MustCompile(`(?i)\bthe\s+said\s+(circular|regulation|clause|notification)\b`)
)

// ResolveReferences resolves every reference in every clause of a document.
//
// clauseByRef maps a canonical clause ref (as normalizeClauseRef produces) to
// its clause id, so intra-document references resolve against the clause tree.
// knownCirculars is the set of circular numbers Stage 3 recorded relations to,
// used to resolve inter-document references.
func ResolveReferences(
	circularID string,
	clauses []clauseText,
	clauseByRef map[string]string,
	knownCirculars map[string]bool,
) ([]ClauseRef, []DanglingRef) {
	var (
		resolved []ClauseRef
		dangling []DanglingRef
		// One edge per (from,to,raw) triple. A clause that cites 3.1 three times
		// is one relationship, not three.
		seenEdge = map[string]bool{}
		seenDang = map[string]bool{}
	)

	record := func(from, to, raw string, kind RefKind) {
		// Self-reference and cycles: record the edge once and stop. Chasing a
		// cycle looking for "the real target" would loop forever and invent a
		// target the text never named.
		if to == from {
			return
		}
		key := from + "|" + to + "|" + raw
		if seenEdge[key] {
			return
		}
		seenEdge[key] = true
		resolved = append(resolved, ClauseRef{FromClauseID: from, ToClauseID: to, RawText: raw, Kind: kind})
	}

	dangle := func(from, raw string, kind RefKind, reason string) {
		key := from + "|" + raw
		if seenDang[key] {
			return
		}
		seenDang[key] = true
		dangling = append(dangling, DanglingRef{
			ID:         fmt.Sprintf("dref:%s:%s", from, normalizeRefKey(raw)),
			CircularID: circularID,
			ClauseID:   from,
			RawText:    raw,
			Kind:       kind,
			Reason:     reason,
		})
	}

	for _, c := range clauses {
		for _, m := range reRefClause.FindAllStringSubmatch(c.Text, -1) {
			raw, target := m[0], normalizeRefKey(m[1])
			if id, ok := clauseByRef[target]; ok {
				record(c.ID, id, raw, RefClause)
			} else {
				dangle(c.ID, raw, RefClause, "no clause with this ref in the document")
			}
		}

		for _, m := range reRefRegulation.FindAllStringSubmatch(c.Text, -1) {
			// A regulation reference points OUTSIDE the circular, at the SEBI
			// (Investment Advisers) Regulations. CHANAKYA does not hold the
			// regulations, so this is always dangling - and saying so is the
			// honest answer, not a failure.
			dangle(c.ID, m[0], RefRegulation, "regulations are not ingested; reference is external")
		}

		for _, m := range reRefAnnexure.FindAllStringSubmatch(c.Text, -1) {
			raw := m[0]
			target := normalizeRefKey(strings.ToLower(m[1]) + "-" + strings.ToLower(m[2]))
			if id, ok := clauseByRef[target]; ok {
				record(c.ID, id, raw, RefAnnexure)
			} else {
				dangle(c.ID, raw, RefAnnexure, "no annexure with this label in the document")
			}
		}

		for _, m := range reRefVague.FindAllString(c.Text, -1) {
			// "the said circular" has a referent only in context. Resolving it
			// to the most recently mentioned document would be a guess with the
			// same shape as a fact, so it is recorded as vague instead.
			if len(knownCirculars) == 1 {
				dangle(c.ID, m, RefVague, "anaphoric reference; single related circular known but not asserted")
			} else {
				dangle(c.ID, m, RefVague, "anaphoric reference with no unambiguous referent")
			}
		}
	}
	return resolved, dangling
}

// clauseText is the minimal view of a clause this stage needs.
type clauseText struct {
	ID   string
	Ref  string
	Text string
}

// normalizeRefKey canonicalises a reference target for lookup, mirroring
// compiler.normalizeClauseRef: lowercase, no spaces, no trailing period.
func normalizeRefKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	return strings.TrimSuffix(s, ".")
}
