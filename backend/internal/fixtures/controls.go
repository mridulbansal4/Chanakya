package fixtures

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

//go:embed ia_controls.json
var iaControlsJSON []byte

type rawEvidence struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SourceSystem string `json:"source_system"`
	Kind         string `json:"kind"`
}

type rawControl struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Kind          string   `json:"kind"`
	CoversClauses []string `json:"covers_clauses"`
	Evidence      []string `json:"evidence"`
}

type rawControlsFixture struct {
	Evidence []rawEvidence `json:"evidence"`
	Controls []rawControl  `json:"controls"`
}

// ControlWiring is the parsed firm controls/evidence layer: the control and
// evidence nodes plus the clause-refs each control covers and the evidence it
// relies on. Note: clause 5.2 (client-notification) is intentionally left
// uncovered, so it surfaces as a gap in Phase 5.
type ControlWiring struct {
	Controls        []domain.Control
	Evidence        []domain.Evidence
	CoversClauses   map[string][]string // controlID -> clause refs
	ControlEvidence map[string][]string // controlID -> evidence ids
}

// LoadControlWiring parses the embedded controls fixture into domain values,
// stamping the given world/system times.
func LoadControlWiring(validFrom, txNow time.Time) (ControlWiring, error) {
	var raw rawControlsFixture
	if err := json.Unmarshal(iaControlsJSON, &raw); err != nil {
		return ControlWiring{}, fmt.Errorf("parse controls fixture: %w", err)
	}
	vf := domain.RFC3339UTC(validFrom)
	tx := domain.RFC3339UTC(txNow)
	tmp := domain.Temporal{ValidFrom: vf, TxFrom: tx}

	w := ControlWiring{
		CoversClauses:   map[string][]string{},
		ControlEvidence: map[string][]string{},
	}
	evByID := map[string]bool{}
	for _, e := range raw.Evidence {
		evByID[e.ID] = true
		w.Evidence = append(w.Evidence, domain.Evidence{
			ID: e.ID, Name: e.Name, SourceSystem: e.SourceSystem, Kind: e.Kind, Temporal: tmp,
		})
	}
	for _, c := range raw.Controls {
		for _, evID := range c.Evidence {
			if !evByID[evID] {
				return ControlWiring{}, fmt.Errorf("control %q references unknown evidence %q", c.ID, evID)
			}
		}
		w.Controls = append(w.Controls, domain.Control{
			ID: c.ID, Name: c.Name, Description: c.Description, Kind: c.Kind, Temporal: tmp,
		})
		w.CoversClauses[c.ID] = c.CoversClauses
		w.ControlEvidence[c.ID] = c.Evidence
	}
	return w, nil
}
