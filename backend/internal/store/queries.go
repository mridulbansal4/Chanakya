package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// ObligationQuery filters an as-of obligation listing. Empty filter fields are
// ignored. AsOf is the world-time reconstruction point.
type ObligationQuery struct {
	AsOf    time.Time
	Bearer  string
	Deontic string
	Status  string
}

// ObligationView is a read model for the register: an obligation joined with
// its clause context. JSON-tagged for direct API serialization.
type ObligationView struct {
	ID              string          `json:"id"`
	ClauseID        string          `json:"clause_id"`
	ClauseRef       string          `json:"clause_ref"`
	ClauseHeading   string          `json:"clause_heading"`
	Bearer          string          `json:"bearer"`
	DeonticType     string          `json:"deontic_type"`
	Condition       string          `json:"condition"`
	Threshold       json.RawMessage `json:"threshold"`
	Deadline        string          `json:"deadline"`
	Penalty         string          `json:"penalty"`
	Status          string          `json:"status"`
	Confidence      float64         `json:"confidence"`
	SourceClauseRef string          `json:"source_clause_ref"`
	SourceSentence  string          `json:"source_sentence"`
	ValidFrom       string          `json:"valid_from"`
	ValidTo         string          `json:"valid_to,omitempty"`
}

// ObligationDetail is a single obligation with its full clause text, for the
// detail view + reasoning chain.
type ObligationDetail struct {
	ObligationView
	ClauseText string `json:"clause_text"`
}

const obligationViewCols = `
	o.id, o.clause_id, c.clause_ref, COALESCE(c.heading, ''), o.bearer,
	o.deontic_type, COALESCE(o.condition, ''), o.threshold_json,
	COALESCE(o.deadline, ''), COALESCE(o.penalty, ''), o.status, o.confidence,
	o.source_clause_ref, o.source_sentence, o.valid_from, COALESCE(o.valid_to, '')`

// scanObligationView scans a joined obligation+clause row.
func scanObligationView(rs interface{ Scan(...any) error }) (ObligationView, error) {
	var (
		v         ObligationView
		threshold string
	)
	if err := rs.Scan(
		&v.ID, &v.ClauseID, &v.ClauseRef, &v.ClauseHeading, &v.Bearer,
		&v.DeonticType, &v.Condition, &threshold, &v.Deadline, &v.Penalty,
		&v.Status, &v.Confidence, &v.SourceClauseRef, &v.SourceSentence,
		&v.ValidFrom, &v.ValidTo,
	); err != nil {
		return ObligationView{}, err
	}
	if threshold == "" {
		threshold = "{}"
	}
	v.Threshold = json.RawMessage(threshold)
	return v, nil
}

// ListObligations returns obligations in force as-of the query time (world +
// system time), joined to clause context, with optional filters. All filter
// values are bound as parameters.
func (s *Store) ListObligations(ctx context.Context, q ObligationQuery) ([]ObligationView, error) {
	at := domain.RFC3339UTC(q.AsOf)
	sqlStr := `
		SELECT ` + obligationViewCols + `
		FROM obligation o
		JOIN clause c ON c.id = o.clause_id
		WHERE o.valid_from <= ? AND (o.valid_to IS NULL OR o.valid_to > ?)
		  AND o.tx_to IS NULL`
	args := []any{at, at}
	if q.Bearer != "" {
		sqlStr += " AND o.bearer = ?"
		args = append(args, q.Bearer)
	}
	if q.Deontic != "" {
		sqlStr += " AND o.deontic_type = ?"
		args = append(args, q.Deontic)
	}
	if q.Status != "" {
		sqlStr += " AND o.status = ?"
		args = append(args, q.Status)
	}
	sqlStr += " ORDER BY c.ordinal, o.deontic_type"

	rows, err := s.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("list obligations: %w", err)
	}
	defer rows.Close()

	out := []ObligationView{}
	for rows.Next() {
		v, err := scanObligationView(rows)
		if err != nil {
			return nil, fmt.Errorf("scan obligation view: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate obligation views: %w", err)
	}
	return out, nil
}

// GetObligation returns one obligation (current system time) with clause text.
func (s *Store) GetObligation(ctx context.Context, id string) (ObligationDetail, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+obligationViewCols+`, c.text
		FROM obligation o
		JOIN clause c ON c.id = o.clause_id
		WHERE o.id = ? AND o.tx_to IS NULL`, id)

	var (
		d         ObligationDetail
		threshold string
	)
	if err := row.Scan(
		&d.ID, &d.ClauseID, &d.ClauseRef, &d.ClauseHeading, &d.Bearer,
		&d.DeonticType, &d.Condition, &threshold, &d.Deadline, &d.Penalty,
		&d.Status, &d.Confidence, &d.SourceClauseRef, &d.SourceSentence,
		&d.ValidFrom, &d.ValidTo, &d.ClauseText,
	); err != nil {
		if err == sql.ErrNoRows {
			return ObligationDetail{}, ErrNotFound
		}
		return ObligationDetail{}, fmt.Errorf("get obligation %q: %w", id, err)
	}
	if threshold == "" {
		threshold = "{}"
	}
	d.Threshold = json.RawMessage(threshold)
	return d, nil
}

// Posture is the top-of-screen compliance posture as-of a date.
type Posture struct {
	AsOf               string `json:"as_of"`
	ObligationsInForce int    `json:"obligations_in_force"`
	Pending            int    `json:"pending"`
	NeedsReview        int    `json:"needs_review"`
	Approved           int    `json:"approved"`
	Gaps               int    `json:"gaps"`             // Phase 5
	PendingSignoffs    int    `json:"pending_signoffs"` // pending + needs_review awaiting sign-off
}

// PostureAsOf computes posture counts over obligations in force as-of a date.
func (s *Store) PostureAsOf(ctx context.Context, asOf time.Time) (Posture, error) {
	at := domain.RFC3339UTC(asOf)
	p := Posture{AsOf: at}
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM obligation
		WHERE valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL
		GROUP BY status`, at, at)
	if err != nil {
		return Posture{}, fmt.Errorf("posture query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return Posture{}, fmt.Errorf("scan posture: %w", err)
		}
		switch status {
		case string(domain.StatusPending):
			p.Pending = n
		case string(domain.StatusNeedsReview):
			p.NeedsReview = n
		case string(domain.StatusApproved):
			p.Approved = n
		}
	}
	if err := rows.Err(); err != nil {
		return Posture{}, fmt.Errorf("iterate posture: %w", err)
	}
	// "In force" excludes rejected extractions, so the total equals the sum of
	// the live statuses.
	p.ObligationsInForce = p.Pending + p.NeedsReview + p.Approved
	p.PendingSignoffs = p.Pending + p.NeedsReview

	// Gaps: obligations with no satisfying evidence path (same as the Evidence
	// screen), so the Overview posture doesn't misreport the firm as gap-free.
	em, err := s.EvidenceMap(ctx, asOf)
	if err != nil {
		return Posture{}, fmt.Errorf("posture gaps: %w", err)
	}
	p.Gaps = em.Gaps
	return p, nil
}

// GraphNode / GraphEdge / Graph form the React Flow payload for the overview.
type GraphNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "clause" | "obligation"
	Label    string `json:"label"`
	Sublabel string `json:"sublabel,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Status   string `json:"status,omitempty"`
	Deontic  string `json:"deontic,omitempty"`
}

type GraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"` // "clause_parent" | "clause_obligation"
}

type Graph struct {
	AsOf  string      `json:"as_of"`
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphAsOf builds the clause-tree + obligation graph in force as-of a date for
// a circular. Clause→clause (parent) and clause→obligation edges are returned.
func (s *Store) GraphAsOf(ctx context.Context, circularID string, asOf time.Time) (Graph, error) {
	at := domain.RFC3339UTC(asOf)
	g := Graph{AsOf: at, Nodes: []GraphNode{}, Edges: []GraphEdge{}}

	// Clause nodes + parent edges.
	crows, err := s.db.QueryContext(ctx, `
		SELECT id, clause_ref, COALESCE(heading, ''), COALESCE(parent_id, '')
		FROM clause
		WHERE circular_id = ?
		  AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL
		ORDER BY ordinal`, circularID, at, at)
	if err != nil {
		return Graph{}, fmt.Errorf("graph clauses: %w", err)
	}
	defer crows.Close()
	for crows.Next() {
		var id, ref, heading, parent string
		if err := crows.Scan(&id, &ref, &heading, &parent); err != nil {
			return Graph{}, fmt.Errorf("scan graph clause: %w", err)
		}
		g.Nodes = append(g.Nodes, GraphNode{
			ID: id, Type: "clause", Label: ref, Sublabel: heading, Ref: ref,
		})
		if parent != "" {
			g.Edges = append(g.Edges, GraphEdge{
				ID: "e:" + parent + "->" + id, Source: parent, Target: id, Kind: "clause_parent",
			})
		}
	}
	if err := crows.Err(); err != nil {
		return Graph{}, fmt.Errorf("iterate graph clauses: %w", err)
	}

	// Obligation nodes + clause→obligation edges.
	orows, err := s.db.QueryContext(ctx, `
		SELECT o.id, o.clause_id, o.deontic_type, o.status, c.clause_ref
		FROM obligation o
		JOIN clause c ON c.id = o.clause_id
		WHERE c.circular_id = ?
		  AND o.valid_from <= ? AND (o.valid_to IS NULL OR o.valid_to > ?) AND o.tx_to IS NULL
		ORDER BY c.ordinal`, circularID, at, at)
	if err != nil {
		return Graph{}, fmt.Errorf("graph obligations: %w", err)
	}
	defer orows.Close()
	for orows.Next() {
		var id, clauseID, deontic, status, ref string
		if err := orows.Scan(&id, &clauseID, &deontic, &status, &ref); err != nil {
			return Graph{}, fmt.Errorf("scan graph obligation: %w", err)
		}
		g.Nodes = append(g.Nodes, GraphNode{
			ID: id, Type: "obligation", Label: deontic, Sublabel: ref,
			Status: status, Deontic: deontic, Ref: ref,
		})
		g.Edges = append(g.Edges, GraphEdge{
			ID: "e:" + clauseID + "->" + id, Source: clauseID, Target: id, Kind: "clause_obligation",
		})
	}
	if err := orows.Err(); err != nil {
		return Graph{}, fmt.Errorf("iterate graph obligations: %w", err)
	}
	return g, nil
}

// FirstCircularID returns the id of the (currently single) seeded circular.
func (s *Store) FirstCircularID(ctx context.Context) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM circular WHERE tx_to IS NULL ORDER BY issued_on LIMIT 1`).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("first circular: %w", err)
	}
	return id, nil
}
