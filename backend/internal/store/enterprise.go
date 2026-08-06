package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/fixtures"
	"chanakya/internal/ingest"
)

// maxOrgDepth caps the recursive org-chart traversal.
//
// employee.manager_id COULD form a cycle if fixture data were inconsistent, and
// a recursive CTE walking a cycle does not terminate. The cap is an explicit
// guard rather than a reliance on the seed data being well-formed: data is not a
// safety mechanism.
const maxOrgDepth = 12

// EnterpriseFirmID is the entity id the enterprise graph belongs to. It matches
// firm.json and is used to address the firm directly rather than by "the first
// investment adviser", which would pick up the regulatory fixture's placeholder.
const EnterpriseFirmID = "firm_alpha_wealth"

// asOfClause is the standard bi-temporal filter: in force in world time at :at,
// and current in system time.
const asOfClause = ` valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL `

// SeedEnterprise loads the Alpha Wealth fixture into the enterprise graph. It is
// idempotent: every insert is an upsert on a deterministic id, so re-seeding an
// unchanged firm changes nothing.
func (s *Store) SeedEnterprise(ctx context.Context, e fixtures.Enterprise) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin enterprise seed: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	vf, txf := e.ValidFrom, e.TxFrom

	// Entity: the firm itself lives in the EXISTING entity table, because an
	// obligation's bearer is an entity - that link already exists and should not
	// be duplicated in the enterprise namespace.
	meta, err := json.Marshal(map[string]any{
		"registration_no": e.Firm.SEBIRegNo,
		"city":            e.Firm.City,
		"incorporated_on": e.Firm.IncorporatedOn,
		"clients":         len(e.Clients),
	})
	if err != nil {
		return fmt.Errorf("encode firm meta: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO entity (id, kind, name, pan, meta_json, valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, NULL, ?, NULL)
		ON CONFLICT(id) DO UPDATE SET
			kind=excluded.kind, name=excluded.name, pan=excluded.pan,
			meta_json=excluded.meta_json, valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
		e.Firm.ID, e.Firm.Kind, e.Firm.Name, e.Firm.PAN, string(meta), vf, txf); err != nil {
		return fmt.Errorf("seed firm entity: %w", err)
	}

	// Employees BEFORE departments: department.head_employee_id points at one.
	for _, emp := range e.Employees {
		certs, err := json.Marshal(emp.Certifications)
		if err != nil {
			return fmt.Errorf("encode certifications for %q: %w", emp.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO employee (id, name, role, department_id, email, certifications,
			                      manager_id, valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				name=excluded.name, role=excluded.role, department_id=excluded.department_id,
				email=excluded.email, certifications=excluded.certifications,
				manager_id=excluded.manager_id, valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			emp.ID, emp.Name, emp.Role, nullStr(emp.DepartmentID), nullStr(emp.Email),
			string(certs), nullStr(emp.ManagerID), vf, txf); err != nil {
			return fmt.Errorf("seed employee %q: %w", emp.ID, err)
		}
	}

	for _, d := range e.Departments {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO department (id, name, head_employee_id, function, valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				name=excluded.name, head_employee_id=excluded.head_employee_id,
				function=excluded.function, valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			d.ID, d.Name, nullStr(d.Head), nullStr(d.Function), vf, txf); err != nil {
			return fmt.Errorf("seed department %q: %w", d.ID, err)
		}
	}

	for _, c := range e.Clients {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO client (id, name, segment, onboarded_on, risk_profile, adviser_id,
			                    service_kind, valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				name=excluded.name, segment=excluded.segment, onboarded_on=excluded.onboarded_on,
				risk_profile=excluded.risk_profile, adviser_id=excluded.adviser_id,
				service_kind=excluded.service_kind, valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			c.ID, c.Name, c.Segment, c.OnboardedOn, nullStr(c.RiskProfile),
			nullStr(c.AdviserID), c.ServiceKind, vf, txf); err != nil {
			return fmt.Errorf("seed client %q: %w", c.ID, err)
		}
	}

	for _, a := range e.Agreements {
		// An agreement is in force in WORLD time from the day it was signed. That
		// is what makes "which clients were on v1 as of 1 March 2025?"
		// answerable, and it is the whole point of storing the firm bi-temporally.
		signed := a.SignedOn + "T00:00:00Z"
		// A superseded agreement's world-time interval CLOSES on the day it was
		// replaced. Without this the re-papering would be invisible to time
		// travel: every client would look as though they had always held their
		// current template.
		var supersededAt string
		if a.SupersededOn != "" {
			supersededAt = a.SupersededOn + "T00:00:00Z"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO agreement (id, client_id, template_version, signed_on, doc_id,
			                       valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				client_id=excluded.client_id, template_version=excluded.template_version,
				signed_on=excluded.signed_on, doc_id=excluded.doc_id,
				valid_from=excluded.valid_from, valid_to=excluded.valid_to,
				tx_from=excluded.tx_from`,
			a.ID, a.ClientID, a.TemplateVersion, a.SignedOn, nullStr(a.DocID),
			signed, nullStr(supersededAt), txf); err != nil {
			return fmt.Errorf("seed agreement %q: %w", a.ID, err)
		}
	}

	for _, d := range e.Documents {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO document (id, kind, title, version, owner_dept, blob_sha, status,
			                      last_reviewed, valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				kind=excluded.kind, title=excluded.title, version=excluded.version,
				owner_dept=excluded.owner_dept, status=excluded.status,
				last_reviewed=excluded.last_reviewed, valid_from=excluded.valid_from,
				tx_from=excluded.tx_from`,
			d.ID, d.Kind, d.Title, d.Version, nullStr(d.OwnerDept), d.Status,
			nullStr(d.LastReviewed), vf, txf); err != nil {
			return fmt.Errorf("seed document %q: %w", d.ID, err)
		}
	}

	for _, r := range e.Registers {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO register (id, kind, schema_json, row_count, source_system, last_updated,
			                      owner_dept, valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, '{}', ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				kind=excluded.kind, row_count=excluded.row_count,
				source_system=excluded.source_system, last_updated=excluded.last_updated,
				owner_dept=excluded.owner_dept, valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			r.ID, r.Kind, r.RowCount, nullStr(r.Source), nullStr(r.LastUpdated),
			nullStr(r.OwnerDept), vf, txf); err != nil {
			return fmt.Errorf("seed register %q: %w", r.ID, err)
		}
	}

	for _, sy := range e.Systems {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system (id, kind, vendor, connector_id, criticality, owner_dept,
			                    valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				kind=excluded.kind, vendor=excluded.vendor, connector_id=excluded.connector_id,
				criticality=excluded.criticality, owner_dept=excluded.owner_dept,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			sy.ID, sy.Kind, nullStr(sy.Vendor), nullStr(sy.ConnectorID),
			nullStr(sy.Criticality), nullStr(sy.OwnerDept), vf, txf); err != nil {
			return fmt.Errorf("seed system %q: %w", sy.ID, err)
		}
	}

	for _, t := range e.Training {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO training (id, employee_id, course, completed_on, certificate_doc, period,
			                      valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				course=excluded.course, completed_on=excluded.completed_on,
				certificate_doc=excluded.certificate_doc, period=excluded.period,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			t.ID, t.EmployeeID, t.Course, nullStr(t.CompletedOn),
			nullStr(t.Certificate), nullStr(t.Period), vf, txf); err != nil {
			return fmt.Errorf("seed training %q: %w", t.ID, err)
		}
	}

	for _, c := range e.Communications {
		parts, err := json.Marshal(c.Participants)
		if err != nil {
			return fmt.Errorf("encode participants for %q: %w", c.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO communication (id, kind, subject, participants, thread_id, sent_on,
			                           system_id, valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				kind=excluded.kind, subject=excluded.subject, participants=excluded.participants,
				thread_id=excluded.thread_id, sent_on=excluded.sent_on, system_id=excluded.system_id,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			c.ID, c.Kind, nullStr(c.Subject), string(parts), nullStr(c.ThreadID),
			nullStr(c.SentOn), nullStr(c.SystemID), vf, txf); err != nil {
			return fmt.Errorf("seed communication %q: %w", c.ID, err)
		}
	}

	for _, ev := range e.Calendar {
		att, err := json.Marshal(ev.Attendees)
		if err != nil {
			return fmt.Errorf("encode attendees for %q: %w", ev.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO calendar_event (id, title, starts_at, attendees, kind,
			                            valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				title=excluded.title, starts_at=excluded.starts_at, attendees=excluded.attendees,
				kind=excluded.kind, valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			ev.ID, ev.Title, ev.StartsAt, string(att), nullStr(ev.Kind), vf, txf); err != nil {
			return fmt.Errorf("seed calendar event %q: %w", ev.ID, err)
		}
	}

	for _, r := range e.Risks {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO risk (id, title, likelihood, impact, owner_dept, control_id,
			                  valid_from, valid_to, tx_from, tx_to)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL)
			ON CONFLICT(id) DO UPDATE SET
				title=excluded.title, likelihood=excluded.likelihood, impact=excluded.impact,
				owner_dept=excluded.owner_dept, control_id=excluded.control_id,
				valid_from=excluded.valid_from, tx_from=excluded.tx_from`,
			r.ID, r.Title, nullStr(r.Likelihood), nullStr(r.Impact),
			nullStr(r.OwnerDept), nullStr(r.ControlID), vf, txf); err != nil {
			return fmt.Errorf("seed risk %q: %w", r.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit enterprise seed: %w", err)
	}
	return nil
}

// --- read models -------------------------------------------------------------

// EnterpriseSummary is the firm's posture at a glance.
type EnterpriseSummary struct {
	AsOf        string           `json:"as_of"`
	Firm        FirmView         `json:"firm"`
	Departments []DepartmentView `json:"departments"`
	Counts      map[string]int   `json:"counts"`
	Gaps        []EnterpriseGap  `json:"gaps"`
	Systems     []SystemView     `json:"systems"`
	Registers   []RegisterView   `json:"registers"`
}

// FirmView is the regulated entity header.
type FirmView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	PAN      string `json:"pan"`
	MetaJSON string `json:"meta_json"`
}

// DepartmentView is a department with its head and headcount.
type DepartmentView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Function  string `json:"function"`
	HeadID    string `json:"head_employee_id"`
	HeadName  string `json:"head_name"`
	Headcount int    `json:"headcount"`
}

// SystemView is a firm system with its read-only connector id.
type SystemView struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Vendor      string `json:"vendor"`
	ConnectorID string `json:"connector_id"`
	Criticality string `json:"criticality"`
	OwnerDept   string `json:"owner_dept"`
}

// RegisterView is a maintained register with its freshness.
type RegisterView struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	RowCount    int    `json:"row_count"`
	Source      string `json:"source_system"`
	LastUpdated string `json:"last_updated"`
	OwnerDept   string `json:"owner_dept"`
	StaleDays   int    `json:"stale_days"`
}

// EnterpriseGap is a discovered compliance gap. Every one of these is the RESULT
// of a query over the graph - none is a constant anywhere in the code.
type EnterpriseGap struct {
	Kind    string   `json:"kind"`
	Title   string   `json:"title"`
	Detail  string   `json:"detail"`
	Count   int      `json:"count"`
	Subject string   `json:"subject,omitempty"`
	Names   []string `json:"names,omitempty"`
}

// EmployeeView is a person with their department name.
type EmployeeView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Role     string   `json:"role"`
	DeptID   string   `json:"department_id"`
	DeptName string   `json:"department_name"`
	Email    string   `json:"email"`
	Certs    []string `json:"certifications"`
	Depth    int      `json:"depth"`
}

// ClientView is a client with the agreement template it is on.
type ClientView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Segment         string `json:"segment"`
	OnboardedOn     string `json:"onboarded_on"`
	RiskProfile     string `json:"risk_profile"`
	AdviserID       string `json:"adviser_id"`
	AdviserName     string `json:"adviser_name"`
	ServiceKind     string `json:"service_kind"`
	TemplateVersion string `json:"template_version"`
	AgreementID     string `json:"agreement_id"`
}

// DocumentView is a firm document with its staleness.
type DocumentView struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Version      int    `json:"version"`
	OwnerDept    string `json:"owner_dept"`
	OwnerName    string `json:"owner_dept_name"`
	Status       string `json:"status"`
	LastReviewed string `json:"last_reviewed"`
	MonthsSince  int    `json:"months_since_review"`
	Stale        bool   `json:"stale"`
}

// EnterpriseSummaryAsOf reconstructs the firm's posture as of a date.
func (s *Store) EnterpriseSummaryAsOf(ctx context.Context, asOf time.Time) (EnterpriseSummary, error) {
	at := domain.RFC3339UTC(asOf)
	out := EnterpriseSummary{AsOf: at, Counts: map[string]int{}}

	// Address the enterprise firm by its own id. The regulatory fixture seeds a
	// separate placeholder entity, and picking "the first investment adviser"
	// silently returned that one instead - the firm on screen was not the firm
	// the enterprise graph describes.
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, kind, COALESCE(pan,''), meta_json FROM entity
		WHERE id = ? AND`+asOfClause, EnterpriseFirmID, at, at).
		Scan(&out.Firm.ID, &out.Firm.Name, &out.Firm.Kind, &out.Firm.PAN, &out.Firm.MetaJSON)
	if err != nil && err != sql.ErrNoRows {
		return EnterpriseSummary{}, fmt.Errorf("load firm as-of %s: %w", at, err)
	}

	for table, key := range map[string]string{
		"department": "departments", "employee": "employees", "client": "clients",
		"agreement": "agreements", "document": "documents", "register": "registers",
		"system": "systems", "risk": "risks", "communication": "communications",
		"calendar_event": "calendar_events",
	} {
		var n int
		// The table name comes from this fixed map, never from input.
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM `+table+` WHERE`+asOfClause, at, at).Scan(&n); err != nil {
			return EnterpriseSummary{}, fmt.Errorf("count %s as-of %s: %w", table, at, err)
		}
		out.Counts[key] = n
	}

	depts, err := s.listDepartments(ctx, at)
	if err != nil {
		return EnterpriseSummary{}, err
	}
	out.Departments = depts

	systems, err := s.ListSystems(ctx, asOf)
	if err != nil {
		return EnterpriseSummary{}, err
	}
	out.Systems = systems

	registers, err := s.ListRegisters(ctx, asOf)
	if err != nil {
		return EnterpriseSummary{}, err
	}
	out.Registers = registers

	gaps, err := s.DetectEnterpriseGaps(ctx, asOf)
	if err != nil {
		return EnterpriseSummary{}, err
	}
	out.Gaps = gaps
	return out, nil
}

func (s *Store) listDepartments(ctx context.Context, at string) ([]DepartmentView, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.name, COALESCE(d.function,''), COALESCE(d.head_employee_id,''),
		       COALESCE(h.name,''),
		       (SELECT COUNT(*) FROM employee e
		         WHERE e.department_id = d.id
		           AND e.valid_from <= ? AND (e.valid_to IS NULL OR e.valid_to > ?) AND e.tx_to IS NULL)
		FROM department d
		LEFT JOIN employee h ON h.id = d.head_employee_id AND h.tx_to IS NULL
		WHERE d.valid_from <= ? AND (d.valid_to IS NULL OR d.valid_to > ?) AND d.tx_to IS NULL
		ORDER BY d.name`, at, at, at, at)
	if err != nil {
		return nil, fmt.Errorf("list departments as-of %s: %w", at, err)
	}
	defer rows.Close()

	var out []DepartmentView
	for rows.Next() {
		var d DepartmentView
		if err := rows.Scan(&d.ID, &d.Name, &d.Function, &d.HeadID, &d.HeadName, &d.Headcount); err != nil {
			return nil, fmt.Errorf("scan department: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListSystems returns the firm's systems as of a date.
func (s *Store) ListSystems(ctx context.Context, asOf time.Time) ([]SystemView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, COALESCE(vendor,''), COALESCE(connector_id,''),
		       COALESCE(criticality,''), COALESCE(owner_dept,'')
		FROM system WHERE`+asOfClause+`ORDER BY kind`, at, at)
	if err != nil {
		return nil, fmt.Errorf("list systems as-of %s: %w", at, err)
	}
	defer rows.Close()

	var out []SystemView
	for rows.Next() {
		var v SystemView
		if err := rows.Scan(&v.ID, &v.Kind, &v.Vendor, &v.ConnectorID, &v.Criticality, &v.OwnerDept); err != nil {
			return nil, fmt.Errorf("scan system: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ListRegisters returns the firm's registers with their staleness in days.
func (s *Store) ListRegisters(ctx context.Context, asOf time.Time) ([]RegisterView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, row_count, COALESCE(source_system,''), COALESCE(last_updated,''),
		       COALESCE(owner_dept,'')
		FROM register WHERE`+asOfClause+`ORDER BY kind`, at, at)
	if err != nil {
		return nil, fmt.Errorf("list registers as-of %s: %w", at, err)
	}
	defer rows.Close()

	var out []RegisterView
	for rows.Next() {
		var v RegisterView
		if err := rows.Scan(&v.ID, &v.Kind, &v.RowCount, &v.Source, &v.LastUpdated, &v.OwnerDept); err != nil {
			return nil, fmt.Errorf("scan register: %w", err)
		}
		v.StaleDays = daysBetween(v.LastUpdated, asOf)
		out = append(out, v)
	}
	return out, rows.Err()
}

// OrgChart walks employee.manager_id downwards from the top of the firm.
//
// The recursive CTE carries an explicit depth counter and stops at maxOrgDepth.
// A manager cycle in the data would otherwise make this query never terminate,
// and "the fixture is well-formed" is not a guarantee a query should depend on.
func (s *Store) OrgChart(ctx context.Context, asOf time.Time) ([]EmployeeView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE org(id, name, role, department_id, email, certifications, depth) AS (
			SELECT id, name, role, department_id, COALESCE(email,''), certifications, 0
			FROM employee
			WHERE manager_id IS NULL
			  AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL
			UNION ALL
			SELECT e.id, e.name, e.role, e.department_id, COALESCE(e.email,''), e.certifications, o.depth + 1
			FROM employee e
			JOIN org o ON e.manager_id = o.id
			WHERE o.depth < ?
			  AND e.valid_from <= ? AND (e.valid_to IS NULL OR e.valid_to > ?) AND e.tx_to IS NULL
		)
		SELECT o.id, o.name, o.role, COALESCE(o.department_id,''), COALESCE(d.name,''),
		       o.email, o.certifications, o.depth
		FROM org o
		LEFT JOIN department d ON d.id = o.department_id AND d.tx_to IS NULL
		ORDER BY o.depth, o.name`, at, at, maxOrgDepth, at, at)
	if err != nil {
		return nil, fmt.Errorf("org chart as-of %s: %w", at, err)
	}
	defer rows.Close()

	var out []EmployeeView
	for rows.Next() {
		var (
			v     EmployeeView
			certs string
		)
		if err := rows.Scan(&v.ID, &v.Name, &v.Role, &v.DeptID, &v.DeptName, &v.Email, &certs, &v.Depth); err != nil {
			return nil, fmt.Errorf("scan employee: %w", err)
		}
		_ = json.Unmarshal([]byte(certs), &v.Certs)
		out = append(out, v)
	}
	return out, rows.Err()
}

// ClientQuery filters the client list.
type ClientQuery struct {
	AsOf            time.Time
	Segment         string
	AdviserID       string
	TemplateVersion string // "v1" / "v2"
	Limit           int
}

// ListClients returns clients joined to their current agreement.
//
// The agreement join is as-of aware, which is what makes the time-travel claim
// real: as of a date before the v2 re-papering, every client comes back on v1.
func (s *Store) ListClients(ctx context.Context, q ClientQuery) ([]ClientView, error) {
	at := domain.RFC3339UTC(q.AsOf)
	sql := `
		SELECT c.id, c.name, c.segment, c.onboarded_on, COALESCE(c.risk_profile,''),
		       COALESCE(c.adviser_id,''), COALESCE(a.name,''), c.service_kind,
		       COALESCE(g.template_version,''), COALESCE(g.id,'')
		FROM client c
		LEFT JOIN employee a ON a.id = c.adviser_id AND a.tx_to IS NULL
		LEFT JOIN agreement g ON g.client_id = c.id
		     AND g.valid_from <= ? AND (g.valid_to IS NULL OR g.valid_to > ?) AND g.tx_to IS NULL
		WHERE c.valid_from <= ? AND (c.valid_to IS NULL OR c.valid_to > ?) AND c.tx_to IS NULL`
	args := []any{at, at, at, at}

	if q.Segment != "" {
		sql += ` AND c.segment = ?`
		args = append(args, q.Segment)
	}
	if q.AdviserID != "" {
		sql += ` AND c.adviser_id = ?`
		args = append(args, q.AdviserID)
	}
	if q.TemplateVersion != "" {
		// COALESCE so a client with no in-force agreement is treated as having
		// no template rather than being silently excluded.
		sql += ` AND COALESCE(g.template_version,'') = ?`
		args = append(args, q.TemplateVersion)
	}
	sql += ` ORDER BY c.id`
	if q.Limit > 0 {
		sql += ` LIMIT ?`
		args = append(args, q.Limit)
	}

	rows, err := s.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list clients as-of %s: %w", at, err)
	}
	defer rows.Close()

	var out []ClientView
	for rows.Next() {
		var v ClientView
		if err := rows.Scan(&v.ID, &v.Name, &v.Segment, &v.OnboardedOn, &v.RiskProfile,
			&v.AdviserID, &v.AdviserName, &v.ServiceKind, &v.TemplateVersion, &v.AgreementID); err != nil {
			return nil, fmt.Errorf("scan client: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// staleReviewMonths is the annual-review window. A policy not reviewed within
// 12 months breaches an annual-review obligation.
const staleReviewMonths = 12

// ListDocuments returns firm documents; staleOnly restricts to those past their
// annual review.
func (s *Store) ListDocuments(ctx context.Context, asOf time.Time, staleOnly bool) ([]DocumentView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.kind, d.title, d.version, COALESCE(d.owner_dept,''),
		       COALESCE(dep.name,''), d.status, COALESCE(d.last_reviewed,'')
		FROM document d
		LEFT JOIN department dep ON dep.id = d.owner_dept AND dep.tx_to IS NULL
		WHERE d.valid_from <= ? AND (d.valid_to IS NULL OR d.valid_to > ?) AND d.tx_to IS NULL
		ORDER BY d.kind, d.title`, at, at)
	if err != nil {
		return nil, fmt.Errorf("list documents as-of %s: %w", at, err)
	}
	defer rows.Close()

	var out []DocumentView
	for rows.Next() {
		var v DocumentView
		if err := rows.Scan(&v.ID, &v.Kind, &v.Title, &v.Version, &v.OwnerDept,
			&v.OwnerName, &v.Status, &v.LastReviewed); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		v.MonthsSince = monthsBetween(v.LastReviewed, asOf)
		v.Stale = v.MonthsSince >= staleReviewMonths
		if staleOnly && !v.Stale {
			continue
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// DetectEnterpriseGaps finds the firm's compliance gaps BY QUERY.
//
// Nothing here reads a list of known problems. Each gap is the result of a
// traversal over the seeded graph, which is what makes the demo honest: change
// the data and the gaps change with it.
func (s *Store) DetectEnterpriseGaps(ctx context.Context, asOf time.Time) ([]EnterpriseGap, error) {
	at := domain.RFC3339UTC(asOf)
	var gaps []EnterpriseGap

	// 1. Clients whose in-force agreement is NOT on the current template.
	var v1Count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM client c
		JOIN agreement g ON g.client_id = c.id
		     AND g.valid_from <= ? AND (g.valid_to IS NULL OR g.valid_to > ?) AND g.tx_to IS NULL
		WHERE g.template_version <> 'v2'
		  AND c.valid_from <= ? AND (c.valid_to IS NULL OR c.valid_to > ?) AND c.tx_to IS NULL`,
		at, at, at, at).Scan(&v1Count); err != nil {
		return nil, fmt.Errorf("count clients on the superseded template: %w", err)
	}
	if v1Count > 0 {
		gaps = append(gaps, EnterpriseGap{
			Kind:  "agreement_template",
			Title: "Clients on a superseded agreement template",
			Detail: "Their in-force agreement predates the current template, so it does not " +
				"incorporate the standardized terms.",
			Count: v1Count,
		})
	}

	// 2. Employees with an incomplete training record in the latest period.
	trainingNames, err := s.employeesMissingTraining(ctx, at)
	if err != nil {
		return nil, err
	}
	if len(trainingNames) > 0 {
		gaps = append(gaps, EnterpriseGap{
			Kind:   "training",
			Title:  "Employees without current-period training",
			Detail: "A mandatory training record exists with no completion date.",
			Count:  len(trainingNames),
			Names:  trainingNames,
		})
	}

	// 3. Segregation: an adviser holding BOTH advisory and distribution clients.
	//    Discoverable only by grouping clients by adviser - nothing in the data
	//    labels this adviser as a problem.
	breaches, err := s.segregationBreaches(ctx, at)
	if err != nil {
		return nil, err
	}
	gaps = append(gaps, breaches...)

	// 4. Registers not updated recently.
	registers, err := s.ListRegisters(ctx, asOf)
	if err != nil {
		return nil, err
	}
	for _, r := range registers {
		if r.StaleDays >= 60 {
			gaps = append(gaps, EnterpriseGap{
				Kind:    "register_freshness",
				Title:   "Register not updated recently",
				Detail:  fmt.Sprintf("The %s register was last updated %d days ago.", r.Kind, r.StaleDays),
				Count:   1,
				Subject: r.ID,
			})
		}
	}

	// 5. Documents past their annual review.
	docs, err := s.ListDocuments(ctx, asOf, true)
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		gaps = append(gaps, EnterpriseGap{
			Kind:    "document_review",
			Title:   "Policy past its annual review",
			Detail:  fmt.Sprintf("%q was last reviewed %d months ago.", d.Title, d.MonthsSince),
			Count:   1,
			Subject: d.ID,
		})
	}

	return gaps, nil
}

// employeesMissingTraining returns the names of employees whose most recent
// training period has no completion date.
func (s *Store) employeesMissingTraining(ctx context.Context, at string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.name
		FROM training t
		JOIN employee e ON e.id = t.employee_id AND e.tx_to IS NULL
		WHERE t.completed_on IS NULL
		  AND t.period = (SELECT MAX(period) FROM training WHERE tx_to IS NULL)
		  AND t.valid_from <= ? AND (t.valid_to IS NULL OR t.valid_to > ?) AND t.tx_to IS NULL
		ORDER BY e.name`, at, at)
	if err != nil {
		return nil, fmt.Errorf("find employees missing training: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan employee name: %w", err)
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// segregationBreaches finds advisers serving both advisory and distribution
// clients - a clause 4.2 breach.
func (s *Store) segregationBreaches(ctx context.Context, at string) ([]EnterpriseGap, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.adviser_id, COALESCE(e.name,''),
		       SUM(CASE WHEN c.service_kind = 'advisory' THEN 1 ELSE 0 END),
		       SUM(CASE WHEN c.service_kind = 'distribution' THEN 1 ELSE 0 END)
		FROM client c
		LEFT JOIN employee e ON e.id = c.adviser_id AND e.tx_to IS NULL
		WHERE c.adviser_id IS NOT NULL
		  AND c.valid_from <= ? AND (c.valid_to IS NULL OR c.valid_to > ?) AND c.tx_to IS NULL
		GROUP BY c.adviser_id
		HAVING SUM(CASE WHEN c.service_kind = 'advisory' THEN 1 ELSE 0 END) > 0
		   AND SUM(CASE WHEN c.service_kind = 'distribution' THEN 1 ELSE 0 END) > 0
		ORDER BY c.adviser_id`, at, at)
	if err != nil {
		return nil, fmt.Errorf("find segregation breaches: %w", err)
	}
	defer rows.Close()

	var out []EnterpriseGap
	for rows.Next() {
		var (
			id, name          string
			advisory, distrib int
		)
		if err := rows.Scan(&id, &name, &advisory, &distrib); err != nil {
			return nil, fmt.Errorf("scan segregation breach: %w", err)
		}
		out = append(out, EnterpriseGap{
			Kind:  "segregation",
			Title: "Adviser serving both advisory and distribution clients",
			Detail: fmt.Sprintf("%s holds %d advisory and %d distribution clients, "+
				"which clause 4.2 forbids at client level.", name, advisory, distrib),
			Count:   advisory + distrib,
			Subject: id,
			Names:   []string{name},
		})
	}
	return out, rows.Err()
}

// CorpusClausesFor returns every current clause of the given circulars, for the
// amendment matcher to diff an incoming document against.
func (s *Store) CorpusClausesFor(ctx context.Context, circularIDs []string) ([]ingest.ExistingClause, error) {
	if len(circularIDs) == 0 {
		return nil, nil
	}
	// Build exactly as many placeholders as there are ids: never concatenate the
	// values themselves.
	placeholders := make([]string, len(circularIDs))
	args := make([]any, len(circularIDs))
	for i, id := range circularIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `SELECT row_uid, id, clause_ref, text FROM clause
	      WHERE circular_id IN (` + strings.Join(placeholders, ",") + `) AND tx_to IS NULL
	      ORDER BY ordinal`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("load corpus clauses: %w", err)
	}
	defer rows.Close()

	var out []ingest.ExistingClause
	for rows.Next() {
		var c ingest.ExistingClause
		if err := rows.Scan(&c.RowUID, &c.ID, &c.ClauseRef, &c.Text); err != nil {
			return nil, fmt.Errorf("scan corpus clause: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ImpactedControlView is a control with the enterprise ownership the Phase 3
// fixture added on top of the Phase 4 controls layer.
type ImpactedControlView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	OwnerDept string `json:"owner_dept"`
	OwnerName string `json:"owner_dept_name"`
}

// ControlsForObligation traverses the ONE seam joining the regulatory and
// enterprise namespaces: obligation → control. The control's owning department
// comes from the enterprise graph; the obligation link comes from the regulatory
// graph; neither namespace is merged into the other.
func (s *Store) ControlsForObligation(ctx context.Context, obligationID string, asOf time.Time) ([]ImpactedControlView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT ct.id, ct.name, COALESCE(ct.kind,''),
		       COALESCE(r.owner_dept,''), COALESCE(d.name,'')
		FROM obligation_control oc
		JOIN control ct ON ct.id = oc.control_id AND ct.tx_to IS NULL
		LEFT JOIN risk r ON r.control_id = ct.id AND r.tx_to IS NULL
		LEFT JOIN department d ON d.id = r.owner_dept AND d.tx_to IS NULL
		WHERE oc.obligation_id = ?
		  AND oc.valid_from <= ? AND (oc.valid_to IS NULL OR oc.valid_to > ?) AND oc.tx_to IS NULL
		GROUP BY ct.id
		ORDER BY ct.name`, obligationID, at, at)
	if err != nil {
		return nil, fmt.Errorf("controls for obligation %q: %w", obligationID, err)
	}
	defer rows.Close()

	var out []ImpactedControlView
	for rows.Next() {
		var v ImpactedControlView
		if err := rows.Scan(&v.ID, &v.Name, &v.Kind, &v.OwnerDept, &v.OwnerName); err != nil {
			return nil, fmt.Errorf("scan control: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// --- small date helpers ------------------------------------------------------

// parseDay accepts either a date or an RFC3339 timestamp.
func parseDay(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func daysBetween(from string, to time.Time) int {
	t, ok := parseDay(from)
	if !ok {
		return 0
	}
	d := to.UTC().Sub(t) / (24 * time.Hour)
	if d < 0 {
		return 0
	}
	return int(d)
}

func monthsBetween(from string, to time.Time) int {
	t, ok := parseDay(from)
	if !ok {
		return 0
	}
	months := (to.Year()-t.Year())*12 + int(to.Month()) - int(t.Month())
	if to.Day() < t.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}
