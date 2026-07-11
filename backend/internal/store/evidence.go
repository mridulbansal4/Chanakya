package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"chanakya/internal/domain"
)

// MappedEvidence is one evidence source reachable from an obligation.
type MappedEvidence struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SourceSystem string `json:"source_system"`
}

// ObligationEvidence maps one obligation to its controls + satisfying evidence,
// and flags a gap when nothing satisfies it.
type ObligationEvidence struct {
	ID             string           `json:"id"`
	ClauseRef      string           `json:"clause_ref"`
	ClauseHeading  string           `json:"clause_heading"`
	Deontic        string           `json:"deontic_type"`
	Status         string           `json:"status"`
	Deadline       string           `json:"deadline"`
	SourceSentence string           `json:"source_sentence"`
	ValidFrom      string           `json:"valid_from"`
	Controls       []string         `json:"controls"`
	Evidence       []MappedEvidence `json:"evidence"`
	Satisfied      bool             `json:"satisfied"`
	GapReason      string           `json:"gap_reason,omitempty"`
}

// EvidenceSource is a read-only firm evidence reference.
type EvidenceSource struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	SourceSystem string `json:"source_system"`
	Kind         string `json:"kind"`
	ReadOnly     bool   `json:"read_only"` // always true — connectors never write back
}

// EvidenceMapping is the payload for the Evidence & Gaps screen.
type EvidenceMapping struct {
	AsOf        string               `json:"as_of"`
	Obligations []ObligationEvidence `json:"obligations"`
	Sources     []EvidenceSource     `json:"sources"`
	Satisfied   int                  `json:"satisfied"`
	Gaps        int                  `json:"gaps"`
}

// EvidenceMap builds the obligation↔evidence mapping as-of a date and flags
// gaps: an obligation with no control mapped, or a control with no evidence.
func (s *Store) EvidenceMap(ctx context.Context, asOf time.Time) (EvidenceMapping, error) {
	at := domain.RFC3339UTC(asOf)
	em := EvidenceMapping{AsOf: at, Obligations: []ObligationEvidence{}, Sources: []EvidenceSource{}}

	// Evidence sources (all read-only).
	erows, err := s.db.QueryContext(ctx, `
		SELECT id, name, COALESCE(source_system, ''), COALESCE(kind, '')
		FROM evidence
		WHERE valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL
		ORDER BY name`, at, at)
	if err != nil {
		return EvidenceMapping{}, fmt.Errorf("evidence sources: %w", err)
	}
	evName := map[string]MappedEvidence{}
	for erows.Next() {
		var s EvidenceSource
		if err := erows.Scan(&s.ID, &s.Name, &s.SourceSystem, &s.Kind); err != nil {
			erows.Close()
			return EvidenceMapping{}, fmt.Errorf("scan evidence source: %w", err)
		}
		s.ReadOnly = true
		em.Sources = append(em.Sources, s)
		evName[s.ID] = MappedEvidence{ID: s.ID, Name: s.Name, SourceSystem: s.SourceSystem}
	}
	if err := erows.Err(); err != nil {
		erows.Close()
		return EvidenceMapping{}, fmt.Errorf("iterate evidence sources: %w", err)
	}
	erows.Close()

	// control -> evidence ids.
	ctlEvidence := map[string][]string{}
	crows, err := s.db.QueryContext(ctx, `
		SELECT control_id, evidence_id FROM control_evidence
		WHERE valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL`, at, at)
	if err != nil {
		return EvidenceMapping{}, fmt.Errorf("control_evidence: %w", err)
	}
	for crows.Next() {
		var ctlID, evID string
		if err := crows.Scan(&ctlID, &evID); err != nil {
			crows.Close()
			return EvidenceMapping{}, fmt.Errorf("scan control_evidence: %w", err)
		}
		ctlEvidence[ctlID] = append(ctlEvidence[ctlID], evID)
	}
	if err := crows.Err(); err != nil {
		crows.Close()
		return EvidenceMapping{}, fmt.Errorf("iterate control_evidence: %w", err)
	}
	crows.Close()

	// obligation -> [(controlID, controlName)].
	type ctlRef struct{ id, name string }
	oblControls := map[string][]ctlRef{}
	ocrows, err := s.db.QueryContext(ctx, `
		SELECT oc.obligation_id, oc.control_id, ctl.name
		FROM obligation_control oc JOIN control ctl ON ctl.id = oc.control_id
		WHERE oc.valid_from <= ? AND (oc.valid_to IS NULL OR oc.valid_to > ?) AND oc.tx_to IS NULL
		  AND ctl.tx_to IS NULL`, at, at)
	if err != nil {
		return EvidenceMapping{}, fmt.Errorf("obligation_control: %w", err)
	}
	for ocrows.Next() {
		var oblID, ctlID, name string
		if err := ocrows.Scan(&oblID, &ctlID, &name); err != nil {
			ocrows.Close()
			return EvidenceMapping{}, fmt.Errorf("scan obligation_control: %w", err)
		}
		oblControls[oblID] = append(oblControls[oblID], ctlRef{ctlID, name})
	}
	if err := ocrows.Err(); err != nil {
		ocrows.Close()
		return EvidenceMapping{}, fmt.Errorf("iterate obligation_control: %w", err)
	}
	ocrows.Close()

	// Obligations in force.
	orows, err := s.db.QueryContext(ctx, `
		SELECT o.id, c.clause_ref, COALESCE(c.heading, ''), o.deontic_type, o.status,
		       COALESCE(o.deadline, ''), o.source_sentence, o.valid_from
		FROM obligation o JOIN clause c ON c.id = o.clause_id
		WHERE o.valid_from <= ? AND (o.valid_to IS NULL OR o.valid_to > ?) AND o.tx_to IS NULL
		ORDER BY c.ordinal, o.deontic_type`, at, at)
	if err != nil {
		return EvidenceMapping{}, fmt.Errorf("evidence obligations: %w", err)
	}
	defer orows.Close()

	for orows.Next() {
		var oe ObligationEvidence
		if err := orows.Scan(&oe.ID, &oe.ClauseRef, &oe.ClauseHeading, &oe.Deontic,
			&oe.Status, &oe.Deadline, &oe.SourceSentence, &oe.ValidFrom); err != nil {
			return EvidenceMapping{}, fmt.Errorf("scan evidence obligation: %w", err)
		}
		ctls := oblControls[oe.ID]
		oe.Controls = []string{}
		oe.Evidence = []MappedEvidence{}
		seenEv := map[string]bool{}
		for _, c := range ctls {
			oe.Controls = append(oe.Controls, c.name)
			for _, evID := range ctlEvidence[c.id] {
				if seenEv[evID] {
					continue
				}
				seenEv[evID] = true
				if me, ok := evName[evID]; ok {
					oe.Evidence = append(oe.Evidence, me)
				}
			}
		}
		sort.Slice(oe.Evidence, func(i, j int) bool { return oe.Evidence[i].Name < oe.Evidence[j].Name })

		oe.Satisfied = len(oe.Evidence) > 0
		if !oe.Satisfied {
			if len(ctls) == 0 {
				oe.GapReason = "No control is mapped to this obligation"
			} else {
				oe.GapReason = "Mapped control has no evidence source"
			}
			em.Gaps++
		} else {
			em.Satisfied++
		}
		em.Obligations = append(em.Obligations, oe)
	}
	if err := orows.Err(); err != nil {
		return EvidenceMapping{}, fmt.Errorf("iterate evidence obligations: %w", err)
	}
	return em, nil
}
