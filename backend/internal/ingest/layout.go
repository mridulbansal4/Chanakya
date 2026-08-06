package ingest

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// ErrNotEnabled is returned by an extractor that is registered but deliberately
// not available in this build (today: OCR).
var ErrNotEnabled = errors.New("extractor not enabled")

// TextRun is a contiguous piece of text drawn on one baseline in one style.
// Positions are in PDF points with the origin at the page's bottom-left, so Y
// INCREASES upwards - the structural parser sorts descending on Y to read a page
// top-to-bottom.
type TextRun struct {
	Text     string  `json:"text"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	FontSize float64 `json:"font_size"`
	FontName string  `json:"font_name"`
	Bold     bool    `json:"bold"`
	Italic   bool    `json:"italic"`
}

// Page is one page's positioned text.
type Page struct {
	Num    int       `json:"num"`
	Width  float64   `json:"width"`
	Height float64   `json:"height"`
	Rotate int       `json:"rotate"`
	Runs   []TextRun `json:"runs"`
}

// LayoutDoc is Stage 1's output: positioned text, no interpretation.
type LayoutDoc struct {
	Pages []Page `json:"pages"`
}

// PageExtractor turns raw PDF bytes into positioned text. Extraction strategies
// are registered behind this interface - mirroring llm.SelectExtractor - so that
// "OCR is a future adapter" is a structural fact in the code rather than a
// promise in a document.
type PageExtractor interface {
	Name() string
	Extract(ctx context.Context, doc RawDoc) (LayoutDoc, error)
}

// SelectPageExtractor chooses the Stage 1 extractor, in precedence order:
//
//  1. CHANAKYA_PDF_EXTRACTOR names one explicitly (rsc | pdftotext | ocr).
//  2. (default) RSCExtractor - pure Go, no external binary, the only one CI
//     relies on.
//
// ExternalCmdExtractor is an opt-in upgrade because it depends on a binary that
// may or may not exist on the host; silently switching to it would make the
// pipeline's output depend on the machine it runs on, which would break the
// golden test's meaning.
func SelectPageExtractor() PageExtractor {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CHANAKYA_PDF_EXTRACTOR"))) {
	case "pdftotext", "external":
		return ExternalCmdExtractor{}
	case "ocr":
		return OCRExtractor{}
	default:
		return RSCExtractor{}
	}
}

// normalizeText applies Unicode NFKC. PDF text extraction routinely yields
// ligature codepoints (ﬁ, ﬂ) and CID-mangled full-width forms; NFKC folds them
// to their ASCII-compatible equivalents so the clause text a human reads and the
// text the citation gate substring-matches are the same string.
func normalizeText(s string) string {
	s = norm.NFKC.String(s)
	// Normalise the typographic characters PDFs use for quotes and dashes to
	// their ASCII forms. Purely a character-identity fix: nothing is added,
	// removed, or reordered, so the text stays a faithful transcription.
	r := strings.NewReplacer(
		"‘", "'", "’", "'", "“", `"`, "”", `"`,
		"–", "-", "—", "-", " ", " ",
	)
	return r.Replace(s)
}

// coalesce merges character-level fragments into line-level runs.
//
// rsc.io/pdf reports text one drawing operation at a time, which for most PDFs
// means one or two characters. Grouping them back into readable runs is what
// makes the numbering lexer possible at all: "3.1" only exists as a token once
// '3', '.', '1' are joined.
//
// Fragments are grouped by baseline (Y rounded to baselineEpsilon) and style,
// then joined left-to-right. A gap wider than gapRatio of the font size becomes
// a single space - the same rule a human applies reading a line.
//
// A much wider gap (columnGapRatio) ends the run instead of joining it, so a
// table row stays several runs rather than collapsing into one string. Stage 2's
// table detector needs those column boundaries; once they are joined they cannot
// be recovered.
func coalesce(frags []TextRun) []TextRun {
	const (
		baselineEpsilon = 1.5  // points; PDF baselines wobble slightly within a line
		gapRatio        = 0.22 // fraction of font size that counts as a word gap
		columnGapRatio  = 2.5  // fraction of font size that counts as a column break
	)
	if len(frags) == 0 {
		return nil
	}

	type key struct {
		band int
		font string
		size int // font size in tenths, so 10.5 and 10.5 group but 10.5 and 12 do not
	}
	groups := map[key][]TextRun{}
	for _, f := range frags {
		if strings.TrimSpace(f.Text) == "" {
			continue
		}
		k := key{
			band: int(f.Y / baselineEpsilon),
			font: f.FontName,
			size: int(f.FontSize * 10),
		}
		groups[k] = append(groups[k], f)
	}

	out := make([]TextRun, 0, len(groups))
	for _, g := range groups {
		sort.SliceStable(g, func(i, j int) bool { return g[i].X < g[j].X })

		var (
			b       strings.Builder
			startX  = g[0].X
			prevEnd = g[0].X
			style   = g[0]
		)
		flush := func() {
			text := normalizeText(strings.TrimSpace(b.String()))
			if text == "" {
				return
			}
			out = append(out, TextRun{
				Text:     text,
				X:        startX,
				Y:        style.Y,
				Width:    prevEnd - startX,
				FontSize: style.FontSize,
				FontName: style.FontName,
				Bold:     style.Bold,
				Italic:   style.Italic,
			})
		}
		for i, f := range g {
			if i > 0 {
				switch gap := f.X - prevEnd; {
				case gap > columnGapRatio*f.FontSize:
					flush()
					b.Reset()
					startX = f.X
				case gap > gapRatio*f.FontSize && !strings.HasSuffix(b.String(), " "):
					b.WriteByte(' ')
				}
			}
			b.WriteString(f.Text)
			prevEnd = f.X + f.Width
		}
		flush()
	}

	// Reading order: top of page first (Y descends), then left to right. Sorting
	// here rather than at parse time keeps LayoutDoc deterministic, which the
	// golden test depends on - Go map iteration above is not ordered.
	sort.SliceStable(out, func(i, j int) bool {
		if diff := out[i].Y - out[j].Y; diff > baselineEpsilon || diff < -baselineEpsilon {
			return out[i].Y > out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

// styleFlags derives bold/italic from the PDF font name, which is the only
// signal available without parsing the embedded font program. PDF font names
// conventionally carry the weight and slant (e.g. "ABCDEF+TimesNewRoman,Bold").
func styleFlags(fontName string) (bold, italic bool) {
	lower := strings.ToLower(fontName)
	bold = strings.Contains(lower, "bold") || strings.Contains(lower, "black") ||
		strings.Contains(lower, "heavy") || strings.Contains(lower, "semib")
	italic = strings.Contains(lower, "italic") || strings.Contains(lower, "oblique")
	return bold, italic
}
