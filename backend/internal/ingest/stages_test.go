package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// --- Stage 3: metadata -------------------------------------------------------

const sampleCircularText = `SECURITIES AND EXCHANGE BOARD OF INDIA
SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2025/19
17 February 2025
Most Important Terms and Conditions (MITC) for Investment Advisers
This circular is issued in supersession of SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2024/11 and shall be read with SEBI/HO/IMD/IMD-PoD-1/P/CIR/2024/49.
The provisions shall come into force from 30 June 2025.
Every investment adviser shall provide the standardized MITC to each client.`

func TestStage3RegexPass(t *testing.T) {
	m := ExtractMetaRegex(sampleCircularText, "mitc.pdf")

	if m.CircularNo != "SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2025/19" {
		t.Errorf("circular_no = %q", m.CircularNo)
	}
	if m.IssuedOn != "2025-02-17T00:00:00Z" {
		t.Errorf("issued_on = %q, want 2025-02-17T00:00:00Z", m.IssuedOn)
	}
	if m.EffectiveFrom != "2025-06-30T00:00:00Z" {
		t.Errorf("effective_from = %q, want 2025-06-30T00:00:00Z", m.EffectiveFrom)
	}
	if m.Department != "MIRSD" {
		t.Errorf("department = %q, want MIRSD", m.Department)
	}
	if len(m.Supersedes) != 1 || m.Supersedes[0] != "SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2024/11" {
		t.Errorf("supersedes = %v", m.Supersedes)
	}
	if len(m.References) != 1 || m.References[0] != "SEBI/HO/IMD/IMD-PoD-1/P/CIR/2024/49" {
		t.Errorf("references = %v", m.References)
	}
	// A circular that supersedes another IS an amendment - and that classification
	// drives the version-graph path, so it must not be a plain "circular".
	if m.DocKind != KindAmendment {
		t.Errorf("doc_kind = %q, want amendment", m.DocKind)
	}
	if len(m.AppliesTo) != 1 || m.AppliesTo[0] != "investment_adviser" {
		t.Errorf("applies_to = %v", m.AppliesTo)
	}
}

func TestStage3DocKinds(t *testing.T) {
	cases := []struct {
		text string
		want DocKind
	}{
		{"Master Circular for Investment Advisers", KindMasterCircular},
		{"Frequently Asked Questions on the IA Regulations", KindFAQ},
		{"Guidance Note on cyber resilience", KindGuidanceNote},
		{"Consultation Paper on review of the IA framework", KindConsultationPaper},
		{"Notification published in the Gazette of India", KindNotification},
		{"Circular on client onboarding", KindCircular},
	}
	for _, tc := range cases {
		if got := ExtractMetaRegex(tc.text, "x.pdf").DocKind; got != tc.want {
			t.Errorf("%q -> %q, want %q", tc.text, got, tc.want)
		}
	}
}

// stubCompleter returns a fixed payload, standing in for a model.
type stubCompleter struct{ payload string }

func (s stubCompleter) Name() string { return "stub" }
func (s stubCompleter) CompleteMeta(context.Context, string, []string) ([]byte, error) {
	return []byte(s.payload), nil
}

// TestStage3PrecedenceIsFixed: the LLM pass fills only what the regex pass left
// empty. It may never overwrite a value read directly off the page, however
// confident it claims to be.
func TestStage3PrecedenceIsFixed(t *testing.T) {
	completer := stubCompleter{payload: `{
		"circular_no": "SEBI/FABRICATED/9999/1",
		"issued_on": "1999-01-01T00:00:00Z",
		"department": "WRONG",
		"applies_to": ["mutual_fund"]
	}`}

	got, err := ExtractMeta(context.Background(), sampleCircularText, "mitc.pdf", completer)
	if err != nil {
		t.Fatalf("ExtractMeta: %v", err)
	}
	if got.CircularNo != "SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2025/19" {
		t.Errorf("the LLM overwrote a regex-established circular_no: %q", got.CircularNo)
	}
	if got.IssuedOn != "2025-02-17T00:00:00Z" {
		t.Errorf("the LLM overwrote a regex-established issued_on: %q", got.IssuedOn)
	}
	if got.Department != "MIRSD" {
		t.Errorf("the LLM overwrote a regex-established department: %q", got.Department)
	}

	// And it DOES fill a genuine gap.
	sparse := "A document with no circular number and no dates, about portfolio managers."
	base := ExtractMetaRegex(sparse, "x.pdf")
	filled := MergeMeta(base, CircularMeta{CircularNo: "SEBI/HO/X/P/CIR/2025/7"})
	if filled.CircularNo != "SEBI/HO/X/P/CIR/2025/7" {
		t.Errorf("a missing field was not filled: %q", filled.CircularNo)
	}
}

// TestStage3RejectsInvalidLLMOutput: model output is DATA, schema-validated
// before it is trusted. Unknown fields are rejected outright.
func TestStage3RejectsInvalidLLMOutput(t *testing.T) {
	if err := ValidateMetaJSON([]byte(`{"circular_no":"SEBI/X/2025/1"}`)); err != nil {
		t.Errorf("valid payload rejected: %v", err)
	}
	if err := ValidateMetaJSON([]byte(`{"exec":"rm -rf /"}`)); err == nil {
		t.Error("an unknown field must be rejected (additionalProperties:false)")
	}
	if err := ValidateMetaJSON([]byte(`{"issued_on":"17 Feb 2025"}`)); err == nil {
		t.Error("a non-RFC3339 date must be rejected")
	}
	if err := ValidateMetaJSON([]byte(`not json`)); err == nil {
		t.Error("non-JSON must be rejected")
	}
}

// --- Stage 4: normalization --------------------------------------------------

// TestStage4IndianNumerals: the Indian digit grouping is not the international
// one. Read with a Western thousands assumption, three crore becomes three
// hundred thousand and a registration threshold is off by two orders of magnitude.
func TestStage4IndianNumerals(t *testing.T) {
	cases := []struct {
		text string
		want float64
	}{
		{"fees exceeding INR 3,00,00,000 in a financial year", 30000000},
		{"fees exceeding ₹3,00,00,000 in a financial year", 30000000},
		{"fees exceeding Rs. 3,00,00,000", 30000000},
		{"fees exceeding INR 3 crore", 30000000},
		{"fees exceeding ₹3 crore", 30000000},
		{"a corpus of Rs. 50 lakh", 5000000},
	}
	for _, tc := range cases {
		amounts := ParseIndianAmounts(tc.text)
		if len(amounts) == 0 {
			t.Errorf("%q: no amount parsed", tc.text)
			continue
		}
		if amounts[0].Value != tc.want {
			t.Errorf("%q -> %v, want %v", tc.text, amounts[0].Value, tc.want)
		}
		if amounts[0].Unit != "INR" {
			t.Errorf("%q -> unit %q, want INR", tc.text, amounts[0].Unit)
		}
	}
}

// TestStage4NeverTouchesVerbatimText: normalisation writes to a PARALLEL field.
// The citation gate substring-matches against Clause.Text, so rewriting it would
// destroy the thing the proof is made of.
func TestStage4NeverTouchesVerbatimText(t *testing.T) {
	const verbatim = "An  adviser   must  retain records for 5 years."
	n := NormalizeClause("5.1", verbatim)
	if n.Text == verbatim {
		t.Error("the normalised view should differ from the raw text (whitespace collapse)")
	}
	if n.Text != "An adviser must retain records for 5 years." {
		t.Errorf("normalised = %q", n.Text)
	}
	// The whitespace rule must match compiler.containsNormalized exactly:
	// collapse every run of whitespace to a single space, nothing wider.
	if NormalizeText("a\t\n  b") != "a b" {
		t.Errorf("whitespace collapse diverged: %q", NormalizeText("a\t\n  b"))
	}
}

// --- Stage 5: semantic segmentation -----------------------------------------

func TestStage5SplitsOnDiscourseMarkers(t *testing.T) {
	const text = "An adviser shall apply within 30 days, provided that an existing applicant need not reapply."
	units := SegmentClause("c1", text)
	if len(units) < 2 {
		t.Fatalf("got %d units, want the norm and the proviso separated", len(units))
	}
	var sawException bool
	for _, u := range units {
		if u.Role == RoleException {
			sawException = true
		}
		// Every unit must be a SLICE of the parent, not a paraphrase. The
		// offsets are the proof, so verify them literally.
		if got := text[u.StartOffset:u.EndOffset]; got != u.Text {
			t.Errorf("unit %d offsets [%d,%d) yield %q but Text is %q",
				u.Ordinal, u.StartOffset, u.EndOffset, got, u.Text)
		}
	}
	if !sawException {
		t.Error("the 'provided that' clause was not tagged as an exception")
	}
}

// TestStage5NestedProvisosUnderSplit: overlapping markers must not compound.
// "provided further that" contains "provided that"; splitting on both would
// fabricate a boundary the regulation does not have.
func TestStage5NestedProvisos(t *testing.T) {
	const text = "An adviser shall report quarterly, provided that a small adviser may report annually, provided further that the Board is informed."
	units := SegmentClause("c1", text)
	for _, u := range units {
		if strings.HasPrefix(strings.ToLower(u.Text), "further that") {
			t.Errorf("split inside 'provided further that': %q", u.Text)
		}
		if got := text[u.StartOffset:u.EndOffset]; got != u.Text {
			t.Errorf("offsets do not slice the parent: %q vs %q", got, u.Text)
		}
	}
	joined := ""
	for _, u := range units {
		joined += u.Text
	}
	if len(joined) > len(text) {
		t.Error("units overlap - the same text appears in more than one unit")
	}
}

// TestStage5DoesNotSplitOnDecimalsOrAbbreviations: llm.splitSentences splits on
// '.' alone, which cuts "3.1" and "Rs." in half and hands the citation gate a
// fragment. Stage 5 upgrades that.
func TestStage5SentenceBoundaries(t *testing.T) {
	const text = "The threshold in clause 3.1 is Rs. 3,00,00,000. It applies from April."
	units := SegmentClause("c1", text)
	for _, u := range units {
		if strings.HasSuffix(strings.TrimSpace(u.Text), "clause 3.") {
			t.Errorf("split inside a clause number: %q", u.Text)
		}
		if strings.HasSuffix(strings.TrimSpace(u.Text), "Rs.") {
			t.Errorf("split on an abbreviation: %q", u.Text)
		}
	}
}

func TestStage5Roles(t *testing.T) {
	cases := []struct {
		text string
		want UnitRole
	}{
		{"An adviser must notify the client within 7 days", RoleDeadline},
		{"provided that a small adviser is exempt", RoleException},
		{"'client' means any person who receives advice", RoleDefinition},
		{"the adviser shall be liable to a penalty", RolePenalty},
		{"subject to the approval of the Board", RoleCondition},
		{"An adviser shall maintain records", RoleNorm},
	}
	for _, tc := range cases {
		if got := classifyRole(tc.text); got != tc.want {
			t.Errorf("%q -> %q, want %q", tc.text, got, tc.want)
		}
	}
}

// --- Stage 6: cross references -----------------------------------------------

func TestStage6ResolvesAndDangles(t *testing.T) {
	clauses := []clauseText{
		{ID: "C#3", Ref: "3", Text: "Registration is governed by this chapter."},
		{ID: "C#3.1", Ref: "3.1", Text: "The threshold is set out here."},
		{ID: "C#3.2", Ref: "3.2", Text: "An adviser who crosses the threshold in clause 3.1 shall apply under regulation 15, as set out in Annexure Z."},
	}
	byRef := map[string]string{"3": "C#3", "3.1": "C#3.1", "3.2": "C#3.2"}

	resolved, dangling := ResolveReferences("C", clauses, byRef, map[string]bool{})

	if len(resolved) != 1 {
		t.Fatalf("resolved = %d, want 1 (clause 3.1)", len(resolved))
	}
	if resolved[0].FromClauseID != "C#3.2" || resolved[0].ToClauseID != "C#3.1" {
		t.Errorf("edge = %s -> %s", resolved[0].FromClauseID, resolved[0].ToClauseID)
	}

	// A dangling reference is NEVER silently dropped: "regulation 15" is
	// external and "Annexure Z" does not exist in this document.
	if len(dangling) != 2 {
		t.Fatalf("dangling = %d, want 2 (regulation 15, Annexure Z): %+v", len(dangling), dangling)
	}
	kinds := map[RefKind]bool{}
	for _, d := range dangling {
		kinds[d.Kind] = true
		if d.Reason == "" {
			t.Errorf("dangling %q has no reason", d.RawText)
		}
	}
	if !kinds[RefRegulation] || !kinds[RefAnnexure] {
		t.Errorf("dangling kinds = %v, want a regulation and an annexure", kinds)
	}
}

// TestStage6CycleIsRecordedOnce: A -> B -> A records each edge once and stops.
// Chasing the cycle looking for "the real target" would loop forever.
func TestStage6Cycles(t *testing.T) {
	clauses := []clauseText{
		{ID: "C#1", Ref: "1", Text: "See clause 2 for details, and see clause 2 again."},
		{ID: "C#2", Ref: "2", Text: "This is subject to clause 1."},
		{ID: "C#3", Ref: "3", Text: "Nothing in clause 3 limits this."}, // self-reference
	}
	byRef := map[string]string{"1": "C#1", "2": "C#2", "3": "C#3"}

	resolved, _ := ResolveReferences("C", clauses, byRef, map[string]bool{})
	if len(resolved) != 2 {
		t.Fatalf("resolved = %d, want 2 (1->2 once, 2->1); self-reference must be dropped: %+v",
			len(resolved), resolved)
	}
	for _, e := range resolved {
		if e.FromClauseID == e.ToClauseID {
			t.Errorf("self-reference recorded: %+v", e)
		}
	}
}

// TestStage6VagueReferenceIsNotGuessed: "the said circular" has a referent only
// in context. Resolving it to the most recently mentioned document would be a
// guess with the same shape as a fact.
func TestStage6VagueReference(t *testing.T) {
	clauses := []clauseText{
		{ID: "C#1", Ref: "1", Text: "The provisions of the said circular continue to apply."},
	}
	resolved, dangling := ResolveReferences("C", clauses, map[string]string{"1": "C#1"},
		map[string]bool{"SEBI/IA/MC/2024": true})
	if len(resolved) != 0 {
		t.Errorf("a vague reference was asserted as an edge: %+v", resolved)
	}
	if len(dangling) != 1 || dangling[0].Kind != RefVague {
		t.Errorf("dangling = %+v, want one vague reference", dangling)
	}
}

// --- Pipeline ----------------------------------------------------------------

// TestPipelineEndToEnd runs Stages 0-6 over the golden fixture and checks the
// proposal is coherent. No compiler is supplied, so no obligation is produced -
// which is itself the point: the pipeline writes a proposal, never graph data.
func TestPipelineEndToEnd(t *testing.T) {
	doc := goldenDoc(t)

	var stages []string
	p, stage, err := RunPipeline(context.Background(), doc, Options{
		Progress: func(s string, _, _ int, _ string) {
			if len(stages) == 0 || stages[len(stages)-1] != s {
				stages = append(stages, s)
			}
		},
	})
	if err != nil {
		t.Fatalf("RunPipeline failed at %s: %v", stage, err)
	}
	if stage != StageReadyReview {
		t.Errorf("final stage = %q, want %q", stage, StageReadyReview)
	}
	if len(stages) < 6 {
		t.Errorf("only %d stages reported: %v", len(stages), stages)
	}
	if len(p.Clauses) == 0 || len(p.Units) == 0 || len(p.Normalized) == 0 {
		t.Fatalf("empty proposal: %d clauses, %d units, %d normalized",
			len(p.Clauses), len(p.Units), len(p.Normalized))
	}
	if p.Meta.DocKind == "" || !p.Meta.DocKind.Valid() {
		t.Errorf("doc_kind = %q", p.Meta.DocKind)
	}
	// Every semantic unit must still slice its parent clause exactly.
	byID := map[string]string{}
	for _, c := range p.Clauses {
		byID[c.ID] = c.Text
	}
	for _, u := range p.Units {
		parent, ok := byID[u.ClauseID]
		if !ok {
			t.Errorf("unit %q has no parent clause", u.ID)
			continue
		}
		if u.EndOffset > len(parent) || parent[u.StartOffset:u.EndOffset] != u.Text {
			t.Errorf("unit %q offsets do not slice its clause", u.ID)
		}
	}
}

// TestPipelineRejectsScannedDocument: Stage 0's product decision holds through
// the whole pipeline.
func TestPipelineRejectsScanned(t *testing.T) {
	doc := RawDoc{SHA256: "0", Filename: "scan.pdf", PageCount: 4, Bytes: []byte("%PDF-1.4")}
	_, _, err := RunPipeline(context.Background(), doc, Options{})
	if err == nil {
		t.Fatal("expected an error for an unreadable document")
	}
	if errors.Is(err, ErrNotEnabled) {
		t.Fatal("OCR must not be silently engaged")
	}
}
