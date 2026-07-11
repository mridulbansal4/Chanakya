// Package llm defines the extraction boundary for the Regulation Compiler.
//
// SAFETY MODEL (invariant #1): an Extractor turns clause text into a raw JSON
// document — pure DATA that must validate against the compiler's strict schema.
// It NEVER returns code that gets executed and it NEVER enforces anything. Two
// implementations exist: a deterministic offline extractor (the default, no API
// key, fully testable) and a real Anthropic client (used only when an API key
// is configured), both behind this one interface.
package llm

import "context"

// ExtractionRequest is the input to an Extractor: a single clause.
type ExtractionRequest struct {
	ClauseRef string // e.g. "3.1"
	Heading   string
	Text      string
}

// Extractor produces a raw JSON document of the shape {"obligations": [...]}.
// The bytes are validated against the strict schema by the compiler before any
// obligation is trusted; an Extractor is never itself the source of truth.
type Extractor interface {
	// Name identifies the extractor (for provenance / the SupTech feed).
	Name() string
	// Extract returns raw JSON (the "obligations" object) for one clause.
	Extract(ctx context.Context, req ExtractionRequest) ([]byte, error)
}
