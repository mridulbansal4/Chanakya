package store

import (
	"context"
	"fmt"

	"chanakya/internal/domain"
)

// UpsertControl inserts or updates a firm control by id.
func (s *Store) UpsertControl(ctx context.Context, c domain.Control) error {
	const q = `
		INSERT INTO control (id, name, description, kind, valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, description=excluded.description, kind=excluded.kind,
			valid_from=excluded.valid_from, valid_to=excluded.valid_to,
			tx_from=excluded.tx_from, tx_to=excluded.tx_to`
	if _, err := s.db.ExecContext(ctx, q,
		c.ID, c.Name, nullStr(c.Description), nullStr(c.Kind),
		c.ValidFrom, nullStr(c.ValidTo), c.TxFrom, nullStr(c.TxTo),
	); err != nil {
		return fmt.Errorf("upsert control %q: %w", c.ID, err)
	}
	return nil
}

// UpsertEvidence inserts or updates a read-only evidence reference by id.
func (s *Store) UpsertEvidence(ctx context.Context, e domain.Evidence) error {
	const q = `
		INSERT INTO evidence (id, name, source_system, description, kind, valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, source_system=excluded.source_system,
			description=excluded.description, kind=excluded.kind,
			valid_from=excluded.valid_from, valid_to=excluded.valid_to,
			tx_from=excluded.tx_from, tx_to=excluded.tx_to`
	if _, err := s.db.ExecContext(ctx, q,
		e.ID, e.Name, nullStr(e.SourceSystem), nullStr(e.Description), nullStr(e.Kind),
		e.ValidFrom, nullStr(e.ValidTo), e.TxFrom, nullStr(e.TxTo),
	); err != nil {
		return fmt.Errorf("upsert evidence %q: %w", e.ID, err)
	}
	return nil
}

// UpsertObligationControl links an obligation to a control (idempotent by id).
func (s *Store) UpsertObligationControl(ctx context.Context, obligationID, controlID, validFrom, txFrom string) error {
	id := "oc:" + obligationID + "->" + controlID
	const q = `
		INSERT INTO obligation_control (id, obligation_id, control_id, valid_from, tx_from)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET valid_from=excluded.valid_from, tx_from=excluded.tx_from`
	if _, err := s.db.ExecContext(ctx, q, id, obligationID, controlID, validFrom, txFrom); err != nil {
		return fmt.Errorf("link obligation %q -> control %q: %w", obligationID, controlID, err)
	}
	return nil
}

// UpsertControlEvidence links a control to an evidence reference.
func (s *Store) UpsertControlEvidence(ctx context.Context, controlID, evidenceID, validFrom, txFrom string) error {
	id := "ce:" + controlID + "->" + evidenceID
	const q = `
		INSERT INTO control_evidence (id, control_id, evidence_id, valid_from, tx_from)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET valid_from=excluded.valid_from, tx_from=excluded.tx_from`
	if _, err := s.db.ExecContext(ctx, q, id, controlID, evidenceID, validFrom, txFrom); err != nil {
		return fmt.Errorf("link control %q -> evidence %q: %w", controlID, evidenceID, err)
	}
	return nil
}

// SetObligationEmbedding stores the JSON embedding for an obligation.
func (s *Store) SetObligationEmbedding(ctx context.Context, obligationID, embeddingJSON string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE obligation SET embedding_json = ? WHERE id = ?`, embeddingJSON, obligationID); err != nil {
		return fmt.Errorf("set embedding for %q: %w", obligationID, err)
	}
	return nil
}

// CountControls / CountEvidence for reporting.
func (s *Store) CountControls(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM control WHERE tx_to IS NULL`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count controls: %w", err)
	}
	return n, nil
}
