package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"chanakya/internal/domain"
)

// newTestStore opens a fresh Store backed by a temp-dir SQLite file (migrations
// applied) that is cleaned up automatically.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// seedTree loads a small circular with a two-level clause tree in force from
// 2024-05-15, known to the system from txNow.
func seedTree(t *testing.T, st *Store, circ string) {
	t.Helper()
	ctx := context.Background()
	vf := domain.RFC3339UTC(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC))
	tx := domain.RFC3339UTC(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC))
	tmp := domain.Temporal{ValidFrom: vf, TxFrom: tx}

	if err := st.UpsertCircular(ctx, domain.Circular{
		ID: circ, Title: "Test Circular", Regulator: "SEBI", IssuedOn: vf, Temporal: tmp,
	}); err != nil {
		t.Fatalf("UpsertCircular: %v", err)
	}

	clauses := []domain.Clause{
		{ClauseRef: "1", Heading: "Chapter One", Text: "root one", Ordinal: 1},
		{ClauseRef: "1.1", ParentID: domain.ClauseID(circ, "1"), Heading: "One-A", Text: "child", Ordinal: 2},
		{ClauseRef: "1.2", ParentID: domain.ClauseID(circ, "1"), Heading: "One-B", Text: "child", Ordinal: 3},
		{ClauseRef: "2", Heading: "Chapter Two", Text: "root two", Ordinal: 4},
		{ClauseRef: "2.1", ParentID: domain.ClauseID(circ, "2"), Heading: "Two-A", Text: "child", Ordinal: 5},
	}
	for _, c := range clauses {
		c.ID = domain.ClauseID(circ, c.ClauseRef)
		c.CircularID = circ
		c.Temporal = tmp
		if err := st.UpsertClause(ctx, c); err != nil {
			t.Fatalf("UpsertClause %s: %v", c.ClauseRef, err)
		}
	}
}

func TestMigrationsAndPragmas(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}

	var fk int
	if err := st.DB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("pragma foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}

	var phase string
	if err := st.DB().QueryRowContext(ctx,
		"SELECT value FROM app_meta WHERE key='schema_phase'").Scan(&phase); err != nil {
		t.Fatalf("read schema_phase: %v", err)
	}
	if phase != "7" {
		// Phase 8 added no migration (lineage + feed are read-only queries).
		t.Errorf("schema_phase = %q, want \"7\"", phase)
	}
}

func TestGetClauseSubtree(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	const circ = "TEST/1"
	seedTree(t, st, circ)
	asOf := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		root      string
		wantRefs  []string // in traversal order
		wantDepth map[string]int
	}{
		{
			name:      "chapter one subtree",
			root:      domain.ClauseID(circ, "1"),
			wantRefs:  []string{"1", "1.1", "1.2"},
			wantDepth: map[string]int{"1": 0, "1.1": 1, "1.2": 1},
		},
		{
			name:      "chapter two subtree",
			root:      domain.ClauseID(circ, "2"),
			wantRefs:  []string{"2", "2.1"},
			wantDepth: map[string]int{"2": 0, "2.1": 1},
		},
		{
			name:     "leaf has only itself",
			root:     domain.ClauseID(circ, "1.1"),
			wantRefs: []string{"1.1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := st.GetClauseSubtree(ctx, tc.root, asOf)
			if err != nil {
				t.Fatalf("GetClauseSubtree: %v", err)
			}
			gotRefs := make([]string, len(nodes))
			for i, n := range nodes {
				gotRefs[i] = n.ClauseRef
			}
			if len(gotRefs) != len(tc.wantRefs) {
				t.Fatalf("refs = %v, want %v", gotRefs, tc.wantRefs)
			}
			for i := range gotRefs {
				if gotRefs[i] != tc.wantRefs[i] {
					t.Errorf("order: refs = %v, want %v", gotRefs, tc.wantRefs)
					break
				}
			}
			for _, n := range nodes {
				if want, ok := tc.wantDepth[n.ClauseRef]; ok && n.Depth != want {
					t.Errorf("clause %s depth = %d, want %d", n.ClauseRef, n.Depth, want)
				}
			}
		})
	}
}

// TestAsOfExcludesFutureAndRetired verifies world-time filtering: a clause is
// invisible before its valid_from and after its valid_to.
func TestAsOfExcludesFutureAndRetired(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	const circ = "TEST/2"

	vf := domain.RFC3339UTC(time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC))
	tx := vf
	if err := st.UpsertCircular(ctx, domain.Circular{
		ID: circ, Title: "T", Regulator: "SEBI", IssuedOn: vf,
		Temporal: domain.Temporal{ValidFrom: vf, TxFrom: tx},
	}); err != nil {
		t.Fatalf("UpsertCircular: %v", err)
	}
	// A clause retired (valid_to) on 2024-08-01.
	retiredTo := domain.RFC3339UTC(time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC))
	c := domain.Clause{
		ID: domain.ClauseID(circ, "9"), CircularID: circ, ClauseRef: "9",
		Heading: "Sunset", Text: "temporary", Ordinal: 1,
		Temporal: domain.Temporal{ValidFrom: vf, ValidTo: retiredTo, TxFrom: tx},
	}
	if err := st.UpsertClause(ctx, c); err != nil {
		t.Fatalf("UpsertClause: %v", err)
	}

	cases := []struct {
		name string
		asOf time.Time
		want int
	}{
		{"before valid_from", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 0},
		{"during validity", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), 1},
		{"after valid_to", time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := st.GetClauseSubtree(ctx, c.ID, tc.asOf)
			if err != nil {
				t.Fatalf("GetClauseSubtree: %v", err)
			}
			if len(nodes) != tc.want {
				t.Errorf("as-of %s: got %d nodes, want %d", tc.asOf.Format("2006-01-02"), len(nodes), tc.want)
			}
		})
	}
}

// TestUpsertClauseIdempotent verifies re-seeding does not duplicate rows.
func TestUpsertClauseIdempotent(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	const circ = "TEST/3"
	seedTree(t, st, circ)
	seedTree(t, st, circ) // second load

	n, err := st.CountClauses(ctx, circ)
	if err != nil {
		t.Fatalf("CountClauses: %v", err)
	}
	if n != 5 {
		t.Errorf("clause count after double seed = %d, want 5", n)
	}
}
