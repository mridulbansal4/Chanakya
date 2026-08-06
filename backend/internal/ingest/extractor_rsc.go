package ingest

import (
	"bytes"
	"context"
	"fmt"

	"rsc.io/pdf"
)

// RSCExtractor is the default Stage 1 extractor: pure Go (no cgo, no external
// binary, works on Windows), and it reports per-glyph positions and font sizes,
// which Stage 2's heading detection and indent reconciliation both need.
type RSCExtractor struct{}

// Name identifies the extractor for provenance.
func (RSCExtractor) Name() string { return "rsc.io/pdf" }

// Extract reads every page's positioned text.
//
// rsc.io/pdf reports malformed structures by panicking, so each page is read
// under a recover: one unreadable page degrades to an empty page rather than
// taking down the request. Stage 2 degrades gracefully from there, and the
// scanned-document check in Run still catches a document that yielded nothing.
func (e RSCExtractor) Extract(ctx context.Context, doc RawDoc) (LayoutDoc, error) {
	r, err := pdf.NewReader(bytes.NewReader(doc.Bytes), int64(len(doc.Bytes)))
	if err != nil {
		return LayoutDoc{}, fmt.Errorf("open pdf %q: %w: %v", doc.Filename, ErrCorrupt, err)
	}

	n := r.NumPage()
	out := LayoutDoc{Pages: make([]Page, 0, n)}
	for i := 1; i <= n; i++ {
		if err := ctx.Err(); err != nil {
			return LayoutDoc{}, fmt.Errorf("extract %q: %w", doc.Filename, err)
		}
		out.Pages = append(out.Pages, e.readPage(r, i))
	}
	return out, nil
}

// readPage extracts one page, recovering from any panic inside rsc.io/pdf.
func (RSCExtractor) readPage(r *pdf.Reader, num int) (p Page) {
	p = Page{Num: num}
	defer func() {
		// A panic here means this page's content stream is malformed. Returning
		// the page empty is strictly better than failing the whole document:
		// the rest of the circular still compiles into obligations.
		_ = recover()
	}()

	page := r.Page(num)
	if page.V.IsNull() {
		return p
	}

	width, height := mediaBox(page)
	rotate := int(page.V.Key("Rotate").Int64())
	// Normalise to [0,360) so 90 and -270 are the same rotation.
	rotate = ((rotate % 360) + 360) % 360
	p.Rotate = rotate
	p.Width, p.Height = width, height
	if rotate == 90 || rotate == 270 {
		p.Width, p.Height = height, width
	}

	content := page.Content()
	frags := make([]TextRun, 0, len(content.Text))
	for _, t := range content.Text {
		x, y := t.X, t.Y
		// Honour /Rotate so a rotated page still reads top-to-bottom,
		// left-to-right in the coordinate space Stage 2 assumes.
		switch rotate {
		case 90:
			x, y = t.Y, width-t.X
		case 180:
			x, y = width-t.X, height-t.Y
		case 270:
			x, y = height-t.Y, t.X
		}
		bold, italic := styleFlags(t.Font)
		frags = append(frags, TextRun{
			Text:     t.S,
			X:        x,
			Y:        y,
			Width:    t.W,
			FontSize: t.FontSize,
			FontName: t.Font,
			Bold:     bold,
			Italic:   italic,
		})
	}
	p.Runs = coalesce(frags)
	return p
}

// mediaBox returns the page's width and height in points, defaulting to A4 when
// the box is missing or degenerate.
func mediaBox(page pdf.Page) (width, height float64) {
	const (
		a4Width  = 595.276
		a4Height = 841.89
	)
	box := page.V.Key("MediaBox")
	if box.Kind() != pdf.Array || box.Len() != 4 {
		return a4Width, a4Height
	}
	x0, y0 := box.Index(0).Float64(), box.Index(1).Float64()
	x1, y1 := box.Index(2).Float64(), box.Index(3).Float64()
	width, height = x1-x0, y1-y0
	if width <= 0 || height <= 0 {
		return a4Width, a4Height
	}
	return width, height
}
