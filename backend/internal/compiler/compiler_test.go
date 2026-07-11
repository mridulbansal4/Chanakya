package compiler

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"chanakya/internal/domain"
	"chanakya/internal/llm"
)

// fakeExtractor returns a fixed raw document, to drive the compiler's
// validation path with hand-crafted (including malicious) output.
type fakeExtractor struct{ raw string }

func (f fakeExtractor) Name() string { return "fake" }
func (f fakeExtractor) Extract(context.Context, llm.ExtractionRequest) ([]byte, error) {
	return []byte(f.raw), nil
}

func newCompiler(t *testing.T, raw string) *Compiler {
	t.Helper()
	c, err := New(fakeExtractor{raw: raw}, DefaultReviewThreshold)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// The clause every candidate must cite against.
var testClause = domain.Clause{
	ID:        "C#3.1",
	ClauseRef: "3.1",
	Heading:   "Threshold",
	Text:      "A person providing advice to 300 or more clients must apply for registration.",
	Temporal:  domain.Temporal{ValidFrom: "2024-05-15T00:00:00Z", TxFrom: "2024-05-15T00:00:00Z"},
}

func TestValidateRaw(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{
			name:    "valid minimal",
			raw:     `{"obligations":[{"bearer":"x","deontic_type":"MUST","source_clause_ref":"3.1","source_sentence":"s","confidence":0.9}]}`,
			wantErr: false,
		},
		{"empty obligations ok", `{"obligations":[]}`, false},
		{"missing obligations key", `{}`, true},
		{"unknown top-level field", `{"obligations":[],"extra":1}`, true},
		{
			name:    "invalid deontic enum",
			raw:     `{"obligations":[{"bearer":"x","deontic_type":"SHOULD","source_clause_ref":"3.1","source_sentence":"s","confidence":0.9}]}`,
			wantErr: true,
		},
		{
			name:    "missing required source_sentence",
			raw:     `{"obligations":[{"bearer":"x","deontic_type":"MUST","source_clause_ref":"3.1","confidence":0.9}]}`,
			wantErr: true,
		},
		{
			name:    "unknown field in obligation",
			raw:     `{"obligations":[{"bearer":"x","deontic_type":"MUST","source_clause_ref":"3.1","source_sentence":"s","confidence":0.9,"exec":"rm -rf"}]}`,
			wantErr: true,
		},
		{"not json", `{oops`, true},
	}
	c := newCompiler(t, "{}")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := c.ValidateRaw([]byte(tc.raw))
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateRaw err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestCompileClause_RejectsMissingCitation(t *testing.T) {
	// Schema-valid, but the source_sentence is NOT in the clause text — a
	// hallucinated citation. It must be rejected before entering the graph.
	raw := `{"obligations":[{"bearer":"investment adviser","deontic_type":"MUST",` +
		`"source_clause_ref":"3.1","source_sentence":"This sentence was never in the clause.",` +
		`"confidence":0.95}]}`
	c := newCompiler(t, raw)

	res, err := c.CompileClause(context.Background(), testClause)
	if err != nil {
		t.Fatalf("CompileClause: %v", err)
	}
	if len(res.Obligations) != 0 {
		t.Fatalf("expected 0 obligations, got %d", len(res.Obligations))
	}
	if len(res.Rejections) != 1 {
		t.Fatalf("expected 1 rejection, got %d", len(res.Rejections))
	}
	if !strings.Contains(res.Rejections[0].Reason, "verbatim substring") {
		t.Errorf("rejection reason = %q, want substring complaint", res.Rejections[0].Reason)
	}
}

func TestCompileClause_RejectsWrongClauseRef(t *testing.T) {
	raw := `{"obligations":[{"bearer":"investment adviser","deontic_type":"MUST",` +
		`"source_clause_ref":"9.9","source_sentence":"A person providing advice to 300 or more clients must apply for registration.",` +
		`"confidence":0.95}]}`
	c := newCompiler(t, raw)

	res, err := c.CompileClause(context.Background(), testClause)
	if err != nil {
		t.Fatalf("CompileClause: %v", err)
	}
	if len(res.Obligations) != 0 || len(res.Rejections) != 1 {
		t.Fatalf("got %d obligations, %d rejections; want 0 and 1", len(res.Obligations), len(res.Rejections))
	}
	if !strings.Contains(res.Rejections[0].Reason, "does not match") {
		t.Errorf("reason = %q, want clause-ref mismatch", res.Rejections[0].Reason)
	}
}

func TestCompileClause_AcceptsAndRoutesByConfidence(t *testing.T) {
	sentence := "A person providing advice to 300 or more clients must apply for registration."
	mk := func(conf float64) string {
		return `{"obligations":[{"bearer":"investment adviser","deontic_type":"MUST",` +
			`"source_clause_ref":"3.1","source_sentence":"` + sentence + `","confidence":` +
			ftoa(conf) + `}]}`
	}

	tests := []struct {
		name       string
		confidence float64
		wantStatus domain.ObligationStatus
	}{
		{"high confidence pending", 0.90, domain.StatusPending},
		{"at threshold pending", DefaultReviewThreshold, domain.StatusPending},
		{"low confidence needs review", 0.50, domain.StatusNeedsReview},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCompiler(t, mk(tc.confidence))
			res, err := c.CompileClause(context.Background(), testClause)
			if err != nil {
				t.Fatalf("CompileClause: %v", err)
			}
			if len(res.Obligations) != 1 {
				t.Fatalf("expected 1 obligation, got %d (rejections=%d)", len(res.Obligations), len(res.Rejections))
			}
			ob := res.Obligations[0]
			if ob.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", ob.Status, tc.wantStatus)
			}
			if ob.ClauseID != testClause.ID {
				t.Errorf("clause id = %q, want %q", ob.ClauseID, testClause.ID)
			}
			if ob.ValidFrom != testClause.ValidFrom {
				t.Errorf("valid_from = %q, want inherited %q", ob.ValidFrom, testClause.ValidFrom)
			}
		})
	}
}

// TestObligationIDDeterministic ensures re-compiling yields the same id.
func TestObligationIDDeterministic(t *testing.T) {
	a := obligationID("C#3.1", "MUST", "some sentence")
	b := obligationID("C#3.1", "MUST", "some sentence")
	if a != b {
		t.Errorf("ids differ: %q vs %q", a, b)
	}
	if obligationID("C#3.1", "MAY", "some sentence") == a {
		t.Errorf("different deontic produced same id")
	}
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }
