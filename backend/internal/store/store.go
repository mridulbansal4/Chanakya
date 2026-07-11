// Package store owns the SQLite database: connection lifecycle, the embedded
// migration runner, and (from Phase 1) parameterized query methods over the
// bi-temporal obligation graph. All SQL uses ? placeholders — never string
// concatenation (rule 4).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"chanakya/db"

	// modernc.org/sqlite is a PURE-GO SQLite driver (no cgo, no gcc): it
	// registers itself under the driver name "sqlite" and works on Windows
	// out of the box. We deliberately do NOT use mattn/go-sqlite3.
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned by query methods when a row does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps a *sql.DB handle to the CHANAKYA SQLite file.
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) the SQLite database at dbPath with WAL mode
// and foreign-key enforcement enabled on every connection, then applies all
// embedded migrations. The returned Store owns the *sql.DB; call Close when done.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	dsn, err := buildDSN(dbPath)
	if err != nil {
		return nil, fmt.Errorf("build dsn for %q: %w", dbPath, err)
	}

	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dbPath, err)
	}

	// SQLite is a single-writer engine. Serialise writes through one connection
	// to avoid spurious "database is locked" errors under the WAL busy_timeout.
	sqldb.SetMaxOpenConns(1)

	if err := sqldb.PingContext(ctx); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("ping sqlite %q: %w", dbPath, err)
	}

	s := &Store{db: sqldb}
	if err := s.migrate(ctx); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return s, nil
}

// buildDSN constructs a modernc.org/sqlite DSN that enables foreign keys, WAL
// journalling, NORMAL synchronous mode, and a busy timeout. The file: URI form
// is required for the _pragma query parameters to take effect.
func buildDSN(dbPath string) (string, error) {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("resolve abs path: %w", err)
	}
	// file: URIs use forward slashes on every platform, including Windows.
	slashed := filepath.ToSlash(abs)

	q := url.Values{}
	// url.Values dedupes by key, so add pragmas via the raw form below instead.
	_ = q

	pragmas := []string{
		"foreign_keys(1)",
		"journal_mode(WAL)",
		"synchronous(NORMAL)",
		"busy_timeout(10000)",
	}
	parts := make([]string, 0, len(pragmas))
	for _, p := range pragmas {
		parts = append(parts, "_pragma="+url.QueryEscape(p))
	}
	return "file:" + slashed + "?" + strings.Join(parts, "&"), nil
}

// DB exposes the underlying *sql.DB for query methods defined in this package.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}
	return nil
}

// Health verifies the database is reachable by issuing a trivial query.
func (s *Store) Health(ctx context.Context) error {
	var one int
	if err := s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmt.Errorf("health query: %w", err)
	}
	if one != 1 {
		return errors.New("health query returned unexpected value")
	}
	return nil
}

// migrate applies every embedded migration not yet recorded in
// schema_migrations, in lexical order, each inside its own transaction.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied, err := s.appliedVersions(ctx)
	if err != nil {
		return fmt.Errorf("read applied versions: %w", err)
	}

	entries, err := fs.ReadDir(db.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if applied[name] {
			continue
		}
		sqlBytes, err := fs.ReadFile(db.Migrations, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := s.applyOne(ctx, name, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

// appliedVersions returns the set of already-applied migration versions.
func (s *Store) appliedVersions(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate versions: %w", err)
	}
	return applied, nil
}

// applyOne runs a single migration's SQL and records it, atomically.
func (s *Store) applyOne(ctx context.Context, name, script string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, script); err != nil {
		return fmt.Errorf("exec script: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
		return fmt.Errorf("record version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
