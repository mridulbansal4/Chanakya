package ingest

import (
	"sort"
	"strings"

	"chanakya/internal/domain"
	"chanakya/internal/vec"
)

// Stage 9 - the amendment matcher.
//
// When a circular is re-uploaded in amended form, the question is not "what does
// this document say" but "what CHANGED". Answering it is what makes bi-temporal
// versioning do real work: an unchanged clause keeps its existing obligation (and
// its sign-off), a modified clause supersedes its predecessor instead of
// overwriting it, and a deleted clause has its world-time interval closed rather
// than vanishing.
//
// EVERY CLASSIFICATION IS PROPOSED, NEVER AUTO-APPLIED. The output lands in the
// same preview/approve queue an initial ingestion does, with old and new text
// side by side. A machine deciding on its own that a clause "did not really
// change" would silently carry a human's sign-off across an amendment they never
// saw.

// ChangeKind is how an incoming clause relates to the existing corpus.
type ChangeKind string

const (
	ChangeUnchanged ChangeKind = "unchanged"
	ChangeModified  ChangeKind = "modified"
	ChangeAdded     ChangeKind = "added"
	ChangeDeleted   ChangeKind = "deleted"
)

// Match thresholds. These are applied EXACTLY as specified - no rounding, no
// buffer zone. A clause that scores 0.919 is `modified`, not `unchanged`, and
// the cost of that is a human looking at a diff, which is the cheap direction to
// be wrong in.
const (
	UnchangedThreshold = 0.92
	ModifiedThreshold  = 0.55
)

// Score weights.
const (
	weightCosine  = 0.45
	weightJaccard = 0.35
	weightRefEq   = 0.20
)

// ClauseChange is one proposed classification.
type ClauseChange struct {
	Kind ChangeKind `json:"kind"`

	NewClauseID  string `json:"new_clause_id,omitempty"`
	NewClauseRef string `json:"new_clause_ref,omitempty"`
	NewText      string `json:"new_text,omitempty"`

	OldClauseID  string `json:"old_clause_id,omitempty"`
	OldClauseRef string `json:"old_clause_ref,omitempty"`
	OldText      string `json:"old_text,omitempty"`

	Score         float64 `json:"score"`
	Cosine        float64 `json:"cosine"`
	Jaccard       float64 `json:"jaccard"`
	RefEqual      bool    `json:"ref_equal"`
	TextIdentical bool    `json:"text_identical"`
	// Rationale explains the verdict in words a reviewer can check.
	Rationale string `json:"rationale"`
}

// AmendmentDiff is the whole proposed change set.
type AmendmentDiff struct {
	Changes []ClauseChange `json:"changes"`
	Counts  map[string]int `json:"counts"`
	// ReusedObligations is the perf win the unchanged path buys: clauses whose
	// obligations do not need re-extraction on a master-circular re-upload.
	ReusedObligations int `json:"reused_obligations"`
}

// ExistingClause is a clause already in the corpus, as the matcher needs it.
type ExistingClause struct {
	ID        string
	ClauseRef string
	Text      string
	// RowUID is the physical row key, used only as the final deterministic
	// tie-break so a match never depends on map or scan order.
	RowUID string
}

// MatchAmendment classifies each incoming clause against the existing corpus.
//
// supersedes reports whether the incoming document supersedes its predecessor.
// It gates the `deleted` verdict: a document that merely REFERENCES another says
// nothing about clauses missing from it, and inferring a deletion from silence
// would retire live regulation.
func MatchAmendment(incoming []domain.Clause, existing []ExistingClause, supersedes bool) AmendmentDiff {
	diff := AmendmentDiff{Counts: map[string]int{}}

	// Pre-compute the old side once. Embedding is the expensive part and each old
	// clause is compared against every new one.
	type oldEntry struct {
		ExistingClause
		vector   []float64
		shingles map[string]bool
		normRef  string
	}
	olds := make([]oldEntry, 0, len(existing))
	for _, e := range existing {
		olds = append(olds, oldEntry{
			ExistingClause: e,
			vector:         vec.Embed(e.Text),
			shingles:       shingle(e.Text),
			normRef:        normalizeRefKey(e.ClauseRef),
		})
	}

	matchedOld := make(map[string]bool, len(existing))

	for _, in := range incoming {
		inVec := vec.Embed(in.Text)
		inShingles := shingle(in.Text)
		inRef := normalizeRefKey(in.ClauseRef)

		best := -1
		bestScore, bestCos, bestJac, bestRefEq := 0.0, 0.0, 0.0, false

		for i, old := range olds {
			cos := vec.Cosine(inVec, old.vector)
			jac := jaccard(inShingles, old.shingles)
			refEq := inRef == old.normRef
			refScore := 0.0
			if refEq {
				refScore = 1.0
			}
			s := weightCosine*cos + weightJaccard*jac + weightRefEq*refScore

			// Many-to-one: take the single highest-scoring match only, never a
			// multi-way merge. Ties break deterministically - same clause ref
			// first, then lowest row_uid - so the same inputs always produce the
			// same proposal.
			better := s > bestScore
			if !better && s == bestScore && best >= 0 {
				cur := olds[best]
				switch {
				case refEq && !(inRef == cur.normRef):
					better = true
				case refEq == (inRef == cur.normRef):
					better = old.RowUID < cur.RowUID
				}
			}
			if better {
				best, bestScore, bestCos, bestJac, bestRefEq = i, s, cos, jac, refEq
			}
		}

		change := ClauseChange{
			NewClauseID: in.ID, NewClauseRef: in.ClauseRef, NewText: in.Text,
			Score: bestScore, Cosine: bestCos, Jaccard: bestJac, RefEqual: bestRefEq,
		}

		switch {
		case best < 0 || bestScore < ModifiedThreshold:
			// Below the modified floor and no ref match: this clause is new.
			change.Kind = ChangeAdded
			change.Rationale = "no existing clause scores at or above the modified threshold"

		case bestScore >= UnchangedThreshold && normalizeWhitespace(in.Text) == normalizeWhitespace(olds[best].Text):
			// BOTH conditions are required: a high score alone is not enough.
			// Two clauses can score 0.95 and still differ by the one word that
			// changes what the firm must do.
			change.Kind = ChangeUnchanged
			change.TextIdentical = true
			change.OldClauseID, change.OldClauseRef, change.OldText = olds[best].ID, olds[best].ClauseRef, olds[best].Text
			change.Rationale = "text is identical and the score is at or above the unchanged threshold; " +
				"the existing obligation can be reused without re-extraction"
			matchedOld[olds[best].ID] = true
			diff.ReusedObligations++

		case bestScore >= ModifiedThreshold:
			change.Kind = ChangeModified
			change.OldClauseID, change.OldClauseRef, change.OldText = olds[best].ID, olds[best].ClauseRef, olds[best].Text
			change.Rationale = "closely matches an existing clause but the text differs; " +
				"the old version will be superseded and the new one re-extracted"
			matchedOld[olds[best].ID] = true
		}

		diff.Changes = append(diff.Changes, change)
		diff.Counts[string(change.Kind)]++
	}

	// An old clause with no incoming match is deleted ONLY if this document
	// supersedes its predecessor. Otherwise its absence here means nothing.
	if supersedes {
		for _, old := range olds {
			if matchedOld[old.ID] {
				continue
			}
			diff.Changes = append(diff.Changes, ClauseChange{
				Kind:         ChangeDeleted,
				OldClauseID:  old.ID,
				OldClauseRef: old.ClauseRef,
				OldText:      old.Text,
				Rationale: "no clause in the superseding document matches it; " +
					"its world-time interval will be closed",
			})
			diff.Counts[string(ChangeDeleted)]++
		}
	}

	// Stable output order: document order for incoming clauses, then deletions.
	sort.SliceStable(diff.Changes, func(i, j int) bool {
		a, b := diff.Changes[i], diff.Changes[j]
		if (a.Kind == ChangeDeleted) != (b.Kind == ChangeDeleted) {
			return b.Kind == ChangeDeleted
		}
		return false
	})
	return diff
}

// shingleSize is the word-gram width for the Jaccard term. Trigrams are wide
// enough that word order matters (a reordered sentence is not "unchanged") and
// narrow enough to survive a single-word edit.
const shingleSize = 3

// shingle builds the set of word trigrams in a text.
func shingle(text string) map[string]bool {
	words := strings.Fields(strings.ToLower(normalizeWhitespace(text)))
	out := map[string]bool{}
	if len(words) < shingleSize {
		if len(words) > 0 {
			out[strings.Join(words, " ")] = true
		}
		return out
	}
	for i := 0; i+shingleSize <= len(words); i++ {
		out[strings.Join(words[i:i+shingleSize], " ")] = true
	}
	return out
}

// jaccard is |A ∩ B| / |A ∪ B|.
func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// normalizeWhitespace collapses runs of whitespace, matching the tolerance the
// citation gate uses. Two clauses differing only in line wrapping are the same
// clause.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
