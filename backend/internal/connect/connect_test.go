package connect

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// TestEveryConnectorIsReadOnly is the phase's non-negotiable assertion.
func TestEveryConnectorIsReadOnly(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("no connectors registered")
	}
	for _, c := range all {
		d := c.Descriptor()
		if !d.ReadOnly {
			t.Errorf("connector %q is not read-only", d.ID)
		}
		if d.ID == "" || d.Kind == "" || d.Vendor == "" {
			t.Errorf("connector %+v is missing identity fields", d)
		}
		if len(d.Scopes) == 0 {
			t.Errorf("connector %q declares no scopes", d.ID)
		}
		if d.RateLimit.Requests <= 0 {
			t.Errorf("connector %q declares no rate limit", d.ID)
		}
	}
}

// TestConnectorInterfaceHasNoWriteMethod enforces read-only through the TYPE
// SYSTEM rather than through convention.
//
// The Connector interface must expose exactly three methods - Descriptor,
// Health and Fetch. Adding any write-shaped method would be caught here, and
// that is the point: a connector cannot write to a customer system because the
// vocabulary to do so does not exist in the interface it satisfies.
func TestConnectorInterfaceHasNoWriteMethod(t *testing.T) {
	iface := reflect.TypeOf((*Connector)(nil)).Elem()

	allowed := map[string]bool{"Descriptor": true, "Health": true, "Fetch": true}
	if iface.NumMethod() != len(allowed) {
		t.Errorf("Connector has %d methods, want exactly %d", iface.NumMethod(), len(allowed))
	}
	for i := 0; i < iface.NumMethod(); i++ {
		name := iface.Method(i).Name
		if !allowed[name] {
			t.Errorf("Connector exposes an unexpected method %q - the interface must stay read-only", name)
		}
	}

	// And explicitly: no method whose name suggests mutation.
	for _, forbidden := range []string{"Write", "Update", "Delete", "Create", "Send", "Post", "Put", "Patch"} {
		if _, ok := iface.MethodByName(forbidden); ok {
			t.Errorf("Connector exposes %q - connectors must never write to a customer system", forbidden)
		}
	}
}

// TestFourteenAdaptersPlusWebhook: the registry is the safety story, so its
// contents are pinned.
func TestFourteenAdaptersPlusWebhook(t *testing.T) {
	want := []string{
		"confluence", "drive", "dropbox", "gmail", "google_calendar", "internal_rest",
		"jira", "notion", "onedrive", "outlook", "outlook_calendar", "sharepoint",
		"slack", "teams", "webhook",
	}
	got := Kinds()
	if len(got) != len(want) {
		t.Fatalf("registered %d connectors, want %d: %v", len(got), len(want), got)
	}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("connector[%d] = %q, want %q", i, got[i], k)
		}
	}
	// Fourteen data adapters plus the webhook receiver.
	if len(got) != 15 {
		t.Errorf("expected 14 adapters + 1 webhook receiver = 15, got %d", len(got))
	}
}

// TestDefaultModeIsMockWithNoNetwork: the default path reads fixtures.
func TestDefaultModeIsMock(t *testing.T) {
	t.Setenv("CHANAKYA_SIMULATE", "")
	for _, kind := range Kinds() {
		c, err := SelectConnector(kind)
		if err != nil {
			t.Errorf("select %q: %v", kind, err)
			continue
		}
		if got := c.Descriptor().Mode; got != ModeMock {
			t.Errorf("connector %q default mode = %q, want mock", kind, got)
		}
	}
}

// TestSimulateEnvSelectsSimulated mirrors llm.SelectExtractor's pattern.
func TestSimulateEnvSelectsSimulated(t *testing.T) {
	t.Setenv("CHANAKYA_SIMULATE", "1")
	c, err := SelectConnector("gmail")
	if err != nil {
		t.Fatalf("select gmail: %v", err)
	}
	if got := c.Descriptor().Mode; got != ModeSimulated {
		t.Errorf("mode = %q, want simulated", got)
	}
	if !c.Descriptor().ReadOnly {
		t.Error("a simulated connector must still be read-only")
	}
}

// TestLiveTokenIsRefusedNotSilentlyMocked: a configured live credential must
// produce an explicit error. Silently serving mock data as though it came from
// the firm's real systems would be worse than refusing.
func TestLiveTokenIsRefusedNotSilentlyMocked(t *testing.T) {
	t.Setenv("CHANAKYA_CONNECTOR_GMAIL_TOKEN", "pretend-oauth-token")
	c, err := SelectConnector("gmail")
	if err == nil {
		t.Fatalf("live mode returned a connector (%v) instead of an error", c.Descriptor())
	}
	if c != nil {
		t.Error("a refused selection must not also return a connector")
	}
}

// TestUnknownKindIsAnError.
func TestUnknownKindIsAnError(t *testing.T) {
	if _, err := SelectConnector("salesforce"); err == nil {
		t.Error("an unregistered kind must be an error")
	}
}

// TestUnsupportedQueryReturnsTypedError, never fabricated records.
//
// This is the edge case the phase names explicitly: a mock adapter asked for a
// kind it has no fixture data for must return a typed error or an empty stale
// result. Inventing plausible records would put fabricated evidence into a
// compliance audit trail.
func TestUnsupportedQueryReturnsTypedError(t *testing.T) {
	c, err := SelectConnector("gmail")
	if err != nil {
		t.Fatalf("select gmail: %v", err)
	}
	res, err := c.Fetch(context.Background(), Query{Kind: QueryIssues})
	if !errors.Is(err, ErrUnsupportedQuery) {
		t.Fatalf("err = %v, want ErrUnsupportedQuery", err)
	}
	if len(res.Records) != 0 {
		t.Errorf("an unsupported query returned %d records - it must return none", len(res.Records))
	}
}

// TestNoDataSourceYieldsStaleNotFabricated: with no seeded data wired, a
// supported query returns an EMPTY result marked stale.
func TestNoDataSourceYieldsStale(t *testing.T) {
	SetEnterpriseData(nil)
	c, err := SelectConnector("gmail")
	if err != nil {
		t.Fatalf("select gmail: %v", err)
	}
	res, err := c.Fetch(context.Background(), Query{Kind: QueryMessages})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !res.Stale {
		t.Error("a result with no data source must be marked stale")
	}
	if len(res.Records) != 0 {
		t.Errorf("got %d records with no data source - records must never be fabricated", len(res.Records))
	}
}

// stubData serves fixed records, standing in for the seeded enterprise graph.
type stubData struct{}

func (stubData) Communications(context.Context, string, int) ([]Record, error) {
	return []Record{{ID: "com_1", Title: "MITC impact assessment"}}, nil
}
func (stubData) CalendarEvents(context.Context, int) ([]Record, error) {
	return []Record{{ID: "cal_1", Title: "Compliance committee"}}, nil
}
func (stubData) Documents(context.Context, int) ([]Record, error) {
	return []Record{{ID: "doc_1", Title: "Fee Disclosure Policy"}}, nil
}
func (stubData) Registers(context.Context, int) ([]Record, error) {
	return []Record{{ID: "reg_1", Title: "complaint register"}}, nil
}
func (stubData) Employees(context.Context, int) ([]Record, error) {
	return []Record{{ID: "emp_1", Title: "Priya Menon"}}, nil
}

// TestMockAdaptersReadFixtures: with data wired, every adapter serves its own
// query kinds from the seeded graph - and still makes no network call.
func TestMockAdaptersReadFixtures(t *testing.T) {
	SetEnterpriseData(stubData{})
	t.Cleanup(func() { SetEnterpriseData(nil) })

	served := 0
	for _, c := range All() {
		health := c.Health(context.Background())
		if health.OK != "ok" {
			t.Errorf("connector %q health = %q (%s)", c.Descriptor().ID, health.OK, health.Detail)
		}
		for _, kind := range []QueryKind{
			QueryMessages, QueryEvents, QueryFiles, QueryIssues,
			QueryPages, QueryRecords, QueryWebhooks,
		} {
			res, err := c.Fetch(context.Background(), Query{Kind: kind, Limit: 5})
			if errors.Is(err, ErrUnsupportedQuery) {
				continue
			}
			if err != nil {
				t.Errorf("connector %q fetch %q: %v", c.Descriptor().ID, kind, err)
				continue
			}
			if len(res.Records) == 0 {
				t.Errorf("connector %q serves %q but returned nothing", c.Descriptor().ID, kind)
			}
			if res.Source == "" {
				t.Errorf("connector %q returned records with no source attribution", c.Descriptor().ID)
			}
			served++
		}
	}
	if served == 0 {
		t.Fatal("no connector served any query kind")
	}
}
