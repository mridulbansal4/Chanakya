package ingest

import (
	"bytes"
	"fmt"
	"strings"
)

// A digitally-generated SEBI-shaped circular, built here rather than committed
// as a binary blob so the golden test's INPUT is reviewable as source.
//
// WHY THIS EXISTS INSTEAD OF Documents/MITC_Circular_17Feb2025.pdf
// That file - and IA_Master_Circular_2025.pdf beside it - is an image-only scan
// ("Microsoft: Print To PDF", zero font objects, DCTDecode images). It is
// exactly the document class Stage 0 is specified to REJECT, so it cannot also
// be the fixture that proves Stages 0-2 parse correctly. TestMITCIsRejectedAsScanned
// pins that rejection as the real assertion about that file. The remaining PDFs
// in Documents/ are ReportLab output using the ASCII85Decode stream filter,
// which rsc.io/pdf does not implement (it panics), so they yield no text either.
//
// The clause text below is the repo's own SEBI IA Master Circular fixture
// (internal/fixtures/ia_master_circular.json) laid out as a circular, so the
// golden tree is about real regulation, not invented prose.

// helveticaWidths and helveticaBoldWidths are the Adobe AFM advance widths (per
// 1000 em) for ASCII 32-126. rsc.io/pdf computes each glyph's X advance from the
// font's /Widths array; without it every character lands at the same X and the
// extractor cannot tell words apart. Real-world circulars embed their fonts with
// widths, so this only matters for a synthesised fixture.
var helveticaWidths = []int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}

var helveticaBoldWidths = []int{
	278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 333, 333, 584, 584, 584, 611,
	975, 722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 333, 278, 333, 584, 556,
	333, 556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611,
	611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500, 389, 280, 389, 584,
}

// textWidth returns the rendered width of s in points.
func textWidth(s string, size float64, bold bool) float64 {
	w := helveticaWidths
	if bold {
		w = helveticaBoldWidths
	}
	total := 0
	for _, r := range s {
		if r < 32 || r > 126 {
			r = 32
		}
		total += w[r-32]
	}
	return float64(total) * size / 1000
}

// fixtureLine is one laid-out line of the synthetic circular.
type fixtureLine struct {
	text string
	x    float64
	size float64
	bold bool
}

// wrapText greedily wraps s to maxWidth points at the given size.
func wrapText(s string, size float64, bold bool, maxWidth float64) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var (
		out  []string
		cur  = words[0]
		curW = textWidth(words[0], size, bold)
		sp   = textWidth(" ", size, bold)
	)
	for _, w := range words[1:] {
		ww := textWidth(w, size, bold)
		if curW+sp+ww > maxWidth {
			out = append(out, cur)
			cur, curW = w, ww
			continue
		}
		cur += " " + w
		curW += sp + ww
	}
	return append(out, cur)
}

const (
	fixturePageWidth  = 595.0
	fixturePageHeight = 842.0
	fixtureMargin     = 64.0
	fixtureBodySize   = 10.0
	fixtureLead       = 14.0
)

// block is an authored piece of the fixture circular before layout.
type block struct {
	text   string
	indent float64
	size   float64
	bold   bool
	gap    float64 // extra vertical space before this block
}

// fixtureBlocks is the circular's content: the repo's IA Master Circular
// clauses, arranged with chapters, numbered clauses, a proviso, an explanation,
// a lettered list, and a table - one instance of every construct Stage 2 parses.
func fixtureBlocks() []block {
	return []block{
		{text: "SECURITIES AND EXCHANGE BOARD OF INDIA", size: 13, bold: true, gap: 0},
		{text: "Master Circular for Investment Advisers", size: 12, bold: true, gap: 6},
		{text: "SEBI/HO/IMD/IMD-PoD-1/P/CIR/2024/49 dated May 15, 2024", size: 9, gap: 4},

		{text: "CHAPTER I - PRELIMINARY", size: 11, bold: true, gap: 16},
		{text: "1. Preliminary", size: 10, bold: true, gap: 10},
		{text: "This Master Circular consolidates the directions issued to Investment Advisers registered with the Board and shall be read with the SEBI (Investment Advisers) Regulations, 2013.", gap: 4},
		{text: "1.1 Applicability", size: 10, bold: true, gap: 8, indent: 12},
		{text: "The provisions of this circular apply to every person registered as an Investment Adviser and to every person carrying on the activity of providing investment advice for consideration.", gap: 4, indent: 12},
		{text: "1.2 Definitions", size: 10, bold: true, gap: 8, indent: 12},
		{text: "For the purposes of this circular, 'client' means any person who receives investment advice for consideration, and 'fees' means the consideration charged for such advice in a financial year.", gap: 4, indent: 12},

		{text: "CHAPTER II - REGISTRATION", size: 11, bold: true, gap: 16},
		{text: "3. Registration of Investment Advisers", size: 10, bold: true, gap: 10},
		{text: "No person shall act as an investment adviser or hold itself out as an investment adviser unless it has obtained a certificate of registration from the Board under these provisions.", gap: 4},
		{text: "3.1 Threshold for registration", size: 10, bold: true, gap: 8, indent: 12},
		{text: "A person providing investment advice to 300 or more clients, or charging fees exceeding INR 3,00,00,000 (Rupees three crore) in a financial year, must apply for registration as a non-individual investment adviser.", gap: 4, indent: 12},
		{text: "3.2 Application timeline", size: 10, bold: true, gap: 8, indent: 12},
		{text: "An investment adviser who crosses the threshold specified in clause 3.1 shall submit a complete application for registration within 30 days of crossing such threshold.", gap: 4, indent: 12},
		{text: "Provided that an adviser who has already applied under regulation 15 need not submit a fresh application.", gap: 4, indent: 24},
		{text: "(a) the application shall be accompanied by the prescribed fee; and", gap: 4, indent: 24},
		{text: "(b) the applicant shall furnish the certifications required by the Board.", gap: 4, indent: 24},

		{text: "CHAPTER III - CONDUCT", size: 11, bold: true, gap: 16},
		{text: "4. Conduct of Business", size: 10, bold: true, gap: 10},
		{text: "Every investment adviser shall act in a fiduciary capacity towards its clients and shall maintain an arm's length relationship between its advisory and any distribution activities.", gap: 4},
		{text: "4.1 Disclosure of fees", size: 10, bold: true, gap: 8, indent: 12},
		{text: "An investment adviser must disclose to the client, in writing and before charging any fee, the complete fee schedule including the basis of computation and any conflicts of interest.", gap: 4, indent: 12},
		{text: "4.2 Client-level segregation", size: 10, bold: true, gap: 8, indent: 12},
		{text: "An investment adviser must not provide both advisory and distribution services to the same client, and shall maintain client-level segregation of advisory and distribution at all times.", gap: 4, indent: 12},
		{text: "Explanation. For the purposes of this clause, segregation means separate records, separate personnel and separate client consent.", gap: 4, indent: 24},

		{text: "CHAPTER IV - RECORDS", size: 11, bold: true, gap: 16},
		{text: "5. Records and Reporting", size: 10, bold: true, gap: 10},
		{text: "Every investment adviser shall maintain records of investment advice provided and interactions with clients in a manner that supports audit and inspection by the Board.", gap: 4},
		{text: "5.1 Record retention", size: 10, bold: true, gap: 8, indent: 12},
		{text: "An investment adviser must retain all records of investment advice, client agreements, and risk profiling for a period of 5 years from the date of the relevant interaction.", gap: 4, indent: 12},
		{text: "5.2 Client notification duty", size: 10, bold: true, gap: 8, indent: 12},
		{text: "An investment adviser must notify each affected client in writing within 7 days of any material change to the fee structure, conflicts of interest, or the adviser's registration status.", gap: 4, indent: 12},
	}
}

// amendedBlocks is the same circular, amended: the retention period changes
// from 5 to 8 years, a new MITC clause is added, and the header carries explicit
// supersession language so Stage 3 classifies it as an amendment and Stage 9 can
// reach the `deleted` path.
func amendedBlocks() []block {
	out := make([]block, 0, len(fixtureBlocks())+2)
	for _, b := range fixtureBlocks() {
		switch {
		case strings.HasPrefix(b.text, "SEBI/HO/IMD"):
			b.text = "SEBI/HO/IMD/IMD-PoD-1/P/CIR/2025/88 dated June 2, 2025, issued in supersession of " +
				"SEBI/HO/IMD/IMD-PoD-1/P/CIR/2024/49"
		case strings.HasPrefix(b.text, "An investment adviser must retain all records"):
			b.text = strings.Replace(b.text, "period of 5 years", "period of 8 years", 1)
		}
		out = append(out, b)
	}
	return append(out,
		block{text: "5.3 Most Important Terms and Conditions", size: 10, bold: true, gap: 8, indent: 12},
		block{text: "An investment adviser must provide every client the standardized Most Important Terms and Conditions specified by the Board, and must obtain the client's acknowledgement.", gap: 4, indent: 12},
	)
}

// buildAmendedFixturePDF emits the amended circular.
func buildAmendedFixturePDF() []byte {
	original := fixtureBlocksOverride
	fixtureBlocksOverride = amendedBlocks()
	defer func() { fixtureBlocksOverride = original }()
	return buildFixturePDF()
}

// fixtureBlocksOverride lets buildAmendedFixturePDF reuse the whole layout and
// PDF-writing path rather than duplicating it, so the two documents differ only
// in their text.
var fixtureBlocksOverride []block

// fixtureTable is a three-row aligned block that must be recognised as a table.
var fixtureTable = [][]string{
	{"Record", "Retention", "Owner"},
	{"Client agreement", "5 years", "Compliance"},
	{"Risk profiling", "5 years", "Advisory"},
	{"Fee schedule", "5 years", "Operations"},
}

// tableColumnX are the column origins, spaced far enough apart that the
// coalescer treats them as separate runs rather than one line of prose.
var tableColumnX = []float64{fixtureMargin, fixtureMargin + 180, fixtureMargin + 340}

// layoutFixture turns the authored blocks into positioned lines per page.
func layoutFixture() [][]fixtureLine {
	var (
		pages [][]fixtureLine
		cur   []fixtureLine
		y     = fixturePageHeight - fixtureMargin
	)
	newPage := func() {
		pages = append(pages, cur)
		cur = nil
		y = fixturePageHeight - fixtureMargin
	}

	blocks := fixtureBlocks()
	if fixtureBlocksOverride != nil {
		blocks = fixtureBlocksOverride
	}
	for _, b := range blocks {
		size := b.size
		if size == 0 {
			size = fixtureBodySize
		}
		y -= b.gap
		maxW := fixturePageWidth - 2*fixtureMargin - b.indent
		for _, ln := range wrapText(b.text, size, b.bold, maxW) {
			if y < fixtureMargin+fixtureLead {
				newPage()
			}
			cur = append(cur, fixtureLine{text: ln, x: fixtureMargin + b.indent, size: size, bold: b.bold})
			y -= fixtureLead
		}
	}

	// The table goes last, all rows on one page so the detector sees them as
	// consecutive lines.
	y -= 16
	if y < fixtureMargin+float64(len(fixtureTable)+1)*fixtureLead {
		newPage()
	}
	for _, row := range fixtureTable {
		for i, cell := range row {
			cur = append(cur, fixtureLine{text: cell, x: tableColumnX[i], size: fixtureBodySize})
		}
		y -= fixtureLead
	}

	pages = append(pages, cur)
	return pages
}

// escapePDFString escapes the characters that would otherwise terminate a PDF
// literal string.
func escapePDFString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}

// buildFixturePDF emits an uncompressed, single-byte-encoded PDF. Uncompressed
// on purpose: the fixture stays inspectable, and nothing about the golden tree
// depends on a compression implementation.
func buildFixturePDF() []byte {
	pages := layoutFixture()

	var (
		contents []string
		yStart   = fixturePageHeight - fixtureMargin
	)
	for _, page := range pages {
		var b strings.Builder
		y := yStart
		lastY := 0.0
		for i, ln := range page {
			font := "F1"
			if ln.bold {
				font = "F2"
			}
			// Table rows share a baseline: cells on the same row must be drawn
			// at the same Y or the line grouper will not see them as one line.
			if i > 0 && page[i-1].x < ln.x && page[i-1].size == ln.size && isTableCell(ln) && isTableCell(page[i-1]) {
				y = lastY
			} else {
				y -= fixtureLead
			}
			lastY = y
			fmt.Fprintf(&b, "BT\n/%s %g Tf\n1 0 0 1 %g %g Tm\n(%s) Tj\nET\n",
				font, ln.size, ln.x, y, escapePDFString(ln.text))
		}
		contents = append(contents, b.String())
	}

	// Object layout: 1 catalog, 2 pages, 3 font F1, 4 font F2, then per page a
	// page dict and a content stream.
	var objs []string
	kids := make([]string, 0, len(pages))
	firstPageObj := 5
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", firstPageObj+i*2))
	}
	objs = append(objs,
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)),
		fontObj("Helvetica", helveticaWidths),
		fontObj("Helvetica-Bold", helveticaBoldWidths),
	)
	for i, c := range contents {
		pageObj := firstPageObj + i*2
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %g %g] "+
				"/Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>",
				fixturePageWidth, fixturePageHeight, pageObj+1),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(c), c),
		)
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs)+1)
	for i, o := range objs {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	start := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for i := 1; i <= len(objs); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objs)+1, start)
	return buf.Bytes()
}

// isTableCell reports whether a laid-out line sits at one of the table columns.
func isTableCell(l fixtureLine) bool {
	for _, x := range tableColumnX {
		if l.x == x {
			return x != fixtureMargin || isTableText(l.text)
		}
	}
	return false
}

// isTableText reports whether the text is one of the table's first-column cells.
func isTableText(s string) bool {
	for _, row := range fixtureTable {
		if row[0] == s {
			return true
		}
	}
	return false
}

// fontObj builds a Type1 base-14 font dictionary carrying explicit widths.
func fontObj(name string, widths []int) string {
	parts := make([]string, 0, len(widths))
	for _, w := range widths {
		parts = append(parts, fmt.Sprint(w))
	}
	return fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding "+
		"/FirstChar 32 /LastChar 126 /Widths [%s] >>", name, strings.Join(parts, " "))
}
