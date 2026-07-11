package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// GetObligationDomain loads the full domain obligation (for canonical hashing).
func (s *Store) GetObligationDomain(ctx context.Context, id string) (domain.Obligation, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+obligationCols+` FROM obligation WHERE id = ? AND tx_to IS NULL`, id)
	var (
		o                                     domain.Obligation
		deontic, status                       string
		condition, deadline, penalty, validTo sql.NullString
		txTo                                  sql.NullString
	)
	if err := row.Scan(
		&o.ID, &o.ClauseID, &o.Bearer, &deontic, &condition, &o.ThresholdJSON,
		&deadline, &penalty, &o.SourceClauseRef, &o.SourceSentence, &o.Confidence,
		&status, &o.ValidFrom, &validTo, &o.TxFrom, &txTo,
	); err != nil {
		if err == sql.ErrNoRows {
			return domain.Obligation{}, ErrNotFound
		}
		return domain.Obligation{}, fmt.Errorf("get obligation domain %q: %w", id, err)
	}
	o.DeonticType = domain.DeonticType(deontic)
	o.Status = domain.ObligationStatus(status)
	o.Condition = ns(condition)
	o.Deadline = ns(deadline)
	o.Penalty = ns(penalty)
	o.ValidTo = ns(validTo)
	o.TxTo = ns(txTo)
	return o, nil
}

// SetObligationStatus updates an obligation's review status.
func (s *Store) SetObligationStatus(ctx context.Context, id string, status domain.ObligationStatus) error {
	if !status.Valid() {
		return fmt.Errorf("invalid status %q", status)
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE obligation SET status = ? WHERE id = ? AND tx_to IS NULL`, string(status), id); err != nil {
		return fmt.Errorf("set status for %q: %w", id, err)
	}
	return nil
}

// ObligationCorrection carries optional field edits applied before an approval.
// A nil pointer / nil Threshold leaves that field unchanged.
type ObligationCorrection struct {
	DeonticType *string
	Condition   *string
	Deadline    *string
	Threshold   json.RawMessage
}

// Empty reports whether the correction changes nothing.
func (c ObligationCorrection) Empty() bool {
	return c.DeonticType == nil && c.Condition == nil && c.Deadline == nil && c.Threshold == nil
}

// ApplyObligationCorrection updates the given fields on an obligation. The
// caller is responsible for re-signing: the correction changes the canonical
// content, so a prior signature (if any) no longer verifies.
func (s *Store) ApplyObligationCorrection(ctx context.Context, id string, c ObligationCorrection) error {
	if c.DeonticType != nil {
		if !domain.DeonticType(*c.DeonticType).Valid() {
			return fmt.Errorf("correction: invalid deontic_type %q", *c.DeonticType)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE obligation SET deontic_type = ? WHERE id = ? AND tx_to IS NULL`, *c.DeonticType, id); err != nil {
			return fmt.Errorf("correct deontic: %w", err)
		}
	}
	if c.Condition != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE obligation SET condition = ? WHERE id = ? AND tx_to IS NULL`, nullStr(*c.Condition), id); err != nil {
			return fmt.Errorf("correct condition: %w", err)
		}
	}
	if c.Deadline != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE obligation SET deadline = ? WHERE id = ? AND tx_to IS NULL`, nullStr(*c.Deadline), id); err != nil {
			return fmt.Errorf("correct deadline: %w", err)
		}
	}
	if c.Threshold != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE obligation SET threshold_json = ? WHERE id = ? AND tx_to IS NULL`, string(c.Threshold), id); err != nil {
			return fmt.Errorf("correct threshold: %w", err)
		}
	}
	return nil
}

// SignoffRecord is the stored + API shape of a sign-off.
type SignoffRecord struct {
	ID             string `json:"id"`
	ObligationID   string `json:"obligation_id"`
	Action         string `json:"action"`
	ObligationHash string `json:"obligation_hash"`
	Signature      string `json:"signature,omitempty"`
	PublicKey      string `json:"public_key,omitempty"`
	SignedBy       string `json:"signed_by"`
	Justification  string `json:"justification"`
	CreatedAt      string `json:"created_at"`
}

// UpsertSignoff writes the latest sign-off for an obligation (id is
// deterministic per obligation).
func (s *Store) UpsertSignoff(ctx context.Context, rec SignoffRecord, validFrom, txFrom string) error {
	const q = `
		INSERT INTO signoff (id, obligation_id, action, obligation_hash, signature,
		                     public_key, signed_by, justification, valid_from, tx_from)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			action=excluded.action, obligation_hash=excluded.obligation_hash,
			signature=excluded.signature, public_key=excluded.public_key,
			signed_by=excluded.signed_by, justification=excluded.justification,
			created_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),
			valid_from=excluded.valid_from, tx_from=excluded.tx_from`
	if _, err := s.db.ExecContext(ctx, q,
		rec.ID, rec.ObligationID, rec.Action, rec.ObligationHash,
		nullStr(rec.Signature), nullStr(rec.PublicKey), rec.SignedBy, rec.Justification,
		validFrom, txFrom,
	); err != nil {
		return fmt.Errorf("upsert signoff %q: %w", rec.ID, err)
	}
	return nil
}

// GetSignoff returns the current sign-off for an obligation, if any.
func (s *Store) GetSignoff(ctx context.Context, obligationID string) (SignoffRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, obligation_id, action, obligation_hash, COALESCE(signature,''),
		       COALESCE(public_key,''), signed_by, justification, created_at
		FROM signoff WHERE obligation_id = ? AND tx_to IS NULL`, obligationID)
	var r SignoffRecord
	if err := row.Scan(&r.ID, &r.ObligationID, &r.Action, &r.ObligationHash,
		&r.Signature, &r.PublicKey, &r.SignedBy, &r.Justification, &r.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return SignoffRecord{}, false, nil
		}
		return SignoffRecord{}, false, fmt.Errorf("get signoff for %q: %w", obligationID, err)
	}
	return r, true, nil
}

// ReviewQueue returns obligations awaiting human review (pending or
// needs_review), lowest-confidence first, with clause context for the reasoning
// chain.
func (s *Store) ReviewQueue(ctx context.Context, asOf time.Time) ([]ObligationView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+obligationViewCols+`
		FROM obligation o JOIN clause c ON c.id = o.clause_id
		WHERE o.valid_from <= ? AND (o.valid_to IS NULL OR o.valid_to > ?) AND o.tx_to IS NULL
		  AND o.status IN ('pending','needs_review')
		ORDER BY o.confidence ASC, c.ordinal`, at, at)
	if err != nil {
		return nil, fmt.Errorf("review queue: %w", err)
	}
	defer rows.Close()
	out := []ObligationView{}
	for rows.Next() {
		v, err := scanObligationView(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review item: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review queue: %w", err)
	}
	return out, nil
}
