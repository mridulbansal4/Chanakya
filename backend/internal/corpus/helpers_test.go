package corpus

import (
	"bytes"
	"fmt"
	"strings"

	"chanakya/internal/domain"
)

// domainClause wraps raw text as a clause for compiler input.
func domainClause(text string) domain.Clause {
	return domain.Clause{
		ID:         "adversarial#1",
		CircularID: "adversarial",
		ClauseRef:  "1",
		Text:       text,
		Ordinal:    1,
	}
}

// buildTextPDF wraps plain text in a minimal, uncompressed PDF so the corpus's
// adversarial document can be pushed through the REAL ingestion path rather than
// only through the compiler. Base-14 Helvetica with explicit widths, matching the
// ingest package's own fixture builder.
func buildTextPDF(text string) []byte {
	const (
		pageWidth  = 595.0
		pageHeight = 842.0
		margin     = 64.0
		fontSize   = 10.0
		lead       = 13.0
	)

	var content strings.Builder
	y := pageHeight - margin
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \r")
		if strings.TrimSpace(line) == "" {
			y -= lead
			continue
		}
		if y < margin {
			break
		}
		fmt.Fprintf(&content, "BT\n/F1 %g Tf\n1 0 0 1 %g %g Tm\n(%s) Tj\nET\n",
			fontSize, margin, y, escapePDF(line))
		y -= lead
	}

	widths := make([]string, 0, 95)
	for i := 0; i < 95; i++ {
		widths = append(widths, "556")
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [4 0 R] /Count 1 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding " +
			"/FirstChar 32 /LastChar 126 /Widths [" + strings.Join(widths, " ") + "] >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %g %g] "+
			"/Resources << /Font << /F1 3 0 R >> >> /Contents 5 0 R >>", pageWidth, pageHeight),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
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

// escapePDF escapes characters that would terminate a PDF literal string.
func escapePDF(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	// Non-ASCII would need a different encoding; the corpus is ASCII.
	var b strings.Builder
	for _, ch := range r.Replace(s) {
		if ch < 32 || ch > 126 {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(ch)
	}
	return b.String()
}
