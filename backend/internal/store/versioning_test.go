package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"chanakya/internal/domain"
)

// clauseTextAsOfTx reads the clause text CHANAKYA knew at system time txAt -
// i.e. the version whose [tx_from, tx_to) interval contains txAt. This is the
// query the audit trail depends on and the one the pre-Phase-1 code could not
// answer, because the old text was overwritten.
func clauseTextAsOfTx(t *testing.T, st *Store, id, txAt string) string {
	t.Helper()
	var text string
	err := st.DB().QueryRow(`
		SELECT text FROM clause
		WHERE id = ? AND tx_from <= ? AND (tx_to IS NULL OR tx_to > ?)`,
		id, txAt, txAt).Scan(&text)
	if err != nil {
		t.Fatalf("clause %q as-of tx %s: %v", id, txAt, err)
	}
	return text
}

// TestSupersedeAndInsert_PreservesPriorText is the regression guard for the
// "history is overwritten, not versioned" defect: after superseding a clause,
// a query as-of a system time BEFORE the supersession must still return the
// original text.
func TestSupersedeAndInsert_PreservesPriorText(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	const circ = "TEST/VER/2024"
	seedTree(t, st, circ)

	id := domain.ClauseID(circ, "1.1")
	before := domain.RFC3339UTC(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))
	at := domain.RFC3339UTC(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC))
	after := domain.RFC3339UTC(time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC))

	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	next := domain.Clause{
		ID: id, CircularID: circ, ClauseRef: "1.1", ParentID: domain.ClauseID(circ, "1"),
		Heading: "One-A", Text: "AMENDED child text", Ordinal: 2,
		Temporal: domain.Temporal{
			ValidFrom: domain.RFC3339UTC(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC)),
		},
	}
	if err := st.SupersedeAndInsert(ctx, tx, "clause", id, next, at); err != nil {
		t.Fatalf("SupersedeAndInsert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got := clauseTextAsOfTx(t, st, id, before); got != "child" {
		t.Errorf("as-of %s: text = %q, want the PRE-amendment text %q", before, got, "child")
	}
	if got := clauseTextAsOfTx(t, st, id, after); got != "AMENDED child text" {
		t.Errorf("as-of %s: text = %q, want the amended text", after, got)
	}

	// Exactly one row is current, and it is the new version - the property the
	// partial unique index enforces and every `tx_to IS NULL` query relies on.
	var current int
	if err := st.DB().QueryRow(
		`SELECT COUNT(*) FROM clause WHERE id = ? AND tx_to IS NULL`, id).Scan(&current); err != nil {
		t.Fatalf("count current: %v", err)
	}
	if current != 1 {
		t.Fatalf("current versions = %d, want exactly 1", current)
	}
	var versions int
	if err := st.DB().QueryRow(
		`SELECT COUNT(*) FROM clause WHERE id = ?`, id).Scan(&versions); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if versions != 2 {
		t.Fatalf("total versions = %d, want 2 (old bounded + new open)", versions)
	}
}

// TestSupersedeAndInsert_ObligationVersioned covers the obligation table, which
// carries the mandatory-provenance invariant on every version.
func TestSupersedeAndInsert_ObligationVersioned(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	const circ = "TEST/VER/OBL"
	seedTree(t, st, circ)

	vf := domain.RFC3339UTC(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC))
	ob := domain.Obligation{
		ID: "obl-1", ClauseID: domain.ClauseID(circ, "1.1"), Bearer: "investment_adviser",
		DeonticType: domain.DeonticMust, SourceClauseRef: "1.1", SourceSentence: "child",
		Confidence: 0.9, Status: domain.StatusPending,
		Temporal: domain.Temporal{ValidFrom: vf, TxFrom: vf},
	}
	if err := st.UpsertObligation(ctx, ob); err != nil {
		t.Fatalf("UpsertObligation: %v", err)
	}

	at := domain.RFC3339UTC(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC))
	next := ob
	next.DeonticType = domain.DeonticMustNot
	next.TxTo = ""

	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := st.SupersedeAndInsert(ctx, tx, "obligation", ob.ID, next, at); err != nil {
		t.Fatalf("SupersedeAndInsert obligation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var deontic string
	if err := st.DB().QueryRow(
		`SELECT deontic_type FROM obligation WHERE id = ? AND tx_to IS NULL`, ob.ID).Scan(&deontic); err != nil {
		t.Fatalf("read current: %v", err)
	}
	if deontic != string(domain.DeonticMustNot) {
		t.Errorf("current deontic = %q, want MUST_NOT", deontic)
	}
	var old string
	if err := st.DB().QueryRow(
		`SELECT deontic_type FROM obligation WHERE id = ? AND tx_to = ?`, ob.ID, at).Scan(&old); err != nil {
		t.Fatalf("read superseded: %v", err)
	}
	if old != string(domain.DeonticMust) {
		t.Errorf("superseded deontic = %q, want the original MUST", old)
	}
}

// TestSupersedeAndInsert_Rejections covers the caller-bug cases: superseding a
// fact with no current row must be an explicit error rather than a silent
// insert, and an unknown table name must never reach SQL.
func TestSupersedeAndInsert_Rejections(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	const circ = "TEST/VER/REJ"
	seedTree(t, st, circ)

	at := domain.RFC3339UTC(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC))

	withTx := func(fn func(tx *sql.Tx) error) error {
		tx, err := st.DB().BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback() }()
		return fn(tx)
	}

	err := withTx(func(tx *sql.Tx) error {
		return st.SupersedeAndInsert(ctx, tx, "clause", "no-such-clause",
			domain.Clause{ID: "no-such-clause", CircularID: circ, ClauseRef: "9", Text: "x"}, at)
	})
	if err == nil {
		t.Error("superseding a nonexistent id must error, not fall back to a plain insert")
	}

	err = withTx(func(tx *sql.Tx) error {
		return st.SupersedeAndInsert(ctx, tx, "signoff; DROP TABLE clause", "x", domain.Clause{}, at)
	})
	if err == nil {
		t.Error("an unversioned/unknown table name must be rejected")
	}

	err = withTx(func(tx *sql.Tx) error {
		return st.SupersedeAndInsert(ctx, tx, "clause", domain.ClauseID(circ, "1.1"),
			domain.Obligation{}, at)
	})
	if err == nil {
		t.Error("a next value of the wrong type must be rejected")
	}
}

// TestRebuiltTablesKeepEveryColumn guards a failure mode that unit tests
// otherwise miss entirely: 0007 REBUILDS circular/clause/obligation, and a
// rebuild only carries the columns its CREATE statement names. `embedding_json`
// was added to obligation by 0003 via ALTER TABLE, so it is absent from the
// original CREATE and is exactly the kind of column a rebuild drops silently -
// the schema still looks right, and the failure only surfaces at the first write.
func TestRebuiltTablesKeepEveryColumn(t *testing.T) {
	st := newTestStore(t)

	want := map[string][]string{
		"circular":    {"row_uid", "id", "title", "regulator", "issued_on", "source_url", "valid_from", "valid_to", "tx_from", "tx_to"},
		"clause":      {"row_uid", "id", "circular_id", "clause_ref", "parent_id", "heading", "text", "ordinal", "valid_from", "valid_to", "tx_from", "tx_to"},
		"obligation":  {"row_uid", "id", "clause_id", "bearer", "deontic_type", "condition", "threshold_json", "deadline", "penalty", "source_clause_ref", "source_sentence", "confidence", "status", "embedding_json", "valid_from", "valid_to", "tx_from", "tx_to"},
		"ticket":      {"id", "obligation_id", "clause_ref", "title", "detail", "owner", "deadline", "citation", "state", "valid_from", "valid_to", "tx_from", "tx_to"},
		"signoff":     {"id", "obligation_id", "action", "obligation_hash", "signature", "public_key", "signed_by", "justification", "created_at", "valid_from", "valid_to", "tx_from", "tx_to"},
		"policy":      {"id", "obligation_id", "package_name", "rego", "stage", "compiled_at", "valid_from", "valid_to", "tx_from", "tx_to"},
		"policy_eval": {"id", "policy_id", "obligation_id", "input_json", "compliant", "applicable", "deny_json", "stage", "blocked", "trace", "created_at", "valid_from", "valid_to", "tx_from", "tx_to"},
	}

	for table, cols := range want {
		have := map[string]bool{}
		rows, err := st.DB().Query(`SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			t.Fatalf("pragma_table_info(%s): %v", table, err)
		}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("scan column of %s: %v", table, err)
			}
			have[name] = true
		}
		_ = rows.Close()

		for _, c := range cols {
			if !have[c] {
				t.Errorf("table %s lost column %q in the 0007 rebuild", table, c)
			}
		}
	}
}

// TestEmbeddingSurvivesRebuild is the behavioural half of the check above: the
// column must not just exist, it must still be writable and readable through the
// store method that uses it.
func TestEmbeddingSurvivesRebuild(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	const circ = "TEST/VER/EMB"
	seedTree(t, st, circ)

	vf := domain.RFC3339UTC(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC))
	ob := domain.Obligation{
		ID: "obl-emb", ClauseID: domain.ClauseID(circ, "1.1"), Bearer: "investment_adviser",
		DeonticType: domain.DeonticMust, SourceClauseRef: "1.1", SourceSentence: "child",
		Confidence: 0.9, Status: domain.StatusPending,
		Temporal: domain.Temporal{ValidFrom: vf, TxFrom: vf},
	}
	if err := st.UpsertObligation(ctx, ob); err != nil {
		t.Fatalf("UpsertObligation: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE obligation SET embedding_json = ? WHERE id = ? AND tx_to IS NULL`,
		"[0.1,0.2]", ob.ID); err != nil {
		t.Fatalf("write embedding: %v", err)
	}
	var got string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT embedding_json FROM obligation WHERE id = ? AND tx_to IS NULL`, ob.ID).Scan(&got); err != nil {
		t.Fatalf("read embedding: %v", err)
	}
	if got != "[0.1,0.2]" {
		t.Errorf("embedding_json = %q", got)
	}
}

// TestCurrentRowUniqueness proves the partial unique index actually enforces
// "at most one current version per logical id".
//
// It also documents a deliberate schema divergence from the Phase 1 prompt:
// SQLite requires a foreign key's parent columns to be covered by a NON-partial
// unique index, so `UNIQUE(id) WHERE tx_to IS NULL` cannot back a foreign key
// (an INSERT into a child fails with "foreign key mismatch"). The prompt's
// suggested fallback - a full UNIQUE(id) - would forbid a second version and
// defeat versioning entirely, so 0007 drops those FK clauses instead and relies
// on this index plus store-level validation.
func TestCurrentRowUniqueness(t *testing.T) {
	st := newTestStore(t)
	const circ = "TEST/VER/UNIQ"
	seedTree(t, st, circ)

	id := domain.ClauseID(circ, "1.1")
	_, err := st.DB().Exec(`
		INSERT INTO clause (row_uid, id, circular_id, clause_ref, parent_id, heading, text,
		                    ordinal, valid_from, valid_to, tx_from, tx_to)
		VALUES ('dup', ?, ?, '1.1', NULL, NULL, 'second open version', 2,
		        '2024-05-15T00:00:00Z', NULL, '2024-09-01T00:00:00Z', NULL)`, id, circ)
	if err == nil {
		t.Fatal("inserting a second row with tx_to IS NULL for the same id must violate the partial unique index")
	}
}
