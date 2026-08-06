package bootstrap

import (
	"context"
	"fmt"
	"time"

	"chanakya/internal/enterprise"
	"chanakya/internal/fixtures"
	"chanakya/internal/store"
)

// enterpriseWorldStart is the world-time date from which the seeded firm's state
// is true. It is the firm's incorporation date, so an as-of query before it
// correctly returns an empty firm rather than a firm that always existed.
var enterpriseWorldStart = time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC)

// SeedEnterprise loads the Alpha Wealth fixture into the enterprise graph.
// Idempotent: every write is an upsert on a deterministic id.
func SeedEnterprise(ctx context.Context, st *store.Store, now time.Time) error {
	fx, err := fixtures.LoadEnterprise(enterpriseWorldStart, now)
	if err != nil {
		return fmt.Errorf("load enterprise fixture: %w", err)
	}
	if err := st.SeedEnterprise(ctx, fx); err != nil {
		return fmt.Errorf("write enterprise fixture: %w", err)
	}
	return nil
}

// ProjectObligations runs the projection layer over every current obligation and
// persists the proposed bindings.
//
// The bindings are INFERENCE and are stored as such (confidence + an unset
// human_confirmed flag). Running this at bootstrap means the /enterprise screen
// has something to show immediately, without any of it being presented as
// asserted fact.
func ProjectObligations(ctx context.Context, st *store.Store, now time.Time) (int, error) {
	obligations, err := st.ListObligations(ctx, store.ObligationQuery{AsOf: now})
	if err != nil {
		return 0, fmt.Errorf("list obligations for projection: %w", err)
	}

	p := enterprise.NewProjector(st)
	validFrom := now.UTC().Format(time.RFC3339)

	total := 0
	for _, ob := range obligations {
		bindings, err := p.Project(ctx, ob.ID, now)
		if err != nil {
			return total, fmt.Errorf("project obligation %q: %w", ob.ID, err)
		}
		if err := p.Persist(ctx, bindings, validFrom, validFrom); err != nil {
			return total, err
		}
		total += len(bindings)
	}
	return total, nil
}
