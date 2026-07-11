package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// PolicyRecord is a compiled policy (JSON-tagged for the API).
type PolicyRecord struct {
	ID           string `json:"id"`
	ObligationID string `json:"obligation_id"`
	PackageName  string `json:"package_name"`
	Rego         string `json:"rego"`
	Stage        string `json:"stage"`
	CompiledAt   string `json:"compiled_at"`
}

// PolicyEvalRecord is a recorded evaluation.
type PolicyEvalRecord struct {
	ID           string   `json:"id"`
	PolicyID     string   `json:"policy_id"`
	ObligationID string   `json:"obligation_id"`
	InputJSON    string   `json:"input_json"`
	Compliant    bool     `json:"compliant"`
	Applicable   bool     `json:"applicable"`
	Denies       []string `json:"denies"`
	Stage        string   `json:"stage"`
	Blocked      bool     `json:"blocked"`
	Trace        string   `json:"trace"`
	CreatedAt    string   `json:"created_at"`
}

// UpsertPolicy stores a compiled policy, preserving an existing stage on update.
func (s *Store) UpsertPolicy(ctx context.Context, p PolicyRecord, validFrom, txFrom string) error {
	stage := p.Stage
	if stage == "" {
		stage = string(domain.StageAudit)
	}
	const q = `
		INSERT INTO policy (id, obligation_id, package_name, rego, stage, valid_from, tx_from)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			package_name=excluded.package_name, rego=excluded.rego,
			compiled_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),
			valid_from=excluded.valid_from, tx_from=excluded.tx_from`
	if _, err := s.db.ExecContext(ctx, q,
		p.ID, p.ObligationID, p.PackageName, p.Rego, stage, validFrom, txFrom); err != nil {
		return fmt.Errorf("upsert policy %q: %w", p.ID, err)
	}
	return nil
}

// GetPolicy returns the current policy for an obligation, if any.
func (s *Store) GetPolicy(ctx context.Context, obligationID string) (PolicyRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, obligation_id, package_name, rego, stage, compiled_at
		FROM policy WHERE obligation_id = ? AND tx_to IS NULL`, obligationID)
	var p PolicyRecord
	if err := row.Scan(&p.ID, &p.ObligationID, &p.PackageName, &p.Rego, &p.Stage, &p.CompiledAt); err != nil {
		if err == sql.ErrNoRows {
			return PolicyRecord{}, false, nil
		}
		return PolicyRecord{}, false, fmt.Errorf("get policy for %q: %w", obligationID, err)
	}
	return p, true, nil
}

// SetPolicyStage promotes/demotes a policy's enforcement stage.
func (s *Store) SetPolicyStage(ctx context.Context, obligationID string, stage domain.PolicyStage) error {
	if !stage.Valid() {
		return fmt.Errorf("invalid stage %q", stage)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE policy SET stage = ? WHERE obligation_id = ? AND tx_to IS NULL`, string(stage), obligationID)
	if err != nil {
		return fmt.Errorf("set stage for %q: %w", obligationID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpsertPolicyEval records the latest evaluation for an obligation.
func (s *Store) UpsertPolicyEval(ctx context.Context, e PolicyEvalRecord, validFrom, txFrom string) error {
	denies, err := json.Marshal(e.Denies)
	if err != nil {
		return fmt.Errorf("marshal denies: %w", err)
	}
	const q = `
		INSERT INTO policy_eval (id, policy_id, obligation_id, input_json, compliant,
		                         applicable, deny_json, stage, blocked, trace, valid_from, tx_from)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			policy_id=excluded.policy_id, input_json=excluded.input_json,
			compliant=excluded.compliant, applicable=excluded.applicable,
			deny_json=excluded.deny_json, stage=excluded.stage, blocked=excluded.blocked,
			trace=excluded.trace, created_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),
			valid_from=excluded.valid_from, tx_from=excluded.tx_from`
	if _, err := s.db.ExecContext(ctx, q,
		e.ID, e.PolicyID, e.ObligationID, e.InputJSON, boolToInt(e.Compliant),
		boolToInt(e.Applicable), string(denies), e.Stage, boolToInt(e.Blocked), e.Trace,
		validFrom, txFrom); err != nil {
		return fmt.Errorf("upsert policy eval %q: %w", e.ID, err)
	}
	return nil
}

// GetPolicyEval returns the latest recorded evaluation for an obligation.
func (s *Store) GetPolicyEval(ctx context.Context, obligationID string) (PolicyEvalRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, policy_id, obligation_id, input_json, compliant, applicable,
		       deny_json, stage, blocked, trace, created_at
		FROM policy_eval WHERE obligation_id = ? AND tx_to IS NULL`, obligationID)
	var (
		e          PolicyEvalRecord
		compliant  int
		applicable int
		blocked    int
		denies     string
	)
	if err := row.Scan(&e.ID, &e.PolicyID, &e.ObligationID, &e.InputJSON, &compliant,
		&applicable, &denies, &e.Stage, &blocked, &e.Trace, &e.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return PolicyEvalRecord{}, false, nil
		}
		return PolicyEvalRecord{}, false, fmt.Errorf("get policy eval for %q: %w", obligationID, err)
	}
	e.Compliant = compliant == 1
	e.Applicable = applicable == 1
	e.Blocked = blocked == 1
	_ = json.Unmarshal([]byte(denies), &e.Denies)
	if e.Denies == nil {
		e.Denies = []string{}
	}
	return e, true, nil
}

// PolicyCandidate is an approved obligation and its policy status, for the UI.
type PolicyCandidate struct {
	ObligationID  string `json:"obligation_id"`
	ClauseRef     string `json:"clause_ref"`
	ClauseHeading string `json:"clause_heading"`
	Deontic       string `json:"deontic_type"`
	Compiled      bool   `json:"compiled"`
	Stage         string `json:"stage,omitempty"`
}

// ListPolicyCandidates returns approved obligations (the only ones eligible for
// a policy) with their compile/stage status.
func (s *Store) ListPolicyCandidates(ctx context.Context, asOf time.Time) ([]PolicyCandidate, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, c.clause_ref, COALESCE(c.heading,''), o.deontic_type,
		       p.stage
		FROM obligation o
		JOIN clause c ON c.id = o.clause_id
		LEFT JOIN policy p ON p.obligation_id = o.id AND p.tx_to IS NULL
		WHERE o.valid_from <= ? AND (o.valid_to IS NULL OR o.valid_to > ?) AND o.tx_to IS NULL
		  AND o.status = 'approved'
		ORDER BY c.ordinal`, at, at)
	if err != nil {
		return nil, fmt.Errorf("list policy candidates: %w", err)
	}
	defer rows.Close()
	out := []PolicyCandidate{}
	for rows.Next() {
		var pc PolicyCandidate
		var stage sql.NullString
		if err := rows.Scan(&pc.ObligationID, &pc.ClauseRef, &pc.ClauseHeading, &pc.Deontic, &stage); err != nil {
			return nil, fmt.Errorf("scan policy candidate: %w", err)
		}
		if stage.Valid {
			pc.Compiled = true
			pc.Stage = stage.String
		}
		out = append(out, pc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy candidates: %w", err)
	}
	return out, nil
}

// FirmState builds a plausible default firm-state input for policy evaluation:
// metrics from the regulated entity, and per-clause attestations derived from
// whether each obligation currently has a satisfying evidence path.
func (s *Store) FirmState(ctx context.Context, asOf time.Time) (map[string]any, error) {
	metrics := map[string]any{}
	// Pull metrics from the entity's meta_json (best effort).
	var meta string
	err := s.db.QueryRowContext(ctx, `
		SELECT meta_json FROM entity
		WHERE valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL
		ORDER BY id LIMIT 1`, domain.RFC3339UTC(asOf), domain.RFC3339UTC(asOf)).Scan(&meta)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("firm-state entity: %w", err)
	}
	if meta != "" {
		var m map[string]any
		if json.Unmarshal([]byte(meta), &m) == nil {
			if v, ok := m["clients"]; ok {
				metrics["clients"] = v
			}
			// Publish under the metric key the compiled policies gate on
			// ("annual_fees"), not the entity's meta key ("annual_fees_inr").
			if v, ok := m["annual_fees_inr"]; ok {
				metrics["annual_fees"] = v
			}
		}
	}
	// Firm's actual record-retention (edit to test a requirement policy).
	metrics["retention_period"] = 5

	// Attestations from evidence coverage: satisfied obligation -> attested true.
	// Keyed by OBLIGATION ID to match the compiled policy (avoids collisions when
	// a clause carries more than one obligation).
	em, err := s.EvidenceMap(ctx, asOf)
	if err != nil {
		return nil, fmt.Errorf("firm-state evidence: %w", err)
	}
	attest := map[string]any{}
	for _, oe := range em.Obligations {
		attest[oe.ID] = oe.Satisfied
	}
	return map[string]any{"metrics": metrics, "attestations": attest}, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
