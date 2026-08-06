package store

import (
	"context"
	"database/sql"
	"fmt"

	"chanakya/internal/domain"
	"chanakya/internal/vec"
)

// versionedTable describes one of the three system-time-versioned tables and
// how to write a new version of a fact into it.
//
// The set is CLOSED and the SQL is fixed per entry: a table name cannot be
// parameterized with ?, so allowing an arbitrary caller-supplied table name
// would mean concatenating untrusted input into SQL. Callers pass a name, we
// look it up here, and an unknown name is an error - never a query.
type versionedTable struct {
	// insert is the full INSERT for a new version. Its placeholders are filled
	// by bind, in order.
	insert string
	// bind maps the domain value to the insert's arguments, stamping tx_from =
	// at and leaving tx_to NULL (the new row IS current knowledge).
	bind func(next any, at string) ([]any, error)
}

var versionedTables = map[string]versionedTable{
	"circular": {
		insert: `INSERT INTO circular (row_uid, id, title, regulator, issued_on, source_url,
		                               valid_from, valid_to, tx_from, tx_to)
		         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		bind: func(next any, at string) ([]any, error) {
			c, ok := next.(domain.Circular)
			if !ok {
				return nil, fmt.Errorf("table circular: next is %T, want domain.Circular", next)
			}
			return []any{
				rowUID(c.ID, at), c.ID, c.Title, c.Regulator, c.IssuedOn, nullStr(c.SourceURL),
				c.ValidFrom, nullStr(c.ValidTo), at,
			}, nil
		},
	},
	"clause": {
		insert: `INSERT INTO clause (row_uid, id, circular_id, clause_ref, parent_id, heading,
		                             text, ordinal, valid_from, valid_to, tx_from, tx_to)
		         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		bind: func(next any, at string) ([]any, error) {
			c, ok := next.(domain.Clause)
			if !ok {
				return nil, fmt.Errorf("table clause: next is %T, want domain.Clause", next)
			}
			return []any{
				rowUID(c.ID, at), c.ID, c.CircularID, c.ClauseRef, nullStr(c.ParentID),
				nullStr(c.Heading), c.Text, c.Ordinal, c.ValidFrom, nullStr(c.ValidTo), at,
			}, nil
		},
	},
	"obligation": {
		// The embedding is recomputed from the NEW version's source sentence.
		// Carrying the old vector forward would leave the semantic diff matching
		// against text that no longer exists.
		insert: `INSERT INTO obligation (row_uid, id, clause_id, bearer, deontic_type, condition,
		                                 threshold_json, deadline, penalty, source_clause_ref,
		                                 source_sentence, confidence, status, embedding_json,
		                                 valid_from, valid_to, tx_from, tx_to)
		         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		bind: func(next any, at string) ([]any, error) {
			o, ok := next.(domain.Obligation)
			if !ok {
				return nil, fmt.Errorf("table obligation: next is %T, want domain.Obligation", next)
			}
			// The store boundary re-validates: a new VERSION of an obligation is
			// still an obligation entering the graph, so the mandatory-provenance
			// invariant applies to it exactly as it does to the first version.
			if err := o.Validate(); err != nil {
				return nil, fmt.Errorf("validate next obligation: %w", err)
			}
			threshold := o.ThresholdJSON
			if threshold == "" {
				threshold = "{}"
			}
			status := o.Status
			if status == "" {
				status = domain.StatusPending
			}
			embedding, err := vec.Marshal(vec.Embed(o.SourceSentence))
			if err != nil {
				return nil, fmt.Errorf("embed next obligation: %w", err)
			}
			return []any{
				rowUID(o.ID, at), o.ID, o.ClauseID, o.Bearer, string(o.DeonticType),
				nullStr(o.Condition), threshold, nullStr(o.Deadline), nullStr(o.Penalty),
				o.SourceClauseRef, o.SourceSentence, o.Confidence, string(status), embedding,
				o.ValidFrom, nullStr(o.ValidTo), at,
			}, nil
		},
	},
}

// rowUID is the deterministic surrogate primary key for one version of a fact.
// Determinism keeps re-applying the same supersession idempotent, matching the
// project's deterministic-id idiom everywhere else.
func rowUID(id, txFrom string) string { return id + "@" + txFrom }

// SupersedeAndInsert closes the current row's system-time interval and inserts
// the new version as current knowledge. This is how a fact CHANGES; Upsert is
// only for idempotent re-seeding of an UNCHANGED fact.
//
// Why this exists: the Upsert path overwrites a row in place, which destroys the
// prior text of an amended clause. Superseding keeps BOTH versions - the old one
// bounded by tx_to, the new one open - so an as-of query issued before the
// supersession timestamp still reconstructs exactly what CHANAKYA knew then.
// That is what makes "what did this clause say before the amendment?" answerable
// and is the property the audit trail rests on.
//
// tx is required: closing the old interval and opening the new one must be
// atomic, or a concurrent reader could observe a fact with no current version
// (or, worse, two). The partial unique index `UNIQUE(id) WHERE tx_to IS NULL`
// is the database-level backstop for that same invariant.
//
// at must be an RFC3339 UTC timestamp (see domain.RFC3339UTC): timestamps are
// compared lexically, so a non-canonical format would silently break ordering.
func (s *Store) SupersedeAndInsert(ctx context.Context, tx *sql.Tx, table, id string, next any, at string) error {
	spec, ok := versionedTables[table]
	if !ok {
		return fmt.Errorf("supersede %q: not a versioned table", table)
	}
	if tx == nil {
		return fmt.Errorf("supersede %s %q: nil transaction", table, id)
	}
	if id == "" {
		return fmt.Errorf("supersede %s: empty id", table)
	}
	if at == "" {
		return fmt.Errorf("supersede %s %q: empty supersession timestamp", table, id)
	}

	args, err := spec.bind(next, at)
	if err != nil {
		return fmt.Errorf("supersede %s %q: %w", table, id, err)
	}

	// Close the current interval first. The table name comes from the closed
	// lookup above, never from caller input.
	res, err := tx.ExecContext(ctx,
		"UPDATE "+table+" SET tx_to = ? WHERE id = ? AND tx_to IS NULL", at, id)
	if err != nil {
		return fmt.Errorf("close current %s %q: %w", table, id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected closing %s %q: %w", table, id, err)
	}
	// An id with no current row means the caller is superseding something that
	// does not exist. Falling back to a plain insert here would silently paper
	// over that caller bug and produce a fact with no history, so it is an error.
	if n == 0 {
		return fmt.Errorf("supersede %s %q: %w (no current row to supersede)", table, id, ErrNotFound)
	}

	if _, err := tx.ExecContext(ctx, spec.insert, args...); err != nil {
		return fmt.Errorf("insert new version of %s %q: %w", table, id, err)
	}
	return nil
}
