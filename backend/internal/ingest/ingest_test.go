package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// -update rewrites the golden file instead of asserting against it. Committing
// the regenerated file is a deliberate act, so a behaviour change to the parser
// always shows up as a reviewable diff rather than a silently-passing test.
var update = flag.Bool("update", false, "rewrite golden files")

// mitcPath is the MITC circular that ships with the repo. It is an image-only
// scan (see TestMITCIsRejectedAsScanned), so it is the fixture for the REJECTION
// path, not for the parsing path.
const mitcPath = "../../../Documents/MITC_Circular_17Feb2025.pdf"

// goldenDoc is the synthetic-but-real-content circular built by
// pdffixture_test.go. See that file for why the repo's own PDFs cannot serve.
func goldenDoc(t *testing.T) RawDoc {
	t.Helper()
	doc, err := Intake(buildFixturePDF(), "ia_master_circular.pdf")
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	return doc
}

// TestDumpFixturePDF writes the golden-fixture PDF to disk when
// CHANAKYA_DUMP_FIXTURE_PDF names a path. It exists so the same document the
// unit tests parse can be uploaded to a running server for an end-to-end check,
// rather than hand-waving that the two are equivalent.
func TestDumpFixturePDF(t *testing.T) {
	path := os.Getenv("CHANAKYA_DUMP_FIXTURE_PDF")
	if path == "" {
		t.Skip("set CHANAKYA_DUMP_FIXTURE_PDF to write the fixture PDF")
	}
	if err := os.WriteFile(path, buildFixturePDF(), 0o644); err != nil {
		t.Fatalf("write fixture pdf: %v", err)
	}
	t.Logf("wrote %s", path)

	// Also emit an AMENDED version, for exercising the Stage 9 matcher against a
	// running server: one clause modified (5 years -> 8 years), one added, and
	// explicit supersession language so the `deleted` path is reachable.
	if amended := os.Getenv("CHANAKYA_DUMP_AMENDED_PDF"); amended != "" {
		if err := os.WriteFile(amended, buildAmendedFixturePDF(), 0o644); err != nil {
			t.Fatalf("write amended fixture pdf: %v", err)
		}
		t.Logf("wrote %s", amended)
	}
}

// TestMITCIsRejectedAsScanned records what the repo's MITC PDF actually is: a
// "Microsoft: Print To PDF" scan with zero font objects. A no-OCR pipeline must
// refuse it with the stated product message rather than silently emitting an
// empty clause tree - which is precisely the failure this asserts against.
func TestMITCIsRejectedAsScanned(t *testing.T) {
	raw, err := os.ReadFile(mitcPath)
	if err != nil {
		t.Skipf("%s unavailable: %v", mitcPath, err)
	}
	doc, err := Intake(raw, "MITC_Circular_17Feb2025.pdf")
	if err != nil {
		t.Fatalf("Intake should accept the bytes (it is a valid, unencrypted PDF): %v", err)
	}
	if _, err := Run(context.Background(), doc); !errors.Is(err, ErrScanned) {
		t.Fatalf("Run err = %v, want ErrScanned", err)
	}
}

// TestGoldenCircular pins the exact clause tree Stages 0-2 produce. This is what
// makes the deterministic front-end testable, and it locks the behaviour down
// before Phase 2 layers anything non-deterministic on top of it.
func TestGoldenCircular(t *testing.T) {
	doc := goldenDoc(t)

	res, err := Run(context.Background(), doc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, err := json.MarshalIndent(res.Structure, "", "  ")
	if err != nil {
		t.Fatalf("marshal structure: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "golden", "ia_master_circular.json")
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden updated: %s (%d nodes)", goldenPath, len(res.Structure.Nodes))
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run `go test ./internal/ingest -update` to create it): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("clause tree does not match the golden file.\n"+
			"If this change is intended, re-run with -update and review the diff.\n"+
			"got %d bytes, want %d bytes", len(got), len(want))
	}
}

// TestRunIsDeterministic guards the property the golden test depends on: the
// same bytes must always produce the same tree. Go map iteration inside the
// coalescer and the font-size histogram are the obvious ways this could rot.
func TestRunIsDeterministic(t *testing.T) {
	doc := goldenDoc(t)

	first, err := Run(context.Background(), doc)
	if err != nil {
		t.Fatalf("Run (1): %v", err)
	}
	for i := 0; i < 3; i++ {
		next, err := Run(context.Background(), doc)
		if err != nil {
			t.Fatalf("Run (%d): %v", i+2, err)
		}
		a, _ := json.Marshal(first.Structure)
		b, _ := json.Marshal(next.Structure)
		if string(a) != string(b) {
			t.Fatalf("run %d produced a different clause tree", i+2)
		}
	}
}

// TestClausesSlotIntoCompiler checks the Phase 1 output contract: every emitted
// clause carries the fields compiler.CompileClause reads, and parents appear
// before their children (the ordering store.UpsertClause requires under
// foreign_keys=ON).
func TestClausesSlotIntoCompiler(t *testing.T) {
	doc := goldenDoc(t)
	res, err := Run(context.Background(), doc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Clauses) == 0 {
		t.Fatal("no clauses produced")
	}

	seen := map[string]bool{}
	for _, c := range res.Clauses {
		if c.ID == "" || c.ClauseRef == "" {
			t.Errorf("clause with empty id/ref: %+v", c)
		}
		if c.Text == "" {
			t.Errorf("clause %q has empty text - the citation gate needs text to match against", c.ClauseRef)
		}
		if c.CircularID != res.Circular.ID {
			t.Errorf("clause %q circular id = %q, want %q", c.ClauseRef, c.CircularID, res.Circular.ID)
		}
		if seen[c.ID] {
			t.Errorf("duplicate clause id %q", c.ID)
		}
		if c.ParentID != "" && !seen[c.ParentID] {
			t.Errorf("clause %q references parent %q that has not been emitted yet", c.ClauseRef, c.ParentID)
		}
		seen[c.ID] = true
	}
}

// TestIntakeRejections covers Stage 0's failure modes. Each must be a DISTINCT
// error: telling a user their encrypted PDF is "damaged" sends them to fix the
// wrong problem.
func TestIntakeRejections(t *testing.T) {
	if _, err := Intake(nil, "empty.pdf"); !errors.Is(err, ErrNotPDF) {
		t.Errorf("empty upload: err = %v, want ErrNotPDF", err)
	}
	if _, err := Intake([]byte("this is not a pdf at all"), "notes.txt"); !errors.Is(err, ErrNotPDF) {
		t.Errorf("non-pdf: err = %v, want ErrNotPDF", err)
	}
	big := make([]byte, MaxDocumentBytes+1)
	copy(big, pdfMagic)
	if _, err := Intake(big, "huge.pdf"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("oversized: err = %v, want ErrTooLarge", err)
	}
	if _, err := Intake([]byte("%PDF-1.7\nbut then garbage"), "broken.pdf"); !errors.Is(err, ErrCorrupt) {
		t.Errorf("corrupt: err = %v, want ErrCorrupt", err)
	}
}

// TestScannedRejection proves the scanned-PDF product decision is enforced: a
// valid PDF with essentially no extractable text is refused with the stated
// message rather than silently producing an empty clause tree.
func TestScannedRejection(t *testing.T) {
	doc := RawDoc{SHA256: "0000", Filename: "scan.pdf", PageCount: 4}
	layout := LayoutDoc{Pages: []Page{{Num: 1, Runs: []TextRun{{Text: "Page 1"}}}}}
	err := checkExtractable(doc, layout)
	if !errors.Is(err, ErrScanned) {
		t.Fatalf("err = %v, want ErrScanned", err)
	}
}

// TestOCRExtractorDisabled: OCR is a registered seam, never a silent capability.
func TestOCRExtractorDisabled(t *testing.T) {
	_, err := OCRExtractor{}.Extract(context.Background(), RawDoc{Filename: "x.pdf"})
	if !errors.Is(err, ErrNotEnabled) {
		t.Fatalf("err = %v, want ErrNotEnabled", err)
	}
}

// TestExternalCmdExtractorDegradesGracefully: pdftotext is optional, so its
// absence must be a clear error and never a panic. CI must not depend on it.
func TestExternalCmdExtractorDegradesGracefully(t *testing.T) {
	e := ExternalCmdExtractor{}
	if e.Available() {
		t.Skip("pdftotext is installed on this host; the missing-binary path cannot be exercised")
	}
	_, err := e.Extract(context.Background(), RawDoc{Filename: "x.pdf", Bytes: []byte("%PDF-1.4")})
	if !errors.Is(err, ErrExternalCmdUnavailable) {
		t.Fatalf("err = %v, want ErrExternalCmdUnavailable", err)
	}
}

// TestSelectPageExtractorDefault: RSC is the default, and it is the only one CI
// relies on.
func TestSelectPageExtractorDefault(t *testing.T) {
	t.Setenv("CHANAKYA_PDF_EXTRACTOR", "")
	if got := SelectPageExtractor().Name(); got != (RSCExtractor{}).Name() {
		t.Fatalf("default extractor = %q, want %q", got, (RSCExtractor{}).Name())
	}
	t.Setenv("CHANAKYA_PDF_EXTRACTOR", "ocr")
	if _, ok := SelectPageExtractor().(OCRExtractor); !ok {
		t.Fatal("CHANAKYA_PDF_EXTRACTOR=ocr must select the OCR seam")
	}
}
