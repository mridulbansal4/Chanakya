package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Job states.
const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
)

// Job is one unit of background work.
type Job struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	PayloadJSON  string `json:"payload_json"`
	State        string `json:"state"`
	Attempts     int    `json:"attempts"`
	Error        string `json:"error"`
	CreatedAt    string `json:"created_at"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	ProgressJSON string `json:"progress_json"`
}

// EnqueueJob inserts a queued job. The id is caller-supplied and deterministic,
// so enqueueing the same work twice is idempotent rather than duplicated.
func (s *Store) EnqueueJob(ctx context.Context, id, kind, payloadJSON string) error {
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO job (id, kind, payload_json, state)
		VALUES (?, ?, ?, 'queued')
		ON CONFLICT(id) DO UPDATE SET
			payload_json=excluded.payload_json, state='queued', error=NULL,
			started_at=NULL, finished_at=NULL, progress_json='{}'`,
		id, kind, payloadJSON,
	); err != nil {
		return fmt.Errorf("enqueue job %q: %w", id, err)
	}
	return nil
}

// ClaimJob atomically takes the oldest queued job of any kind and marks it
// running, returning it. It returns ErrNotFound when the queue is empty.
//
// The claim is a single UPDATE ... RETURNING, so two workers cannot take the
// same job: SQLite serialises writers, and the WHERE clause re-checks the state
// inside the same statement. With MaxOpenConns(1) this is also the only write
// happening at that instant.
func (s *Store) ClaimJob(ctx context.Context) (Job, error) {
	var j Job
	var errText, startedAt, finishedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		UPDATE job
		SET state = 'running',
		    attempts = attempts + 1,
		    started_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE id = (
			SELECT id FROM job WHERE state = 'queued' ORDER BY created_at, id LIMIT 1
		)
		RETURNING id, kind, payload_json, state, attempts, error,
		          created_at, started_at, finished_at, progress_json`).
		Scan(&j.ID, &j.Kind, &j.PayloadJSON, &j.State, &j.Attempts, &errText,
			&j.CreatedAt, &startedAt, &finishedAt, &j.ProgressJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("claim job: %w", err)
	}
	j.Error, j.StartedAt, j.FinishedAt = ns(errText), ns(startedAt), ns(finishedAt)
	return j, nil
}

func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	var j Job
	var errText, startedAt, finishedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, payload_json, state, attempts, error,
		       created_at, started_at, finished_at, progress_json
		FROM job WHERE id = ?`, id).
		Scan(&j.ID, &j.Kind, &j.PayloadJSON, &j.State, &j.Attempts, &errText,
			&j.CreatedAt, &startedAt, &finishedAt, &j.ProgressJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, fmt.Errorf("job %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return Job{}, fmt.Errorf("get job %q: %w", id, err)
	}
	j.Error, j.StartedAt, j.FinishedAt = ns(errText), ns(startedAt), ns(finishedAt)
	return j, nil
}

// SetJobProgress records a progress snapshot.
func (s *Store) SetJobProgress(ctx context.Context, id, progressJSON string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE job SET progress_json = ? WHERE id = ?`, progressJSON, id); err != nil {
		return fmt.Errorf("set progress on job %q: %w", id, err)
	}
	return nil
}

// FinishJob marks a job succeeded or failed.
func (s *Store) FinishJob(ctx context.Context, id, state, errText string) error {
	if state != JobSucceeded && state != JobFailed {
		return fmt.Errorf("finish job %q: invalid terminal state %q", id, state)
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE job SET state = ?, error = ?,
		       finished_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE id = ?`, state, nullStr(errText), id); err != nil {
		return fmt.Errorf("finish job %q: %w", id, err)
	}
	return nil
}

// CountJobs returns how many jobs are in a given state.
func (s *Store) CountJobs(ctx context.Context, state string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM job WHERE state = ?`, state).Scan(&n); err != nil {
		return 0, fmt.Errorf("count jobs in state %q: %w", state, err)
	}
	return n, nil
}
