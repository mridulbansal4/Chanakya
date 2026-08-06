package store

import (
	"context"
	"testing"
	"time"

	"chanakya/internal/fixtures"
)

// enterpriseNow is the demo "today" the fixture's relative gaps are expressed
// against (a register 90 days stale, a policy reviewed 14 months ago).
var enterpriseNow = time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)

var enterpriseWorldStart = time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC)

func seedEnterprise(t *testing.T, st *Store) fixtures.Enterprise {
	t.Helper()
	fx, err := fixtures.LoadEnterprise(enterpriseWorldStart, enterpriseNow)
	if err != nil {
		t.Fatalf("LoadEnterprise: %v", err)
	}
	if err := st.SeedEnterprise(context.Background(), fx); err != nil {
		t.Fatalf("SeedEnterprise: %v", err)
	}
	return fx
}

// TestFixtureExactCounts pins the numbers the demo narrative depends on. A later
// refactor of the fixture generator cannot silently drift them: if the MITC
// obligation stops lighting up exactly 118 clients, that is a regression in the
// story, not just in the data.
func TestFixtureExactCounts(t *testing.T) {
	fx, err := fixtures.LoadEnterprise(enterpriseWorldStart, enterpriseNow)
	if err != nil {
		t.Fatalf("LoadEnterprise: %v", err)
	}

	if got := len(fx.Departments); got != 8 {
		t.Errorf("departments = %d, want 8", got)
	}
	if got := len(fx.Employees); got != 24 {
		t.Errorf("employees = %d, want 24", got)
	}
	if got := len(fx.Clients); got != 140 {
		t.Errorf("clients = %d, want 140", got)
	}
	// 140 original agreements plus a second row for each of the 22 re-papered
	// clients, so the supersession is representable in world time.
	if got := len(fx.Agreements); got != 162 {
		t.Errorf("agreement rows = %d, want 162 (140 originals + 22 replacements)", got)
	}
	if got := len(fx.Risks); got != 14 {
		t.Errorf("risks = %d, want 14", got)
	}
	if got := len(fx.Documents); got != 22 {
		t.Errorf("documents = %d, want 22", got)
	}
	if got := len(fx.Systems); got != 7 {
		t.Errorf("systems = %d, want 7", got)
	}

	v2 := 0
	for _, a := range fx.Agreements {
		if a.TemplateVersion == "v2" {
			v2++
		}
	}
	if v2 != 22 {
		t.Errorf("agreements on v2 = %d, want exactly 22 (so 118 remain on v1)", v2)
	}
	superseded := 0
	for _, a := range fx.Agreements {
		if a.SupersededOn != "" {
			superseded++
		}
	}
	if superseded != 22 {
		t.Errorf("superseded v1 agreements = %d, want 22", superseded)
	}
	if 140-v2 != 118 {
		t.Errorf("clients on v1 = %d, want exactly 118", 140-v2)
	}

	// The Principal Officer / Compliance Officer persona the app shell shows.
	var priya bool
	for _, e := range fx.Employees {
		if e.Name == "Priya Menon" {
			priya = true
		}
	}
	if !priya {
		t.Error("Priya Menon (Principal Officer & Compliance Officer) is missing from the fixture")
	}
}

// TestGapsAreDiscoveredByQuery is the heart of Phase 3: every gap comes out of a
// traversal over the seeded graph. None of these numbers is a constant in the
// code - delete a distribution client and the segregation breach disappears.
func TestGapsAreDiscoveredByQuery(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	gaps, err := st.DetectEnterpriseGaps(ctx, enterpriseNow)
	if err != nil {
		t.Fatalf("DetectEnterpriseGaps: %v", err)
	}

	byKind := map[string][]EnterpriseGap{}
	for _, g := range gaps {
		byKind[g.Kind] = append(byKind[g.Kind], g)
	}

	// 1. The 118-client agreement-template gap.
	tmpl := byKind["agreement_template"]
	if len(tmpl) != 1 {
		t.Fatalf("agreement_template gaps = %d, want 1", len(tmpl))
	}
	if tmpl[0].Count != 118 {
		t.Errorf("clients on the superseded template = %d, want 118", tmpl[0].Count)
	}

	// 2. The 3-employee training gap, with NAMES.
	training := byKind["training"]
	if len(training) != 1 {
		t.Fatalf("training gaps = %d, want 1", len(training))
	}
	if training[0].Count != 3 {
		t.Errorf("employees missing training = %d, want 3", training[0].Count)
	}
	if len(training[0].Names) != 3 {
		t.Errorf("training gap names = %v, want 3 named people", training[0].Names)
	}

	// 3. The segregation breach - discoverable ONLY by grouping clients by
	//    adviser and counting distinct service kinds.
	seg := byKind["segregation"]
	if len(seg) != 1 {
		t.Fatalf("segregation breaches = %d, want exactly 1", len(seg))
	}
	if seg[0].Subject != "emp_006" {
		t.Errorf("segregation breach adviser = %q, want emp_006", seg[0].Subject)
	}

	// 4. The stale complaint register.
	var sawComplaint bool
	for _, g := range byKind["register_freshness"] {
		if g.Subject == "reg_complaint" {
			sawComplaint = true
		}
	}
	if !sawComplaint {
		t.Error("the 90-day-stale complaint register was not detected")
	}

	// 5. The cybersecurity policy past its annual review - and ONLY it, so the
	//    deliberate breach is unambiguous rather than one of twenty.
	reviews := byKind["document_review"]
	if len(reviews) != 1 {
		t.Fatalf("document_review gaps = %d, want exactly 1: %+v", len(reviews), reviews)
	}
	if reviews[0].Subject != "doc_pol_cybersecurity" {
		t.Errorf("stale document = %q, want doc_pol_cybersecurity", reviews[0].Subject)
	}

	// 6. Exactly one stale register, likewise.
	if got := len(byKind["register_freshness"]); got != 1 {
		t.Errorf("register_freshness gaps = %d, want exactly 1", got)
	}
}

// TestClientsAsOfTimeTravel proves the enterprise graph is genuinely bi-temporal:
// as of a date BEFORE the v2 re-papering, every client is on v1. This is the
// enterprise half of the time-travel claim - the regulatory half alone would not
// demonstrate it.
func TestClientsAsOfTimeTravel(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	// The fixture signs every v2 agreement on 2025-05-15.
	before := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	nowV1, err := st.ListClients(ctx, ClientQuery{AsOf: enterpriseNow, TemplateVersion: "v1"})
	if err != nil {
		t.Fatalf("ListClients now: %v", err)
	}
	if len(nowV1) != 118 {
		t.Errorf("clients on v1 today = %d, want 118", len(nowV1))
	}

	beforeV2, err := st.ListClients(ctx, ClientQuery{AsOf: before, TemplateVersion: "v2"})
	if err != nil {
		t.Fatalf("ListClients before: %v", err)
	}
	if len(beforeV2) != 0 {
		t.Errorf("clients on v2 as of %s = %d, want 0 - the re-papering had not happened yet",
			before.Format("2006-01-02"), len(beforeV2))
	}

	gapsBefore, err := st.DetectEnterpriseGaps(ctx, before)
	if err != nil {
		t.Fatalf("DetectEnterpriseGaps before: %v", err)
	}
	for _, g := range gapsBefore {
		if g.Kind == "agreement_template" && g.Count != 140 {
			t.Errorf("as of %s, clients on the superseded template = %d, want all 140",
				before.Format("2006-01-02"), g.Count)
		}
	}
}

// TestOrgChartHasDepthGuard: employee.manager_id could theoretically form a
// cycle, and a recursive CTE walking one never terminates. The guard must be in
// the query, not in an assumption about the data - so this test INTRODUCES a
// cycle and requires the query to still return.
func TestOrgChartHasDepthGuard(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	normal, err := st.OrgChart(ctx, enterpriseNow)
	if err != nil {
		t.Fatalf("OrgChart: %v", err)
	}
	if len(normal) != 24 {
		t.Errorf("org chart = %d employees, want all 24", len(normal))
	}

	// Make the Principal Officer report to one of their own reports.
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE employee SET manager_id = 'emp_002' WHERE id = 'emp_001'`); err != nil {
		t.Fatalf("introduce cycle: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := st.OrgChart(ctx, enterpriseNow); err != nil {
			t.Errorf("OrgChart with a cycle returned an error: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("OrgChart did not terminate with a manager cycle - the depth cap is not working")
	}
}

// TestSeedEnterpriseIsIdempotent: re-seeding an unchanged firm changes nothing.
func TestSeedEnterpriseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	first, err := st.EnterpriseSummaryAsOf(ctx, enterpriseNow)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	seedEnterprise(t, st)
	second, err := st.EnterpriseSummaryAsOf(ctx, enterpriseNow)
	if err != nil {
		t.Fatalf("summary after re-seed: %v", err)
	}

	for k, v := range first.Counts {
		if second.Counts[k] != v {
			t.Errorf("re-seeding changed %s: %d -> %d", k, v, second.Counts[k])
		}
	}
}

// TestEnterpriseSummaryBeforeIncorporation: an as-of date before the firm
// existed returns an empty firm, not a firm that always existed.
func TestEnterpriseSummaryBeforeIncorporation(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	seedEnterprise(t, st)

	before := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	summary, err := st.EnterpriseSummaryAsOf(ctx, before)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Counts["employees"] != 0 || summary.Counts["clients"] != 0 {
		t.Errorf("as of %s the firm should not exist yet: %v",
			before.Format("2006-01-02"), summary.Counts)
	}
}
