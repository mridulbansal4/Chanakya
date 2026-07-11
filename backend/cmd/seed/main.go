// Command seed loads the SEBI Investment Advisers Master Circular fixture into
// chanakya.db as a clause tree (plus the regulated entity), then verifies the
// load with a recursive-CTE traversal. Idempotent. No Docker, no external files.
//
//	go run ./backend/cmd/seed
//
// Note: `go run ./backend/cmd/api` self-seeds on first run, so this command is
// only needed to (re)seed explicitly or to inspect the clause tree.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"chanakya/internal/bootstrap"
	"chanakya/internal/config"
	"chanakya/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("seed: fatal: %v", err)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = st.Close() }()

	res, err := bootstrap.Seed(ctx, st, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("seed: loaded circular %q + entity %q\n", res.CircularID, res.EntityID)
	fmt.Printf("seed: %d clauses in force\n\n", res.Clauses)

	// Verify with the recursive-CTE traversal.
	asOf := time.Now()
	roots, err := st.ListTopLevelClauses(ctx, res.CircularID, asOf)
	if err != nil {
		return fmt.Errorf("list top-level clauses: %w", err)
	}
	fmt.Println("seed: clause tree (recursive-CTE traversal, as-of today):")
	total := 0
	for _, rootID := range roots {
		nodes, err := st.GetClauseSubtree(ctx, rootID, asOf)
		if err != nil {
			return fmt.Errorf("traverse subtree %q: %w", rootID, err)
		}
		for _, n := range nodes {
			fmt.Printf("  %s[%s] %s\n", strings.Repeat("  ", n.Depth), n.ClauseRef, n.Heading)
			total++
		}
	}
	fmt.Printf("\nseed: traversal returned %d clause nodes across %d chapters — OK\n", total, len(roots))
	return nil
}
