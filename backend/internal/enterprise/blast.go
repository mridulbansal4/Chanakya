package enterprise

import (
	"context"
	"fmt"
	"time"

	"chanakya/internal/store"
)

// Impact is one obligation's projection onto the firm: not counts, but NAMES.
//
// "118 clients affected" is a statistic; "these 118 named clients, holding these
// agreements, owned by these named people" is something a compliance officer can
// act on. The whole point of promoting the firm into the graph was to be able to
// answer the second question.
type Impact struct {
	AsOf         string                      `json:"as_of"`
	ObligationID string                      `json:"obligation_id"`
	ClauseRef    string                      `json:"clause_ref"`
	Summary      string                      `json:"summary"`
	Bindings     []Binding                   `json:"bindings"`
	Controls     []store.ImpactedControlView `json:"controls"`
	Documents    []store.DocumentView        `json:"documents"`
	Registers    []store.RegisterView        `json:"registers"`
	Systems      []store.SystemView          `json:"systems"`
	Clients      []store.ClientView          `json:"clients"`
	Departments  []ImpactedDepartment        `json:"departments"`
	Owners       []store.EmployeeView        `json:"owners"`
	Counts       map[string]int              `json:"counts"`
	// Unbound records that the obligation projected onto nothing. An empty
	// impact is a real and important answer - it means the firm has no artefact
	// addressing this duty - and it must not look like a failed query.
	Unbound bool `json:"unbound"`
}

// ImpactedDepartment is a department that owns something in the blast radius.
type ImpactedDepartment struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	HeadName string `json:"head_name"`
	Reason   string `json:"reason"`
}

// ImpactOf traverses obligation → {control, binds_to} → {department, system} and
// resolves the affected client population.
//
// This is the JOIN between the two namespaces, and it goes through exactly the
// seam the design allows: `control` (a firm answer to an obligation) and
// `binds_to` (an explicit, confidence-carrying inference). It reads only - no
// firm system is written to and nothing is enforced.
func (p *Projector) ImpactOf(ctx context.Context, obligationID string, asOf time.Time) (Impact, error) {
	ob, err := p.store.GetObligation(ctx, obligationID)
	if err != nil {
		return Impact{}, fmt.Errorf("load obligation %q: %w", obligationID, err)
	}

	out := Impact{
		AsOf:         asOf.UTC().Format(time.RFC3339),
		ObligationID: obligationID,
		ClauseRef:    ob.ClauseRef,
		Counts:       map[string]int{},
	}

	// Bindings: stored ones if the projector has already run, otherwise computed
	// on the fly so the endpoint is useful before anything is persisted.
	stored, err := p.store.ListBindings(ctx, obligationID, asOf)
	if err != nil {
		return Impact{}, err
	}
	if len(stored) > 0 {
		for _, b := range stored {
			out.Bindings = append(out.Bindings, Binding{
				ObligationID: b.ObligationID, TargetType: b.TargetType, TargetID: b.TargetID,
				TargetLabel: b.TargetID, Confidence: b.Confidence,
				HumanConfirmed: b.HumanConfirmed, Rationale: b.Rationale,
			})
		}
	} else {
		computed, err := p.Project(ctx, obligationID, asOf)
		if err != nil {
			return Impact{}, err
		}
		out.Bindings = computed
	}

	byType := map[string]map[string]bool{}
	for _, b := range out.Bindings {
		if byType[b.TargetType] == nil {
			byType[b.TargetType] = map[string]bool{}
		}
		byType[b.TargetType][b.TargetID] = true
	}

	// Documents, registers and systems named by the bindings.
	allDocs, err := p.store.ListDocuments(ctx, asOf, false)
	if err != nil {
		return Impact{}, err
	}
	deptReasons := map[string]string{}
	for _, d := range allDocs {
		if byType[TargetDocument][d.ID] {
			out.Documents = append(out.Documents, d)
			if d.OwnerDept != "" {
				deptReasons[d.OwnerDept] = "owns " + d.Title
			}
		}
	}

	allRegisters, err := p.store.ListRegisters(ctx, asOf)
	if err != nil {
		return Impact{}, err
	}
	for _, r := range allRegisters {
		if byType[TargetRegister][r.ID] {
			out.Registers = append(out.Registers, r)
			if r.OwnerDept != "" {
				deptReasons[r.OwnerDept] = "maintains the " + r.Kind + " register"
			}
		}
	}

	allSystems, err := p.store.ListSystems(ctx, asOf)
	if err != nil {
		return Impact{}, err
	}
	for _, sy := range allSystems {
		if byType[TargetSystem][sy.ID] {
			out.Systems = append(out.Systems, sy)
			if sy.OwnerDept != "" {
				deptReasons[sy.OwnerDept] = "operates " + sy.Vendor
			}
		}
	}

	// Controls satisfying this obligation, via the one shared seam.
	controls, err := p.store.ControlsForObligation(ctx, obligationID, asOf)
	if err != nil {
		return Impact{}, err
	}
	for _, c := range controls {
		out.Controls = append(out.Controls, c)
		if c.OwnerDept != "" {
			deptReasons[c.OwnerDept] = "owns the control " + c.Name
		}
	}

	// The affected client population. An obligation that binds to the client
	// agreement affects exactly those clients whose IN-FORCE agreement is on the
	// superseded template - which as-of a past date is a different set.
	if byType[TargetClientSegment]["all_clients"] {
		clients, err := p.store.ListClients(ctx, store.ClientQuery{AsOf: asOf, TemplateVersion: "v1"})
		if err != nil {
			return Impact{}, err
		}
		out.Clients = clients
		for _, c := range clients {
			if c.AdviserID != "" {
				deptReasons["dep_advisory"] = "advises affected clients"
			}
		}
	}

	// Departments, and the named person who heads each one. "Owned by
	// Compliance" is not actionable; "owned by Priya Menon" is.
	depts, err := p.store.EnterpriseSummaryAsOf(ctx, asOf)
	if err != nil {
		return Impact{}, err
	}
	for _, d := range depts.Departments {
		if reason, ok := deptReasons[d.ID]; ok {
			out.Departments = append(out.Departments, ImpactedDepartment{
				ID: d.ID, Name: d.Name, HeadName: d.HeadName, Reason: reason,
			})
			if d.HeadID != "" {
				out.Owners = append(out.Owners, store.EmployeeView{
					ID: d.HeadID, Name: d.HeadName, DeptID: d.ID, DeptName: d.Name,
				})
			}
		}
	}

	out.Counts["bindings"] = len(out.Bindings)
	out.Counts["controls"] = len(out.Controls)
	out.Counts["documents"] = len(out.Documents)
	out.Counts["registers"] = len(out.Registers)
	out.Counts["systems"] = len(out.Systems)
	out.Counts["clients"] = len(out.Clients)
	out.Counts["departments"] = len(out.Departments)
	out.Unbound = len(out.Bindings) == 0 && len(out.Controls) == 0

	if out.Unbound {
		out.Summary = "This obligation projects onto nothing in the firm: no policy, register, " +
			"system or control currently addresses it."
	} else {
		out.Summary = fmt.Sprintf(
			"%d firm artefacts, %d controls and %d clients are in this obligation's blast radius, "+
				"owned across %d departments.",
			len(out.Documents)+len(out.Registers)+len(out.Systems),
			len(out.Controls), len(out.Clients), len(out.Departments))
	}
	return out, nil
}
