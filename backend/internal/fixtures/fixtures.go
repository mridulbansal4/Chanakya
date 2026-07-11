// Package fixtures embeds and parses CHANAKYA's seed data — currently the SEBI
// Investment Advisers Master Circular fixture. The JSON is compiled into the
// binary with go:embed so `go run ./backend/cmd/seed` works from any directory
// with no external files.
package fixtures

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

//go:embed ia_master_circular.json
var iaMasterCircularJSON []byte

// rawCircular mirrors the fixture JSON's "circular" object.
type rawCircular struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Regulator string `json:"regulator"`
	IssuedOn  string `json:"issued_on"` // "YYYY-MM-DD"
	SourceURL string `json:"source_url"`
}

// rawEntity mirrors the fixture JSON's "entity" object.
type rawEntity struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	PAN      string `json:"pan"`
	MetaJSON string `json:"meta_json"`
}

// rawClause mirrors one element of the fixture JSON's "clauses" array.
type rawClause struct {
	ClauseRef string  `json:"clause_ref"`
	Parent    *string `json:"parent"`
	Heading   string  `json:"heading"`
	Text      string  `json:"text"`
}

type rawFixture struct {
	Circular rawCircular `json:"circular"`
	Entity   rawEntity   `json:"entity"`
	Clauses  []rawClause `json:"clauses"`
}

// IACircular is the parsed, domain-typed IA Master Circular fixture, with all
// bi-temporal columns stamped. World time (valid_from) is the circular's issue
// date; system time (tx_from) is the moment of loading, supplied by the caller
// so seeding is reproducible in tests.
type IACircular struct {
	Circular domain.Circular
	Entity   domain.Entity
	Clauses  []domain.Clause // in document (parents-first) order
}

// LoadIACircular parses the embedded fixture into domain types. txNow is used as
// the system-time lower bound (tx_from) for every row.
func LoadIACircular(txNow time.Time) (IACircular, error) {
	var raw rawFixture
	if err := json.Unmarshal(iaMasterCircularJSON, &raw); err != nil {
		return IACircular{}, fmt.Errorf("parse ia fixture: %w", err)
	}

	issued, err := time.Parse("2006-01-02", raw.Circular.IssuedOn)
	if err != nil {
		return IACircular{}, fmt.Errorf("parse issued_on %q: %w", raw.Circular.IssuedOn, err)
	}
	validFrom := domain.RFC3339UTC(issued)
	txFrom := domain.RFC3339UTC(txNow)

	out := IACircular{
		Circular: domain.Circular{
			ID:        raw.Circular.ID,
			Title:     raw.Circular.Title,
			Regulator: raw.Circular.Regulator,
			IssuedOn:  validFrom,
			SourceURL: raw.Circular.SourceURL,
			Temporal:  domain.Temporal{ValidFrom: validFrom, TxFrom: txFrom},
		},
		Entity: domain.Entity{
			ID:       raw.Entity.ID,
			Kind:     raw.Entity.Kind,
			Name:     raw.Entity.Name,
			PAN:      raw.Entity.PAN,
			MetaJSON: raw.Entity.MetaJSON,
			Temporal: domain.Temporal{ValidFrom: validFrom, TxFrom: txFrom},
		},
	}

	// Validate clause references and build domain clauses in order. We verify
	// that every declared parent has already appeared, which both guarantees
	// the parents-first insertion order the FK constraint needs and catches a
	// malformed fixture early.
	seen := map[string]bool{}
	for i, rc := range raw.Clauses {
		if rc.ClauseRef == "" {
			return IACircular{}, fmt.Errorf("clause[%d]: empty clause_ref", i)
		}
		if seen[rc.ClauseRef] {
			return IACircular{}, fmt.Errorf("clause %q: duplicate clause_ref", rc.ClauseRef)
		}
		parentID := ""
		if rc.Parent != nil && *rc.Parent != "" {
			if !seen[*rc.Parent] {
				return IACircular{}, fmt.Errorf("clause %q: parent %q not defined before it", rc.ClauseRef, *rc.Parent)
			}
			parentID = domain.ClauseID(raw.Circular.ID, *rc.Parent)
		}
		out.Clauses = append(out.Clauses, domain.Clause{
			ID:         domain.ClauseID(raw.Circular.ID, rc.ClauseRef),
			CircularID: raw.Circular.ID,
			ClauseRef:  rc.ClauseRef,
			ParentID:   parentID,
			Heading:    rc.Heading,
			Text:       rc.Text,
			Ordinal:    i + 1,
			Temporal:   domain.Temporal{ValidFrom: validFrom, TxFrom: txFrom},
		})
		seen[rc.ClauseRef] = true
	}
	if len(out.Clauses) == 0 {
		return IACircular{}, fmt.Errorf("ia fixture contains no clauses")
	}
	return out, nil
}
