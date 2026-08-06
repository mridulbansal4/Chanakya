package ingest

import (
	"fmt"
	"sort"
	"strings"

	"chanakya/internal/domain"
)

// Node kinds. The kind changes what a downstream stage may do with the text -
// a proviso qualifies its parent obligation, an illustration never generates one.
const (
	KindDivision     = "division"
	KindClause       = "clause"
	KindProviso      = "proviso"
	KindExplanation  = "explanation"
	KindIllustration = "illustration"
	KindTable        = "table"
	KindParagraph    = "paragraph"
)

// BBox is a node's bounding box on its page, in PDF points.
type BBox struct {
	X0 float64 `json:"x0"`
	Y0 float64 `json:"y0"`
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
}

// StructNode is one node of the parsed clause tree.
type StructNode struct {
	Ref        string  `json:"ref"`
	ParentRef  string  `json:"parent_ref"`
	Heading    string  `json:"heading"`
	Text       string  `json:"text"`
	Ordinal    int     `json:"ordinal"`
	Page       int     `json:"page"`
	BBox       BBox    `json:"bbox"`
	Level      int     `json:"level"`
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence"`
}

// StructuredDoc is Stage 2's output.
type StructuredDoc struct {
	Circular domain.Circular `json:"circular"`
	Nodes    []StructNode    `json:"nodes"`
	// Degraded records that the document's structure was unrecognisable and the
	// nodes are a flat paragraph list. The document still compiles into
	// obligations; only the graph hierarchy is poorer.
	Degraded bool    `json:"degraded"`
	BodySize float64 `json:"body_size"`
}

// line is one visual line of a page: the runs sharing a baseline.
type line struct {
	page int
	y    float64
	runs []TextRun
}

func (l line) text() string {
	parts := make([]string, 0, len(l.runs))
	for _, r := range l.runs {
		parts = append(parts, r.Text)
	}
	return strings.Join(parts, " ")
}

func (l line) minX() float64 {
	m := l.runs[0].X
	for _, r := range l.runs[1:] {
		if r.X < m {
			m = r.X
		}
	}
	return m
}

func (l line) maxSize() float64 {
	m := l.runs[0].FontSize
	for _, r := range l.runs[1:] {
		if r.FontSize > m {
			m = r.FontSize
		}
	}
	return m
}

func (l line) bold() bool {
	for _, r := range l.runs {
		if !r.Bold {
			return false
		}
	}
	return len(l.runs) > 0
}

func (l line) bbox() BBox {
	b := BBox{X0: l.runs[0].X, Y0: l.y, X1: l.runs[0].X + l.runs[0].Width, Y1: l.y + l.maxSize()}
	for _, r := range l.runs[1:] {
		if r.X < b.X0 {
			b.X0 = r.X
		}
		if r.X+r.Width > b.X1 {
			b.X1 = r.X + r.Width
		}
	}
	return b
}

// ParseStructure runs Stage 2: LayoutDoc in, clause tree out. It is fully
// deterministic - no model is consulted - which is what lets the golden test pin
// the exact tree the pipeline produces.
func ParseStructure(doc RawDoc, layout LayoutDoc) (StructuredDoc, error) {
	lines := toLines(layout)
	if len(lines) == 0 {
		return StructuredDoc{}, fmt.Errorf("no text lines in %q", doc.Filename)
	}

	body := modalFontSize(lines)
	out := StructuredDoc{Circular: minimalCircular(doc), BodySize: body}

	tables := detectTables(lines)
	nodes := assemble(lines, tables, body)

	// Degradation, never fail closed: if the numbering lexer recognised almost
	// nothing, the document is not shaped like a circular (a letter, a form, a
	// press release). A flat paragraph list still compiles into obligations with
	// real citations; only the hierarchy is lost. Failing the upload instead
	// would throw away a document CHANAKYA can still reason about.
	if structuredEnough(nodes) {
		out.Nodes = nodes
	} else {
		out.Nodes = flatten(lines)
		out.Degraded = true
	}
	return out, nil
}

// toLines groups each page's runs into visual lines, in reading order.
func toLines(layout LayoutDoc) []line {
	const baselineEpsilon = 1.5
	var out []line
	for _, p := range layout.Pages {
		runs := append([]TextRun(nil), p.Runs...)
		sort.SliceStable(runs, func(i, j int) bool {
			if d := runs[i].Y - runs[j].Y; d > baselineEpsilon || d < -baselineEpsilon {
				return runs[i].Y > runs[j].Y
			}
			return runs[i].X < runs[j].X
		})
		var cur *line
		for _, r := range runs {
			if cur == nil || cur.y-r.Y > baselineEpsilon || r.Y-cur.y > baselineEpsilon {
				out = append(out, line{page: p.Num, y: r.Y, runs: []TextRun{r}})
				cur = &out[len(out)-1]
				continue
			}
			cur.runs = append(cur.runs, r)
		}
	}
	return out
}

// modalFontSize returns the most common font size weighted by character count -
// the body-text baseline every heading test is relative to. Weighting by
// characters rather than by run count keeps a page full of short bold headings
// from outvoting the prose.
func modalFontSize(lines []line) float64 {
	weight := map[int]int{}
	for _, l := range lines {
		for _, r := range l.runs {
			weight[int(r.FontSize*10)] += len([]rune(r.Text))
		}
	}
	best, bestW := 0, -1
	for size, w := range weight {
		// Deterministic tie-break: map iteration order is random, so ties must
		// resolve on the value, not on which key came out first.
		if w > bestW || (w == bestW && size < best) {
			best, bestW = size, w
		}
	}
	if best == 0 {
		return 10
	}
	return float64(best) / 10
}

// indentEpsilon is the horizontal wobble tolerated before two lines are called
// differently indented.
const indentEpsilon = 4.0

// isHeadingCandidate applies the font-based vote: visibly larger than body text,
// or bold.
func isHeadingCandidate(l line, body float64) bool {
	return l.maxSize() > body*1.08 || l.bold()
}

// isFootnote reports whether a line is set small enough to be a footnote or a
// superscript marker.
func isFootnote(l line, body float64) bool {
	return l.maxSize() < body*0.8
}

// specialKind classifies the discourse markers that introduce a sub-node of the
// clause they follow rather than a sibling of it.
func specialKind(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	switch {
	case strings.HasPrefix(lower, "provided that"), strings.HasPrefix(lower, "provided further that"),
		strings.HasPrefix(lower, "provided also that"):
		return KindProviso
	case strings.HasPrefix(lower, "explanation"):
		return KindExplanation
	case strings.HasPrefix(lower, "illustration"):
		return KindIllustration
	default:
		return ""
	}
}

// detectTables returns the set of line indices that belong to a table block.
//
// Two independent conditions must BOTH hold before a block is called a table:
// at least minTableRows consecutive lines, and the same column count with
// aligned column starts across all of them. A missed table degrades to ordinary
// paragraphs, which is harmless; a false-positive table rewrites prose into
// pipe-separated cells and would corrupt the verbatim-citation invariant, so the
// test is deliberately strict.
func detectTables(lines []line) map[int]int {
	const (
		minTableRows = 3
		xTolerance   = 6.0 // points
	)
	blocks := map[int]int{} // line index -> table block id
	blockID := 0

	i := 0
	for i < len(lines) {
		if len(lines[i].runs) < 2 {
			i++
			continue
		}
		cols := columnStarts(lines[i])
		j := i + 1
		for j < len(lines) &&
			lines[j].page == lines[i].page &&
			len(lines[j].runs) == len(lines[i].runs) &&
			alignedColumns(cols, columnStarts(lines[j]), xTolerance) {
			j++
		}
		if j-i >= minTableRows {
			blockID++
			for k := i; k < j; k++ {
				blocks[k] = blockID
			}
			i = j
			continue
		}
		i++
	}
	return blocks
}

func columnStarts(l line) []float64 {
	xs := make([]float64, 0, len(l.runs))
	for _, r := range l.runs {
		xs = append(xs, r.X)
	}
	return xs
}

func alignedColumns(a, b []float64, tol float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if d := a[i] - b[i]; d > tol || d < -tol {
			return false
		}
	}
	return true
}

// tableText serialises a table block to a Markdown-ish form.
//
// Every original cell string is reproduced unchanged; only separators are added.
// The node's text therefore stays a VERBATIM SUPERSET of the page runs, so a
// citation into a table cell still satisfies the compiler's substring check.
func tableText(rows []line) string {
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		cells := make([]string, 0, len(r.runs))
		for _, run := range r.runs {
			cells = append(cells, run.Text)
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |")
	}
	return b.String()
}

// assemble runs the stack machine that turns classified lines into a tree,
// emitting parents strictly before children - the ordering store.UpsertClause
// requires and fixtures.LoadIACircular validates.
func assemble(lines []line, tables map[int]int, body float64) []StructNode {
	var (
		stack    []stackFrame
		nodes    []StructNode
		byRef    = map[string]int{} // ref -> index into nodes
		used     = map[string]bool{}
		prevList string // last alpha/roman marker, for "(i)" disambiguation
		ordinal  int
		// previous emitted node's level and left edge, for level reconciliation
		prevLevel int
		prevX     float64
	)

	// Text that appears BEFORE the first structural node - the issuing authority,
	// the circular number, the date line, the subject - is front matter. It has
	// no numbering, so it would otherwise be dropped on the floor, taking the
	// circular number and issue date with it and leaving Stage 3 with nothing to
	// read. It is collected into an explicit preamble node instead.
	var preamble []string

	appendText := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		if len(nodes) == 0 {
			preamble = append(preamble, text)
			return
		}
		n := &nodes[len(nodes)-1]
		if n.Text == "" {
			n.Text = text
			return
		}
		n.Text += " " + text
	}

	// relativeRef is set for parenthesised markers, which are only unique within
	// their parent: "(a)" under 3.2 and "(a)" under 4.1 are different clauses.
	// The parent is only known AFTER the stack has been popped to this node's
	// level, so the qualification happens inside emit rather than at the call
	// site - computing it earlier would nest "(b)" under its own sibling "(a)".
	// flushPreamble emits the collected front matter as the document's first
	// node, immediately before the first real node is created.
	flushPreamble := func(l line) {
		if len(preamble) == 0 {
			return
		}
		text := strings.Join(preamble, " ")
		preamble = nil
		ordinal++
		ref := uniqueRef("preamble", used)
		nodes = append(nodes, StructNode{
			Ref: ref, Text: text, Ordinal: ordinal,
			Page: l.page, BBox: l.bbox(), Level: levelDivision,
			Kind: KindParagraph, Confidence: 1,
		})
		byRef[ref] = len(nodes) - 1
	}

	emit := func(ref, heading, text string, level int, l line, kind string, conf float64, relativeRef bool) {
		flushPreamble(l)
		// Orphan levels resolve here: popping to the first frame with a strictly
		// lower level attaches a stray "3.2" to whatever preceded it (or to the
		// document root), instead of failing the parse over one bad number.
		for len(stack) > 0 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1]
		}
		parent := ""
		if len(stack) > 0 {
			parent = stack[len(stack)-1].ref
		}
		if relativeRef && parent != "" {
			ref = parent + ref
		}
		ref = uniqueRef(ref, used)
		ordinal++
		nodes = append(nodes, StructNode{
			Ref: ref, ParentRef: parent, Heading: heading, Text: text,
			Ordinal: ordinal, Page: l.page, BBox: l.bbox(), Level: level,
			Kind: kind, Confidence: conf,
		})
		byRef[ref] = len(nodes) - 1
		stack = append(stack, stackFrame{
			ref:     ref,
			level:   level,
			special: kind == KindProviso || kind == KindExplanation || kind == KindIllustration,
		})
	}

	seenTable := map[int]bool{}
	for i, l := range lines {
		text := strings.TrimSpace(l.text())
		if text == "" {
			continue
		}

		// Tables first: a table row must not be run through the numbering lexer,
		// or a leading "1" in a cell would open a phantom clause.
		if id, ok := tables[i]; ok {
			if seenTable[id] {
				continue
			}
			seenTable[id] = true
			var rows []line
			for k := i; k < len(lines) && tables[k] == id; k++ {
				rows = append(rows, lines[k])
			}
			lvl := 1
			if len(stack) > 0 {
				lvl = stack[len(stack)-1].level + 1
			}
			emit(fmt.Sprintf("p%d.tbl%d", l.page, id), "", tableText(rows), lvl, l, KindTable, 1, false)
			continue
		}

		// Footnotes attach to the clause they annotate, never as a sibling: a
		// footnote is part of the host clause's meaning, and promoting it to a
		// node would let it generate an obligation of its own.
		if isFootnote(l, body) {
			appendText(text)
			continue
		}

		if kind := specialKind(text); kind != "" {
			lvl, parentRef := 1, "doc"
			if host, ok := hostFrame(stack); ok {
				lvl, parentRef = host.level+1, host.ref
			}
			emit(fmt.Sprintf("%s.%s%d", parentRef, kind[:4], ordinal+1), "", text, lvl, l, kind, 1, false)
			continue
		}

		n := lexNumbering(text, prevList)
		if n.Level == levelUnnumber {
			appendText(text)
			continue
		}
		if n.Level == levelAlpha || n.Level == levelRoman {
			prevList = n.Ref
		}

		// Level reconciliation. Numbering depth and INDENTATION depth are two
		// independent votes on how deep a clause sits. They usually agree; when
		// they contradict each other - the numbering goes deeper while the text
		// moves left, or vice versa - the parser records reduced confidence
		// rather than picking a winner, so a reviewer can see exactly which
		// nodes it was unsure about instead of trusting a silent guess.
		conf := n.Confidence
		if prevLevel > 0 {
			numDeeper := n.Level > prevLevel
			numShallower := n.Level < prevLevel
			indentDeeper := l.minX() > prevX+indentEpsilon
			indentShallower := l.minX() < prevX-indentEpsilon
			if (numDeeper && indentShallower) || (numShallower && indentDeeper) {
				conf *= 0.75
			}
		}
		prevLevel, prevX = n.Level, l.minX()

		ref, relative := n.Ref, false
		switch {
		case n.Level == levelDivision:
			ref = divisionRef(text)
		case strings.HasPrefix(n.Ref, "("):
			relative = true
		}

		heading, bodyText := "", n.Rest
		if n.Level <= levelOne || isHeadingCandidate(l, body) {
			heading = n.Rest
			bodyText = ""
		}
		kind := KindClause
		if n.Level == levelDivision {
			kind = KindDivision
		}
		emit(ref, heading, bodyText, n.Level, l, kind, conf, relative)
	}
	return nodes
}

// stackFrame is one open node in the tree-assembly stack machine.
type stackFrame struct {
	ref     string
	level   int
	special bool // proviso / explanation / illustration
}

// hostFrame returns the innermost frame that is NOT a proviso/explanation/
// illustration, i.e. the clause such a block qualifies.
//
// Consecutive provisos and explanations are SIBLINGS of one another, all
// qualifying the same clause. Without this, "Provided that ..." followed by
// "Explanation. ..." would nest the explanation inside the proviso and claim the
// explanation qualifies the proviso - a different legal reading of the text.
func hostFrame(stack []stackFrame) (stackFrame, bool) {
	for i := len(stack) - 1; i >= 0; i-- {
		if !stack[i].special {
			return stack[i], true
		}
	}
	return stackFrame{}, false
}

// divisionRef slugifies a division heading into a stable ref, e.g.
// "CHAPTER II - REGISTRATION" -> "chapter-ii".
func divisionRef(text string) string {
	fields := strings.Fields(strings.ToLower(text))
	if len(fields) > 2 {
		fields = fields[:2]
	}
	slug := strings.Join(fields, "-")
	return strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return -1
		}
	}, slug), "-")
}

// uniqueRef disambiguates a repeated ref (a circular that restarts numbering in
// an annexure) so clause ids stay unique.
func uniqueRef(ref string, used map[string]bool) string {
	if ref == "" {
		ref = "node"
	}
	if !used[ref] {
		used[ref] = true
		return ref
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s~%d", ref, i)
		if !used[cand] {
			used[cand] = true
			return cand
		}
	}
}

// structuredEnough decides whether the numbering lexer found a real hierarchy.
func structuredEnough(nodes []StructNode) bool {
	if len(nodes) < 3 {
		return false
	}
	numbered := 0
	for _, n := range nodes {
		if n.Kind == KindClause || n.Kind == KindDivision {
			numbered++
		}
	}
	return numbered >= 3
}

// flatten is the degraded representation: one node per paragraph, refs of the
// form "p{page}.¶{n}".
func flatten(lines []line) []StructNode {
	var (
		out     []StructNode
		perPage = map[int]int{}
	)
	for _, l := range lines {
		text := strings.TrimSpace(l.text())
		if text == "" {
			continue
		}
		perPage[l.page]++
		out = append(out, StructNode{
			Ref:        fmt.Sprintf("p%d.¶%d", l.page, perPage[l.page]),
			Text:       text,
			Ordinal:    len(out) + 1,
			Page:       l.page,
			BBox:       l.bbox(),
			Level:      1,
			Kind:       KindParagraph,
			Confidence: 0.5,
		})
	}
	return out
}

// minimalCircular is the placeholder document record Stage 2 emits. The real
// metadata (circular number, issue date, effective date, supersessions) is
// Stage 3's job in Phase 2; deriving it from the content address here keeps the
// id deterministic and collision-free until then.
func minimalCircular(doc RawDoc) domain.Circular {
	return domain.Circular{
		ID:        "doc:" + doc.SHA256[:16],
		Title:     doc.Filename,
		Regulator: "SEBI",
	}
}

// Clauses projects the parsed tree onto the type the compiler consumes.
//
// Nodes are already in document pre-order with parents before children, so the
// result satisfies the ordering store.UpsertClause requires. Temporal columns
// are left zero: ingestion does not know when a regulation came into force.
func (d StructuredDoc) Clauses() []domain.Clause {
	out := make([]domain.Clause, 0, len(d.Nodes))
	for _, n := range d.Nodes {
		text := n.Text
		if text == "" {
			// A heading-only node still needs text: the citation gate checks
			// the source sentence against Clause.Text, and an empty string
			// would make every citation into that clause fail.
			text = n.Heading
		}
		parent := ""
		if n.ParentRef != "" {
			parent = domain.ClauseID(d.Circular.ID, n.ParentRef)
		}
		out = append(out, domain.Clause{
			ID:         domain.ClauseID(d.Circular.ID, n.Ref),
			CircularID: d.Circular.ID,
			ClauseRef:  n.Ref,
			ParentID:   parent,
			Heading:    n.Heading,
			Text:       text,
			Ordinal:    n.Ordinal,
		})
	}
	return out
}
