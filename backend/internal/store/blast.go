package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/vec"
)

// BlastNode is a node in the blast-radius graph. Kind classifies why it is
// affected: "amended" (the edited clause), "direct" (obligation on that clause),
// "semantic" (obligation elsewhere matched by cosine similarity), "control",
// "evidence".
type BlastNode struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"` // clause | obligation | control | evidence
	Label      string  `json:"label"`
	Sublabel   string  `json:"sublabel,omitempty"`
	Ref        string  `json:"ref,omitempty"`
	Status     string  `json:"status,omitempty"`
	Deontic    string  `json:"deontic,omitempty"`
	Kind       string  `json:"kind"`
	Layer      int     `json:"layer"`
	Similarity float64 `json:"similarity,omitempty"`
}

// BlastEdge connects blast-radius nodes.
type BlastEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"` // clause_obligation | semantic | obligation_control | control_evidence
}

// BlastChange is one item of work the amendment creates.
type BlastChange struct {
	Category string `json:"category"` // obligation | control | evidence
	Ref      string `json:"ref,omitempty"`
	Detail   string `json:"detail"`
}

// BlastRadius is the full payload for the amendment screen.
type BlastRadius struct {
	AsOf        string        `json:"as_of"`
	ClauseRef   string        `json:"clause_ref"`
	AmendedText string        `json:"amended_text"`
	Threshold   float64       `json:"threshold"`
	Nodes       []BlastNode   `json:"nodes"`
	Edges       []BlastEdge   `json:"edges"`
	Changes     []BlastChange `json:"changes"`
	Summary     struct {
		Obligations int `json:"obligations"`
		Controls    int `json:"controls"`
		Evidence    int `json:"evidence"`
	} `json:"summary"`
}

// BlastRadius computes the downstream impact of amending clause clauseRef with
// amendedText, WITHOUT persisting anything (a what-if preview). It cosine-diffs
// the amended text against every obligation's stored embedding to find
// semantically-affected obligations, unions them with the obligations directly
// on the clause, then traverses obligation→control→evidence.
func (s *Store) BlastRadius(ctx context.Context, circularID, clauseRef, amendedText string, threshold float64, asOf time.Time) (BlastRadius, error) {
	at := domain.RFC3339UTC(asOf)
	br := BlastRadius{
		AsOf: at, ClauseRef: clauseRef, AmendedText: amendedText, Threshold: threshold,
		Nodes: []BlastNode{}, Edges: []BlastEdge{}, Changes: []BlastChange{},
	}

	// Resolve the amended clause.
	var clauseID, clauseHeading string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(heading, '') FROM clause
		WHERE circular_id = ? AND clause_ref = ?
		  AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL`,
		circularID, clauseRef, at, at).Scan(&clauseID, &clauseHeading)
	if err != nil {
		if err == sql.ErrNoRows {
			return BlastRadius{}, ErrNotFound
		}
		return BlastRadius{}, fmt.Errorf("resolve amended clause %q: %w", clauseRef, err)
	}

	amendedVec := vec.Embed(amendedText)

	// Load all current obligations with their embeddings and clause context.
	type obl struct {
		id, clauseID, ref, deontic, status string
		sim                                float64
		direct                             bool
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, o.clause_id, c.clause_ref, o.deontic_type, o.status,
		       o.embedding_json, o.source_sentence
		FROM obligation o JOIN clause c ON c.id = o.clause_id
		WHERE o.valid_from <= ? AND (o.valid_to IS NULL OR o.valid_to > ?) AND o.tx_to IS NULL`,
		at, at)
	if err != nil {
		return BlastRadius{}, fmt.Errorf("blast obligations: %w", err)
	}
	defer rows.Close()

	affectedObl := map[string]obl{}
	for rows.Next() {
		var o obl
		var embJSON, sentence string
		if err := rows.Scan(&o.id, &o.clauseID, &o.ref, &o.deontic, &o.status, &embJSON, &sentence); err != nil {
			return BlastRadius{}, fmt.Errorf("scan blast obligation: %w", err)
		}
		emb, err := vec.Unmarshal(embJSON)
		if err != nil {
			return BlastRadius{}, fmt.Errorf("obligation %q embedding: %w", o.id, err)
		}
		if emb == nil {
			emb = vec.Embed(sentence) // fallback if not yet embedded
		}
		o.sim = vec.Cosine(amendedVec, emb)
		o.direct = o.clauseID == clauseID
		if o.direct || o.sim >= threshold {
			affectedObl[o.id] = o
		}
	}
	if err := rows.Err(); err != nil {
		return BlastRadius{}, fmt.Errorf("iterate blast obligations: %w", err)
	}

	// Amended clause node (layer 0).
	br.Nodes = append(br.Nodes, BlastNode{
		ID: clauseID, Type: "clause", Label: clauseRef, Sublabel: clauseHeading,
		Ref: clauseRef, Kind: "amended", Layer: 0,
	})

	// Obligation nodes + edges (layer 1). Deterministic order for stable output.
	oblIDs := make([]string, 0, len(affectedObl))
	for id := range affectedObl {
		oblIDs = append(oblIDs, id)
	}
	sort.Strings(oblIDs)
	for _, id := range oblIDs {
		o := affectedObl[id]
		kind := "semantic"
		edgeKind := "semantic"
		if o.direct {
			kind, edgeKind = "direct", "clause_obligation"
		}
		br.Nodes = append(br.Nodes, BlastNode{
			ID: o.id, Type: "obligation", Label: o.deontic, Sublabel: o.ref,
			Ref: o.ref, Status: o.status, Deontic: o.deontic, Kind: kind,
			Layer: 1, Similarity: round3(o.sim),
		})
		br.Edges = append(br.Edges, BlastEdge{
			ID: "b:" + clauseID + "->" + o.id, Source: clauseID, Target: o.id, Kind: edgeKind,
		})
	}

	// Traverse obligation → control (only edges from affected obligations).
	affectedCtl := map[string]string{} // controlID -> name
	crows, err := s.db.QueryContext(ctx, `
		SELECT oc.obligation_id, oc.control_id, ctl.name
		FROM obligation_control oc JOIN control ctl ON ctl.id = oc.control_id
		WHERE oc.valid_from <= ? AND (oc.valid_to IS NULL OR oc.valid_to > ?) AND oc.tx_to IS NULL
		  AND ctl.tx_to IS NULL`, at, at)
	if err != nil {
		return BlastRadius{}, fmt.Errorf("blast obligation_control: %w", err)
	}
	defer crows.Close()
	type ocEdge struct{ obl, ctl string }
	var ocEdges []ocEdge
	for crows.Next() {
		var oblID, ctlID, name string
		if err := crows.Scan(&oblID, &ctlID, &name); err != nil {
			return BlastRadius{}, fmt.Errorf("scan obligation_control: %w", err)
		}
		if _, ok := affectedObl[oblID]; !ok {
			continue
		}
		affectedCtl[ctlID] = name
		ocEdges = append(ocEdges, ocEdge{oblID, ctlID})
	}
	if err := crows.Err(); err != nil {
		return BlastRadius{}, fmt.Errorf("iterate obligation_control: %w", err)
	}

	ctlIDs := sortedKeys(affectedCtl)
	for _, id := range ctlIDs {
		br.Nodes = append(br.Nodes, BlastNode{
			ID: id, Type: "control", Label: affectedCtl[id], Kind: "control", Layer: 2,
		})
	}
	for _, e := range ocEdges {
		br.Edges = append(br.Edges, BlastEdge{
			ID: "b:" + e.obl + "->" + e.ctl, Source: e.obl, Target: e.ctl, Kind: "obligation_control",
		})
	}

	// Traverse control → evidence (only from affected controls).
	affectedEv := map[string]string{}
	erows, err := s.db.QueryContext(ctx, `
		SELECT ce.control_id, ce.evidence_id, ev.name
		FROM control_evidence ce JOIN evidence ev ON ev.id = ce.evidence_id
		WHERE ce.valid_from <= ? AND (ce.valid_to IS NULL OR ce.valid_to > ?) AND ce.tx_to IS NULL
		  AND ev.tx_to IS NULL`, at, at)
	if err != nil {
		return BlastRadius{}, fmt.Errorf("blast control_evidence: %w", err)
	}
	defer erows.Close()
	type ceEdge struct{ ctl, ev string }
	var ceEdges []ceEdge
	for erows.Next() {
		var ctlID, evID, name string
		if err := erows.Scan(&ctlID, &evID, &name); err != nil {
			return BlastRadius{}, fmt.Errorf("scan control_evidence: %w", err)
		}
		if _, ok := affectedCtl[ctlID]; !ok {
			continue
		}
		affectedEv[evID] = name
		ceEdges = append(ceEdges, ceEdge{ctlID, evID})
	}
	if err := erows.Err(); err != nil {
		return BlastRadius{}, fmt.Errorf("iterate control_evidence: %w", err)
	}

	evIDs := sortedKeys(affectedEv)
	for _, id := range evIDs {
		br.Nodes = append(br.Nodes, BlastNode{
			ID: id, Type: "evidence", Label: affectedEv[id], Kind: "evidence", Layer: 3,
		})
	}
	for _, e := range ceEdges {
		br.Edges = append(br.Edges, BlastEdge{
			ID: "b:" + e.ctl + "->" + e.ev, Source: e.ctl, Target: e.ev, Kind: "control_evidence",
		})
	}

	// Change list + summary.
	for _, id := range oblIDs {
		o := affectedObl[id]
		how := "directly on the amended clause"
		if !o.direct {
			how = fmt.Sprintf("semantic match (cosine %.2f)", o.sim)
		}
		br.Changes = append(br.Changes, BlastChange{
			Category: "obligation", Ref: o.ref,
			Detail: fmt.Sprintf("Re-review %s obligation on clause %s — %s", o.deontic, o.ref, how),
		})
	}
	for _, id := range ctlIDs {
		br.Changes = append(br.Changes, BlastChange{
			Category: "control", Detail: "Update control: " + affectedCtl[id],
		})
	}
	for _, id := range evIDs {
		br.Changes = append(br.Changes, BlastChange{
			Category: "evidence", Detail: "Re-verify evidence: " + affectedEv[id],
		})
	}
	br.Summary.Obligations = len(oblIDs)
	br.Summary.Controls = len(ctlIDs)
	br.Summary.Evidence = len(evIDs)
	return br, nil
}

func round3(f float64) float64 {
	return float64(int(f*1000+0.5)) / 1000
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
