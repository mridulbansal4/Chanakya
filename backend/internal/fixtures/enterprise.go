package fixtures

import (
	"embed"
	"encoding/json"
	"fmt"
	"time"
)

// EnterpriseFS holds the Alpha Wealth Advisors enterprise fixture: the mock firm
// promoted out of PDFs-on-disk and frontend constants into queryable, as-of-able
// data.
//
// The fixture carries DELIBERATE, non-obvious gaps. They are the demo payload,
// not defects, and none of them is labelled as a problem anywhere in the data -
// each is discoverable only by querying the graph:
//
//   - 22 of 140 agreements are on template v2, so a MITC-style obligation lights
//     up exactly 118 clients.
//   - 3 employees have a current-quarter training row with no completion date.
//   - one adviser (emp_006) holds both advisory and distribution clients, which
//     is a clause 4.2 segregation breach findable only by grouping clients by
//     adviser.
//   - the complaint register was last updated 90 days ago (a freshness gap).
//   - the cybersecurity policy was last reviewed 14 months ago (an annual-review
//     breach).
//
//go:embed enterprise/*.json
var EnterpriseFS embed.FS

// Firm is the regulated entity the enterprise graph belongs to.
type Firm struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SEBIRegNo      string `json:"sebi_reg_no"`
	PAN            string `json:"pan"`
	City           string `json:"city"`
	IncorporatedOn string `json:"incorporated_on"`
	Kind           string `json:"kind"`
}

// Department is an organisational unit.
type Department struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Head     string `json:"head_employee_id"`
	Function string `json:"function"`
}

// Employee is a person in the firm.
type Employee struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Role           string   `json:"role"`
	DepartmentID   string   `json:"department_id"`
	Email          string   `json:"email"`
	Certifications []string `json:"certifications"`
	ManagerID      string   `json:"manager_id"`
}

// Client is an advised (or distribution) client.
type Client struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Segment     string `json:"segment"`
	OnboardedOn string `json:"onboarded_on"`
	RiskProfile string `json:"risk_profile"`
	AdviserID   string `json:"adviser_id"`
	ServiceKind string `json:"service_kind"`
}

// Agreement is a client's signed advisory agreement.
//
// A re-papered client has TWO rows: the original, bounded in world time by
// SupersededOn, and its replacement. That is what makes "which clients were on
// v1 on 1 March 2025?" answerable rather than a statement about today.
type Agreement struct {
	ID              string `json:"id"`
	ClientID        string `json:"client_id"`
	TemplateVersion string `json:"template_version"`
	SignedOn        string `json:"signed_on"`
	DocID           string `json:"doc_id"`
	SupersededOn    string `json:"superseded_on,omitempty"`
}

// Document is a firm-authored policy, SOP or manual.
//
// Review recency is stored as an OFFSET in months, resolved against the load
// time. "Last reviewed 14 months ago" is the fact the annual-review gap depends
// on; the calendar date it maps to depends on when you look. With an absolute
// date baked in, every policy would become overdue a year after the fixture was
// written and the single deliberate breach would be lost among twenty-one
// accidental ones.
type Document struct {
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	Title             string `json:"title"`
	Version           int    `json:"version"`
	OwnerDept         string `json:"owner_dept"`
	Status            string `json:"status"`
	ReviewedMonthsAgo int    `json:"reviewed_months_ago"`
	// LastReviewed is derived at load time from ReviewedMonthsAgo.
	LastReviewed string `json:"-"`
}

// Register is a maintained record set. Freshness is an offset, for the same
// reason as Document.
type Register struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	RowCount       int    `json:"row_count"`
	Source         string `json:"source_system"`
	UpdatedDaysAgo int    `json:"updated_days_ago"`
	OwnerDept      string `json:"owner_dept"`
	// LastUpdated is derived at load time from UpdatedDaysAgo.
	LastUpdated string `json:"-"`
}

// System is a firm system, each carrying the id of the read-only connector that
// will front it in Phase 4.
type System struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Vendor      string `json:"vendor"`
	ConnectorID string `json:"connector_id"`
	Criticality string `json:"criticality"`
	OwnerDept   string `json:"owner_dept"`
}

// TrainingRecord is one employee's completion (or non-completion) of a course.
// CompletedDaysAgo of -1 means NOT completed - the deliberate gap.
type TrainingRecord struct {
	ID               string `json:"id"`
	EmployeeID       string `json:"employee_id"`
	Course           string `json:"course"`
	CompletedDaysAgo int    `json:"completed_days_ago"`
	Period           string `json:"period"`
	Certificate      string `json:"certificate_doc"`
	// CompletedOn is derived at load time; empty means not completed.
	CompletedOn string `json:"-"`
}

// Communication is an email thread, meeting or call.
type Communication struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Subject      string   `json:"subject"`
	Participants []string `json:"participants"`
	ThreadID     string   `json:"thread_id"`
	SentOn       string   `json:"sent_on"`
	SystemID     string   `json:"system_id"`
}

// CalendarEvent is a scheduled internal event.
type CalendarEvent struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	StartsAt  string   `json:"starts_at"`
	Attendees []string `json:"attendees"`
	Kind      string   `json:"kind"`
}

// Risk is a risk-register entry, optionally mitigated by a control.
type Risk struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Likelihood string `json:"likelihood"`
	Impact     string `json:"impact"`
	OwnerDept  string `json:"owner_dept"`
	ControlID  string `json:"control_id"`
}

// EnterpriseControl extends the Phase 4 control fixture with the owner
// department, backing system and risk links the enterprise graph needs.
type EnterpriseControl struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Kind          string   `json:"kind"`
	CoversClauses []string `json:"covers_clauses"`
	Evidence      []string `json:"evidence"`
	OwnerDept     string   `json:"owner_dept"`
	System        string   `json:"system"`
	Risk          string   `json:"risk"`
}

type enterpriseControlsFile struct {
	Evidence []rawEvidence       `json:"evidence"`
	Controls []EnterpriseControl `json:"controls"`
}

// Enterprise is the whole parsed fixture.
type Enterprise struct {
	Firm           Firm
	Departments    []Department
	Employees      []Employee
	Clients        []Client
	Agreements     []Agreement
	Documents      []Document
	Registers      []Register
	Systems        []System
	Controls       []EnterpriseControl
	Training       []TrainingRecord
	Communications []Communication
	Calendar       []CalendarEvent
	Risks          []Risk
	// ValidFrom / TxFrom are the bi-temporal stamps applied to every row.
	ValidFrom string
	TxFrom    string
}

// LoadEnterprise parses every embedded enterprise fixture.
//
// validFrom is world time (when the firm's state is true) and txNow is system
// time (when CHANAKYA learned it), supplied by the caller so seeding is
// reproducible in tests rather than dependent on the wall clock.
func LoadEnterprise(validFrom, txNow time.Time) (Enterprise, error) {
	e := Enterprise{
		ValidFrom: rfc3339(validFrom),
		TxFrom:    rfc3339(txNow),
	}
	var controls enterpriseControlsFile

	for _, step := range []struct {
		file string
		out  any
	}{
		{"firm.json", &e.Firm},
		{"departments.json", &e.Departments},
		{"employees.json", &e.Employees},
		{"clients.json", &e.Clients},
		{"agreements.json", &e.Agreements},
		{"documents.json", &e.Documents},
		{"registers.json", &e.Registers},
		{"systems.json", &e.Systems},
		{"controls.json", &controls},
		{"training.json", &e.Training},
		{"communications.json", &e.Communications},
		{"calendar.json", &e.Calendar},
		{"risks.json", &e.Risks},
	} {
		raw, err := EnterpriseFS.ReadFile("enterprise/" + step.file)
		if err != nil {
			return Enterprise{}, fmt.Errorf("read enterprise fixture %s: %w", step.file, err)
		}
		if err := json.Unmarshal(raw, step.out); err != nil {
			return Enterprise{}, fmt.Errorf("parse enterprise fixture %s: %w", step.file, err)
		}
	}
	e.Controls = controls.Controls
	e.resolveOffsets(txNow)

	if err := e.validate(); err != nil {
		return Enterprise{}, err
	}
	return e, nil
}

// resolveOffsets turns the fixture's relative recency offsets into absolute
// dates against the reference time, so the seeded gaps stay exactly as designed
// no matter when the fixture is loaded.
func (e *Enterprise) resolveOffsets(ref time.Time) {
	ref = ref.UTC()
	for i := range e.Documents {
		e.Documents[i].LastReviewed = ref.AddDate(0, -e.Documents[i].ReviewedMonthsAgo, 0).Format("2006-01-02")
	}
	for i := range e.Registers {
		e.Registers[i].LastUpdated = ref.AddDate(0, 0, -e.Registers[i].UpdatedDaysAgo).Format("2006-01-02")
	}
	for i := range e.Training {
		if e.Training[i].CompletedDaysAgo < 0 {
			e.Training[i].CompletedOn = "" // the gap
			continue
		}
		e.Training[i].CompletedOn = ref.AddDate(0, 0, -e.Training[i].CompletedDaysAgo).Format("2006-01-02")
	}
}

// validate checks referential integrity BEFORE anything is written. Loading a
// fixture whose agreements point at clients that do not exist would produce a
// graph whose traversals quietly return less than they should - a much harder
// failure to notice than a load error.
func (e Enterprise) validate() error {
	clients := idSet(e.Clients, func(c Client) string { return c.ID })
	employees := idSet(e.Employees, func(x Employee) string { return x.ID })
	depts := idSet(e.Departments, func(d Department) string { return d.ID })

	for _, a := range e.Agreements {
		if !clients[a.ClientID] {
			return fmt.Errorf("agreement %q references unknown client %q", a.ID, a.ClientID)
		}
	}
	for _, c := range e.Clients {
		if c.AdviserID != "" && !employees[c.AdviserID] {
			return fmt.Errorf("client %q references unknown adviser %q", c.ID, c.AdviserID)
		}
	}
	for _, emp := range e.Employees {
		if emp.DepartmentID != "" && !depts[emp.DepartmentID] {
			return fmt.Errorf("employee %q references unknown department %q", emp.ID, emp.DepartmentID)
		}
		if emp.ManagerID != "" && !employees[emp.ManagerID] {
			return fmt.Errorf("employee %q references unknown manager %q", emp.ID, emp.ManagerID)
		}
	}
	for _, t := range e.Training {
		if !employees[t.EmployeeID] {
			return fmt.Errorf("training %q references unknown employee %q", t.ID, t.EmployeeID)
		}
	}
	return nil
}

func idSet[T any](items []T, id func(T) string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, it := range items {
		out[id(it)] = true
	}
	return out
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }
