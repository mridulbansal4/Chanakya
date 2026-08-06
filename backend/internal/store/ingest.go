package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/ingest"
	"chanakya/internal/vec"
)

// Ingest-run states.
const (
	RunQueued    = "queued"
	RunRunning   = "running"
	RunPreview   = "preview"
	RunApproved  = "approved"
	RunDiscarded = "discarded"
	RunFailed    = "failed"
)

// ErrRunSettled is returned when an operation is attempted on a run that has
// already been approved or discarded. It exists so the HTTP layer can answer
// 409 rather than silently no-op or, worse, commit a run twice.
var ErrRunSettled = errors.New("ingest run already settled")

// ErrRunNotReady is returned when a run has no proposal to act on yet. It is a
// DIFFERENT condition from ErrRunSettled - "come back in a moment" rather than
// "this decision has already been made" - and conflating them would tell a user
// their in-flight upload had already been approved.
var ErrRunNotReady = errors.New("ingest run has no proposal yet")

// IngestRun is the audit record of one ingestion attempt.
type IngestRun struct {
	ID         string          `json:"id"`
	JobID      string          `json:"job_id"`
	SHA256     string          `json:"sha256"`
	Filename   string          `json:"filename"`
	State      string          `json:"state"`
	Stage      string          `json:"stage"`
	DocKind    string          `json:"doc_kind"`
	CircularID string          `json:"circular_id"`
	Proposal   ingest.Proposal `json:"proposal"`
	Error      string          `json:"error"`
	CreatedAt  string          `json:"created_at"`
	ApprovedAt string          `json:"approved_at"`
	ApprovedBy string          `json:"approved_by"`
}

// IngestRunID is the deterministic run id for a document's content address.
// Deterministic so a re-upload of identical bytes maps to the same run rather
// than accumulating parallel proposals for one document.
func IngestRunID(sha string) string {
	if len(sha) > 16 {
		sha = sha[:16]
	}
	return "ing:" + sha
}

// CreateIngestRun records a queued run. If a run for this content address
// already exists and is still queued or running, the existing run is returned
// unchanged and created is false - a duplicate upload must not enqueue a second
// pipeline over the same bytes.
func (s *Store) CreateIngestRun(ctx context.Context, sha, filename, jobID string) (IngestRun, bool, error) {
	existing, err := s.GetIngestRun(ctx, IngestRunID(sha))
	switch {
	case err == nil && (existing.State == RunQueued || existing.State == RunRunning):
		return existing, false, nil
	case err != nil && !errors.Is(err, ErrNotFound):
		return IngestRun{}, false, fmt.Errorf("check existing run for %q: %w", sha, err)
	}

	now := domain.RFC3339UTC(time.Now())
	id := IngestRunID(sha)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO ingest_run (id, job_id, sha256, filename, state, stage, proposal_json,
		                        created_at, valid_from, tx_from)
		VALUES (?, ?, ?, ?, 'queued', '', '{}', ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			job_id=excluded.job_id, filename=excluded.filename, state='queued',
			stage='', proposal_json='{}', error=NULL,
			approved_at=NULL, approved_by=NULL, created_at=excluded.created_at`,
		id, nullStr(jobID), sha, filename, now, now, now,
	); err != nil {
		return IngestRun{}, false, fmt.Errorf("create ingest run %q: %w", id, err)
	}
	run, err := s.GetIngestRun(ctx, id)
	if err != nil {
		return IngestRun{}, false, fmt.Errorf("reload ingest run %q: %w", id, err)
	}
	return run, true, nil
}

// GetIngestRun loads one run.
func (s *Store) GetIngestRun(ctx context.Context, id string) (IngestRun, error) {
	var (
		r                                      IngestRun
		jobID, errText, approvedAt, approvedBy sql.NullString
		proposalJSON                           string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, job_id, sha256, filename, state, stage, doc_kind, circular_id,
		       proposal_json, error, created_at, approved_at, approved_by
		FROM ingest_run WHERE id = ?`, id).
		Scan(&r.ID, &jobID, &r.SHA256, &r.Filename, &r.State, &r.Stage, &r.DocKind,
			&r.CircularID, &proposalJSON, &errText, &r.CreatedAt, &approvedAt, &approvedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return IngestRun{}, fmt.Errorf("ingest run %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return IngestRun{}, fmt.Errorf("get ingest run %q: %w", id, err)
	}
	r.JobID, r.Error, r.ApprovedAt, r.ApprovedBy = ns(jobID), ns(errText), ns(approvedAt), ns(approvedBy)
	if proposalJSON != "" && proposalJSON != "{}" {
		if err := json.Unmarshal([]byte(proposalJSON), &r.Proposal); err != nil {
			return IngestRun{}, fmt.Errorf("decode proposal for run %q: %w", id, err)
		}
	}
	return r, nil
}

// ListIngestRuns returns runs newest-first.
func (s *Store) ListIngestRuns(ctx context.Context, limit int) ([]IngestRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, sha256, filename, state, stage, doc_kind, circular_id, created_at
		FROM ingest_run ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list ingest runs: %w", err)
	}
	defer rows.Close()

	var out []IngestRun
	for rows.Next() {
		var r IngestRun
		if err := rows.Scan(&r.ID, &r.SHA256, &r.Filename, &r.State, &r.Stage,
			&r.DocKind, &r.CircularID, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ingest run: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ingest runs: %w", err)
	}
	return out, nil
}

// SetIngestRunStage records progress without touching the proposal.
func (s *Store) SetIngestRunStage(ctx context.Context, id, state, stage string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE ingest_run SET state = ?, stage = ? WHERE id = ?`, state, stage, id); err != nil {
		return fmt.Errorf("set stage %q on run %q: %w", stage, id, err)
	}
	return nil
}

// SaveProposal stores a completed pipeline run's output and moves it to
// 'preview'. Nothing has entered the regulatory graph at this point.
func (s *Store) SaveProposal(ctx context.Context, id string, p ingest.Proposal) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode proposal for run %q: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE ingest_run
		SET state = 'preview', stage = ?, doc_kind = ?, circular_id = ?, proposal_json = ?, error = NULL
		WHERE id = ?`,
		ingest.StageReadyReview, string(p.Meta.DocKind), p.Circular.ID, string(raw), id,
	); err != nil {
		return fmt.Errorf("save proposal for run %q: %w", id, err)
	}
	return nil
}

// UpdateProposal overwrites an existing proposal in the 'preview' state.
// This is used to persist user edits and filtering before committing the run.
func (s *Store) UpdateProposal(ctx context.Context, id string, p ingest.Proposal) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode proposal for run %q: %w", id, err)
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE ingest_run
		SET proposal_json = ?
		WHERE id = ? AND state = 'preview'`,
		string(raw), id,
	); err != nil {
		return fmt.Errorf("update proposal for run %q: %w", id, err)
	}
	return nil
}

// FailIngestRun records a failed run with the stage it died in.
func (s *Store) FailIngestRun(ctx context.Context, id, stage, msg string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE ingest_run SET state = 'failed', stage = ?, error = ? WHERE id = ?`,
		stage, msg, id); err != nil {
		return fmt.Errorf("fail run %q: %w", id, err)
	}
	return nil
}

// DiscardIngestRun drops a proposal. A run that has already been approved cannot
// be discarded - the graph facts it created are real.
func (s *Store) DiscardIngestRun(ctx context.Context, id string) error {
	run, err := s.GetIngestRun(ctx, id)
	if err != nil {
		return err
	}
	if run.State == RunApproved || run.State == RunDiscarded {
		return fmt.Errorf("discard run %q in state %q: %w", id, run.State, ErrRunSettled)
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE ingest_run SET state = 'discarded' WHERE id = ?`, id); err != nil {
		return fmt.Errorf("discard run %q: %w", id, err)
	}
	return nil
}

// ApproveIngestRun commits an entire proposal to the regulatory graph in ONE
// transaction: circular, clauses, obligations, semantic units, circular
// relations, dangling references, and the run's own audit record.
//
// WHY ONE TRANSACTION. A partially-committed circular is worse than no circular
// at all: clauses with no obligations look like a document with no duties, and
// obligations with no clauses have no citable source. Either the whole document
// enters the graph or none of it does. On failure the run is recorded as failed
// with the stage and error, and the graph is byte-for-byte unchanged.
//
// This is also the SAFETY GATE itself. Nothing in the pipeline writes to the
// graph; only this function does, and only a human calls it.
func (s *Store) ApproveIngestRun(ctx context.Context, id, approvedBy string) (IngestRun, error) {
	run, err := s.GetIngestRun(ctx, id)
	if err != nil {
		return IngestRun{}, err
	}
	switch run.State {
	case RunPreview:
		// The only approvable state.
	case RunQueued, RunRunning:
		return IngestRun{}, fmt.Errorf("run %q is still at stage %q: %w", id, run.Stage, ErrRunNotReady)
	case RunFailed:
		return IngestRun{}, fmt.Errorf("run %q failed at stage %q: %w", id, run.Stage, ErrRunNotReady)
	default:
		// Double-approve and approve-after-discard are caller errors, never
		// silent no-ops: re-committing would duplicate or overwrite graph facts.
		return IngestRun{}, fmt.Errorf("approve run %q in state %q: %w", id, run.State, ErrRunSettled)
	}

	now := domain.RFC3339UTC(time.Now())
	p := run.Proposal

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return IngestRun{}, fmt.Errorf("begin approve tx for run %q: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := commitProposal(ctx, tx, p, now); err != nil {
		// Record the failure OUTSIDE the rolled-back transaction so the audit
		// trail survives even though the graph write did not.
		_ = tx.Rollback()
		if ferr := s.FailIngestRun(ctx, id, "commit", err.Error()); ferr != nil {
			return IngestRun{}, fmt.Errorf("commit run %q failed (%v) and recording the failure also failed: %w", id, err, ferr)
		}
		return IngestRun{}, fmt.Errorf("commit run %q: %w", id, err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE ingest_run SET state = 'approved', approved_at = ?, approved_by = ?
		WHERE id = ? AND state = 'preview'`, now, approvedBy, id); err != nil {
		return IngestRun{}, fmt.Errorf("mark run %q approved: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return IngestRun{}, fmt.Errorf("commit approve tx for run %q: %w", id, err)
	}
	return s.GetIngestRun(ctx, id)
}

// commitProposal writes an entire proposal inside the caller's transaction.
func commitProposal(ctx context.Context, tx *sql.Tx, p ingest.Proposal, now string) error {
	if p.Circular.ID == "" {
		return errors.New("proposal has no circular id")
	}

	validFrom := p.Circular.ValidFrom
	if validFrom == "" {
		validFrom = now
	}
	issued := p.Circular.IssuedOn
	if issued == "" {
		issued = validFrom
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO circular (row_uid, id, title, regulator, issued_on, source_url,
		                      valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, NULL, ?, NULL, ?, NULL)
		ON CONFLICT(id) WHERE tx_to IS NULL DO UPDATE SET
			row_uid=excluded.row_uid, title=excluded.title, regulator=excluded.regulator,
			issued_on=excluded.issued_on, valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
		p.Circular.ID+"@"+now, p.Circular.ID, p.Circular.Title, p.Circular.Regulator,
		issued, validFrom, now,
	); err != nil {
		return fmt.Errorf("insert circular %q: %w", p.Circular.ID, err)
	}

	// Clauses arrive in document pre-order, parents before children - the order
	// the clause tree's parent reference requires.
	for _, c := range p.Clauses {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO clause (row_uid, id, circular_id, clause_ref, parent_id, heading, text,
			                    ordinal, valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) WHERE tx_to IS NULL DO UPDATE SET
				row_uid=excluded.row_uid, circular_id=excluded.circular_id,
				clause_ref=excluded.clause_ref, parent_id=excluded.parent_id,
				heading=excluded.heading, text=excluded.text, ordinal=excluded.ordinal,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			c.ID+"@"+now, c.ID, p.Circular.ID, c.ClauseRef, nullStr(c.ParentID),
			nullStr(c.Heading), c.Text, c.Ordinal, validFrom, now,
		); err != nil {
			return fmt.Errorf("insert clause %q: %w", c.ClauseRef, err)
		}
	}

	for _, po := range p.Obligations {
		o := po.Obligation
		// Last guard before the graph, exactly as UpsertObligation does: an
		// obligation without provenance must never enter, whatever produced it.
		if err := o.Validate(); err != nil {
			return fmt.Errorf("reject obligation before commit: %w", err)
		}
		threshold := o.ThresholdJSON
		if threshold == "" {
			threshold = "{}"
		}
		status := o.Status
		if status == "" {
			status = domain.StatusPending
		}
		// The embedding is computed and committed HERE, inside the same
		// transaction. Leaving it to a later pass would mean a newly approved
		// circular's obligations were invisible to the semantic half of blast
		// radius until something else happened to run - an obligation that is in
		// the graph but not in the diff is a silent gap.
		embedding, err := vec.Marshal(vec.Embed(o.SourceSentence))
		if err != nil {
			return fmt.Errorf("embed obligation %q: %w", o.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO obligation (row_uid, id, clause_id, bearer, deontic_type, condition,
			                        threshold_json, deadline, penalty, source_clause_ref,
			                        source_sentence, confidence, status, embedding_json,
			                        valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) WHERE tx_to IS NULL DO UPDATE SET
				row_uid=excluded.row_uid, clause_id=excluded.clause_id, bearer=excluded.bearer,
				deontic_type=excluded.deontic_type, condition=excluded.condition,
				threshold_json=excluded.threshold_json, deadline=excluded.deadline,
				penalty=excluded.penalty, source_clause_ref=excluded.source_clause_ref,
				source_sentence=excluded.source_sentence, confidence=excluded.confidence,
				status=excluded.status, embedding_json=excluded.embedding_json,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			o.ID+"@"+now, o.ID, o.ClauseID, o.Bearer, string(o.DeonticType), nullStr(o.Condition),
			threshold, nullStr(o.Deadline), nullStr(o.Penalty), o.SourceClauseRef,
			o.SourceSentence, o.Confidence, string(status), embedding, validFrom, now,
		); err != nil {
			return fmt.Errorf("insert obligation %q: %w", o.ID, err)
		}
	}

	for _, u := range p.Units {
		if !u.Role.Valid() {
			return fmt.Errorf("semantic unit %q: invalid role %q", u.ID, u.Role)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO semantic_unit (id, clause_id, ordinal, role, text,
			                           start_offset, end_offset, valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				role=excluded.role, text=excluded.text,
				start_offset=excluded.start_offset, end_offset=excluded.end_offset,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			u.ID, u.ClauseID, u.Ordinal, string(u.Role), u.Text,
			u.StartOffset, u.EndOffset, validFrom, now,
		); err != nil {
			return fmt.Errorf("insert semantic unit %q: %w", u.ID, err)
		}
	}

	for _, r := range p.Relations {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO circular_relation (id, from_circular, to_ref, to_circular, kind,
			                               valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				to_circular=excluded.to_circular, valid_from=excluded.valid_from,
				tx_from=excluded.tx_from`,
			r.ID, r.FromCircular, r.ToRef, nullStr(r.ToCircular), r.Kind, validFrom, now,
		); err != nil {
			return fmt.Errorf("insert circular relation %q: %w", r.ID, err)
		}
	}

	// The amendment diff, applied only now that a human has approved it.
	//
	// This is where SupersedeAndInsert earns its existence: a `modified` clause
	// gets its previous version CLOSED in system time and a new version opened,
	// so querying as-of a pre-amendment date still returns the old text. The
	// plain upsert above would have overwritten it.
	if p.Amendment != nil {
		byRef := make(map[string]domain.Clause, len(p.Clauses))
		for _, c := range p.Clauses {
			byRef[c.ClauseRef] = c
		}
		for _, ch := range p.Amendment.Changes {
			switch ch.Kind {
			case ingest.ChangeModified:
				next, ok := byRef[ch.NewClauseRef]
				if !ok {
					return fmt.Errorf("amendment names clause %q which is not in the proposal", ch.NewClauseRef)
				}
				next.ID = ch.OldClauseID // the new version of the SAME logical clause
				next.ValidFrom = validFrom
				if err := supersedeClause(ctx, tx, ch.OldClauseID, next, now); err != nil {
					return err
				}
			case ingest.ChangeDeleted:
				// A deleted clause is retired in WORLD time - it ceased to be in
				// force - not erased. Its text and its history remain queryable
				// as-of any earlier date.
				if _, err := tx.ExecContext(ctx,
					`UPDATE clause SET valid_to = ? WHERE id = ? AND tx_to IS NULL`,
					validFrom, ch.OldClauseID); err != nil {
					return fmt.Errorf("retire deleted clause %q: %w", ch.OldClauseID, err)
				}
			}
			newID := ch.NewClauseID
			if newID == "" {
				newID = ch.OldClauseID
			}
			if err := recordLineage(ctx, tx, newID, ch.OldClauseID, string(ch.Kind), ch.Score, validFrom, now); err != nil {
				return err
			}
		}
	}

	// Dangling references are committed too: a citation the pipeline could not
	// follow is part of the honest record of this document.
	for _, d := range p.Dangling {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO dangling_reference (id, circular_id, clause_id, raw_text, kind, reason,
			                                valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				raw_text=excluded.raw_text, kind=excluded.kind, reason=excluded.reason,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			d.ID, d.CircularID, d.ClauseID, d.RawText, string(d.Kind), d.Reason, validFrom, now,
		); err != nil {
			return fmt.Errorf("insert dangling reference %q: %w", d.ID, err)
		}
	}
	return nil
}

// supersedeClause closes the current version of a clause and opens the amended
// one, inside the caller's transaction. It mirrors SupersedeAndInsert, which is
// a method on *Store and therefore unavailable to this package-level helper.
func supersedeClause(ctx context.Context, tx *sql.Tx, id string, next domain.Clause, at string) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE clause SET tx_to = ? WHERE id = ? AND tx_to IS NULL`, at, id)
	if err != nil {
		return fmt.Errorf("close current clause %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected closing clause %q: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("supersede clause %q: %w (no current row)", id, ErrNotFound)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO clause (row_uid, id, circular_id, clause_ref, parent_id, heading, text,
		                    ordinal, valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)`,
		id+"@"+at, id, next.CircularID, next.ClauseRef, nullStr(next.ParentID),
		nullStr(next.Heading), next.Text, next.Ordinal, next.ValidFrom, at,
	); err != nil {
		return fmt.Errorf("insert amended clause %q: %w", id, err)
	}
	return nil
}

// recordLineage stores how a clause version relates to its predecessor.
func recordLineage(ctx context.Context, tx *sql.Tx, newID, oldID, relation string, score float64, validFrom, txFrom string) error {
	id := fmt.Sprintf("lin:%s|%s", newID, oldID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO clause_lineage (id, new_clause_id, old_clause_id, relation, score,
		                            valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, NULL, ?, NULL)
		ON CONFLICT(id) DO UPDATE SET
			relation=excluded.relation, score=excluded.score,
			valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
		id, newID, nullStr(oldID), relation, score, validFrom, txFrom); err != nil {
		return fmt.Errorf("record clause lineage %q: %w", id, err)
	}
	return nil
}

// CountDanglingReferences returns how many unresolved references exist for a
// circular - surfaced in the review queue.
func (s *Store) CountDanglingReferences(ctx context.Context, circularID string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM dangling_reference WHERE circular_id = ? AND tx_to IS NULL`,
		circularID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count dangling references for %q: %w", circularID, err)
	}
	return n, nil
}
