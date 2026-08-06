package connect

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// registry holds every adapter, keyed by kind.
var (
	registryMu sync.RWMutex
	registry   = map[string]Connector{}
)

// register adds an adapter. Called from init below.
func register(c Connector) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[c.Descriptor().Kind] = c
}

// EnterpriseData is the seeded firm data the mock adapters read from. It is
// supplied by the caller (the API server) so this package has no dependency on
// the store, and so the adapters demonstrably read fixtures rather than reaching
// out to a network.
type EnterpriseData interface {
	Communications(ctx context.Context, kind string, limit int) ([]Record, error)
	CalendarEvents(ctx context.Context, limit int) ([]Record, error)
	Documents(ctx context.Context, limit int) ([]Record, error)
	Registers(ctx context.Context, limit int) ([]Record, error)
	Employees(ctx context.Context, limit int) ([]Record, error)
}

// data is the shared source every mock adapter reads. Nil until the server wires
// it, in which case adapters return an empty, STALE result rather than invented
// records.
var (
	dataMu sync.RWMutex
	data   EnterpriseData
)

// SetEnterpriseData wires the seeded firm data into the mock adapters.
func SetEnterpriseData(d EnterpriseData) {
	dataMu.Lock()
	defer dataMu.Unlock()
	data = d
}

func enterpriseData() EnterpriseData {
	dataMu.RLock()
	defer dataMu.RUnlock()
	return data
}

// mockAdapter is the shared implementation behind all fourteen adapters.
//
// They differ in descriptor and in which query kinds they serve; they are
// identical in the property that matters, which is that none of them can write.
type mockAdapter struct {
	desc Descriptor
	// serves maps a query kind to the fixture reader that answers it. A kind
	// absent from this map is UNSUPPORTED, and Fetch says so.
	serves map[QueryKind]func(ctx context.Context, limit int) ([]Record, error)
}

// Descriptor returns the adapter's self-description.
func (m mockAdapter) Descriptor() Descriptor { return m.desc }

// Health reports readiness. A mock adapter is healthy whenever its fixture
// source is wired; it never contacts a network, so there is nothing else to
// check.
func (m mockAdapter) Health(ctx context.Context) Status {
	now := time.Now().UTC().Format(time.RFC3339)
	if enterpriseData() == nil {
		return Status{OK: "degraded", Detail: "no seeded enterprise data is wired", CheckedAt: now}
	}
	return Status{OK: "ok", Detail: "reading seeded fixtures; no network access", CheckedAt: now}
}

// Fetch returns records of the requested kind.
//
// An unsupported kind returns a TYPED ERROR, and a supported kind with no data
// returns an empty result marked Stale. Neither path fabricates records: invented
// evidence in a compliance audit trail is the worst failure this system could
// have.
func (m mockAdapter) Fetch(ctx context.Context, q Query) (Result, error) {
	read, ok := m.serves[q.Kind]
	if !ok {
		return Result{}, fmt.Errorf("connector %q, query kind %q: %w",
			m.desc.ID, q.Kind, ErrUnsupportedQuery)
	}
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	src := enterpriseData()
	if src == nil {
		return Result{Kind: q.Kind, Records: []Record{}, Stale: true, Source: m.desc.ID}, nil
	}
	records, err := read(ctx, limit)
	if err != nil {
		return Result{}, fmt.Errorf("connector %q fetch %q: %w", m.desc.ID, q.Kind, err)
	}
	return Result{Kind: q.Kind, Records: records, Source: m.desc.ID}, nil
}

// simulated wraps a mock adapter and reports itself in simulated mode. The data
// is the same seeded fixture set; only the declared mode changes, because
// generating time-varying evidence is not something the demo needs and inventing
// it would violate the no-fabrication rule above.
type simulated struct{ base Connector }

func (s simulated) Descriptor() Descriptor {
	d := s.base.Descriptor()
	d.Mode = ModeSimulated
	return d
}
func (s simulated) Health(ctx context.Context) Status { return s.base.Health(ctx) }
func (s simulated) Fetch(ctx context.Context, q Query) (Result, error) {
	return s.base.Fetch(ctx, q)
}

// readers, shared by adapters that serve the same shape of record.
func emails(ctx context.Context, limit int) ([]Record, error) {
	return enterpriseData().Communications(ctx, "email", limit)
}
func meetings(ctx context.Context, limit int) ([]Record, error) {
	return enterpriseData().Communications(ctx, "meeting", limit)
}
func events(ctx context.Context, limit int) ([]Record, error) {
	return enterpriseData().CalendarEvents(ctx, limit)
}
func files(ctx context.Context, limit int) ([]Record, error) {
	return enterpriseData().Documents(ctx, limit)
}
func registers(ctx context.Context, limit int) ([]Record, error) {
	return enterpriseData().Registers(ctx, limit)
}
func people(ctx context.Context, limit int) ([]Record, error) {
	return enterpriseData().Employees(ctx, limit)
}

// perHour builds a rate limit.
func perHour(n int) Limit {
	return Limit{Requests: n, Per: time.Hour, PerLabel: "hour"}
}

// The fourteen adapters plus the webhook receiver.
//
// Every one is ReadOnly:true and Mode:mock, with READ-only scopes. This list IS
// the safety story - it is surfaced verbatim at GET /api/connectors so a reviewer
// can see, rather than be told, that nothing here can write.
func init() {
	adapters := []struct {
		id, kind, vendor, desc string
		scopes                 []string
		rate                   int
		serves                 map[QueryKind]func(context.Context, int) ([]Record, error)
	}{
		{"conn_gmail", "gmail", "Google Workspace",
			"Reads message metadata for evidence of client communication. Cannot send, modify or delete mail.",
			[]string{"gmail.readonly"}, 250,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryMessages: emails}},

		{"conn_outlook", "outlook", "Microsoft 365",
			"Reads message metadata from Exchange. Cannot send, modify or delete mail.",
			[]string{"Mail.Read"}, 250,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryMessages: emails}},

		{"conn_google_calendar", "google_calendar", "Google Calendar",
			"Reads events as evidence of committee and review meetings. Cannot create or cancel events.",
			[]string{"calendar.readonly"}, 200,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryEvents: events}},

		{"conn_outlook_calendar", "outlook_calendar", "Microsoft 365 Calendar",
			"Reads events from Exchange calendars. Cannot create or cancel events.",
			[]string{"Calendars.Read"}, 200,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryEvents: events}},

		{"conn_drive", "drive", "Google Drive",
			"Reads document metadata and versions. Cannot upload, edit or delete files.",
			[]string{"drive.metadata.readonly"}, 300,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryFiles: files}},

		{"conn_onedrive", "onedrive", "Microsoft OneDrive",
			"Reads document metadata and versions. Cannot upload, edit or delete files.",
			[]string{"Files.Read.All"}, 300,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryFiles: files}},

		{"conn_sharepoint", "sharepoint", "Microsoft SharePoint",
			"Reads policy libraries and their version history. Cannot publish or delete.",
			[]string{"Sites.Read.All"}, 200,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryFiles: files}},

		{"conn_dropbox", "dropbox", "Dropbox",
			"Reads file metadata for evidence of retained records. Cannot write.",
			[]string{"files.metadata.read"}, 200,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryFiles: files}},

		{"conn_jira", "jira", "Atlassian Jira",
			"Reads issues as evidence of remediation work. Cannot create, transition or comment on issues.",
			[]string{"read:jira-work"}, 300,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryIssues: registers}},

		{"conn_slack", "slack", "Slack",
			"Reads channel messages for evidence of escalation and approval. Cannot post.",
			[]string{"channels:history", "channels:read"}, 200,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryMessages: meetings}},

		{"conn_teams", "teams", "Microsoft Teams",
			"Reads channel messages for evidence of escalation and approval. Cannot post.",
			[]string{"ChannelMessage.Read.All"}, 200,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryMessages: meetings}},

		{"conn_notion", "notion", "Notion",
			"Reads pages documenting procedures. Cannot create or edit pages.",
			[]string{"read_content"}, 180,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryPages: files}},

		{"conn_confluence", "confluence", "Atlassian Confluence",
			"Reads spaces and pages documenting procedures. Cannot create or edit pages.",
			[]string{"read:confluence-content.all"}, 180,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryPages: files}},

		{"conn_internal_rest", "internal_rest", "Internal REST",
			"Reads the firm's own CRM, HRMS, billing and archive endpoints. GET only.",
			[]string{"read"}, 600,
			map[QueryKind]func(context.Context, int) ([]Record, error){
				QueryRecords: registers, QueryFiles: people}},

		{"conn_webhook", "webhook", "Inbound webhook",
			"RECEIVES notifications from firm systems. Inbound only - it has no outbound capability at all.",
			[]string{"receive"}, 1000,
			map[QueryKind]func(context.Context, int) ([]Record, error){QueryWebhooks: registers}},
	}

	for _, a := range adapters {
		serves := make(map[QueryKind]func(context.Context, int) ([]Record, error), len(a.serves))
		for k, fn := range a.serves {
			serves[k] = fn
		}
		register(mockAdapter{
			desc: Descriptor{
				ID:     a.id,
				Kind:   a.kind,
				Vendor: a.vendor,
				Mode:   ModeMock,
				// Always true. Never conditional, never configurable.
				ReadOnly:    true,
				Scopes:      a.scopes,
				RateLimit:   perHour(a.rate),
				Description: a.desc,
			},
			serves: serves,
		})
	}
}
