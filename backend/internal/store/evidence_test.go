package store

import (
	"context"
	"testing"
	"time"

	"chanakya/internal/domain"
)

// seedEvidenceScenario builds a circular with two clauses each bearing one
// obligation; only clause A's obligation is covered by a control→evidence path.
// Returns the two obligation ids (covered, uncovered).
func seedEvidenceScenario(t *testing.T, st *Store) (string, string) {
	t.Helper()
	ctx := context.Background()
	const circ = "EV/1"
	vf := domain.RFC3339UTC(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC))
	tmp := domain.Temporal{ValidFrom: vf, TxFrom: vf}

	if err := st.UpsertCircular(ctx, domain.Circular{ID: circ, Title: "T", Regulator: "SEBI", IssuedOn: vf, Temporal: tmp}); err != nil {
		t.Fatal(err)
	}
	mkClause := func(ref string, ord int) string {
		id := domain.ClauseID(circ, ref)
		if err := st.UpsertClause(ctx, domain.Clause{
			ID: id, CircularID: circ, ClauseRef: ref, Heading: "H " + ref,
			Text: "An adviser must do " + ref + ".", Ordinal: ord, Temporal: tmp,
		}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	clA := mkClause("A", 1)
	clB := mkClause("B", 2)

	mkObl := func(clauseID, ref string) string {
		o := domain.Obligation{
			ID: "obl/" + ref, ClauseID: clauseID, Bearer: "adviser", DeonticType: domain.DeonticMust,
			SourceClauseRef: ref, SourceSentence: "An adviser must do " + ref + ".",
			Confidence: 0.9, Status: domain.StatusPending, Temporal: tmp,
		}
		if err := st.UpsertObligation(ctx, o); err != nil {
			t.Fatal(err)
		}
		return o.ID
	}
	oblA := mkObl(clA, "A")
	oblB := mkObl(clB, "B")

	// Cover A only: obligation A -> control -> evidence.
	if err := st.UpsertControl(ctx, domain.Control{ID: "ctl_a", Name: "Control A", Temporal: tmp}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertEvidence(ctx, domain.Evidence{ID: "ev_a", Name: "Evidence A", SourceSystem: "sys", Temporal: tmp}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertObligationControl(ctx, oblA, "ctl_a", vf, vf); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertControlEvidence(ctx, "ctl_a", "ev_a", vf, vf); err != nil {
		t.Fatal(err)
	}
	return oblA, oblB
}

func TestEvidenceMapDetectsGap(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	oblA, oblB := seedEvidenceScenario(t, st)
	asOf := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	em, err := st.EvidenceMap(ctx, asOf)
	if err != nil {
		t.Fatalf("EvidenceMap: %v", err)
	}
	if em.Satisfied != 1 || em.Gaps != 1 {
		t.Fatalf("satisfied=%d gaps=%d, want 1 and 1", em.Satisfied, em.Gaps)
	}
	byID := map[string]ObligationEvidence{}
	for _, oe := range em.Obligations {
		byID[oe.ID] = oe
	}
	if !byID[oblA].Satisfied || len(byID[oblA].Evidence) != 1 {
		t.Errorf("obligation A should be satisfied with 1 evidence, got %+v", byID[oblA])
	}
	if byID[oblB].Satisfied {
		t.Errorf("obligation B should be a gap")
	}
	if byID[oblB].GapReason == "" {
		t.Errorf("gap B should carry a reason")
	}
}

func TestGenerateAndListDraftTickets(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	_, oblB := seedEvidenceScenario(t, st)
	asOf := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	n, err := st.GenerateDraftTickets(ctx, asOf)
	if err != nil {
		t.Fatalf("GenerateDraftTickets: %v", err)
	}
	if n != 1 {
		t.Fatalf("drafted %d tickets, want 1", n)
	}
	// Idempotent: regenerating keeps a single ticket.
	if _, err := st.GenerateDraftTickets(ctx, asOf); err != nil {
		t.Fatalf("re-generate: %v", err)
	}

	tickets, err := st.ListTickets(ctx, asOf)
	if err != nil {
		t.Fatalf("ListTickets: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("listed %d tickets, want 1", len(tickets))
	}
	tk := tickets[0]
	if tk.ObligationID != oblB {
		t.Errorf("ticket obligation = %q, want %q", tk.ObligationID, oblB)
	}
	if tk.State != "draft" {
		t.Errorf("ticket state = %q, want draft (never filed)", tk.State)
	}
	if tk.Citation == "" {
		t.Errorf("ticket must carry a citation")
	}
}
