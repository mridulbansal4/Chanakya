// Package compiler is the Regulation Compiler: it drives an llm.Extractor over
// clauses, validates the returned DATA against a strict JSON schema, enforces
// the mandatory causal citation (safety invariant #5), and routes low-confidence
// extractions to human review instead of trusting them. Nothing here executes
// model output or enforces anything — it only produces validated obligations.
package compiler

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"chanakya/internal/domain"
	"chanakya/internal/llm"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// SchemaJSON is the strict extraction schema, embedded so it is identical for
// both validation and (when used) the Anthropic tool input_schema.
//
//go:embed schema.json
var SchemaJSON []byte

// DefaultReviewThreshold: obligations at or above this confidence are marked
// "pending" (ready for the review queue); below it they are "needs_review"
// (explicitly flagged as low-confidence). Nothing is ever auto-approved — that
// requires a human sign-off in Phase 6.
const DefaultReviewThreshold = 0.75

// Compiler validates and compiles extractor output into typed obligations.
type Compiler struct {
	extractor       llm.Extractor
	schema          *jsonschema.Schema
	reviewThreshold float64
}

// Rejection records an extractor output that failed validation and was kept
// out of the graph, with the reason (for auditability).
type Rejection struct {
	Index  int
	Reason string
	Raw    json.RawMessage
}

// ClauseResult is the outcome of compiling one clause.
type ClauseResult struct {
	Obligations []domain.Obligation // validated (pending or needs_review)
	Rejections  []Rejection
}

// candidate mirrors one schema obligation for decoding.
type candidate struct {
	Bearer          string          `json:"bearer"`
	DeonticType     string          `json:"deontic_type"`
	Condition       string          `json:"condition"`
	Threshold       json.RawMessage `json:"threshold"`
	Deadline        string          `json:"deadline"`
	Penalty         string          `json:"penalty"`
	SourceClauseRef string          `json:"source_clause_ref"`
	SourceSentence  string          `json:"source_sentence"`
	Confidence      float64         `json:"confidence"`
}

type extractionDoc struct {
	Obligations []candidate `json:"obligations"`
}

// New builds a Compiler. reviewThreshold <= 0 uses DefaultReviewThreshold.
func New(extractor llm.Extractor, reviewThreshold float64) (*Compiler, error) {
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("chanakya:obligation-extraction", schemaDoc); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	sch, err := c.Compile("chanakya:obligation-extraction")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	if reviewThreshold <= 0 {
		reviewThreshold = DefaultReviewThreshold
	}
	return &Compiler{extractor: extractor, schema: sch, reviewThreshold: reviewThreshold}, nil
}

// ExtractorName exposes the underlying extractor's identity for provenance.
func (c *Compiler) ExtractorName() string { return c.extractor.Name() }

// ValidateRaw validates a raw extractor document against the strict schema.
// Exposed for unit testing the validation boundary directly.
func (c *Compiler) ValidateRaw(raw []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("raw output is not valid JSON: %w", err)
	}
	if err := c.schema.Validate(inst); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}

// CompileClause extracts obligations from one clause and returns the validated
// obligations plus any rejections. It:
//  1. calls the extractor (DATA only),
//  2. validates the whole document against the strict schema,
//  3. for each candidate, enforces the causal citation (clause ref matches and
//     the source sentence is a verbatim substring of the clause text) and the
//     domain invariants, then assigns a review status by confidence.
func (c *Compiler) CompileClause(ctx context.Context, clause domain.Clause) (ClauseResult, error) {
	raw, err := c.extractor.Extract(ctx, llm.ExtractionRequest{
		ClauseRef: clause.ClauseRef,
		Heading:   clause.Heading,
		Text:      clause.Text,
	})
	if err != nil {
		return ClauseResult{}, fmt.Errorf("extract clause %s: %w", clause.ClauseRef, err)
	}
	if err := c.ValidateRaw(raw); err != nil {
		return ClauseResult{}, fmt.Errorf("clause %s: %w", clause.ClauseRef, err)
	}

	var doc extractionDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ClauseResult{}, fmt.Errorf("decode clause %s extraction: %w", clause.ClauseRef, err)
	}

	res := ClauseResult{}
	for i, cand := range doc.Obligations {
		ob, reason := c.buildObligation(clause, cand)
		if reason != "" {
			res.Rejections = append(res.Rejections, Rejection{Index: i, Reason: reason, Raw: raw})
			continue
		}
		res.Obligations = append(res.Obligations, ob)
	}
	return res, nil
}

// buildObligation maps a validated candidate to a domain.Obligation, enforcing
// the causal-citation and domain invariants. A non-empty reason means the
// candidate was rejected and must not enter the graph.
func (c *Compiler) buildObligation(clause domain.Clause, cand candidate) (domain.Obligation, string) {
	// Causal citation, part 1: the cited clause ref must be THIS clause.
	if cand.SourceClauseRef != clause.ClauseRef {
		return domain.Obligation{}, fmt.Sprintf(
			"citation clause ref %q does not match clause %q", cand.SourceClauseRef, clause.ClauseRef)
	}
	// Causal citation, part 2: the cited sentence must be a verbatim substring
	// of the clause text. This rejects hallucinated or paraphrased citations.
	if cand.SourceSentence == "" {
		return domain.Obligation{}, "missing source_sentence"
	}
	if !containsNormalized(clause.Text, cand.SourceSentence) {
		return domain.Obligation{}, "source_sentence is not a verbatim substring of the clause text"
	}

	thresholdJSON := "{}"
	if len(cand.Threshold) > 0 && string(cand.Threshold) != "null" {
		thresholdJSON = string(cand.Threshold)
	}

	status := domain.StatusNeedsReview
	if cand.Confidence >= c.reviewThreshold {
		status = domain.StatusPending
	}

	ob := domain.Obligation{
		ID:              obligationID(clause.ID, cand.DeonticType, cand.SourceSentence),
		ClauseID:        clause.ID,
		Bearer:          cand.Bearer,
		DeonticType:     domain.DeonticType(cand.DeonticType),
		Condition:       cand.Condition,
		ThresholdJSON:   thresholdJSON,
		Deadline:        cand.Deadline,
		Penalty:         cand.Penalty,
		SourceClauseRef: cand.SourceClauseRef,
		SourceSentence:  cand.SourceSentence,
		Confidence:      cand.Confidence,
		Status:          status,
		// The obligation is in force in WORLD time from when its clause is; the
		// system learns it now (tx_from stamped by the caller/store layer).
		Temporal: domain.Temporal{
			ValidFrom: clause.ValidFrom,
			ValidTo:   clause.ValidTo,
			TxFrom:    clause.TxFrom,
		},
	}
	if err := ob.Validate(); err != nil {
		return domain.Obligation{}, err.Error()
	}
	return ob, ""
}

// obligationID is a deterministic id so re-compiling upserts the same rows.
func obligationID(clauseID, deontic, sentence string) string {
	sum := sha256.Sum256([]byte(deontic + "|" + sentence))
	return clauseID + "/obl/" + hex.EncodeToString(sum[:])[:12]
}

// containsNormalized reports whether needle appears in haystack after
// collapsing all runs of whitespace to single spaces (so citation matching is
// robust to formatting) — but it remains a substring check, not fuzzy matching.
func containsNormalized(haystack, needle string) bool {
	return strings.Contains(normalizeWS(haystack), normalizeWS(needle))
}

func normalizeWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
