package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"chanakya/internal/llm"
)

// metaSystemPrompt tells the model exactly what it may and may not do.
//
// "Leave a field out rather than guess" is the important instruction: a
// fabricated circular number or effective date is far more damaging than a
// missing one, because a missing field is visibly missing in review while a
// fabricated one looks like evidence.
const metaSystemPrompt = `You extract bibliographic metadata from Indian securities-market regulatory documents (SEBI circulars, notifications, FAQs).
Return ONLY the fields you are asked for, read directly from the document text.
If a field is not stated in the document, OMIT it entirely - never infer, never guess, never construct a plausible value.
Dates must be RFC3339 UTC (e.g. 2025-02-17T00:00:00Z).
Circular numbers must be copied exactly as printed.`

// llmMetaCompleter adapts a llm.JSONCompleter to the Stage 3 interface.
type llmMetaCompleter struct {
	c llm.JSONCompleter
}

// SelectMetaCompleter returns the Stage 3 gap-filler, or nil when no model is
// configured.
//
// Nil is a valid, fully-supported configuration: Stage 3 then runs regex-only.
// That is the default in CI and offline demos, and it is why the deterministic
// pass has to be the one that establishes DocKind - the pipeline must produce
// correct metadata with no model at all.
func SelectMetaCompleter() MetaCompleter {
	c := llm.SelectJSONCompleter()
	if c == nil {
		return nil
	}
	return llmMetaCompleter{c: c}
}

// Name identifies the completer for provenance.
func (m llmMetaCompleter) Name() string { return m.c.Name() }

// maxMetaChars bounds how much of the document is sent. Bibliographic metadata
// lives in the first page or two; sending the whole circular would cost tokens
// for text that cannot contain the answer.
const maxMetaChars = 8000

// CompleteMeta asks the model for exactly the missing fields.
func (m llmMetaCompleter) CompleteMeta(ctx context.Context, docText string, missing []string) ([]byte, error) {
	user := fmt.Sprintf("Extract ONLY these fields: %s\n\nDocument:\n%s",
		strings.Join(missing, ", "), firstN(docText, maxMetaChars))
	raw, err := m.c.Complete(ctx, metaSystemPrompt, user, json.RawMessage(MetaSchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("complete circular metadata: %w", err)
	}
	return raw, nil
}
