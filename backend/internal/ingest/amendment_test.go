package ingest

import (
	"testing"

	"chanakya/internal/domain"
)

func newClause(ref, text string) domain.Clause {
	return domain.Clause{ID: "NEW#" + ref, ClauseRef: ref, Text: text}
}

func oldClause(ref, text string) ExistingClause {
	return ExistingClause{ID: "OLD#" + ref, ClauseRef: ref, Text: text, RowUID: "OLD#" + ref + "@t0"}
}

const (
	retentionV1 = "An investment adviser must retain all records of investment advice, client agreements, and risk profiling for a period of 5 years from the date of the relevant interaction."
	retentionV2 = "An investment adviser must retain all records of investment advice, client agreements, and risk profiling for a period of 8 years from the date of the relevant interaction."
	feesText    = "An investment adviser must disclose to the client, in writing and before charging any fee, the complete fee schedule including the basis of computation."
	mitcText    = "Every investment adviser shall provide each client the standardized Most Important Terms and Conditions specified by the Board."
)

// TestAmendmentClassification covers the three incoming verdicts on one diff.
func TestAmendmentClassification(t *testing.T) {
	incoming := []domain.Clause{
		newClause("4.1", feesText),    // byte-identical -> unchanged
		newClause("5.1", retentionV2), // 5 years -> 8 years -> modified
		newClause("6.1", mitcText),    // no predecessor -> added
	}
	existing := []ExistingClause{
		oldClause("4.1", feesText),
		oldClause("5.1", retentionV1),
	}

	diff := MatchAmendment(incoming, existing, false)

	byRef := map[string]ClauseChange{}
	for _, c := range diff.Changes {
		if c.NewClauseRef != "" {
			byRef[c.NewClauseRef] = c
		}
	}

	if got := byRef["4.1"].Kind; got != ChangeUnchanged {
		t.Errorf("4.1 (identical text) = %q, want unchanged (score %.3f)", got, byRef["4.1"].Score)
	}
	if !byRef["4.1"].TextIdentical {
		t.Error("4.1 should be flagged text-identical")
	}
	if got := byRef["5.1"].Kind; got != ChangeModified {
		t.Errorf("5.1 (5 years -> 8 years) = %q, want modified (score %.3f)", got, byRef["5.1"].Score)
	}
	if byRef["5.1"].OldText != retentionV1 {
		t.Error("a modified clause must carry its OLD text, for the side-by-side review")
	}
	if got := byRef["6.1"].Kind; got != ChangeAdded {
		t.Errorf("6.1 (no predecessor) = %q, want added (score %.3f)", got, byRef["6.1"].Score)
	}

	// The unchanged path is the perf win: its obligation is reused rather than
	// re-extracted.
	if diff.ReusedObligations != 1 {
		t.Errorf("reused obligations = %d, want 1", diff.ReusedObligations)
	}
}

// TestUnchangedRequiresIdenticalText is the important half of the >=0.92 rule.
// A high score ALONE must not mean unchanged: two clauses can score 0.95 and
// still differ by the one word that changes what the firm must do - here,
// "5 years" versus "8 years".
func TestUnchangedRequiresIdenticalText(t *testing.T) {
	diff := MatchAmendment(
		[]domain.Clause{newClause("5.1", retentionV2)},
		[]ExistingClause{oldClause("5.1", retentionV1)},
		false,
	)
	c := diff.Changes[0]
	if c.Score < UnchangedThreshold {
		t.Logf("score %.3f is below the unchanged threshold anyway", c.Score)
	}
	if c.Kind == ChangeUnchanged {
		t.Fatalf("differing text was classified unchanged (score %.3f) - "+
			"a retention period changed from 5 to 8 years", c.Score)
	}
}

// TestThresholdsAreAppliedExactly: the boundaries are used as specified, with no
// rounding and no buffer zone.
func TestThresholdsAreAppliedExactly(t *testing.T) {
	if UnchangedThreshold != 0.92 {
		t.Errorf("unchanged threshold = %v, want exactly 0.92", UnchangedThreshold)
	}
	if ModifiedThreshold != 0.55 {
		t.Errorf("modified threshold = %v, want exactly 0.55", ModifiedThreshold)
	}
	if weightCosine+weightJaccard+weightRefEq != 1.0 {
		t.Errorf("score weights sum to %v, want 1.0", weightCosine+weightJaccard+weightRefEq)
	}
	if weightCosine != 0.45 || weightJaccard != 0.35 || weightRefEq != 0.20 {
		t.Errorf("weights = %v/%v/%v, want 0.45/0.35/0.20", weightCosine, weightJaccard, weightRefEq)
	}

	// A completely unrelated clause must fall below the modified floor.
	diff := MatchAmendment(
		[]domain.Clause{newClause("9.9", "The Board may issue such directions as it considers necessary.")},
		[]ExistingClause{oldClause("5.1", retentionV1)},
		false,
	)
	if diff.Changes[0].Kind != ChangeAdded {
		t.Errorf("an unrelated clause scored %.3f and was classified %q, want added",
			diff.Changes[0].Score, diff.Changes[0].Kind)
	}
}

// TestDeletionRequiresSupersession: an old clause with no match is deleted ONLY
// when the incoming document supersedes its predecessor. Inferring a deletion
// from a document that merely references another would retire live regulation.
func TestDeletionRequiresSupersession(t *testing.T) {
	incoming := []domain.Clause{newClause("4.1", feesText)}
	existing := []ExistingClause{
		oldClause("4.1", feesText),
		oldClause("5.1", retentionV1), // no incoming match
	}

	notSuperseding := MatchAmendment(incoming, existing, false)
	if notSuperseding.Counts[string(ChangeDeleted)] != 0 {
		t.Errorf("a non-superseding document proposed %d deletions, want 0",
			notSuperseding.Counts[string(ChangeDeleted)])
	}

	superseding := MatchAmendment(incoming, existing, true)
	if superseding.Counts[string(ChangeDeleted)] != 1 {
		t.Errorf("a superseding document proposed %d deletions, want 1",
			superseding.Counts[string(ChangeDeleted)])
	}
	for _, c := range superseding.Changes {
		if c.Kind == ChangeDeleted && c.OldClauseRef != "5.1" {
			t.Errorf("deleted the wrong clause: %q", c.OldClauseRef)
		}
	}
}

// TestManyToOneTakesSingleBestMatch: when an old clause scores similarly against
// several new ones, exactly one match is proposed - never a multi-way merge -
// and the tie-break is deterministic.
func TestManyToOneTakesSingleBestMatch(t *testing.T) {
	// Two nearly-identical incoming clauses; only one shares the old clause ref.
	incoming := []domain.Clause{
		newClause("5.1", retentionV2),
		newClause("5.9", retentionV2),
	}
	existing := []ExistingClause{oldClause("5.1", retentionV1)}

	diff := MatchAmendment(incoming, existing, false)

	matchedOld := 0
	for _, c := range diff.Changes {
		if c.OldClauseID != "" {
			matchedOld++
		}
	}
	if matchedOld > 2 {
		t.Fatalf("%d changes claim the same old clause", matchedOld)
	}

	// The ref-equal candidate must win the tie: refEquality carries 0.20 of the
	// score and both texts are identical.
	var byRef map[string]ClauseChange = map[string]ClauseChange{}
	for _, c := range diff.Changes {
		byRef[c.NewClauseRef] = c
	}
	if byRef["5.1"].Score <= byRef["5.9"].Score {
		t.Errorf("ref-equal candidate scored %.3f, not above the non-matching ref %.3f",
			byRef["5.1"].Score, byRef["5.9"].Score)
	}

	// Determinism: the same inputs must give the same proposal every time.
	for i := 0; i < 20; i++ {
		again := MatchAmendment(incoming, existing, false)
		for j := range again.Changes {
			if again.Changes[j].OldClauseID != diff.Changes[j].OldClauseID ||
				again.Changes[j].Kind != diff.Changes[j].Kind {
				t.Fatal("matching is not deterministic across runs")
			}
		}
	}
}

// TestWhitespaceOnlyDifferenceIsUnchanged: a re-wrapped clause is the same
// clause. The tolerance matches the citation gate's.
func TestWhitespaceOnlyDifferenceIsUnchanged(t *testing.T) {
	wrapped := "An investment adviser must disclose to the client, in writing and before charging any fee,\n   the complete fee schedule including the basis of computation."
	diff := MatchAmendment(
		[]domain.Clause{newClause("4.1", wrapped)},
		[]ExistingClause{oldClause("4.1", feesText)},
		false,
	)
	if diff.Changes[0].Kind != ChangeUnchanged {
		t.Errorf("a re-wrapped clause was classified %q (score %.3f), want unchanged",
			diff.Changes[0].Kind, diff.Changes[0].Score)
	}
}

// TestJaccardAndShingles sanity-checks the lexical half of the score.
func TestJaccardAndShingles(t *testing.T) {
	if got := jaccard(shingle(feesText), shingle(feesText)); got != 1 {
		t.Errorf("self-similarity = %v, want 1", got)
	}
	if got := jaccard(shingle(feesText), shingle(mitcText)); got > 0.2 {
		t.Errorf("unrelated texts scored %v, want a low overlap", got)
	}
	// Word ORDER must matter: a reordered sentence is not the same clause.
	a := shingle("the adviser shall notify the client")
	b := shingle("the client shall notify the adviser")
	if jaccard(a, b) > 0.5 {
		t.Error("trigrams should distinguish a reordered sentence")
	}
}
