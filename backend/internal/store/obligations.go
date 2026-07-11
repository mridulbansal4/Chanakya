package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// UpsertObligation inserts or updates an obligation by id (idempotent
// compilation). The store is the final guard: the DB schema itself enforces the
// deontic/status domains and NOT NULL provenance, and this method also runs
// domain.Validate before writing.
func (s *Store) UpsertObligation(ctx context.Context, o domain.Obligation) error {
	if err := o.Validate(); err != nil {
		return fmt.Errorf("reject obligation before store: %w", err)
	}
	threshold := o.ThresholdJSON
	if threshold == "" {
		threshold = "{}"
	}
	status := o.Status
	if status == "" {
		status = domain.StatusPending
	}
	const q = `
		INSERT INTO obligation (id, clause_id, bearer, deontic_type, condition,
		                        threshold_json, deadline, penalty, source_clause_ref,
		                        source_sentence, confidence, status,
		                        valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			clause_id=excluded.clause_id, bearer=excluded.bearer,
			deontic_type=excluded.deontic_type, condition=excluded.condition,
			threshold_json=excluded.threshold_json, deadline=excluded.deadline,
			penalty=excluded.penalty, source_clause_ref=excluded.source_clause_ref,
			source_sentence=excluded.source_sentence, confidence=excluded.confidence,
			status=excluded.status, valid_from=excluded.valid_from,
			valid_to=excluded.valid_to, tx_from=excluded.tx_from, tx_to=excluded.tx_to`
	if _, err := s.db.ExecContext(ctx, q,
		o.ID, o.ClauseID, o.Bearer, string(o.DeonticType), nullStr(o.Condition),
		threshold, nullStr(o.Deadline), nullStr(o.Penalty), o.SourceClauseRef,
		o.SourceSentence, o.Confidence, string(status),
		o.ValidFrom, nullStr(o.ValidTo), o.TxFrom, nullStr(o.TxTo),
	); err != nil {
		return fmt.Errorf("upsert obligation %q: %w", o.ID, err)
	}
	return nil
}

// CountObligations returns the number of current obligations (tx_to IS NULL),
// optionally filtered by status ("" for all).
func (s *Store) CountObligations(ctx context.Context, status string) (int, error) {
	var (
		n   int
		err error
	)
	if status == "" {
		err = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM obligation WHERE tx_to IS NULL`).Scan(&n)
	} else {
		err = s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM obligation WHERE tx_to IS NULL AND status = ?`, status).Scan(&n)
	}
	if err != nil {
		return 0, fmt.Errorf("count obligations: %w", err)
	}
	return n, nil
}

// scanObligations reads obligation rows into domain values.
func scanObligations(rows *sql.Rows) ([]domain.Obligation, error) {
	defer rows.Close()
	var out []domain.Obligation
	for rows.Next() {
		var (
			o                                     domain.Obligation
			deontic, status                       string
			condition, deadline, penalty, validTo sql.NullString
			txTo                                  sql.NullString
		)
		if err := rows.Scan(
			&o.ID, &o.ClauseID, &o.Bearer, &deontic, &condition, &o.ThresholdJSON,
			&deadline, &penalty, &o.SourceClauseRef, &o.SourceSentence, &o.Confidence,
			&status, &o.ValidFrom, &validTo, &o.TxFrom, &txTo,
		); err != nil {
			return nil, fmt.Errorf("scan obligation: %w", err)
		}
		o.DeonticType = domain.DeonticType(deontic)
		o.Status = domain.ObligationStatus(status)
		o.Condition = ns(condition)
		o.Deadline = ns(deadline)
		o.Penalty = ns(penalty)
		o.ValidTo = ns(validTo)
		o.TxTo = ns(txTo)
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate obligations: %w", err)
	}
	return out, nil
}

const obligationCols = `id, clause_id, bearer, deontic_type, condition, threshold_json,
	deadline, penalty, source_clause_ref, source_sentence, confidence, status,
	valid_from, valid_to, tx_from, tx_to`

// ListObligationsByClause returns the current obligations for a clause.
func (s *Store) ListObligationsByClause(ctx context.Context, clauseID string) ([]domain.Obligation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+obligationCols+` FROM obligation
		 WHERE clause_id = ? AND tx_to IS NULL
		 ORDER BY deontic_type, id`, clauseID)
	if err != nil {
		return nil, fmt.Errorf("query obligations for clause %q: %w", clauseID, err)
	}
	return scanObligations(rows)
}

// ListClauses returns all of a circular's clauses (flat) that are in force in
// world time at asOf and current in system time, in document order.
func (s *Store) ListClauses(ctx context.Context, circularID string, asOf time.Time) ([]domain.Clause, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, circular_id, clause_ref, parent_id, heading, text, ordinal,
		       valid_from, valid_to, tx_from, tx_to
		FROM clause
		WHERE circular_id = ?
		  AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?)
		  AND tx_to IS NULL
		ORDER BY ordinal`, circularID, at, at)
	if err != nil {
		return nil, fmt.Errorf("list clauses for %q: %w", circularID, err)
	}
	defer rows.Close()

	var out []domain.Clause
	for rows.Next() {
		var (
			c                        domain.Clause
			parent, heading, validTo sql.NullString
			txTo                     sql.NullString
		)
		if err := rows.Scan(
			&c.ID, &c.CircularID, &c.ClauseRef, &parent, &heading, &c.Text, &c.Ordinal,
			&c.ValidFrom, &validTo, &c.TxFrom, &txTo,
		); err != nil {
			return nil, fmt.Errorf("scan clause: %w", err)
		}
		c.ParentID = ns(parent)
		c.Heading = ns(heading)
		c.ValidTo = ns(validTo)
		c.TxTo = ns(txTo)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clauses: %w", err)
	}
	return out, nil
}
