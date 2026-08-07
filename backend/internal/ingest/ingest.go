// Package ingest is the compiler FRONT-END: it turns an uploaded PDF into the
// clause tree the Regulation Compiler already consumes.
//
// Safety role. Everything in this package is DETERMINISTIC - no LLM runs here.
// Stage 0 content-addresses and validates the bytes, Stage 1 extracts positioned
// text without OCR, and Stage 2 builds the clause tree with a numbering lexer and
// a stack machine. That matters because the citation gate downstream
// (compiler.buildObligation) proves an obligation is real by checking that its
// cited sentence is a VERBATIM substring of the clause text. If clause text were
// paraphrased, summarised, or reflowed by a model on the way in, that proof would
// be checking a model's output against another model's output. So the text this
// package emits is a faithful transcription of the page: normalised for Unicode
// form and whitespace only, never rewritten.
//
// Its only output contract is []domain.Clause (+ a minimal domain.Circular),
// which is exactly what compiler.CompileClause takes.
package ingest

import (
	"context"
	"fmt"

	"chanakya/internal/domain"
)

// Result is the output of the front-end: the clause tree in parent-before-child
// order (the ordering store.UpsertClause requires) plus the minimal circular the
// clauses hang off.
type Result struct {
	Circular  domain.Circular
	Clauses   []domain.Clause
	Structure StructuredDoc
	Extractor string // which Stage 1 extractor produced the layout
}

// Run executes Stages 0-2 over an already-intaken document and returns the
// clause tree. Callers get []domain.Clause that slots directly into
// compiler.CompileClause.
//
// The temporal columns are left to the caller: ingestion does not decide when a
// regulation came into force. Stage 3 (Phase 2) extracts the issue/effective
// dates that populate them.
func Run(ctx context.Context, doc RawDoc) (Result, error) {
	ex := SelectPageExtractor()
	layout, err := ex.Extract(ctx, doc)
	if err != nil {
		return Result{}, fmt.Errorf("stage 1 layout extraction with %s: %w", ex.Name(), err)
	}

	// A PDF with essentially no extractable text is a scan. That is a product
	// decision, not a failure: CHANAKYA's guarantee is a verbatim citation back
	// to the source, and OCR output is a probabilistic transcription that cannot
	// support that guarantee. Rejecting is the honest answer.
	if err := checkExtractable(doc, layout); err != nil {
		return Result{}, err
	}
	
	// Abort pipeline immediately if the document is irrelevant (e.g. random PDF upload)
	if err := checkRelevance(layout); err != nil {
		return Result{}, err
	}

	structured, err := ParseStructure(doc, layout)
	if err != nil {
		return Result{}, fmt.Errorf("stage 2 structural parse of %q: %w", doc.Filename, err)
	}

	clauses := structured.Clauses()
	return Result{
		Circular:  structured.Circular,
		Clauses:   clauses,
		Structure: structured,
		Extractor: ex.Name(),
	}, nil
}
