package store

import (
	"context"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// RegulatoryFeedItem is one circular CHANAKYA knows about, with how it came in
// and what it relates to.
type RegulatoryFeedItem struct {
	CircularID string `json:"circular_id"`
	Title      string `json:"title"`
	Regulator  string `json:"regulator"`
	IssuedOn   string `json:"issued_on"`
	DocKind    string `json:"doc_kind"`
	// Source records how this circular entered the graph: an ingested upload, or
	// the seeded fixture. Saying so is the honest alternative to implying
	// CHANAKYA fetched it from SEBI.
	Source      string                 `json:"source"`
	IngestRunID string                 `json:"ingest_run_id,omitempty"`
	IngestState string                 `json:"ingest_state,omitempty"`
	ApprovedBy  string                 `json:"approved_by,omitempty"`
	ApprovedAt  string                 `json:"approved_at,omitempty"`
	Clauses     int                    `json:"clauses"`
	Obligations int                    `json:"obligations"`
	Relations   []CircularRelationView `json:"relations"`
	Amendment   *ClauseLineageSummary  `json:"amendment,omitempty"`
}

// CircularRelationView is a document-to-document edge.
type CircularRelationView struct {
	Kind       string `json:"kind"`
	ToRef      string `json:"to_ref"`
	ToCircular string `json:"to_circular"`
}

// ClauseLineageSummary rolls up how an amending circular changed the corpus.
type ClauseLineageSummary struct {
	Counts  map[string]int        `json:"counts"`
	Changes []ClauseLineageChange `json:"changes"`
}

// ClauseLineageChange is one recorded classification, with both texts so the
// screen can show the diff that was actually applied.
type ClauseLineageChange struct {
	Relation    string  `json:"relation"`
	Score       float64 `json:"score"`
	NewClauseID string  `json:"new_clause_id"`
	OldClauseID string  `json:"old_clause_id"`
	ClauseRef   string  `json:"clause_ref"`
	NewText     string  `json:"new_text"`
	OldText     string  `json:"old_text"`
}

// RegulatoryFeed returns the real state of CHANAKYA's regulatory corpus: which
// circulars it holds, how each arrived, what they supersede or amend, and what
// an amendment actually changed.
//
// This replaces the scripted client-side simulation the screen used to render.
// Everything here is a query result, so the screen can no longer claim something
// the database does not contain.
func (s *Store) RegulatoryFeedItems(ctx context.Context, asOf time.Time) ([]RegulatoryFeedItem, error) {
	at := domain.RFC3339UTC(asOf)

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.title, c.regulator, c.issued_on,
		       (SELECT COUNT(*) FROM clause cl WHERE cl.circular_id = c.id AND cl.tx_to IS NULL),
		       (SELECT COUNT(*) FROM obligation o
		         JOIN clause cl2 ON cl2.id = o.clause_id AND cl2.tx_to IS NULL
		         WHERE cl2.circular_id = c.id AND o.tx_to IS NULL)
		FROM circular c
		WHERE c.valid_from <= ? AND (c.valid_to IS NULL OR c.valid_to > ?) AND c.tx_to IS NULL
		ORDER BY c.issued_on DESC, c.id`, at, at)
	if err != nil {
		return nil, fmt.Errorf("list circulars as-of %s: %w", at, err)
	}
	defer rows.Close()

	var out []RegulatoryFeedItem
	for rows.Next() {
		var it RegulatoryFeedItem
		if err := rows.Scan(&it.CircularID, &it.Title, &it.Regulator, &it.IssuedOn,
			&it.Clauses, &it.Obligations); err != nil {
			return nil, fmt.Errorf("scan circular: %w", err)
		}
		it.Source = "seeded fixture"
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate circulars: %w", err)
	}

	for i := range out {
		if err := s.attachIngestRun(ctx, &out[i]); err != nil {
			return nil, err
		}
		relations, err := s.circularRelations(ctx, out[i].CircularID)
		if err != nil {
			return nil, err
		}
		out[i].Relations = relations

		// An amendment summary only makes sense for a circular that supersedes
		// or amends something.
		amends := false
		for _, r := range relations {
			if r.Kind == "supersedes" || r.Kind == "amends" {
				amends = true
			}
		}
		if amends {
			summary, err := s.clauseLineageFor(ctx, out[i].CircularID)
			if err != nil {
				return nil, err
			}
			if summary != nil {
				out[i].Amendment = summary
			}
		}
	}
	return out, nil
}

// attachIngestRun records how a circular entered the graph.
func (s *Store) attachIngestRun(ctx context.Context, it *RegulatoryFeedItem) error {
	var (
		runID, state, docKind  string
		approvedBy, approvedAt *string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, state, doc_kind, approved_by, approved_at
		FROM ingest_run WHERE circular_id = ? ORDER BY created_at DESC LIMIT 1`,
		it.CircularID).Scan(&runID, &state, &docKind, &approvedBy, &approvedAt)
	if err != nil {
		// No ingest run: this circular came from the seeded fixture, which the
		// Source field already says.
		return nil
	}
	it.IngestRunID, it.IngestState, it.DocKind = runID, state, docKind
	it.Source = "ingested upload"
	if approvedBy != nil {
		it.ApprovedBy = *approvedBy
	}
	if approvedAt != nil {
		it.ApprovedAt = *approvedAt
	}
	return nil
}

// circularRelations returns a circular's supersedes/amends/references edges.
func (s *Store) circularRelations(ctx context.Context, circularID string) ([]CircularRelationView, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT kind, to_ref, COALESCE(to_circular,'')
		FROM circular_relation
		WHERE from_circular = ? AND tx_to IS NULL
		ORDER BY kind, to_ref`, circularID)
	if err != nil {
		return nil, fmt.Errorf("list relations for %q: %w", circularID, err)
	}
	defer rows.Close()

	out := []CircularRelationView{}
	for rows.Next() {
		var r CircularRelationView
		if err := rows.Scan(&r.Kind, &r.ToRef, &r.ToCircular); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// clauseLineageFor returns the amendment classifications recorded when this
// circular was approved, with both clause texts.
//
// The OLD text comes from the SUPERSEDED clause version - the row whose tx_to is
// set - which is exactly what bi-temporal versioning made retrievable.
func (s *Store) clauseLineageFor(ctx context.Context, circularID string) (*ClauseLineageSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.relation, l.score, l.new_clause_id, COALESCE(l.old_clause_id,''),
		       COALESCE(nc.clause_ref, oc.clause_ref, ''),
		       COALESCE(nc.text,''), COALESCE(oc.text,'')
		FROM clause_lineage l
		LEFT JOIN clause nc ON nc.id = l.new_clause_id AND nc.tx_to IS NULL
		LEFT JOIN clause oc ON oc.id = l.old_clause_id AND oc.tx_to IS NOT NULL
		WHERE l.tx_to IS NULL
		  AND (nc.circular_id = ? OR oc.circular_id = ?)
		ORDER BY l.relation, COALESCE(nc.clause_ref, oc.clause_ref)`,
		circularID, circularID)
	if err != nil {
		return nil, fmt.Errorf("clause lineage for %q: %w", circularID, err)
	}
	defer rows.Close()

	summary := &ClauseLineageSummary{Counts: map[string]int{}, Changes: []ClauseLineageChange{}}
	for rows.Next() {
		var c ClauseLineageChange
		if err := rows.Scan(&c.Relation, &c.Score, &c.NewClauseID, &c.OldClauseID,
			&c.ClauseRef, &c.NewText, &c.OldText); err != nil {
			return nil, fmt.Errorf("scan lineage: %w", err)
		}
		summary.Counts[c.Relation]++
		// Unchanged clauses are counted but not listed: a reviewer wants the diff,
		// not twenty rows saying nothing happened.
		if c.Relation != "unchanged" {
			summary.Changes = append(summary.Changes, c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lineage: %w", err)
	}
	if len(summary.Counts) == 0 {
		return nil, nil
	}
	return summary, nil
}
