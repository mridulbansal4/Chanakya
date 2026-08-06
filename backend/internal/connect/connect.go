// Package connect is CHANAKYA's evidence-connector layer.
//
// SAFETY ROLE - THE CENTRAL ONE. Connectors are READ-ONLY, and that is enforced
// by the TYPE SYSTEM rather than by convention. The Connector interface exposes
// exactly one data method, Fetch. There is no Write, no Update, no Delete, no
// Send - not unimplemented, not returning "not supported", but ABSENT. A
// connector cannot write to a customer system because the vocabulary to do so
// does not exist in the interface it must satisfy.
//
// This matters more than any other guarantee here. CHANAKYA reads a firm's Gmail,
// its Jira, its document store. A compliance tool that could also write to them
// is a tool that can destroy the evidence it exists to preserve.
//
// Descriptor.ReadOnly is always true, and a test asserts it for every registered
// adapter. It exists as a field so the property is VISIBLE in the API response,
// not just true in the code.
package connect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Mode is how a connector obtains its data.
type Mode string

const (
	// ModeMock reads static fixtures from the seeded enterprise graph. Zero
	// network calls. This is the default and the only mode CI exercises.
	ModeMock Mode = "mock"
	// ModeSimulated generates time-varying data, deterministic by seed.
	ModeSimulated Mode = "simulated"
	// ModeLive talks to a real API with READ scopes only. No adapter ships in
	// this mode; live integration is an explicit non-goal for the hackathon.
	ModeLive Mode = "live"
)

// Limit is a connector's rate limit.
type Limit struct {
	Requests int           `json:"requests"`
	Per      time.Duration `json:"-"`
	PerLabel string        `json:"per"`
}

// Descriptor is a connector's self-description, returned by the API so the
// read-only property is inspectable rather than merely asserted in prose.
type Descriptor struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Vendor string `json:"vendor"`
	Mode   Mode   `json:"mode"`
	// ReadOnly is ALWAYS true. See the package comment.
	ReadOnly  bool     `json:"read_only"`
	Scopes    []string `json:"scopes"`
	RateLimit Limit    `json:"rate_limit"`
	// Description says what the connector reads and, importantly, what it
	// cannot do.
	Description string `json:"description"`
}

// Status is a connector's health.
type Status struct {
	OK        string `json:"ok"`
	Detail    string `json:"detail"`
	CheckedAt string `json:"checked_at"`
}

// QueryKind names a class of records a connector can return.
type QueryKind string

const (
	QueryMessages QueryKind = "messages"
	QueryEvents   QueryKind = "events"
	QueryFiles    QueryKind = "files"
	QueryIssues   QueryKind = "issues"
	QueryPages    QueryKind = "pages"
	QueryRecords  QueryKind = "records"
	QueryWebhooks QueryKind = "webhooks"
)

// Query asks a connector for records.
type Query struct {
	Kind  QueryKind
	Since time.Time
	Limit int
}

// Record is one returned item, deliberately generic: CHANAKYA reads evidence
// metadata, not the contents of a firm's mailbox.
type Record struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Actor     string            `json:"actor"`
	Timestamp string            `json:"timestamp"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// Result is a Fetch response.
type Result struct {
	Kind    QueryKind `json:"kind"`
	Records []Record  `json:"records"`
	// Stale marks data the connector could not freshly obtain. It is how an
	// adapter says "I have nothing for this" without inventing records.
	Stale bool `json:"stale"`
	// Source names where the data came from, for the audit trail.
	Source string `json:"source"`
}

// ErrUnsupportedQuery is returned when an adapter has no data of the requested
// kind.
//
// This is the honest answer, and it is the ONLY acceptable one. Fabricating
// plausible-looking records to fill a gap would put invented evidence into a
// compliance audit trail - the single worst thing this system could do.
var ErrUnsupportedQuery = errors.New("connector does not serve this query kind")

// Connector is the read-only interface every adapter satisfies.
//
// Fetch is the ONLY data method. There is no write method, by construction.
type Connector interface {
	Descriptor() Descriptor
	Health(ctx context.Context) Status
	Fetch(ctx context.Context, q Query) (Result, error)
}

// SelectConnector chooses an adapter for a kind, mirroring llm.SelectExtractor:
//
//  1. a per-kind env token (e.g. CHANAKYA_CONNECTOR_GMAIL_TOKEN) -> live
//  2. CHANAKYA_SIMULATE=1                                        -> simulated
//  3. (default)                                                  -> mock
//
// No live adapter is implemented, so case 1 returns an explicit error rather
// than silently falling back to mock data. A tool that reported mock evidence as
// if it came from the firm's real systems would be worse than one that refused.
func SelectConnector(kind string) (Connector, error) {
	registryMu.RLock()
	c, ok := registry[kind]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("select connector %q: no adapter registered", kind)
	}

	token := strings.TrimSpace(os.Getenv(liveTokenEnv(kind)))
	if token != "" {
		return nil, fmt.Errorf("select connector %q: live mode is not implemented; "+
			"unset %s to use the mock adapter", kind, liveTokenEnv(kind))
	}
	if strings.TrimSpace(os.Getenv("CHANAKYA_SIMULATE")) == "1" {
		return simulated{base: c}, nil
	}
	return c, nil
}

// liveTokenEnv is the env var that would carry a live credential for a kind.
func liveTokenEnv(kind string) string {
	return "CHANAKYA_CONNECTOR_" + strings.ToUpper(strings.ReplaceAll(kind, "-", "_")) + "_TOKEN"
}

// All returns every registered connector, sorted by id.
func All() []Connector {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Connector, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Descriptor().ID < out[j].Descriptor().ID
	})
	return out
}

// Kinds returns every registered connector kind, sorted.
func Kinds() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
