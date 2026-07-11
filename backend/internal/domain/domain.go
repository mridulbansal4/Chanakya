// Package domain holds CHANAKYA's pure types and business rules. It has no I/O
// and no dependency on storage or HTTP: everything here is deterministic and
// unit-testable in isolation.
package domain

import (
	"fmt"
	"time"
)

// RFC3339UTC formats t as an RFC3339 string in UTC. All timestamps are stored
// this way so that lexical string comparison equals chronological comparison —
// the property the bi-temporal as-of queries depend on.
func RFC3339UTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// Temporal carries the four bi-temporal columns shared by every graph node and
// edge. Open-ended intervals use the zero value (empty string) for ValidTo /
// TxTo, which maps to SQL NULL in the store.
type Temporal struct {
	ValidFrom string // world time: fact becomes true
	ValidTo   string // world time: fact ceases to be true (empty = open)
	TxFrom    string // system time: CHANAKYA learns the fact
	TxTo      string // system time: fact superseded in the DB (empty = current)
}

// Circular is a source regulatory document (e.g. a SEBI Master Circular).
type Circular struct {
	ID        string
	Title     string
	Regulator string
	IssuedOn  string // RFC3339 UTC
	SourceURL string
	Temporal
}

// Entity is a regulated party that obligations can bear upon.
type Entity struct {
	ID       string
	Kind     string
	Name     string
	PAN      string
	MetaJSON string
	Temporal
}

// Clause is a single node in a circular's clause tree.
type Clause struct {
	ID         string // deterministic surrogate, see ClauseID
	CircularID string
	ClauseRef  string // human id, e.g. "3.1"
	ParentID   string // empty for a top-level clause
	Heading    string
	Text       string
	Ordinal    int // document order
	Temporal
}

// ClauseID builds the deterministic surrogate id for a clause. Determinism
// makes seeding idempotent (re-running upserts the same rows) and keeps parent
// references stable without a UUID lookup table.
func ClauseID(circularID, clauseRef string) string {
	return circularID + "#" + clauseRef
}

// ClauseNode is a clause enriched with its position in a traversal: Depth is
// the distance from the traversal root, and Path is a sortable pre-order key.
type ClauseNode struct {
	Clause
	Depth int
	Path  string
}

// Control is a firm-side compliance control that satisfies obligations.
type Control struct {
	ID          string
	Name        string
	Description string
	Kind        string
	Temporal
}

// Evidence is a READ-ONLY reference to a firm system artefact that a control
// relies on. CHANAKYA never writes to the source system.
type Evidence struct {
	ID           string
	Name         string
	SourceSystem string
	Description  string
	Kind         string
	Temporal
}

// DeonticType is the modal force of an obligation.
type DeonticType string

const (
	DeonticMust    DeonticType = "MUST"
	DeonticMustNot DeonticType = "MUST_NOT"
	DeonticMay     DeonticType = "MAY"
)

// Valid reports whether d is one of the three permitted deontic types.
func (d DeonticType) Valid() bool {
	switch d {
	case DeonticMust, DeonticMustNot, DeonticMay:
		return true
	default:
		return false
	}
}

// ObligationStatus is the review lifecycle state of an obligation.
type ObligationStatus string

const (
	StatusPending     ObligationStatus = "pending"
	StatusNeedsReview ObligationStatus = "needs_review"
	StatusApproved    ObligationStatus = "approved"
	StatusRejected    ObligationStatus = "rejected"
)

// Valid reports whether s is a known obligation status.
func (s ObligationStatus) Valid() bool {
	switch s {
	case StatusPending, StatusNeedsReview, StatusApproved, StatusRejected:
		return true
	default:
		return false
	}
}

// TicketState is the lifecycle state of a remediation ticket. CHANAKYA only
// ever creates 'draft' tickets — it never files them into a customer system.
type TicketState string

const (
	TicketDraft    TicketState = "draft"
	TicketFiled    TicketState = "filed"
	TicketResolved TicketState = "resolved"
)

// Valid reports whether s is a known ticket state.
func (s TicketState) Valid() bool {
	switch s {
	case TicketDraft, TicketFiled, TicketResolved:
		return true
	default:
		return false
	}
}

// PolicyStage is the staged enforcement level of a compiled policy. Enforcement
// is staged: audit (evaluate + record only) → soft (warn) → hard (block).
// Nothing hard-blocks before a sign-off exists; policies start at audit.
type PolicyStage string

const (
	StageAudit PolicyStage = "audit"
	StageSoft  PolicyStage = "soft"
	StageHard  PolicyStage = "hard"
)

// Valid reports whether s is a known policy stage.
func (s PolicyStage) Valid() bool {
	switch s {
	case StageAudit, StageSoft, StageHard:
		return true
	default:
		return false
	}
}

// Ticket is a DRAFTED remediation item for a compliance gap. It is never filed
// automatically.
type Ticket struct {
	ID           string
	ObligationID string
	ClauseRef    string
	Title        string
	Detail       string
	Owner        string
	Deadline     string
	Citation     string
	State        TicketState
	Temporal
}

// Obligation is a typed, cited duty extracted from a clause. Provenance
// (SourceClauseRef + SourceSentence) is mandatory: see Validate.
type Obligation struct {
	ID              string
	ClauseID        string
	Bearer          string
	DeonticType     DeonticType
	Condition       string
	ThresholdJSON   string
	Deadline        string
	Penalty         string
	SourceClauseRef string
	SourceSentence  string
	Confidence      float64
	Status          ObligationStatus
	Temporal
}

// Validate enforces the structural + safety invariants on an obligation before
// it may enter the graph. In particular it rejects any obligation lacking a
// causal citation (safety invariant #5). Deeper schema validation of raw LLM
// output lives in the compiler (Phase 2); this guards the store boundary.
func (o Obligation) Validate() error {
	if o.ClauseID == "" {
		return fmt.Errorf("obligation %q: missing clause_id", o.ID)
	}
	if o.Bearer == "" {
		return fmt.Errorf("obligation %q: missing bearer", o.ID)
	}
	if !o.DeonticType.Valid() {
		return fmt.Errorf("obligation %q: invalid deontic_type %q", o.ID, o.DeonticType)
	}
	if o.Status != "" && !o.Status.Valid() {
		return fmt.Errorf("obligation %q: invalid status %q", o.ID, o.Status)
	}
	if o.SourceClauseRef == "" || o.SourceSentence == "" {
		return fmt.Errorf("obligation %q: missing provenance (source_clause_ref + source_sentence are mandatory)", o.ID)
	}
	if o.Confidence < 0 || o.Confidence > 1 {
		return fmt.Errorf("obligation %q: confidence %.3f out of range [0,1]", o.ID, o.Confidence)
	}
	return nil
}
