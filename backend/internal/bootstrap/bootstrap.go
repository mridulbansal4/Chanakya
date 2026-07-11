// Package bootstrap holds the shared seed + compile pipeline used by the seed
// and compile commands and by the API's self-bootstrap on first run. Extracting
// it here means `go run ./backend/cmd/api` alone yields a fully-seeded app —
// the two documented run commands (api + web) need no manual seed step.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chanakya/internal/compiler"
	"chanakya/internal/fixtures"
	"chanakya/internal/llm"
	"chanakya/internal/store"
	"chanakya/internal/vec"
)

// SeedResult summarises a seed run.
type SeedResult struct {
	CircularID string
	EntityID   string
	Clauses    int
}

// Seed loads the embedded IA Master Circular fixture (circular, entity, clause
// tree) into the store. Idempotent.
func Seed(ctx context.Context, st *store.Store, now time.Time) (SeedResult, error) {
	data, err := fixtures.LoadIACircular(now)
	if err != nil {
		return SeedResult{}, fmt.Errorf("load ia fixture: %w", err)
	}
	if err := st.UpsertCircular(ctx, data.Circular); err != nil {
		return SeedResult{}, fmt.Errorf("seed circular: %w", err)
	}
	if err := st.UpsertEntity(ctx, data.Entity); err != nil {
		return SeedResult{}, fmt.Errorf("seed entity: %w", err)
	}
	for _, c := range data.Clauses {
		if err := st.UpsertClause(ctx, c); err != nil {
			return SeedResult{}, fmt.Errorf("seed clause %q: %w", c.ClauseRef, err)
		}
	}
	n, err := st.CountClauses(ctx, data.Circular.ID)
	if err != nil {
		return SeedResult{}, fmt.Errorf("count clauses: %w", err)
	}
	return SeedResult{CircularID: data.Circular.ID, EntityID: data.Entity.ID, Clauses: n}, nil
}

// CompileResult summarises a compile run.
type CompileResult struct {
	Extractor   string
	Created     int
	Pending     int
	NeedsReview int
	Rejected    int
	Controls    int
	Evidence    int
	Links       int
	Satisfied   int
	Gaps        int
	Tickets     int
}

// Compile runs the Regulation Compiler over the seeded clauses (extract → schema
// validate → cite → store, with embeddings), wires the firm controls/evidence
// layer, detects gaps and drafts remediation tickets. Idempotent.
func Compile(ctx context.Context, st *store.Store, extractor llm.Extractor, now time.Time) (CompileResult, error) {
	comp, err := compiler.New(extractor, compiler.DefaultReviewThreshold)
	if err != nil {
		return CompileResult{}, fmt.Errorf("build compiler: %w", err)
	}
	res := CompileResult{Extractor: comp.ExtractorName()}

	fx, err := fixtures.LoadIACircular(now)
	if err != nil {
		return CompileResult{}, fmt.Errorf("load fixture id: %w", err)
	}
	circularID := fx.Circular.ID

	clauses, err := st.ListClauses(ctx, circularID, now)
	if err != nil {
		return CompileResult{}, fmt.Errorf("list clauses: %w", err)
	}
	for _, cl := range clauses {
		cr, err := comp.CompileClause(ctx, cl)
		if err != nil {
			return CompileResult{}, fmt.Errorf("compile clause %s: %w", cl.ClauseRef, err)
		}
		for _, ob := range cr.Obligations {
			if err := st.UpsertObligation(ctx, ob); err != nil {
				return CompileResult{}, fmt.Errorf("store obligation from clause %s: %w", cl.ClauseRef, err)
			}
			emb, err := vec.Marshal(vec.Embed(ob.SourceSentence))
			if err != nil {
				return CompileResult{}, fmt.Errorf("embed obligation %s: %w", ob.ID, err)
			}
			if err := st.SetObligationEmbedding(ctx, ob.ID, emb); err != nil {
				return CompileResult{}, fmt.Errorf("store embedding %s: %w", ob.ID, err)
			}
			res.Created++
			if ob.Status == "pending" {
				res.Pending++
			} else {
				res.NeedsReview++
			}
		}
		res.Rejected += len(cr.Rejections)
	}

	if err := wireControls(ctx, st, fx.Circular.ValidFrom, &res, now); err != nil {
		return CompileResult{}, fmt.Errorf("wire controls: %w", err)
	}

	em, err := st.EvidenceMap(ctx, now)
	if err != nil {
		return CompileResult{}, fmt.Errorf("evidence map: %w", err)
	}
	res.Satisfied, res.Gaps = em.Satisfied, em.Gaps
	drafted, err := st.GenerateDraftTickets(ctx, now)
	if err != nil {
		return CompileResult{}, fmt.Errorf("generate tickets: %w", err)
	}
	res.Tickets = drafted
	return res, nil
}

// wireControls upserts the controls/evidence fixture and links obligations to
// controls (by clause coverage) and controls to evidence.
func wireControls(ctx context.Context, st *store.Store, validFrom string, res *CompileResult, now time.Time) error {
	w, err := fixtures.LoadControlWiring(now, now)
	if err != nil {
		return fmt.Errorf("load control wiring: %w", err)
	}
	txNow := now.UTC().Format(time.RFC3339)

	for _, e := range w.Evidence {
		e.ValidFrom, e.TxFrom = validFrom, txNow
		if err := st.UpsertEvidence(ctx, e); err != nil {
			return err
		}
	}
	for _, c := range w.Controls {
		c.ValidFrom, c.TxFrom = validFrom, txNow
		if err := st.UpsertControl(ctx, c); err != nil {
			return err
		}
		for _, evID := range w.ControlEvidence[c.ID] {
			if err := st.UpsertControlEvidence(ctx, c.ID, evID, validFrom, txNow); err != nil {
				return err
			}
		}
	}
	res.Controls = len(w.Controls)
	res.Evidence = len(w.Evidence)

	obls, err := st.ListObligations(ctx, store.ObligationQuery{AsOf: now})
	if err != nil {
		return fmt.Errorf("list obligations for wiring: %w", err)
	}
	byRef := map[string][]string{}
	for _, o := range obls {
		byRef[o.ClauseRef] = append(byRef[o.ClauseRef], o.ID)
	}
	for _, c := range w.Controls {
		for _, ref := range w.CoversClauses[c.ID] {
			for _, oblID := range byRef[ref] {
				if err := st.UpsertObligationControl(ctx, oblID, c.ID, validFrom, txNow); err != nil {
					return err
				}
				res.Links++
			}
		}
	}
	return nil
}

// EnsureSeeded seeds + compiles the fixture only if the store is empty (no
// circular). Returns whether it bootstrapped. Uses the deterministic offline
// extractor so startup needs no API key.
func EnsureSeeded(ctx context.Context, st *store.Store) (bool, error) {
	if _, err := st.FirstCircularID(ctx); err == nil {
		return false, nil // already seeded
	} else if !errors.Is(err, store.ErrNotFound) {
		return false, fmt.Errorf("check seeded: %w", err)
	}
	now := time.Now()
	if _, err := Seed(ctx, st, now); err != nil {
		return false, err
	}
	if _, err := Compile(ctx, st, llm.NewOfflineExtractor(), now); err != nil {
		return false, err
	}
	return true, nil
}
