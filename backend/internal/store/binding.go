package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// BindingRecord is one obligation → firm-object edge, as stored.
//
// It is INFERENCE. Confidence and HumanConfirmed are on the record itself, so a
// consumer cannot read a binding without also seeing how much to trust it.
type BindingRecord struct {
	ID             string  `json:"id"`
	ObligationID   string  `json:"obligation_id"`
	TargetType     string  `json:"target_type"`
	TargetID       string  `json:"target_id"`
	Confidence     float64 `json:"confidence"`
	HumanConfirmed bool    `json:"human_confirmed"`
	Rationale      string  `json:"rationale"`
}

// bindingID is deterministic on the (obligation, target_type, target_id) tuple,
// which is exactly the tuple the table's UNIQUE constraint dedupes on. Two runs
// proposing the same edge produce the same row rather than a duplicate.
func bindingID(obligationID, targetType, targetID string) string {
	sum := sha256.Sum256([]byte(obligationID + "|" + targetType + "|" + targetID))
	return "bind:" + hex.EncodeToString(sum[:])[:16]
}

// UpsertBinding records a proposed binding, deduping on its natural key.
func (s *Store) UpsertBinding(ctx context.Context, b BindingRecord, validFrom, txFrom string) error {
	if b.ObligationID == "" || b.TargetType == "" || b.TargetID == "" {
		return fmt.Errorf("upsert binding: obligation, target type and target id are all required")
	}
	id := bindingID(b.ObligationID, b.TargetType, b.TargetID)
	// human_confirmed is deliberately NOT in the DO UPDATE list: re-running the
	// projector must never silently un-confirm something a person confirmed.
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO binds_to (id, obligation_id, target_type, target_id, confidence,
		                      human_confirmed, rationale, valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, NULL, ?, NULL)
		ON CONFLICT(obligation_id, target_type, target_id) DO UPDATE SET
			confidence=excluded.confidence, rationale=excluded.rationale,
			valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
		id, b.ObligationID, b.TargetType, b.TargetID, b.Confidence,
		nullStr(b.Rationale), validFrom, txFrom,
	); err != nil {
		return fmt.Errorf("upsert binding %q: %w", id, err)
	}
	return nil
}

// ConfirmBinding marks a binding as human-confirmed - the only way that flag is
// ever set.
func (s *Store) ConfirmBinding(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE binds_to SET human_confirmed = 1 WHERE id = ? AND tx_to IS NULL`, id)
	if err != nil {
		return fmt.Errorf("confirm binding %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected confirming %q: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("confirm binding %q: %w", id, ErrNotFound)
	}
	return nil
}

// ListBindings returns an obligation's proposed bindings as of a date.
func (s *Store) ListBindings(ctx context.Context, obligationID string, asOf time.Time) ([]BindingRecord, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, obligation_id, target_type, target_id, confidence,
		       human_confirmed, COALESCE(rationale,'')
		FROM binds_to
		WHERE obligation_id = ? AND`+asOfClause+`
		ORDER BY confidence DESC, target_type, target_id`, obligationID, at, at)
	if err != nil {
		return nil, fmt.Errorf("list bindings for %q: %w", obligationID, err)
	}
	defer rows.Close()

	var out []BindingRecord
	for rows.Next() {
		var (
			b         BindingRecord
			confirmed int
		)
		if err := rows.Scan(&b.ID, &b.ObligationID, &b.TargetType, &b.TargetID,
			&b.Confidence, &confirmed, &b.Rationale); err != nil {
			return nil, fmt.Errorf("scan binding: %w", err)
		}
		b.HumanConfirmed = confirmed == 1
		out = append(out, b)
	}
	return out, rows.Err()
}

// CountBindings returns how many bindings exist in total.
func (s *Store) CountBindings(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM binds_to WHERE tx_to IS NULL`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count bindings: %w", err)
	}
	return n, nil
}
