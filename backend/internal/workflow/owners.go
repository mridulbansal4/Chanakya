package workflow

import (
	"context"
	"strings"
)

// DepartmentLookup is the slice of the enterprise graph owner resolution needs.
type DepartmentLookup interface {
	// Departments returns (department name, head employee id, head name) for
	// every department in force.
	Departments(ctx context.Context) ([]DepartmentHead, error)
}

// DepartmentHead is one department and the person who heads it.
type DepartmentHead struct {
	ID       string
	Name     string
	HeadID   string
	HeadName string
}

// GraphOwnerResolver resolves a template's OwnerRole to a REAL EMPLOYEE via the
// enterprise graph, rather than leaving a placeholder string on the task.
//
// "Owned by Compliance" is not actionable; "owned by Priya Menon" is. That is the
// whole reason Phase 3 promoted the firm into the database - so a generated task
// can name a person who actually exists.
//
// A role with no resolvable head is reported as UNRESOLVED. Falling back to any
// available employee would put a real person's name against work nobody agreed
// they own, which is worse than an obviously unassigned task.
type GraphOwnerResolver struct {
	byDepartment map[string]DepartmentHead
}

// NewGraphOwnerResolver loads the department heads once, so resolving a whole
// workflow's tasks does not re-query per task.
func NewGraphOwnerResolver(ctx context.Context, lookup DepartmentLookup) (*GraphOwnerResolver, error) {
	depts, err := lookup.Departments(ctx)
	if err != nil {
		return nil, err
	}
	r := &GraphOwnerResolver{byDepartment: make(map[string]DepartmentHead, len(depts))}
	for _, d := range depts {
		r.byDepartment[normalizeRole(d.Name)] = d
	}
	return r, nil
}

// roleAliases maps a template's role name to the department that owns it where
// the two differ. Kept explicit rather than fuzzy-matched: a near-miss that
// silently resolved to the wrong department would assign work to the wrong
// person without anyone noticing.
var roleAliases = map[string]string{
	"clientservicing": "clientservicing",
	"compliance":      "compliance",
	"legal":           "legal",
	"operations":      "operations",
	"technology":      "technology",
	"risk":            "risk",
	"hr":              "hr",
	"advisory":        "advisory",
	"board":           "compliance", // the Principal Officer carries board matters
}

// ResolveRole returns the employee who owns work for a role.
func (r *GraphOwnerResolver) ResolveRole(role string) (employeeID, name string, ok bool) {
	if r == nil {
		return "", "", false
	}
	key := normalizeRole(role)
	if alias, exists := roleAliases[key]; exists {
		key = alias
	}
	d, exists := r.byDepartment[key]
	if !exists {
		return "", "", false
	}
	// A department with no head is exactly the "unresolvable owner" case: the
	// department exists, but there is nobody to hand the task to.
	if d.HeadID == "" || d.HeadName == "" {
		return "", "", false
	}
	return d.HeadID, d.HeadName, true
}

// normalizeRole lowercases and strips spaces/punctuation so "Client Servicing"
// and "client_servicing" resolve to the same key.
func normalizeRole(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
