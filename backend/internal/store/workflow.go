package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/workflow"
)

// ErrWorkflowSettled is returned when a workflow has already been approved.
var ErrWorkflowSettled = errors.New("workflow already approved")

// WorkflowView is a generated workflow with its tasks, as the API returns it.
type WorkflowView struct {
	ID           string        `json:"id"`
	Template     string        `json:"template"`
	Title        string        `json:"title"`
	ObligationID string        `json:"obligation_id"`
	ClauseRef    string        `json:"clause_ref"`
	Verb         string        `json:"verb"`
	State        string        `json:"state"`
	SLA          string        `json:"sla"`
	Rationale    string        `json:"rationale"`
	GeneratedAt  string        `json:"generated_at"`
	TaskCount    int           `json:"task_count"`
	Unresolved   int           `json:"unresolved_owners"`
	Tasks        []TaskView    `json:"tasks,omitempty"`
	Approval     *ApprovalView `json:"approval,omitempty"`
}

// TaskView is one task of a generated workflow.
type TaskView struct {
	ID              string   `json:"id"`
	WorkflowID      string   `json:"workflow_id"`
	Title           string   `json:"title"`
	Detail          string   `json:"detail"`
	OwnerRole       string   `json:"owner_role"`
	OwnerEmployeeID string   `json:"owner_employee_id"`
	OwnerName       string   `json:"owner_name"`
	OwnerUnresolved bool     `json:"owner_unresolved"`
	State           string   `json:"state"`
	Deadline        string   `json:"deadline"`
	Ordinal         int      `json:"ordinal"`
	DependsOn       []string `json:"depends_on"`
}

// ApprovalView is the human decision on a workflow.
type ApprovalView struct {
	Approver  string `json:"approver"`
	Decision  string `json:"decision"`
	Note      string `json:"note"`
	DecidedAt string `json:"decided_at"`
}

// SaveWorkflows persists generated workflows and their tasks.
//
// EVERY task is written with state='draft', regardless of what the caller put on
// the spec. The database is the last place this invariant can be enforced, and
// it is enforced here rather than trusted from the synthesis layer.
func (s *Store) SaveWorkflows(ctx context.Context, specs []workflow.WorkflowSpec, validFrom, txFrom string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, w := range specs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow (id, template, obligation_id, state, sla, title,
			                      clause_ref, verb, rationale, generated_at,
			                      valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				template=excluded.template, sla=excluded.sla, title=excluded.title,
				clause_ref=excluded.clause_ref, verb=excluded.verb,
				rationale=excluded.rationale, generated_at=excluded.generated_at,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			w.ID, string(w.Template), w.ObligationID, w.SLA, w.Title,
			w.ClauseRef, string(w.Verb), w.Rationale, txFrom, validFrom, txFrom,
		); err != nil {
			return fmt.Errorf("save workflow %q: %w", w.ID, err)
		}

		for _, t := range w.Tasks {
			deps, err := json.Marshal(t.DependsOn)
			if err != nil {
				return fmt.Errorf("encode dependencies for task %q: %w", t.ID, err)
			}
			unresolved := 0
			if t.OwnerUnresolved {
				unresolved = 1
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO task (id, workflow_id, owner_employee_id, owner_role, title,
				                  state, deadline, ordinal, depends_on, detail,
				                  source_unit_id, owner_unresolved,
				                  valid_from, valid_to, tx_from, tx_to)
				VALUES (?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, '', ?, ?, NULL, ?, NULL)
				ON CONFLICT(id) DO UPDATE SET
					owner_employee_id=excluded.owner_employee_id, owner_role=excluded.owner_role,
					title=excluded.title, deadline=excluded.deadline, ordinal=excluded.ordinal,
					depends_on=excluded.depends_on, detail=excluded.detail,
					owner_unresolved=excluded.owner_unresolved,
					valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
				t.ID, w.ID, nullStr(t.OwnerEmployeeID), t.OwnerRole, t.Title,
				nullStr(t.Deadline), t.Ordinal, string(deps), t.Detail,
				unresolved, validFrom, txFrom,
			); err != nil {
				return fmt.Errorf("save task %q: %w", t.ID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow save: %w", err)
	}
	return nil
}

// ListWorkflows returns generated workflows as of a date.
func (s *Store) ListWorkflows(ctx context.Context, asOf time.Time) ([]WorkflowView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT w.id, w.template, COALESCE(w.title,''), w.obligation_id, w.clause_ref,
		       w.verb, w.state, COALESCE(w.sla,''), w.rationale, w.generated_at,
		       (SELECT COUNT(*) FROM task t WHERE t.workflow_id = w.id AND t.tx_to IS NULL),
		       (SELECT COUNT(*) FROM task t WHERE t.workflow_id = w.id AND t.tx_to IS NULL
		                                     AND t.owner_unresolved = 1)
		FROM workflow w
		WHERE w.valid_from <= ? AND (w.valid_to IS NULL OR w.valid_to > ?) AND w.tx_to IS NULL
		ORDER BY w.clause_ref, w.template`, at, at)
	if err != nil {
		return nil, fmt.Errorf("list workflows as-of %s: %w", at, err)
	}
	defer rows.Close()

	var out []WorkflowView
	for rows.Next() {
		var w WorkflowView
		if err := rows.Scan(&w.ID, &w.Template, &w.Title, &w.ObligationID, &w.ClauseRef,
			&w.Verb, &w.State, &w.SLA, &w.Rationale, &w.GeneratedAt,
			&w.TaskCount, &w.Unresolved); err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflows: %w", err)
	}
	return out, nil
}

// GetWorkflow returns one workflow with its full task DAG.
func (s *Store) GetWorkflow(ctx context.Context, id string, asOf time.Time) (WorkflowView, error) {
	at := domain.RFC3339UTC(asOf)
	var w WorkflowView
	// The as-of filter applies to the workflow row too, not only to its tasks:
	// without it, a workflow generated after the requested date would come back
	// with an empty task list rather than honestly not existing yet.
	err := s.db.QueryRowContext(ctx, `
		SELECT id, template, COALESCE(title,''), obligation_id, clause_ref, verb,
		       state, COALESCE(sla,''), rationale, generated_at
		FROM workflow WHERE id = ? AND`+asOfClause, id, at, at).
		Scan(&w.ID, &w.Template, &w.Title, &w.ObligationID, &w.ClauseRef, &w.Verb,
			&w.State, &w.SLA, &w.Rationale, &w.GeneratedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkflowView{}, fmt.Errorf("workflow %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return WorkflowView{}, fmt.Errorf("get workflow %q: %w", id, err)
	}

	tasks, err := s.ListWorkflowTasks(ctx, id, asOf)
	if err != nil {
		return WorkflowView{}, err
	}
	w.Tasks = tasks
	w.TaskCount = len(tasks)
	for _, t := range tasks {
		if t.OwnerUnresolved {
			w.Unresolved++
		}
	}

	if ap, ok, err := s.getWorkflowApproval(ctx, id); err != nil {
		return WorkflowView{}, err
	} else if ok {
		w.Approval = &ap
	}
	return w, nil
}

// ListWorkflowTasks returns a workflow's task DAG in dependency order.
func (s *Store) ListWorkflowTasks(ctx context.Context, workflowID string, asOf time.Time) ([]TaskView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.workflow_id, t.title, t.detail, COALESCE(t.owner_role,''),
		       COALESCE(t.owner_employee_id,''), COALESCE(e.name,''), t.owner_unresolved,
		       t.state, COALESCE(t.deadline,''), t.ordinal, t.depends_on
		FROM task t
		LEFT JOIN employee e ON e.id = t.owner_employee_id AND e.tx_to IS NULL
		WHERE t.workflow_id = ?
		  AND t.valid_from <= ? AND (t.valid_to IS NULL OR t.valid_to > ?) AND t.tx_to IS NULL
		ORDER BY t.ordinal`, workflowID, at, at)
	if err != nil {
		return nil, fmt.Errorf("list tasks for workflow %q: %w", workflowID, err)
	}
	defer rows.Close()

	var out []TaskView
	for rows.Next() {
		var (
			t          TaskView
			unresolved int
			deps       string
		)
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Title, &t.Detail, &t.OwnerRole,
			&t.OwnerEmployeeID, &t.OwnerName, &unresolved, &t.State, &t.Deadline,
			&t.Ordinal, &deps); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		t.OwnerUnresolved = unresolved == 1
		_ = json.Unmarshal([]byte(deps), &t.DependsOn)
		out = append(out, t)
	}
	return out, rows.Err()
}

// getWorkflowApproval loads the human decision on a workflow, if any.
func (s *Store) getWorkflowApproval(ctx context.Context, workflowID string) (ApprovalView, bool, error) {
	var a ApprovalView
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(approver,''), decision, COALESCE(note,''), COALESCE(decided_at,'')
		FROM approval
		WHERE subject_type = 'workflow' AND subject_id = ? AND tx_to IS NULL`, workflowID).
		Scan(&a.Approver, &a.Decision, &a.Note, &a.DecidedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ApprovalView{}, false, nil
	}
	if err != nil {
		return ApprovalView{}, false, fmt.Errorf("get approval for workflow %q: %w", workflowID, err)
	}
	return a, true, nil
}

// ApproveWorkflow records the human gate on a generated workflow.
//
// WHAT THIS DOES NOT DO: it does not dispatch anything. Approving marks the
// workflow approved and records who decided; the tasks stay 'draft' because
// CHANAKYA does not send email, file tickets or create calendar invites. Actual
// dispatch is out of scope by design and is logged, never performed.
func (s *Store) ApproveWorkflow(ctx context.Context, workflowID, approver, note string, now time.Time) (WorkflowView, error) {
	current, err := s.GetWorkflow(ctx, workflowID, now)
	if err != nil {
		return WorkflowView{}, err
	}
	if current.State == "approved" {
		return WorkflowView{}, fmt.Errorf("workflow %q: %w", workflowID, ErrWorkflowSettled)
	}

	at := domain.RFC3339UTC(now)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WorkflowView{}, fmt.Errorf("begin workflow approve: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE workflow SET state = 'approved' WHERE id = ? AND tx_to IS NULL`, workflowID); err != nil {
		return WorkflowView{}, fmt.Errorf("mark workflow %q approved: %w", workflowID, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO approval (id, subject_type, subject_id, approver_id, approver, decision,
		                      note, decided_at, valid_from, valid_to, tx_from, tx_to)
		VALUES (?, 'workflow', ?, NULL, ?, 'approved', ?, ?, ?, NULL, ?, NULL)
		ON CONFLICT(id) DO UPDATE SET
			approver=excluded.approver, decision=excluded.decision, note=excluded.note,
			decided_at=excluded.decided_at, valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
		"apr:wf:"+workflowID, workflowID, approver, nullStr(note), at, at, at,
	); err != nil {
		return WorkflowView{}, fmt.Errorf("record approval for workflow %q: %w", workflowID, err)
	}

	if err := tx.Commit(); err != nil {
		return WorkflowView{}, fmt.Errorf("commit workflow approve: %w", err)
	}
	return s.GetWorkflow(ctx, workflowID, now)
}

// Departments implements workflow.DepartmentLookup, giving the owner resolver
// the department heads it needs from the enterprise graph.
func (s *Store) Departments(ctx context.Context) ([]workflow.DepartmentHead, error) {
	depts, err := s.listDepartments(ctx, domain.RFC3339UTC(time.Now()))
	if err != nil {
		return nil, err
	}
	out := make([]workflow.DepartmentHead, 0, len(depts))
	for _, d := range depts {
		out = append(out, workflow.DepartmentHead{
			ID: d.ID, Name: d.Name, HeadID: d.HeadID, HeadName: d.HeadName,
		})
	}
	return out, nil
}

// CountTasksByState is used by tests and the UI to prove nothing left 'draft'.
func (s *Store) CountTasksByState(ctx context.Context, state string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM task WHERE state = ? AND tx_to IS NULL`, state).Scan(&n); err != nil {
		return 0, fmt.Errorf("count tasks in state %q: %w", state, err)
	}
	return n, nil
}
