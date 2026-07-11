// Package db holds the embedded SQL migrations for CHANAKYA.
//
// Migrations live under db/migrations/*.sql and are applied in-process on
// startup by internal/store. There is no external migration tool, no goose,
// and no Docker: the schema is compiled into the binary via go:embed so the
// operator never manages a database by hand. This is what lets chanakya.db be
// a single self-describing file that any auditor can open.
package db

import "embed"

// Migrations contains every .sql file, applied in lexical (numeric) order.
//
//go:embed migrations/*.sql
var Migrations embed.FS
