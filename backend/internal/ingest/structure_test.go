package ingest

import (
	"strings"
	"testing"
)

// TestNumberingLexer covers the precedence order. Applied in the order the
// prompt lists them, `^\d+\.` would claim "3.1" before the two-part pattern ran;
// deepest-first is that precedence expressed as evaluation order.
func TestNumberingLexer(t *testing.T) {
	cases := []struct {
		line      string
		wantLevel int
		wantRef   string
	}{
		{"CHAPTER II - REGISTRATION", levelDivision, ""},
		{"ANNEXURE A", levelDivision, ""},
		{"3. Registration", levelOne, "3"},
		{"3 Registration", levelOne, "3"},
		{"3.1 Threshold", levelTwo, "3.1"},
		{"3.1.1 Sub threshold", levelThree, "3.1.1"},
		{"(1) first item", levelThree, "(1)"},
		{"(a) alpha item", levelAlpha, "(a)"},
		{"(ii) roman item", levelRoman, "(ii)"},
		{"Ordinary body text with no numbering.", levelUnnumber, ""},
	}
	for _, tc := range cases {
		got := lexNumbering(tc.line, "")
		if got.Level != tc.wantLevel {
			t.Errorf("lexNumbering(%q).Level = %d, want %d", tc.line, got.Level, tc.wantLevel)
		}
		if tc.wantRef != "" && got.Ref != tc.wantRef {
			t.Errorf("lexNumbering(%q).Ref = %q, want %q", tc.line, got.Ref, tc.wantRef)
		}
	}
}

// TestRomanAlphaAmbiguity: "(i)" is both the ninth letter and the first roman
// numeral. Sequence continuity decides; with nothing to continue, the answer is
// alpha at REDUCED confidence rather than a confident guess.
func TestRomanAlphaAmbiguity(t *testing.T) {
	afterH := lexNumbering("(i) continues the letters", "(h)")
	if afterH.Level != levelAlpha || afterH.Confidence != 1 {
		t.Errorf("after (h): level=%d conf=%v, want alpha at full confidence", afterH.Level, afterH.Confidence)
	}

	afterRoman := lexNumbering("(i) restarts?", "(iii)")
	if afterRoman.Level != levelRoman {
		t.Errorf("after (iii): level=%d, want roman", afterRoman.Level)
	}

	orphan := lexNumbering("(i) first in its list", "")
	if orphan.Level != levelAlpha {
		t.Errorf("no preceding sibling: level=%d, want alpha (the documented default)", orphan.Level)
	}
	if orphan.Confidence >= 1 {
		t.Errorf("no preceding sibling: confidence = %v, want < 1 - the ambiguity must be recorded", orphan.Confidence)
	}

	// Unambiguous cases must stay fully confident.
	if got := lexNumbering("(b) second", "(a)"); got.Level != levelAlpha || got.Confidence != 1 {
		t.Errorf("(b): level=%d conf=%v, want alpha at full confidence", got.Level, got.Confidence)
	}
	if got := lexNumbering("(iv) fourth", ""); got.Level != levelRoman || got.Confidence != 1 {
		t.Errorf("(iv): level=%d conf=%v, want roman at full confidence", got.Level, got.Confidence)
	}
}

// mkLine is a test helper building a single-run line.
func mkLine(page int, y, x, size float64, bold bool, text string) line {
	return line{page: page, y: y, runs: []TextRun{{Text: text, X: x, Y: y, Width: float64(len(text)) * size * 0.5, FontSize: size, FontName: "T", Bold: bold}}}
}

// TestOrphanedClauseLevel: a "3.2" appearing with no "3" or "3.1" before it must
// attach to the nearest preceding shallower node - never fail the whole parse.
func TestOrphanedClauseLevel(t *testing.T) {
	lines := []line{
		mkLine(1, 700, 64, 10, true, "CHAPTER I - PRELIMINARY"),
		mkLine(1, 686, 64, 10, false, "3.2 An orphan sub-clause with no parent numbering."),
		mkLine(1, 672, 64, 10, false, "More body text for the orphan."),
		mkLine(1, 658, 64, 10, false, "4. A properly numbered clause."),
	}
	nodes := assemble(lines, map[int]int{}, 10)
	if len(nodes) < 2 {
		t.Fatalf("expected the parse to survive, got %d nodes", len(nodes))
	}
	var orphan *StructNode
	for i := range nodes {
		if nodes[i].Ref == "3.2" {
			orphan = &nodes[i]
		}
	}
	if orphan == nil {
		t.Fatal("orphan clause 3.2 was dropped")
	}
	if orphan.ParentRef != "chapter-i" {
		t.Errorf("orphan parent = %q, want the nearest preceding shallower node %q", orphan.ParentRef, "chapter-i")
	}
}

// TestTableFalsePositiveGuard: both conditions - >= 3 rows AND a consistent,
// aligned column count - must hold. A missed table degrades to paragraphs
// harmlessly; a false-positive table rewrites prose into cells and would corrupt
// the verbatim-citation invariant.
func TestTableFalsePositiveGuard(t *testing.T) {
	twoRows := []line{
		{page: 1, y: 700, runs: []TextRun{{Text: "A", X: 64, Y: 700, FontSize: 10}, {Text: "B", X: 240, Y: 700, FontSize: 10}}},
		{page: 1, y: 686, runs: []TextRun{{Text: "C", X: 64, Y: 686, FontSize: 10}, {Text: "D", X: 240, Y: 686, FontSize: 10}}},
	}
	if got := detectTables(twoRows); len(got) != 0 {
		t.Errorf("two aligned rows were classified as a table: %v", got)
	}

	misaligned := []line{
		{page: 1, y: 700, runs: []TextRun{{Text: "A", X: 64, Y: 700, FontSize: 10}, {Text: "B", X: 240, Y: 700, FontSize: 10}}},
		{page: 1, y: 686, runs: []TextRun{{Text: "C", X: 64, Y: 686, FontSize: 10}, {Text: "D", X: 330, Y: 686, FontSize: 10}}},
		{page: 1, y: 672, runs: []TextRun{{Text: "E", X: 64, Y: 672, FontSize: 10}, {Text: "F", X: 410, Y: 672, FontSize: 10}}},
	}
	if got := detectTables(misaligned); len(got) != 0 {
		t.Errorf("misaligned columns were classified as a table: %v", got)
	}

	aligned := []line{
		{page: 1, y: 700, runs: []TextRun{{Text: "A", X: 64, Y: 700, FontSize: 10}, {Text: "B", X: 240, Y: 700, FontSize: 10}}},
		{page: 1, y: 686, runs: []TextRun{{Text: "C", X: 64, Y: 686, FontSize: 10}, {Text: "D", X: 240, Y: 686, FontSize: 10}}},
		{page: 1, y: 672, runs: []TextRun{{Text: "E", X: 64, Y: 672, FontSize: 10}, {Text: "F", X: 240, Y: 672, FontSize: 10}}},
	}
	if got := detectTables(aligned); len(got) != 3 {
		t.Errorf("three aligned rows: got %d table lines, want 3", len(got))
	}
}

// TestTableTextIsVerbatimSuperset: serialising a table adds separators only.
// Every original cell string must still appear, or a citation into a table cell
// would stop matching.
func TestTableTextIsVerbatimSuperset(t *testing.T) {
	rows := []line{
		{page: 1, y: 700, runs: []TextRun{{Text: "Client agreement", X: 64}, {Text: "5 years", X: 240}}},
		{page: 1, y: 686, runs: []TextRun{{Text: "Risk profiling", X: 64}, {Text: "5 years", X: 240}}},
	}
	got := tableText(rows)
	for _, r := range rows {
		for _, run := range r.runs {
			if !strings.Contains(got, run.Text) {
				t.Errorf("serialised table lost cell %q:\n%s", run.Text, got)
			}
		}
	}
}

// TestDegradation: an unrecognisable document becomes a flat paragraph list with
// "p{page}.¶{n}" refs rather than a failed upload. It still compiles into
// obligations; only the hierarchy is poorer.
func TestDegradation(t *testing.T) {
	lines := []line{
		mkLine(1, 700, 64, 10, false, "Dear Sir or Madam,"),
		mkLine(1, 686, 64, 10, false, "We write in connection with your recent query."),
		mkLine(1, 672, 64, 10, false, "Kindly note that the position remains unchanged."),
		mkLine(1, 658, 64, 10, false, "Yours faithfully."),
	}
	doc := StructuredDoc{}
	nodes := assemble(lines, map[int]int{}, 10)
	if structuredEnough(nodes) {
		t.Fatal("a letter with no numbering must not be treated as structured")
	}
	doc.Nodes = flatten(lines)
	doc.Degraded = true
	if len(doc.Nodes) != 4 {
		t.Fatalf("flattened to %d nodes, want 4", len(doc.Nodes))
	}
	if doc.Nodes[0].Ref != "p1.¶1" {
		t.Errorf("degraded ref = %q, want %q", doc.Nodes[0].Ref, "p1.¶1")
	}
	for _, n := range doc.Nodes {
		if n.Text == "" {
			t.Error("degraded node has empty text")
		}
	}
}

// TestFootnoteAttachesToHostClause: a footnote is part of the clause's meaning.
// Promoting it to a sibling node would let it generate an obligation of its own.
func TestFootnoteAttachesToHostClause(t *testing.T) {
	lines := []line{
		mkLine(1, 700, 64, 10, false, "5.1 An adviser must retain records for 5 years."),
		mkLine(1, 686, 64, 6, false, "1 Computed from the date of the relevant interaction."),
		mkLine(1, 672, 64, 10, false, "5.2 An adviser must notify clients."),
	}
	nodes := assemble(lines, map[int]int{}, 10)
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2 (the footnote must not become a node)", len(nodes))
	}
	if !strings.Contains(nodes[0].Text, "Computed from the date") {
		t.Errorf("footnote text did not attach to its host clause: %q", nodes[0].Text)
	}
}

// TestSpecialBlocks: provisos, explanations and illustrations become children of
// the clause they qualify, tagged by kind, so a later stage can treat them
// differently (an illustration never generates an obligation).
func TestSpecialBlocks(t *testing.T) {
	lines := []line{
		mkLine(1, 700, 64, 10, false, "3.2 An adviser shall apply within 30 days."),
		mkLine(1, 686, 88, 10, false, "Provided that an existing applicant need not reapply."),
		mkLine(1, 672, 88, 10, false, "Explanation. Days means calendar days."),
		mkLine(1, 658, 88, 10, false, "Illustration: an adviser crossing the threshold on 1 April applies by 1 May."),
	}
	nodes := assemble(lines, map[int]int{}, 10)
	want := []string{KindClause, KindProviso, KindExplanation, KindIllustration}
	if len(nodes) != len(want) {
		t.Fatalf("got %d nodes, want %d", len(nodes), len(want))
	}
	for i, w := range want {
		if nodes[i].Kind != w {
			t.Errorf("node %d kind = %q, want %q", i, nodes[i].Kind, w)
		}
	}
	for _, n := range nodes[1:] {
		if n.ParentRef != "3.2" {
			t.Errorf("%s parent = %q, want the clause it qualifies (3.2)", n.Kind, n.ParentRef)
		}
	}
}

// TestModalFontSizeIsDeterministic guards the histogram tie-break: Go map
// iteration is randomised, so a tie must resolve on the value.
func TestModalFontSizeIsDeterministic(t *testing.T) {
	lines := []line{
		mkLine(1, 700, 64, 10, false, "aaaa"),
		mkLine(1, 686, 64, 12, false, "bbbb"),
	}
	first := modalFontSize(lines)
	for i := 0; i < 50; i++ {
		if got := modalFontSize(lines); got != first {
			t.Fatalf("modalFontSize is not deterministic: %v then %v", first, got)
		}
	}
	if first != 10 {
		t.Errorf("tie broken to %v, want the smaller size 10", first)
	}
}
