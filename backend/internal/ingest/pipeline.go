package ingest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"chanakya/internal/domain"
	"golang.org/x/sync/errgroup"
)

// Stage names, in execution order. The SSE stream and the ingest_run audit
// record both use these exact strings, so a failure always names a real stage.
const (
	StageIntake      = "intake"
	StageLayout      = "layout"
	StageStructure   = "structure"
	StageMetadata    = "metadata"
	StageNormalize   = "normalize"
	StageSegment     = "segment"
	StageCrossRef    = "cross_reference"
	StageCompile     = "compile"
	StageAmendment   = "amendment_match"
	StageReadyReview = "ready_for_review"
)

// Stages is the ordered pipeline, for progress reporting.
var Stages = []string{
	StageIntake, StageLayout, StageStructure, StageMetadata,
	StageNormalize, StageSegment, StageCrossRef, StageCompile, StageAmendment, StageReadyReview,
}

// ProposedObligation is an obligation the pipeline proposes, carried with the
// clause text it was cited from so the preview can show the citation in context
// WITHOUT the obligation existing in the graph.
type ProposedObligation struct {
	Obligation domain.Obligation `json:"obligation"`
	ClauseRef  string            `json:"clause_ref"`
	ClauseText string            `json:"clause_text"`
}

// RejectedExtraction records an extraction that failed validation and was kept
// out of the proposal. Surfaced in the preview: what the pipeline REFUSED is as
// much a part of the audit record as what it accepted.
type RejectedExtraction struct {
	ClauseRef string `json:"clause_ref"`
	Reason    string `json:"reason"`
}

// Proposal is everything a run produced. It is NOT graph data: nothing here
// exists in circular/clause/obligation until a human approves the run.
type Proposal struct {
	SHA256      string               `json:"sha256"`
	Filename    string               `json:"filename"`
	Meta        CircularMeta         `json:"meta"`
	Circular    domain.Circular      `json:"circular"`
	Clauses     []domain.Clause      `json:"clauses"`
	Normalized  []Normalized         `json:"normalized"`
	Units       []SemanticUnit       `json:"units"`
	ClauseRefs  []ClauseRef          `json:"clause_refs"`
	Dangling    []DanglingRef        `json:"dangling_references"`
	Relations   []CircularRelation   `json:"circular_relations"`
	Obligations []ProposedObligation `json:"obligations"`
	Rejected    []RejectedExtraction `json:"rejected"`
	Degraded    bool                 `json:"degraded"`
	Extractor   string               `json:"extractor"`
	Compiler    string               `json:"compiler"`
	// Amendment is the Stage 9 diff against the existing corpus. It is present
	// only when this document amends or supersedes one already in the graph, and
	// every classification in it is a PROPOSAL awaiting the same human approval
	// as the rest of the run.
	Amendment *AmendmentDiff `json:"amendment,omitempty"`
}

// CircularRelation is a document-to-document edge proposed by Stage 3.
type CircularRelation struct {
	ID           string `json:"id"`
	FromCircular string `json:"from_circular"`
	ToRef        string `json:"to_ref"`
	ToCircular   string `json:"to_circular"`
	Kind         string `json:"kind"` // supersedes | amends | references
}

// ClauseCompiler is the obligation-extraction step the pipeline calls. The real
// implementation is *compiler.Compiler; the interface keeps this package free of
// a dependency on it (and lets tests run the pipeline without an extractor).
type ClauseCompiler interface {
	ExtractorName() string
	CompileClause(ctx context.Context, clause domain.Clause) (ClauseCompileResult, error)
}

// ClauseCompileResult mirrors compiler.ClauseResult in this package's terms.
type ClauseCompileResult struct {
	Obligations []domain.Obligation
	Rejections  []string
}

// ProgressFunc is called as each stage begins and ends. It is how the SSE
// endpoint learns where a run is without the pipeline knowing HTTP exists.
type ProgressFunc func(stage string, done, total int, detail string)

// CorpusLookup supplies the clauses already in the graph for a circular, so the
// amendment matcher can diff against them. Returning an empty slice means "this
// document is new to CHANAKYA", which is the correct answer for a first upload.
type CorpusLookup func(ctx context.Context, circularRefs []string) ([]ExistingClause, error)

// Options configures a pipeline run.
type Options struct {
	Completer MetaCompleter
	Compiler  ClauseCompiler
	Corpus    CorpusLookup
	Progress  ProgressFunc
}

// RunPipeline executes Stages 0-6 plus obligation extraction and returns a
// Proposal. It writes NOTHING: the caller persists the proposal against an
// ingest_run, and only an explicit human approval commits it to the graph.
func RunPipeline(ctx context.Context, doc RawDoc, opts Options) (Proposal, string, error) {
	report := func(stage string, done, total int, detail string) {
		if opts.Progress != nil {
			opts.Progress(stage, done, total, detail)
		}
	}

	report(StageIntake, 1, 1, doc.Filename)

	// Stages 0-2 (Phase 1).
	report(StageLayout, 0, doc.PageCount, "")
	base, err := Run(ctx, doc)
	if err != nil {
		// Run covers both layout and structure; attribute the failure to whichever
		// stage the error names so the audit record is accurate.
		stage := StageLayout
		if strings.Contains(err.Error(), "stage 2") {
			stage = StageStructure
		}
		return Proposal{}, stage, err
	}
	report(StageLayout, doc.PageCount, doc.PageCount, base.Extractor)
	report(StageStructure, len(base.Clauses), len(base.Clauses), "")

	p := Proposal{
		SHA256:    doc.SHA256,
		Filename:  doc.Filename,
		Circular:  base.Circular,
		Clauses:   base.Clauses,
		Degraded:  base.Structure.Degraded,
		Extractor: base.Extractor,
	}

	// Stage 3 - metadata. Sequential.
	report(StageMetadata, 0, 1, "")
	docText := documentText(base)
	meta, err := ExtractMeta(ctx, docText, doc.Filename, opts.Completer)
	if err != nil {
		return p, StageMetadata, fmt.Errorf("stage 3 metadata: %w", err)
	}
	p.Meta = meta
	p.Circular = applyMeta(base.Circular, meta)
	p.Clauses = reparentClauses(base.Clauses, base.Circular.ID, p.Circular.ID)
	p.Relations = relationsFrom(p.Circular.ID, meta)
	report(StageMetadata, 1, 1, string(meta.DocKind))

	// Stage 4 - normalization. Parallel output only: Clause.Text is untouched.
	report(StageNormalize, 0, len(p.Clauses), "")
	for i, c := range p.Clauses {
		p.Normalized = append(p.Normalized, NormalizeClause(c.ClauseRef, c.Text))
		if i%10 == 0 {
			report(StageNormalize, i, len(p.Clauses), "")
		}
	}
	report(StageNormalize, len(p.Clauses), len(p.Clauses), "")

	// Stage 5 - semantic segmentation.
	report(StageSegment, 0, len(p.Clauses), "")
	for _, c := range p.Clauses {
		p.Units = append(p.Units, SegmentClause(c.ID, c.Text)...)
	}
	report(StageSegment, len(p.Clauses), len(p.Clauses), fmt.Sprintf("%d units", len(p.Units)))

	// Stage 6 - cross-reference resolution.
	report(StageCrossRef, 0, len(p.Clauses), "")
	byRef := make(map[string]string, len(p.Clauses))
	texts := make([]clauseText, 0, len(p.Clauses))
	for _, c := range p.Clauses {
		byRef[normalizeRefKey(c.ClauseRef)] = c.ID
		texts = append(texts, clauseText{ID: c.ID, Ref: c.ClauseRef, Text: c.Text})
	}
	known := map[string]bool{}
	for _, r := range p.Relations {
		known[r.ToRef] = true
	}
	p.ClauseRefs, p.Dangling = ResolveReferences(p.Circular.ID, texts, byRef, known)
	report(StageCrossRef, len(p.Clauses), len(p.Clauses),
		fmt.Sprintf("%d resolved, %d dangling", len(p.ClauseRefs), len(p.Dangling)))

	// Obligation extraction. An FAQ produces interpretive material, never
	// obligations - which is exactly why Stage 3 had to run first.
	if opts.Compiler != nil && meta.DocKind != KindFAQ && meta.DocKind != KindConsultationPaper {
		p.Compiler = opts.Compiler.ExtractorName()
		report(StageCompile, 0, len(p.Clauses), p.Compiler)

		eg, egCtx := errgroup.WithContext(ctx)
		eg.SetLimit(100)

		var mu sync.Mutex
		var doneCount int32

		for _, c := range p.Clauses {
			c := c // capture for goroutine
			eg.Go(func() error {
				if err := egCtx.Err(); err != nil {
					return err
				}
				res, err := opts.Compiler.CompileClause(egCtx, c)

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					p.Rejected = append(p.Rejected, RejectedExtraction{
						ClauseRef: c.ClauseRef, Reason: err.Error(),
					})
				} else {
					for _, ob := range res.Obligations {
						p.Obligations = append(p.Obligations, ProposedObligation{
							Obligation: ob, ClauseRef: c.ClauseRef, ClauseText: c.Text,
						})
					}
					for _, r := range res.Rejections {
						p.Rejected = append(p.Rejected, RejectedExtraction{ClauseRef: c.ClauseRef, Reason: r})
					}
				}

				completed := atomic.AddInt32(&doneCount, 1)
				report(StageCompile, int(completed), len(p.Clauses), c.ClauseRef)
				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return p, StageCompile, fmt.Errorf("stage compile cancelled: %w", err)
		}

		report(StageCompile, len(p.Clauses), len(p.Clauses), fmt.Sprintf("%d obligations", len(p.Obligations)))
	} else {
		report(StageCompile, len(p.Clauses), len(p.Clauses), "skipped for "+string(meta.DocKind))
	}

	// Stage 9 - amendment matching. Only runs when the document actually claims a
	// relationship to something already in the graph. Diffing an unrelated
	// circular against the corpus would manufacture spurious "modified" verdicts
	// out of ordinary topical similarity.
	if opts.Corpus != nil {
		related := append(append([]string{}, meta.Supersedes...), meta.Amends...)
		// A re-upload of the same circular number is itself an amendment
		// candidate, even with no explicit supersession language.
		if p.Circular.ID != "" {
			related = append(related, p.Circular.ID)
		}
		report(StageAmendment, 0, 1, "")
		existing, err := opts.Corpus(ctx, related)
		if err != nil {
			return p, StageAmendment, fmt.Errorf("stage 9 corpus lookup: %w", err)
		}
		if len(existing) > 0 {
			// `deleted` requires an actual supersession: a document that merely
			// references another says nothing about clauses missing from it.
			supersedes := len(meta.Supersedes) > 0
			diff := MatchAmendment(p.Clauses, existing, supersedes)
			p.Amendment = &diff
			report(StageAmendment, 1, 1, fmt.Sprintf(
				"%d unchanged, %d modified, %d added, %d deleted",
				diff.Counts[string(ChangeUnchanged)], diff.Counts[string(ChangeModified)],
				diff.Counts[string(ChangeAdded)], diff.Counts[string(ChangeDeleted)]))
		} else {
			report(StageAmendment, 1, 1, "no prior version in the corpus")
		}
	}

	report(StageReadyReview, 1, 1, "awaiting human approval")
	return p, StageReadyReview, nil
}

// documentText reassembles the plain text of the document for the metadata pass,
// in reading order.
func documentText(r Result) string {
	var b strings.Builder
	for _, n := range r.Structure.Nodes {
		if n.Heading != "" {
			b.WriteString(n.Heading)
			b.WriteByte('\n')
		}
		if n.Text != "" {
			b.WriteString(n.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// applyMeta upgrades the minimal Stage 2 circular with the Stage 3 metadata.
//
// The circular id becomes the real circular NUMBER once one is known, because
// that is the identity the rest of the regulatory world uses; the content-address
// id remains only when the document names no number.
func applyMeta(c domain.Circular, m CircularMeta) domain.Circular {
	if m.CircularNo != "" {
		c.ID = m.CircularNo
	}
	if m.Title != "" {
		c.Title = m.Title
	}
	if m.Regulator != "" {
		c.Regulator = m.Regulator
	}
	if m.IssuedOn != "" {
		c.IssuedOn = m.IssuedOn
		c.ValidFrom = m.IssuedOn
	}
	// The world-time start is when the document takes EFFECT, which is not
	// always its issue date - that distinction is the whole point of separating
	// world time from system time.
	if m.EffectiveFrom != "" {
		c.ValidFrom = m.EffectiveFrom
	}
	return c
}

// reparentClauses rewrites clause ids after the circular id changed in Stage 3.
func reparentClauses(clauses []domain.Clause, oldID, newID string) []domain.Clause {
	if oldID == newID {
		return clauses
	}
	out := make([]domain.Clause, 0, len(clauses))
	for _, c := range clauses {
		c.CircularID = newID
		c.ID = domain.ClauseID(newID, c.ClauseRef)
		if c.ParentID != "" {
			c.ParentID = strings.Replace(c.ParentID, oldID+"#", newID+"#", 1)
		}
		out = append(out, c)
	}
	return out
}

// relationsFrom turns the Stage 3 relation lists into proposed edges.
func relationsFrom(circularID string, m CircularMeta) []CircularRelation {
	var out []CircularRelation
	add := func(kind string, refs []string) {
		for _, r := range refs {
			out = append(out, CircularRelation{
				ID:           fmt.Sprintf("%s|%s|%s", circularID, kind, r),
				FromCircular: circularID,
				ToRef:        r,
				Kind:         kind,
			})
		}
	}
	add("supersedes", m.Supersedes)
	add("amends", m.Amends)
	add("references", m.References)
	return out
}
