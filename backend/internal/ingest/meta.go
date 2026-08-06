package ingest

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// MetaSchemaJSON is the strict schema an LLM metadata completion must satisfy.
// Same discipline as the obligation schema: model output is DATA, validated
// before it is trusted.
//
//go:embed meta_schema.json
var MetaSchemaJSON []byte

// DocKind classifies a source document. It is established BEFORE obligation
// extraction because it changes what extraction means: an `amendment` drives the
// version-graph path, and an `faq` produces interpretive material that must
// never become an obligation on its own.
type DocKind string

const (
	KindMasterCircular    DocKind = "master_circular"
	KindCircular          DocKind = "circular"
	KindAmendment         DocKind = "amendment"
	KindNotification      DocKind = "notification"
	KindFAQ               DocKind = "faq"
	KindGuidanceNote      DocKind = "guidance_note"
	KindConsultationPaper DocKind = "consultation_paper"
)

// Valid reports whether k is a known document kind.
func (k DocKind) Valid() bool {
	switch k {
	case KindMasterCircular, KindCircular, KindAmendment, KindNotification,
		KindFAQ, KindGuidanceNote, KindConsultationPaper:
		return true
	default:
		return false
	}
}

// CircularMeta is Stage 3's output.
type CircularMeta struct {
	CircularNo    string   `json:"circular_no"`
	Title         string   `json:"title"`
	IssuedOn      string   `json:"issued_on"`      // RFC3339 UTC
	EffectiveFrom string   `json:"effective_from"` // RFC3339 UTC
	Regulator     string   `json:"regulator"`
	Department    string   `json:"department"`
	Supersedes    []string `json:"supersedes"`
	Amends        []string `json:"amends"`
	References    []string `json:"references"`
	AppliesTo     []string `json:"applies_to"`
	DocKind       DocKind  `json:"doc_kind"`

	// FromRegex lists the field names the deterministic pass established. The
	// LLM pass may only fill fields absent from this set - see MergeMeta.
	FromRegex []string `json:"from_regex"`
}

// MetaCompleter fills metadata fields the deterministic pass could not find.
// It returns raw JSON, validated against MetaSchemaJSON before use.
type MetaCompleter interface {
	Name() string
	CompleteMeta(ctx context.Context, docText string, missing []string) ([]byte, error)
}

// --- deterministic pass ------------------------------------------------------

var (
	// The canonical modern SEBI circular number, plus a looser fallback for the
	// older formats that predate it.
	// Division segments are mixed case in practice ("MIRSD-PoD", "IMD-PoD-1"),
	// so the character class cannot be upper-case only.
	reCircularNoStrict = regexp.MustCompile(`SEBI/HO/[A-Za-z0-9-]+/[A-Za-z0-9-]+/P/CIR/\d{4}/\d+`)
	reCircularNoLoose  = regexp.MustCompile(`SEBI/[A-Za-z0-9/_-]{2,60}/\d{4}/\d+`)

	reDateLong  = regexp.MustCompile(`(?i)\b(\d{1,2})\s+(January|February|March|April|May|June|July|August|September|October|November|December),?\s+(\d{4})\b`)
	reDateComma = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2}),\s*(\d{4})\b`)
	reDateSlash = regexp.MustCompile(`\b(\d{1,2})[./-](\d{1,2})[./-](\d{4})\b`)

	reEffective = regexp.MustCompile(`(?i)(?:shall come into force|comes into force|effective|applicable)\s+(?:on and )?from\s+([^.;\n]{4,40})`)

	// Relation cues, each capturing the trailing text that holds the circular
	// number(s) being related to.
	reSupersession = regexp.MustCompile(`(?i)in\s+supersession\s+of\s+([^.;\n]{4,200})`)
	reReadWith     = regexp.MustCompile(`(?i)read\s+with\s+([^.;\n]{4,200})`)
	reModified     = regexp.MustCompile(`(?i)([^.;\n]{4,200}?)\s+stands?\s+(?:modified|amended)`)
	reAmends       = regexp.MustCompile(`(?i)(?:amendment|amendments)\s+to\s+([^.;\n]{4,200})`)

	reDepartment = regexp.MustCompile(`(?i)\b(MIRSD|IMD|CFD|MRD|DDHS|ISD|AFD|IVD|CDMRD|SEBI)\b`)
)

// appliesToTerms maps a phrase in the document to a regulated population. Only
// exact phrases are matched: inferring "this probably applies to X" from context
// is a judgement, and Stage 3 is the deterministic pass.
var appliesToTerms = map[string]string{
	"investment adviser":  "investment_adviser",
	"investment advisers": "investment_adviser",
	"research analyst":    "research_analyst",
	"research analysts":   "research_analyst",
	"mutual fund":         "mutual_fund",
	"mutual funds":        "mutual_fund",
	"stock broker":        "stock_broker",
	"stock brokers":       "stock_broker",
	"portfolio manager":   "portfolio_manager",
	"portfolio managers":  "portfolio_manager",
	"merchant banker":     "merchant_banker",
	"merchant bankers":    "merchant_banker",
	"depository":          "depository",
	"listed entity":       "listed_entity",
	"listed entities":     "listed_entity",
}

// ExtractMetaRegex runs the deterministic first pass over the document text.
//
// Everything it finds is recorded in FromRegex, which is the precedence record:
// a deterministic hit is evidence read straight off the page, and the LLM pass
// may not overwrite it. That rule is fixed - not "whichever is more confident".
func ExtractMetaRegex(docText, filename string) CircularMeta {
	m := CircularMeta{Regulator: "SEBI"}
	found := map[string]bool{"regulator": true}

	if s := reCircularNoStrict.FindString(docText); s != "" {
		m.CircularNo, found["circular_no"] = s, true
	} else if s := reCircularNoLoose.FindString(docText); s != "" {
		m.CircularNo, found["circular_no"] = s, true
	}

	if t := detectTitle(docText, filename); t != "" {
		m.Title, found["title"] = t, true
	}

	if d := firstDate(docText); d != "" {
		m.IssuedOn, found["issued_on"] = d, true
	}
	if mm := reEffective.FindStringSubmatch(docText); mm != nil {
		if d := firstDate(mm[1]); d != "" {
			m.EffectiveFrom, found["effective_from"] = d, true
		}
	}

	// "SEBI" matches the department pattern but names the regulator, not a
	// department, so skip past it to the first real one.
	if dep := firstDepartment(docText); dep != "" {
		m.Department, found["department"] = dep, true
	} else if m.CircularNo != "" {
		// The department is encoded in the circular number itself:
		// SEBI/HO/<DEPT>/<DIVISION>/P/CIR/...
		if parts := strings.Split(m.CircularNo, "/"); len(parts) > 2 {
			m.Department, found["department"] = parts[2], true
		}
	}

	m.Supersedes = circularRefsIn(reSupersession, docText)
	m.Amends = append(circularRefsIn(reModified, docText), circularRefsIn(reAmends, docText)...)
	m.References = circularRefsIn(reReadWith, docText)
	m.Amends = dedupeStrings(m.Amends)
	if len(m.Supersedes) > 0 {
		found["supersedes"] = true
	}
	if len(m.Amends) > 0 {
		found["amends"] = true
	}
	if len(m.References) > 0 {
		found["references"] = true
	}

	m.AppliesTo = detectAppliesTo(docText)
	if len(m.AppliesTo) > 0 {
		found["applies_to"] = true
	}

	m.DocKind = detectDocKind(docText, m)
	found["doc_kind"] = true

	for k := range found {
		m.FromRegex = append(m.FromRegex, k)
	}
	sortStrings(m.FromRegex)
	return m
}

// firstDepartment returns the first departmental code in the text, skipping
// "SEBI" itself.
func firstDepartment(docText string) string {
	for _, m := range reDepartment.FindAllString(docText, -1) {
		if up := strings.ToUpper(m); up != "SEBI" {
			return up
		}
	}
	return ""
}

// detectTitle takes the longest of the first few substantial lines, which is
// where a SEBI circular puts its subject line. Falling back to the filename
// keeps Title non-empty without inventing one.
func detectTitle(docText, filename string) string {
	lines := strings.Split(docText, "\n")
	best := ""
	for i, l := range lines {
		if i > 25 {
			break
		}
		l = strings.TrimSpace(l)
		if len(l) < 12 || len(l) > 160 {
			continue
		}
		lower := strings.ToLower(l)
		if strings.HasPrefix(lower, "sebi/") || strings.Contains(lower, "securities and exchange board") {
			continue
		}
		if len(l) > len(best) {
			best = l
		}
	}
	if best != "" {
		return best
	}
	return strings.TrimSuffix(filename, ".pdf")
}

// detectDocKind classifies from explicit vocabulary in the document.
//
// Order matters: "Master Circular" wins over the bare word "circular", and an
// amendment cue wins over "circular" because an amending circular IS a circular.
func detectDocKind(docText string, m CircularMeta) DocKind {
	head := strings.ToLower(firstN(docText, 4000))
	switch {
	case strings.Contains(head, "consultation paper"):
		return KindConsultationPaper
	case strings.Contains(head, "guidance note"):
		return KindGuidanceNote
	case strings.Contains(head, "frequently asked questions"), strings.Contains(head, " faq"),
		strings.HasPrefix(head, "faq"):
		return KindFAQ
	case strings.Contains(head, "master circular"):
		return KindMasterCircular
	case len(m.Supersedes) > 0 || len(m.Amends) > 0,
		strings.Contains(head, "amendment to"), strings.Contains(head, "stands modified"):
		return KindAmendment
	case strings.Contains(head, "notification"), strings.Contains(head, "gazette"):
		return KindNotification
	default:
		return KindCircular
	}
}

// detectAppliesTo returns the regulated populations named in the document.
func detectAppliesTo(docText string) []string {
	lower := strings.ToLower(docText)
	seen := map[string]bool{}
	var out []string
	for phrase, id := range appliesToTerms {
		if !seen[id] && strings.Contains(lower, phrase) {
			seen[id] = true
			out = append(out, id)
		}
	}
	sortStrings(out)
	return out
}

// relationCues are the phrases that START a relation clause. A capture is cut at
// the next one so a sentence carrying two relations - "in supersession of X and
// shall be read with Y" - does not attribute Y to the supersession.
var relationCues = []string{"in supersession of", "read with", "stands modified",
	"stands amended", "amendment to", "amendments to"}

// circularRefsIn pulls circular numbers out of the text captured by a relation
// cue. A cue with no recognisable circular number yields nothing rather than the
// surrounding prose - a relation to an unidentifiable document is not a relation.
func circularRefsIn(re *regexp.Regexp, docText string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(docText, -1) {
		if len(m) < 2 {
			continue
		}
		captured := cutAtNextCue(m[1])
		refs := reCircularNoStrict.FindAllString(captured, -1)
		if len(refs) == 0 {
			refs = reCircularNoLoose.FindAllString(captured, -1)
		}
		out = append(out, refs...)
	}
	return dedupeStrings(out)
}

// cutAtNextCue truncates a relation capture at the next relation cue.
func cutAtNextCue(s string) string {
	lower := strings.ToLower(s)
	cut := len(s)
	for _, cue := range relationCues {
		if i := strings.Index(lower, cue); i >= 0 && i < cut {
			cut = i
		}
	}
	return s[:cut]
}

var monthNumbers = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June, "july": time.July,
	"august": time.August, "september": time.September, "october": time.October,
	"november": time.November, "december": time.December,
}

// firstDate returns the first parseable date in s as an RFC3339 UTC timestamp.
func firstDate(s string) string {
	if m := reDateLong.FindStringSubmatch(s); m != nil {
		return buildDate(m[1], m[2], m[3])
	}
	if m := reDateComma.FindStringSubmatch(s); m != nil {
		return buildDate(m[2], m[1], m[3])
	}
	if m := reDateSlash.FindStringSubmatch(s); m != nil {
		// Indian convention is DD/MM/YYYY. Assuming US MM/DD would silently
		// shift an effective date by months, so the convention is fixed here.
		var d, mo, y int
		if _, err := fmt.Sscanf(m[1]+" "+m[2]+" "+m[3], "%d %d %d", &d, &mo, &y); err != nil {
			return ""
		}
		if mo < 1 || mo > 12 || d < 1 || d > 31 {
			return ""
		}
		return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	}
	return ""
}

func buildDate(day, month, year string) string {
	mo, ok := monthNumbers[strings.ToLower(month)]
	if !ok {
		return ""
	}
	var d, y int
	if _, err := fmt.Sscanf(day+" "+year, "%d %d", &d, &y); err != nil {
		return ""
	}
	if d < 1 || d > 31 {
		return ""
	}
	return time.Date(y, mo, d, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
}

// --- LLM pass ----------------------------------------------------------------

// MissingMetaFields lists the fields the deterministic pass did not establish.
func MissingMetaFields(m CircularMeta) []string {
	have := map[string]bool{}
	for _, f := range m.FromRegex {
		have[f] = true
	}
	var missing []string
	for _, f := range []string{"circular_no", "title", "issued_on", "effective_from",
		"department", "supersedes", "amends", "references", "applies_to"} {
		if !have[f] {
			missing = append(missing, f)
		}
	}
	return missing
}

// ExtractMeta runs Stage 3: the deterministic pass, then - only for the fields
// it could not establish - the LLM pass.
//
// PRECEDENCE IS FIXED: the regex pass wins, always. A model cannot overwrite a
// circular number or a date read directly off the page, however confident it
// sounds. It only fills gaps, and its output is schema-validated first.
func ExtractMeta(ctx context.Context, docText, filename string, completer MetaCompleter) (CircularMeta, error) {
	m := ExtractMetaRegex(docText, filename)
	missing := MissingMetaFields(m)
	if completer == nil || len(missing) == 0 {
		return m, nil
	}

	raw, err := completer.CompleteMeta(ctx, docText, missing)
	if err != nil {
		// A failed completion is not a failed ingestion: the deterministic
		// metadata is still usable and the run continues with gaps a reviewer
		// can fill. Returning the regex result is strictly better than failing.
		return m, nil
	}
	if err := ValidateMetaJSON(raw); err != nil {
		return m, nil
	}

	var filled CircularMeta
	if err := json.Unmarshal(raw, &filled); err != nil {
		return m, nil
	}
	return MergeMeta(m, filled), nil
}

// MergeMeta applies the fixed precedence rule: a field established by the
// deterministic pass is never overwritten.
func MergeMeta(base, filled CircularMeta) CircularMeta {
	have := map[string]bool{}
	for _, f := range base.FromRegex {
		have[f] = true
	}
	out := base
	if !have["circular_no"] && filled.CircularNo != "" {
		out.CircularNo = filled.CircularNo
	}
	if !have["title"] && filled.Title != "" {
		out.Title = filled.Title
	}
	if !have["issued_on"] && filled.IssuedOn != "" {
		out.IssuedOn = filled.IssuedOn
	}
	if !have["effective_from"] && filled.EffectiveFrom != "" {
		out.EffectiveFrom = filled.EffectiveFrom
	}
	if !have["department"] && filled.Department != "" {
		out.Department = filled.Department
	}
	if !have["supersedes"] && len(filled.Supersedes) > 0 {
		out.Supersedes = filled.Supersedes
	}
	if !have["amends"] && len(filled.Amends) > 0 {
		out.Amends = filled.Amends
	}
	if !have["references"] && len(filled.References) > 0 {
		out.References = filled.References
	}
	if !have["applies_to"] && len(filled.AppliesTo) > 0 {
		out.AppliesTo = filled.AppliesTo
	}
	// DocKind is never taken from the model: it decides how the rest of the
	// pipeline treats the document, and the deterministic pass always sets it.
	return out
}

var metaSchema *jsonschema.Schema

// ValidateMetaJSON validates raw LLM metadata against the strict schema.
func ValidateMetaJSON(raw []byte) error {
	if metaSchema == nil {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(MetaSchemaJSON))
		if err != nil {
			return fmt.Errorf("parse meta schema: %w", err)
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource("chanakya:circular-meta", doc); err != nil {
			return fmt.Errorf("add meta schema: %w", err)
		}
		sch, err := c.Compile("chanakya:circular-meta")
		if err != nil {
			return fmt.Errorf("compile meta schema: %w", err)
		}
		metaSchema = sch
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("meta output is not valid JSON: %w", err)
	}
	if err := metaSchema.Validate(inst); err != nil {
		return fmt.Errorf("meta schema validation failed: %w", err)
	}
	return nil
}

// --- small helpers -----------------------------------------------------------

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
