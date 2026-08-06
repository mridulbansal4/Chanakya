package store

import (
	"context"
	"errors"
	"testing"

	"chanakya/internal/domain"
	"chanakya/internal/ingest"
)

// sampleProposal builds a small, valid proposal: one circular, two clauses
// (parent before child), one obligation whose citation is verbatim, one semantic
// unit, one relation and one dangling reference.
func sampleProposal() ingest.Proposal {
	const circ = "SEBI/TEST/CIR/2025/1"
	parent := domain.ClauseID(circ, "3")
	child := domain.ClauseID(circ, "3.1")
	const childText = "An adviser must apply for registration within 30 days of crossing the threshold."

	return ingest.Proposal{
		SHA256:   "abc123",
		Filename: "test.pdf",
		Meta:     ingest.CircularMeta{DocKind: ingest.KindCircular, CircularNo: circ},
		Circular: domain.Circular{
			ID: circ, Title: "Test Circular", Regulator: "SEBI",
			IssuedOn: "2025-02-17T00:00:00Z",
			Temporal: domain.Temporal{ValidFrom: "2025-02-17T00:00:00Z"},
		},
		Clauses: []domain.Clause{
			{ID: parent, CircularID: circ, ClauseRef: "3", Heading: "Registration", Text: "Registration", Ordinal: 1},
			{ID: child, CircularID: circ, ClauseRef: "3.1", ParentID: parent, Text: childText, Ordinal: 2},
		},
		Obligations: []ingest.ProposedObligation{{
			Obligation: domain.Obligation{
				ID: circ + "#3.1/obl/aaa", ClauseID: child, Bearer: "investment adviser",
				DeonticType: domain.DeonticMust, SourceClauseRef: "3.1",
				SourceSentence: childText, Confidence: 0.9, Status: domain.StatusPending,
			},
			ClauseRef: "3.1", ClauseText: childText,
		}},
		Units: []ingest.SemanticUnit{{
			ID: child + "/u1", ClauseID: child, Ordinal: 1, Role: ingest.RoleNorm,
			Text: childText, StartOffset: 0, EndOffset: len(childText),
		}},
		Relations: []ingest.CircularRelation{{
			ID: circ + "|references|SEBI/IA/MC/2024", FromCircular: circ,
			ToRef: "SEBI/IA/MC/2024", Kind: "references",
		}},
		Dangling: []ingest.DanglingRef{{
			ID: "dref:" + child + ":regulation15", CircularID: circ, ClauseID: child,
			RawText: "regulation 15", Kind: ingest.RefRegulation,
			Reason: "regulations are not ingested; reference is external",
		}},
	}
}

func stageRun(t *testing.T, st *Store) IngestRun {
	t.Helper()
	ctx := context.Background()
	p := sampleProposal()
	run, created, err := st.CreateIngestRun(ctx, p.SHA256, p.Filename, "job:1")
	if err != nil {
		t.Fatalf("CreateIngestRun: %v", err)
	}
	if !created {
		t.Fatal("expected a newly created run")
	}
	if err := st.SaveProposal(ctx, run.ID, p); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}
	got, err := st.GetIngestRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetIngestRun: %v", err)
	}
	return got
}

func graphCounts(t *testing.T, st *Store, circularID string) (circulars, clauses, obligations int) {
	t.Helper()
	q := func(sql string) int {
		var n int
		if err := st.DB().QueryRow(sql, circularID).Scan(&n); err != nil {
			t.Fatalf("count query %q: %v", sql, err)
		}
		return n
	}
	return q(`SELECT COUNT(*) FROM circular WHERE id = ? AND tx_to IS NULL`),
		q(`SELECT COUNT(*) FROM clause WHERE circular_id = ? AND tx_to IS NULL`),
		q(`SELECT COUNT(*) FROM obligation WHERE clause_id LIKE ? || '%' AND tx_to IS NULL`)
}

// TestNothingEntersGraphBeforeApproval is the Phase 2 safety invariant: a
// completed pipeline run has written a PROPOSAL and nothing else. The regulatory
// graph is byte-for-byte unchanged until a human approves.
func TestNothingEntersGraphBeforeApproval(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	run := stageRun(t, st)
	const circ = "SEBI/TEST/CIR/2025/1"

	if run.State != RunPreview {
		t.Fatalf("state after pipeline = %q, want %q", run.State, RunPreview)
	}
	if len(run.Proposal.Clauses) != 2 {
		t.Fatalf("proposal round-trip lost clauses: got %d, want 2", len(run.Proposal.Clauses))
	}

	c, cl, ob := graphCounts(t, st, circ)
	if c != 0 || cl != 0 || ob != 0 {
		t.Fatalf("BEFORE approve the graph must be empty: circulars=%d clauses=%d obligations=%d", c, cl, ob)
	}

	if _, err := st.ApproveIngestRun(ctx, run.ID, "Priya Menon"); err != nil {
		t.Fatalf("ApproveIngestRun: %v", err)
	}

	c, cl, ob = graphCounts(t, st, circ)
	if c != 1 || cl != 2 || ob != 1 {
		t.Fatalf("AFTER approve: circulars=%d clauses=%d obligations=%d, want 1/2/1", c, cl, ob)
	}

	var units, rels, dangling int
	_ = st.DB().QueryRow(`SELECT COUNT(*) FROM semantic_unit`).Scan(&units)
	_ = st.DB().QueryRow(`SELECT COUNT(*) FROM circular_relation`).Scan(&rels)
	_ = st.DB().QueryRow(`SELECT COUNT(*) FROM dangling_reference`).Scan(&dangling)
	if units != 1 || rels != 1 || dangling != 1 {
		t.Errorf("units=%d relations=%d dangling=%d, want 1/1/1", units, rels, dangling)
	}
}

// TestApproveIsAtomic forces a failure mid-commit and proves the graph is left
// unchanged - no half-ingested circular with clauses but no obligations.
func TestApproveIsAtomic(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	p := sampleProposal()
	// An obligation with no provenance. It passes JSON round-tripping but must
	// be refused at the store boundary, aborting the whole commit.
	p.Obligations = append(p.Obligations, ingest.ProposedObligation{
		Obligation: domain.Obligation{
			ID: "bad", ClauseID: domain.ClauseID(p.Circular.ID, "3.1"),
			Bearer: "investment adviser", DeonticType: domain.DeonticMust,
			SourceClauseRef: "", SourceSentence: "", Confidence: 0.9,
		},
		ClauseRef: "3.1",
	})

	run, _, err := st.CreateIngestRun(ctx, p.SHA256, p.Filename, "job:1")
	if err != nil {
		t.Fatalf("CreateIngestRun: %v", err)
	}
	if err := st.SaveProposal(ctx, run.ID, p); err != nil {
		t.Fatalf("SaveProposal: %v", err)
	}

	if _, err := st.ApproveIngestRun(ctx, run.ID, "Priya Menon"); err == nil {
		t.Fatal("expected the commit to fail on the obligation with no provenance")
	}

	c, cl, ob := graphCounts(t, st, p.Circular.ID)
	if c != 0 || cl != 0 || ob != 0 {
		t.Fatalf("a failed commit left partial state: circulars=%d clauses=%d obligations=%d", c, cl, ob)
	}

	after, err := st.GetIngestRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetIngestRun: %v", err)
	}
	if after.State != RunFailed {
		t.Errorf("state after failed commit = %q, want %q", after.State, RunFailed)
	}
	if after.Error == "" {
		t.Error("a failed commit must record why")
	}
}

// TestDoubleApproveAndApproveAfterDiscard: a second approve must be a clear
// error, never a silent no-op and never a second commit.
func TestDoubleApproveAndApproveAfterDiscard(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	run := stageRun(t, st)

	if _, err := st.ApproveIngestRun(ctx, run.ID, "Priya Menon"); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	_, err := st.ApproveIngestRun(ctx, run.ID, "Priya Menon")
	if !errors.Is(err, ErrRunSettled) {
		t.Errorf("second approve: err = %v, want ErrRunSettled", err)
	}
	if err := st.DiscardIngestRun(ctx, run.ID); !errors.Is(err, ErrRunSettled) {
		t.Errorf("discard after approve: err = %v, want ErrRunSettled", err)
	}

	// And the reverse order.
	st2 := newTestStore(t)
	run2 := stageRun(t, st2)
	if err := st2.DiscardIngestRun(ctx, run2.ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if _, err := st2.ApproveIngestRun(ctx, run2.ID, "Priya Menon"); !errors.Is(err, ErrRunSettled) {
		t.Errorf("approve after discard: err = %v, want ErrRunSettled", err)
	}
	c, cl, ob := graphCounts(t, st2, run2.Proposal.Circular.ID)
	if c != 0 || cl != 0 || ob != 0 {
		t.Fatalf("a discarded run wrote to the graph: circulars=%d clauses=%d obligations=%d", c, cl, ob)
	}
}

// TestDuplicateUploadReusesRun: the same content address, still queued or
// running, must return the EXISTING run rather than enqueue a second pipeline.
func TestDuplicateUploadReusesRun(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	first, created, err := st.CreateIngestRun(ctx, "deadbeefdeadbeef01", "a.pdf", "job:1")
	if err != nil || !created {
		t.Fatalf("first create: run=%v created=%v err=%v", first.ID, created, err)
	}
	second, created, err := st.CreateIngestRun(ctx, "deadbeefdeadbeef01", "a-copy.pdf", "job:2")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if created {
		t.Error("a duplicate upload of queued content must not create a second run")
	}
	if second.ID != first.ID {
		t.Errorf("duplicate returned run %q, want the existing %q", second.ID, first.ID)
	}
}

// TestJobQueueClaimIsExclusive: claiming moves a job out of 'queued' so a second
// worker cannot take the same one.
func TestJobQueueClaimIsExclusive(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	if err := st.EnqueueJob(ctx, "job:a", "ingest", `{"run_id":"r1"}`); err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}
	first, err := st.ClaimJob(ctx)
	if err != nil {
		t.Fatalf("ClaimJob: %v", err)
	}
	if first.ID != "job:a" || first.State != JobRunning {
		t.Fatalf("claimed %q in state %q, want job:a running", first.ID, first.State)
	}
	if _, err := st.ClaimJob(ctx); !errors.Is(err, ErrNotFound) {
		t.Errorf("second claim: err = %v, want ErrNotFound (the job is already taken)", err)
	}
	if first.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", first.Attempts)
	}

	if err := st.FinishJob(ctx, "job:a", JobSucceeded, ""); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}
	done, err := st.GetJob(ctx, "job:a")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if done.State != JobSucceeded || done.FinishedAt == "" {
		t.Errorf("finished job = %q at %q, want succeeded with a timestamp", done.State, done.FinishedAt)
	}
}

// TestJobRowsAreRetained: completed jobs are part of the audit trail and must
// not be cleaned up.
func TestJobRowsAreRetained(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	if err := st.EnqueueJob(ctx, "job:keep", "ingest", "{}"); err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}
	if _, err := st.ClaimJob(ctx); err != nil {
		t.Fatalf("ClaimJob: %v", err)
	}
	if err := st.FinishJob(ctx, "job:keep", JobFailed, "boom"); err != nil {
		t.Fatalf("FinishJob: %v", err)
	}
	n, err := st.CountJobs(ctx, JobFailed)
	if err != nil {
		t.Fatalf("CountJobs: %v", err)
	}
	if n != 1 {
		t.Errorf("failed jobs retained = %d, want 1", n)
	}
}
