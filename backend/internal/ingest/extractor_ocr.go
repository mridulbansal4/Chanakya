package ingest

import (
	"context"
	"fmt"
)

// OCRExtractor is a registered-but-disabled Stage 1 adapter.
//
// It exists as code, not as a roadmap entry, for a safety reason. CHANAKYA's
// central guarantee is that every obligation cites a sentence that appears
// VERBATIM in the source clause. OCR output is a probabilistic transcription:
// a single mis-recognised character silently changes what the firm is told it
// must do, and the citation check would happily confirm the wrong text against
// itself. Enabling OCR is therefore a deliberate product decision with its own
// confidence model, not a dependency swap - and having the seam here, returning
// ErrNotEnabled, makes that boundary visible to anyone reading the pipeline.
type OCRExtractor struct{}

// Name identifies the extractor for provenance.
func (OCRExtractor) Name() string { return "ocr (not enabled)" }

// Extract always fails with ErrNotEnabled.
func (OCRExtractor) Extract(_ context.Context, doc RawDoc) (LayoutDoc, error) {
	return LayoutDoc{}, fmt.Errorf("ocr extraction of %q: %w", doc.Filename, ErrNotEnabled)
}
