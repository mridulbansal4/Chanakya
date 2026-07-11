package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// nullStr maps an empty string to SQL NULL, otherwise to the string itself.
// Used for open-ended temporal bounds (valid_to/tx_to) and optional columns.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ns unwraps a sql.NullString to a plain string ("" when NULL).
func ns(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}

// UpsertCircular inserts or updates a circular by id (idempotent seeding).
func (s *Store) UpsertCircular(ctx context.Context, c domain.Circular) error {
	const q = `
		INSERT INTO circular (id, title, regulator, issued_on, source_url,
		                      valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title, regulator=excluded.regulator,
			issued_on=excluded.issued_on, source_url=excluded.source_url,
			valid_from=excluded.valid_from, valid_to=excluded.valid_to,
			tx_from=excluded.tx_from, tx_to=excluded.tx_to`
	if _, err := s.db.ExecContext(ctx, q,
		c.ID, c.Title, c.Regulator, c.IssuedOn, nullStr(c.SourceURL),
		c.ValidFrom, nullStr(c.ValidTo), c.TxFrom, nullStr(c.TxTo),
	); err != nil {
		return fmt.Errorf("upsert circular %q: %w", c.ID, err)
	}
	return nil
}

// UpsertEntity inserts or updates a regulated entity by id.
func (s *Store) UpsertEntity(ctx context.Context, e domain.Entity) error {
	meta := e.MetaJSON
	if meta == "" {
		meta = "{}"
	}
	const q = `
		INSERT INTO entity (id, kind, name, pan, meta_json,
		                    valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kind=excluded.kind, name=excluded.name, pan=excluded.pan,
			meta_json=excluded.meta_json, valid_from=excluded.valid_from,
			valid_to=excluded.valid_to, tx_from=excluded.tx_from, tx_to=excluded.tx_to`
	if _, err := s.db.ExecContext(ctx, q,
		e.ID, e.Kind, e.Name, nullStr(e.PAN), meta,
		e.ValidFrom, nullStr(e.ValidTo), e.TxFrom, nullStr(e.TxTo),
	); err != nil {
		return fmt.Errorf("upsert entity %q: %w", e.ID, err)
	}
	return nil
}

// UpsertClause inserts or updates a clause by id. The caller must upsert a
// clause's parent before the clause itself (foreign_keys is ON); the seeder
// guarantees this by processing clauses in document (parents-first) order.
func (s *Store) UpsertClause(ctx context.Context, c domain.Clause) error {
	const q = `
		INSERT INTO clause (id, circular_id, clause_ref, parent_id, heading, text,
		                    ordinal, valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			circular_id=excluded.circular_id, clause_ref=excluded.clause_ref,
			parent_id=excluded.parent_id, heading=excluded.heading, text=excluded.text,
			ordinal=excluded.ordinal, valid_from=excluded.valid_from,
			valid_to=excluded.valid_to, tx_from=excluded.tx_from, tx_to=excluded.tx_to`
	if _, err := s.db.ExecContext(ctx, q,
		c.ID, c.CircularID, c.ClauseRef, nullStr(c.ParentID), nullStr(c.Heading),
		c.Text, c.Ordinal, c.ValidFrom, nullStr(c.ValidTo), c.TxFrom, nullStr(c.TxTo),
	); err != nil {
		return fmt.Errorf("upsert clause %q: %w", c.ID, err)
	}
	return nil
}

// CountClauses returns the number of currently-known clause rows (tx_to IS NULL)
// for a circular. Handy for seed verification.
func (s *Store) CountClauses(ctx context.Context, circularID string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM clause WHERE circular_id = ? AND tx_to IS NULL`,
		circularID,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("count clauses for %q: %w", circularID, err)
	}
	return n, nil
}

// GetClauseSubtree returns the clause identified by rootID together with all of
// its descendants that are in force in WORLD time at asOf and current in SYSTEM
// time (latest knowledge), ordered in document pre-order. It is implemented
// with a recursive CTE (WITH RECURSIVE), which SQLite supports natively.
//
// Passing the root's own id returns the whole tree under a chapter; passing a
// circular's synthetic root is not required because top-level clauses have no
// parent — call once per top-level clause, or use ListClauses for the flat set.
func (s *Store) GetClauseSubtree(ctx context.Context, rootID string, asOf time.Time) ([]domain.ClauseNode, error) {
	at := domain.RFC3339UTC(asOf)
	const q = `
		WITH RECURSIVE subtree(id, circular_id, clause_ref, parent_id, heading, text,
		                       ordinal, valid_from, valid_to, tx_from, tx_to, depth, path) AS (
			SELECT id, circular_id, clause_ref, parent_id, heading, text, ordinal,
			       valid_from, valid_to, tx_from, tx_to,
			       0 AS depth, printf('%06d', ordinal) AS path
			FROM clause
			WHERE id = ?
			  AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?)
			  AND tx_to IS NULL
			UNION ALL
			SELECT c.id, c.circular_id, c.clause_ref, c.parent_id, c.heading, c.text,
			       c.ordinal, c.valid_from, c.valid_to, c.tx_from, c.tx_to,
			       s.depth + 1, s.path || '.' || printf('%06d', c.ordinal)
			FROM clause c
			JOIN subtree s ON c.parent_id = s.id
			WHERE c.valid_from <= ? AND (c.valid_to IS NULL OR c.valid_to > ?)
			  AND c.tx_to IS NULL
		)
		SELECT id, circular_id, clause_ref, parent_id, heading, text, ordinal,
		       valid_from, valid_to, tx_from, tx_to, depth, path
		FROM subtree
		ORDER BY path`
	rows, err := s.db.QueryContext(ctx, q, rootID, at, at, at, at)
	if err != nil {
		return nil, fmt.Errorf("query clause subtree %q as-of %s: %w", rootID, at, err)
	}
	defer rows.Close()

	var out []domain.ClauseNode
	for rows.Next() {
		var (
			n                        domain.ClauseNode
			parent, heading, validTo sql.NullString
			txTo                     sql.NullString
		)
		if err := rows.Scan(
			&n.ID, &n.CircularID, &n.ClauseRef, &parent, &heading, &n.Text, &n.Ordinal,
			&n.ValidFrom, &validTo, &n.TxFrom, &txTo, &n.Depth, &n.Path,
		); err != nil {
			return nil, fmt.Errorf("scan clause node: %w", err)
		}
		n.ParentID = ns(parent)
		n.Heading = ns(heading)
		n.ValidTo = ns(validTo)
		n.TxTo = ns(txTo)
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clause subtree: %w", err)
	}
	return out, nil
}

// ListTopLevelClauses returns the ids of a circular's top-level clauses (no
// parent) that are current in system time, in document order. Combine with
// GetClauseSubtree to walk the whole document.
func (s *Store) ListTopLevelClauses(ctx context.Context, circularID string, asOf time.Time) ([]string, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM clause
		WHERE circular_id = ? AND parent_id IS NULL
		  AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?)
		  AND tx_to IS NULL
		ORDER BY ordinal`, circularID, at, at)
	if err != nil {
		return nil, fmt.Errorf("query top-level clauses for %q: %w", circularID, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan top-level clause id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top-level clauses: %w", err)
	}
	return ids, nil
}
