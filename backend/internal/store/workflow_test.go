package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"chanakya/internal/workflow"
)

// sampleWorkflow builds a two-task workflow with one resolved and one
// unresolved owner.
func sampleWorkflow() workflow.WorkflowSpec {
	return workflow.WorkflowSpec{
		ID:           "wf:test1",
		Template:     workflow.TemplateClientNotification,
		Title:        "Client notification - clause 5.2",
		ObligationID: "obl-1",
		ClauseRef:    "5.2",
		Verb:         workflow.VerbNotify,
		State:        "draft",
		SLA:          "P30D",
		Rationale:    "the obligation's act is notify",
		Tasks: []workflow.TaskPlan{
			{
				ID: "wf:test1/draft", Key: "draft", Title: "Draft the client communication",
				Detail: "Prepare the notice text.", OwnerRole: "Compliance",
				OwnerEmployeeID: "emp_001", OwnerName: "Priya Menon",
				Ordinal: 1, State: "draft", Deadline: "2025-08-01T00:00:00Z",
			},
			{
				ID: "wf:test1/send", Key: "send", Title: "Send the notice (DRAFT - not dispatched)",
				Detail: "CHANAKYA never sends.", OwnerRole: "Nonexistent Department",
				OwnerUnresolved: true, DependsOn: []string{"wf:test1/draft"},
				Ordinal: 2, State: "draft", Deadline: "2025-08-10T00:00:00Z",
			},
		},
		UnresolvedOwners: []string{"Nonexistent Department"},
	}
}

func saveSampleWorkflow(t *testing.T, st *Store, at string) {
	t.Helper()
	if err := st.SaveWorkflows(context.Background(),
		[]workflow.WorkflowSpec{sampleWorkflow()}, at, at); err != nil {
		t.Fatalf("SaveWorkflows: %v", err)
	}
}

// TestWorkflowRoundTrip covers save → list → get, including the task DAG and the
// unresolved-owner flag surviving the database.
func TestWorkflowRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	now := time.Now()
	at := now.Add(-time.Hour).UTC().Format(time.RFC3339)
	saveSampleWorkflow(t, st, at)

	list, err := st.ListWorkflows(ctx, now)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("listed %d workflows, want 1", len(list))
	}
	if list[0].TaskCount != 2 {
		t.Errorf("task_count = %d, want 2", list[0].TaskCount)
	}
	if list[0].Unresolved != 1 {
		t.Errorf("unresolved_owners = %d, want 1", list[0].Unresolved)
	}

	got, err := st.GetWorkflow(ctx, "wf:test1", now)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if len(got.Tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(got.Tasks))
	}
	if got.Tasks[0].OwnerName != "Priya Menon" {
		t.Errorf("task 1 owner = %q, want a real person", got.Tasks[0].OwnerName)
	}
	if !got.Tasks[1].OwnerUnresolved {
		t.Error("task 2 should be flagged unresolved")
	}
	if got.Tasks[1].OwnerName != "" || got.Tasks[1].OwnerEmployeeID != "" {
		t.Errorf("an unresolved task was still assigned to %q/%q",
			got.Tasks[1].OwnerEmployeeID, got.Tasks[1].OwnerName)
	}
	if len(got.Tasks[1].DependsOn) != 1 || got.Tasks[1].DependsOn[0] != "wf:test1/draft" {
		t.Errorf("task 2 dependencies = %v, want the draft task", got.Tasks[1].DependsOn)
	}
}

// TestWorkflowGetHonoursAsOf: the as-of filter applies to the workflow row, not
// only to its tasks. Without it, a workflow generated after the requested date
// would come back with an empty task list instead of honestly not existing.
func TestWorkflowGetHonoursAsOf(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	generatedAt := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	saveSampleWorkflow(t, st, generatedAt.Format(time.RFC3339))

	// After generation: visible.
	if _, err := st.GetWorkflow(ctx, "wf:test1", generatedAt.Add(24*time.Hour)); err != nil {
		t.Fatalf("workflow should exist after its generation date: %v", err)
	}

	// Before generation: it did not exist yet.
	_, err := st.GetWorkflow(ctx, "wf:test1", generatedAt.Add(-24*time.Hour))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("as-of before generation: err = %v, want ErrNotFound", err)
	}
	list, err := st.ListWorkflows(ctx, generatedAt.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("listed %d workflows as-of before generation, want 0", len(list))
	}
}

// TestApproveWorkflowDispatchesNothing is the phase's core safety invariant:
// approval records a human decision and moves NO task out of draft.
func TestApproveWorkflowDispatchesNothing(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	now := time.Now()
	saveSampleWorkflow(t, st, now.Add(-time.Hour).UTC().Format(time.RFC3339))

	before, err := st.CountTasksByState(ctx, "draft")
	if err != nil {
		t.Fatalf("CountTasksByState: %v", err)
	}
	if before != 2 {
		t.Fatalf("draft tasks before approval = %d, want 2", before)
	}

	got, err := st.ApproveWorkflow(ctx, "wf:test1", "Priya Menon",
		"Reviewed each task and its owner against the obligation.", now)
	if err != nil {
		t.Fatalf("ApproveWorkflow: %v", err)
	}
	if got.State != "approved" {
		t.Errorf("workflow state = %q, want approved", got.State)
	}
	if got.Approval == nil || got.Approval.Approver != "Priya Menon" {
		t.Errorf("approval record = %+v, want one naming the approver", got.Approval)
	}

	// The invariant: nothing was dispatched.
	after, err := st.CountTasksByState(ctx, "draft")
	if err != nil {
		t.Fatalf("CountTasksByState: %v", err)
	}
	if after != 2 {
		t.Errorf("draft tasks after approval = %d, want 2 - approving must dispatch nothing", after)
	}
	for _, task := range got.Tasks {
		if task.State != "draft" {
			t.Errorf("task %q state = %q after approval, want draft", task.Title, task.State)
		}
	}
}

// TestDoubleApproveWorkflowIsRejected: a second approval must be a clear error.
func TestDoubleApproveWorkflowIsRejected(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	now := time.Now()
	saveSampleWorkflow(t, st, now.Add(-time.Hour).UTC().Format(time.RFC3339))

	note := "Reviewed each task and its owner against the obligation."
	if _, err := st.ApproveWorkflow(ctx, "wf:test1", "Priya Menon", note, now); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	if _, err := st.ApproveWorkflow(ctx, "wf:test1", "Priya Menon", note, now); !errors.Is(err, ErrWorkflowSettled) {
		t.Errorf("second approve: err = %v, want ErrWorkflowSettled", err)
	}
}

// TestSaveWorkflowsForcesDraft: the store is the last place the draft-only
// invariant can be enforced, so it does not trust the synthesis layer.
func TestSaveWorkflowsForcesDraft(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	spec := sampleWorkflow()
	// A caller trying to write a dispatched task.
	spec.State = "approved"
	spec.Tasks[0].State = "dispatched"
	spec.Tasks[1].State = "done"

	now := time.Now()
	at := now.Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := st.SaveWorkflows(ctx, []workflow.WorkflowSpec{spec}, at, at); err != nil {
		t.Fatalf("SaveWorkflows: %v", err)
	}

	got, err := st.GetWorkflow(ctx, "wf:test1", now)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.State != "draft" {
		t.Errorf("workflow state = %q, want draft - the store must not trust the caller", got.State)
	}
	for _, task := range got.Tasks {
		if task.State != "draft" {
			t.Errorf("task %q state = %q, want draft", task.Title, task.State)
		}
	}
}

// TestSaveWorkflowsIsIdempotent: deterministic ids mean re-synthesising an
// unchanged obligation updates rather than duplicates.
func TestSaveWorkflowsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	now := time.Now()
	at := now.Add(-time.Hour).UTC().Format(time.RFC3339)
	saveSampleWorkflow(t, st, at)
	saveSampleWorkflow(t, st, at)

	list, err := st.ListWorkflows(ctx, now)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("re-saving produced %d workflows, want 1", len(list))
	}
	if list[0].TaskCount != 2 {
		t.Errorf("re-saving produced %d tasks, want 2", list[0].TaskCount)
	}
}
