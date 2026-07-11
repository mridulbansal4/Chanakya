package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// LineageNode is a node in the reconstructed audit lineage.
type LineageNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // clause | obligation | control | evidence | signoff | policy
	Label    string `json:"label"`
	Sublabel string `json:"sublabel,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Status   string `json:"status,omitempty"`
}

// LineageEdge connects lineage nodes.
type LineageEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

// Lineage is the full clause→obligation→control→evidence→sign-off→policy chain
// reconstructed as-of a date.
type Lineage struct {
	AsOf   string         `json:"as_of"`
	Nodes  []LineageNode  `json:"nodes"`
	Edges  []LineageEdge  `json:"edges"`
	Counts map[string]int `json:"counts"`
}

// Lineage reconstructs the audit lineage in force as-of a date (world time,
// current system knowledge). Because sign-offs and policies become world-time
// facts when they are made, reconstructing as-of a date before they existed
// shows the obligations un-signed and un-enforced — the bi-temporal audit view.
func (s *Store) Lineage(ctx context.Context, circularID string, asOf time.Time) (Lineage, error) {
	at := domain.RFC3339UTC(asOf)
	lin := Lineage{AsOf: at, Nodes: []LineageNode{}, Edges: []LineageEdge{}, Counts: map[string]int{}}

	add := func(n LineageNode) {
		lin.Nodes = append(lin.Nodes, n)
		lin.Counts[n.Type]++
	}
	edge := func(id, src, tgt, kind string) {
		lin.Edges = append(lin.Edges, LineageEdge{ID: id, Source: src, Target: tgt, Kind: kind})
	}

	// 1. Clauses (+ parent edges).
	if err := query(ctx, s.db, `
		SELECT id, clause_ref, COALESCE(heading,''), COALESCE(parent_id,'')
		FROM clause
		WHERE circular_id = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL
		ORDER BY ordinal`,
		[]any{circularID, at, at},
		func(sc scanner) error {
			var id, ref, heading, parent string
			if err := sc.Scan(&id, &ref, &heading, &parent); err != nil {
				return err
			}
			add(LineageNode{ID: id, Type: "clause", Label: ref, Sublabel: heading, Ref: ref})
			if parent != "" {
				edge("le:"+parent+"->"+id, parent, id, "clause_parent")
			}
			return nil
		}); err != nil {
		return Lineage{}, fmt.Errorf("lineage clauses: %w", err)
	}

	// 2. Obligations (+ clause→obligation edges).
	if err := query(ctx, s.db, `
		SELECT o.id, o.clause_id, c.clause_ref, o.deontic_type, o.status
		FROM obligation o JOIN clause c ON c.id = o.clause_id
		WHERE c.circular_id = ? AND o.valid_from <= ? AND (o.valid_to IS NULL OR o.valid_to > ?) AND o.tx_to IS NULL
		ORDER BY c.ordinal`,
		[]any{circularID, at, at},
		func(sc scanner) error {
			var id, clauseID, ref, deontic, status string
			if err := sc.Scan(&id, &clauseID, &ref, &deontic, &status); err != nil {
				return err
			}
			add(LineageNode{ID: id, Type: "obligation", Label: deontic, Sublabel: ref, Ref: ref, Status: status})
			edge("le:"+clauseID+"->"+id, clauseID, id, "clause_obligation")
			return nil
		}); err != nil {
		return Lineage{}, fmt.Errorf("lineage obligations: %w", err)
	}

	// 3. Controls (+ obligation→control edges), gated on in-force edges/nodes.
	seenCtl := map[string]bool{}
	if err := query(ctx, s.db, `
		SELECT oc.obligation_id, ctl.id, ctl.name
		FROM obligation_control oc
		JOIN control ctl ON ctl.id = oc.control_id
		JOIN obligation o ON o.id = oc.obligation_id
		JOIN clause c ON c.id = o.clause_id
		WHERE c.circular_id = ?
		  AND oc.valid_from <= ? AND (oc.valid_to IS NULL OR oc.valid_to > ?) AND oc.tx_to IS NULL
		  AND ctl.tx_to IS NULL AND o.tx_to IS NULL`,
		[]any{circularID, at, at},
		func(sc scanner) error {
			var oblID, ctlID, name string
			if err := sc.Scan(&oblID, &ctlID, &name); err != nil {
				return err
			}
			if !seenCtl[ctlID] {
				seenCtl[ctlID] = true
				add(LineageNode{ID: ctlID, Type: "control", Label: name})
			}
			edge("le:"+oblID+"->"+ctlID, oblID, ctlID, "obligation_control")
			return nil
		}); err != nil {
		return Lineage{}, fmt.Errorf("lineage controls: %w", err)
	}

	// 4. Evidence (+ control→evidence edges).
	seenEv := map[string]bool{}
	if err := query(ctx, s.db, `
		SELECT ce.control_id, ev.id, ev.name
		FROM control_evidence ce
		JOIN evidence ev ON ev.id = ce.evidence_id
		WHERE ce.valid_from <= ? AND (ce.valid_to IS NULL OR ce.valid_to > ?) AND ce.tx_to IS NULL
		  AND ev.tx_to IS NULL`,
		[]any{at, at},
		func(sc scanner) error {
			var ctlID, evID, name string
			if err := sc.Scan(&ctlID, &evID, &name); err != nil {
				return err
			}
			if !seenCtl[ctlID] {
				return nil // control not in this lineage
			}
			if !seenEv[evID] {
				seenEv[evID] = true
				add(LineageNode{ID: evID, Type: "evidence", Label: name})
			}
			edge("le:"+ctlID+"->"+evID, ctlID, evID, "control_evidence")
			return nil
		}); err != nil {
		return Lineage{}, fmt.Errorf("lineage evidence: %w", err)
	}

	// 5. Sign-offs (obligation→signoff), scoped to this circular's obligations.
	if err := query(ctx, s.db, `
		SELECT so.id, so.obligation_id, so.action, so.signed_by
		FROM signoff so
		JOIN obligation o ON o.id = so.obligation_id
		JOIN clause c ON c.id = o.clause_id
		WHERE c.circular_id = ?
		  AND so.valid_from <= ? AND (so.valid_to IS NULL OR so.valid_to > ?) AND so.tx_to IS NULL
		  AND o.tx_to IS NULL`,
		[]any{circularID, at, at},
		func(sc scanner) error {
			var id, oblID, action, signedBy string
			if err := sc.Scan(&id, &oblID, &action, &signedBy); err != nil {
				return err
			}
			add(LineageNode{ID: id, Type: "signoff", Label: action, Sublabel: signedBy})
			edge("le:"+oblID+"->"+id, oblID, id, "obligation_signoff")
			return nil
		}); err != nil {
		return Lineage{}, fmt.Errorf("lineage signoffs: %w", err)
	}

	// 6. Policies (obligation→policy), scoped to this circular's obligations.
	if err := query(ctx, s.db, `
		SELECT p.id, p.obligation_id, p.stage
		FROM policy p
		JOIN obligation o ON o.id = p.obligation_id
		JOIN clause c ON c.id = o.clause_id
		WHERE c.circular_id = ?
		  AND p.valid_from <= ? AND (p.valid_to IS NULL OR p.valid_to > ?) AND p.tx_to IS NULL
		  AND o.tx_to IS NULL`,
		[]any{circularID, at, at},
		func(sc scanner) error {
			var id, oblID, stage string
			if err := sc.Scan(&id, &oblID, &stage); err != nil {
				return err
			}
			add(LineageNode{ID: id, Type: "policy", Label: "policy", Sublabel: stage, Status: stage})
			edge("le:"+oblID+"->"+id, oblID, id, "obligation_policy")
			return nil
		}); err != nil {
		return Lineage{}, fmt.Errorf("lineage policies: %w", err)
	}

	return lin, nil
}

type scanner interface{ Scan(...any) error }

// query runs a parameterized query and invokes fn per row.
func query(ctx context.Context, db *sql.DB, q string, args []any, fn func(scanner) error) error {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
