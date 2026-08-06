package ingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExternalCmdExtractor shells out to poppler's `pdftotext -layout`, which
// handles some pathological font encodings better than a pure-Go reader.
//
// It is an OPTIONAL upgrade, never a dependency: the binary is absent on most
// machines and on CI. It is opt-in (CHANAKYA_PDF_EXTRACTOR=pdftotext) and, when
// pdftotext is missing, it returns a clear error instead of panicking or
// silently falling back - a silent fallback would mean the same PDF parses
// differently on two machines.
//
// Its output has no font metrics, so every run is reported at the same nominal
// size. Stage 2's numbering lexer still works; only font-based heading detection
// degrades, which is exactly the tradeoff of a layout-only text dump.
type ExternalCmdExtractor struct{}

// Name identifies the extractor for provenance.
func (ExternalCmdExtractor) Name() string { return "pdftotext -layout" }

// ErrExternalCmdUnavailable is returned when pdftotext is not on PATH.
var ErrExternalCmdUnavailable = errors.New("pdftotext is not installed on PATH")

// Available reports whether pdftotext can be found, so callers can probe without
// attempting an extraction.
func (ExternalCmdExtractor) Available() bool {
	_, err := exec.LookPath("pdftotext")
	return err == nil
}

// nominalFontSize is the size reported for every run, since `pdftotext -layout`
// emits no font metrics. Stage 2 treats a document with a single font size as
// having no font signal and falls back to numbering + indentation alone.
const nominalFontSize = 10.0

// lineHeight is the synthetic vertical spacing between output lines. Only the
// ORDER of Y values matters to Stage 2, not their absolute scale.
const lineHeight = 12.0

// charWidth approximates a monospaced column width, so leading spaces in
// `-layout` output become a usable indent signal.
const charWidth = 5.0

// Extract writes the document to a temp file (pdftotext needs a path), runs the
// converter, and reconstructs positioned runs from the layout-preserved text.
func (e ExternalCmdExtractor) Extract(ctx context.Context, doc RawDoc) (LayoutDoc, error) {
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		return LayoutDoc{}, fmt.Errorf("extract %q: %w", doc.Filename, ErrExternalCmdUnavailable)
	}

	dir, err := os.MkdirTemp("", "chanakya-ingest-*")
	if err != nil {
		return LayoutDoc{}, fmt.Errorf("create temp dir for %q: %w", doc.Filename, err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, doc.Bytes, 0o600); err != nil {
		return LayoutDoc{}, fmt.Errorf("write temp pdf for %q: %w", doc.Filename, err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "-layout", "-enc", "UTF-8", in, "-")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return LayoutDoc{}, fmt.Errorf("run pdftotext on %q: %w (%s)",
			doc.Filename, err, strings.TrimSpace(stderr.String()))
	}

	return parseLayoutText(stdout.String()), nil
}

// parseLayoutText turns `pdftotext -layout` output into positioned runs.
// Pages are separated by form feeds; within a page, each line becomes one run
// whose X is derived from its leading indentation.
func parseLayoutText(text string) LayoutDoc {
	var out LayoutDoc
	for i, pageText := range strings.Split(text, "\f") {
		if strings.TrimSpace(pageText) == "" {
			continue
		}
		page := Page{Num: i + 1, Width: 612, Height: 792}
		lines := strings.Split(pageText, "\n")
		for j, line := range lines {
			trimmed := strings.TrimRight(line, " \t\r")
			if strings.TrimSpace(trimmed) == "" {
				continue
			}
			indent := len(trimmed) - len(strings.TrimLeft(trimmed, " \t"))
			body := strings.TrimLeft(trimmed, " \t")
			page.Runs = append(page.Runs, TextRun{
				Text:     normalizeText(body),
				X:        float64(indent) * charWidth,
				Y:        page.Height - float64(j)*lineHeight,
				Width:    float64(len(body)) * charWidth,
				FontSize: nominalFontSize,
				FontName: "pdftotext",
			})
		}
		out.Pages = append(out.Pages, page)
	}
	return out
}
