package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// OfflineExtractor is a deterministic, dependency-free extractor. It scans a
// clause for deontic modals ("must", "shall", "must not", "may") and emits one
// schema-valid obligation per governing sentence, with a verbatim citation
// sliced directly from the clause text. It is the default extractor: it needs
// no API key, is fully reproducible, and exercises the exact same validation
// path a real LLM's output would go through. It emits DATA only.
type OfflineExtractor struct{}

// NewOfflineExtractor returns the deterministic offline extractor.
func NewOfflineExtractor() *OfflineExtractor { return &OfflineExtractor{} }

// Name identifies this extractor for provenance.
func (e *OfflineExtractor) Name() string { return "offline-deterministic-v1" }

// candidate mirrors one element of the schema's "obligations" array.
type candidate struct {
	Bearer          string     `json:"bearer"`
	DeonticType     string     `json:"deontic_type"`
	Condition       string     `json:"condition,omitempty"`
	Threshold       *threshold `json:"threshold,omitempty"`
	Deadline        string     `json:"deadline,omitempty"`
	Penalty         string     `json:"penalty,omitempty"`
	SourceClauseRef string     `json:"source_clause_ref"`
	SourceSentence  string     `json:"source_sentence"`
	Confidence      float64    `json:"confidence"`
}

type threshold struct {
	Metric   string  `json:"metric,omitempty"`
	Operator string  `json:"operator,omitempty"`
	Value    float64 `json:"value,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	// Kind distinguishes a "trigger" threshold (crossing it triggers the duty,
	// e.g. >=300 clients -> must register) from a "requirement" threshold (the
	// firm must MEET it, e.g. retain records for >=5 years). See the policy
	// compiler, which enforces them differently.
	Kind string `json:"kind,omitempty"`
}

type extraction struct {
	Obligations []candidate `json:"obligations"`
}

var (
	reWithinDays = regexp.MustCompile(`(?i)within\s+(\d+)\s+days`)
	reClients    = regexp.MustCompile(`(?i)(\d[\d,]*)\s+or\s+more\s+clients|(\d[\d,]*)\s+clients`)
	reYears      = regexp.MustCompile(`(?i)(\d+)\s+years`)
	reINR        = regexp.MustCompile(`(?i)INR\s+([\d,]+)`)

	// Word-boundary modal matchers. Negatives are checked first. The \b anchors
	// prevent false positives like "must notify" matching "must not".
	reMustNot = regexp.MustCompile(`(?i)\b(?:must|shall|may)\s+not\b`)
	reMust    = regexp.MustCompile(`(?i)\b(?:must|shall)\b`)
	reMay     = regexp.MustCompile(`(?i)\bmay\b`)
)

// Extract implements Extractor. It always returns schema-valid JSON (possibly
// with zero obligations, e.g. for a definitions clause).
func (e *OfflineExtractor) Extract(_ context.Context, req ExtractionRequest) ([]byte, error) {
	out := extraction{Obligations: []candidate{}}

	for _, sent := range splitSentences(req.Text) {
		deontic, ok := detectDeontic(sent)
		if !ok {
			continue
		}
		c := candidate{
			Bearer:          "investment adviser",
			DeonticType:     deontic,
			SourceClauseRef: req.ClauseRef,
			SourceSentence:  sent, // verbatim substring of req.Text
			Confidence:      0.70,
		}
		if th := detectThreshold(sent); th != nil {
			c.Threshold = th
			c.Confidence += 0.15
		}
		if d := detectDeadline(sent); d != "" {
			c.Deadline = d
			c.Confidence += 0.10
		}
		if c.Confidence > 0.95 {
			c.Confidence = 0.95
		}
		out.Obligations = append(out.Obligations, c)
	}

	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal offline extraction: %w", err)
	}
	return b, nil
}

// splitSentences returns the clause's sentences as VERBATIM substrings (so each
// can serve as an exact citation). It splits on a period followed by whitespace
// or end-of-text; the returned strings retain their original characters.
func splitSentences(text string) []string {
	var out []string
	start := 0
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '.' {
			continue
		}
		// A sentence boundary: '.' at end, or '.' followed by whitespace.
		if i == len(runes)-1 || isSpace(runes[i+1]) {
			seg := strings.TrimSpace(string(runes[start : i+1]))
			if seg != "" {
				out = append(out, seg)
			}
			start = i + 1
		}
	}
	if start < len(runes) {
		if seg := strings.TrimSpace(string(runes[start:])); seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

func isSpace(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '\r' }

// detectDeontic classifies the modal force of a sentence. Negative modals are
// checked first so "must not" / "shall not" become MUST_NOT rather than MUST.
func detectDeontic(sentence string) (string, bool) {
	switch {
	case reMustNot.MatchString(sentence):
		return "MUST_NOT", true
	case reMust.MatchString(sentence):
		return "MUST", true
	case reMay.MatchString(sentence):
		return "MAY", true
	default:
		return "", false
	}
}

// detectDeadline maps "within N days" to an ISO-8601 duration.
func detectDeadline(sentence string) string {
	if m := reWithinDays.FindStringSubmatch(sentence); m != nil {
		return "P" + m[1] + "D"
	}
	return ""
}

// detectThreshold extracts the first recognised numeric threshold.
func detectThreshold(sentence string) *threshold {
	// A client/fee threshold is a TRIGGER (crossing it triggers a registration
	// duty); a retention-period threshold is a REQUIREMENT (the firm must meet it).
	if m := reClients.FindStringSubmatch(sentence); m != nil {
		raw := m[1]
		if raw == "" {
			raw = m[2]
		}
		if v, ok := parseNum(raw); ok {
			return &threshold{Metric: "clients", Operator: ">=", Value: v, Unit: "count", Kind: "trigger"}
		}
	}
	if m := reINR.FindStringSubmatch(sentence); m != nil {
		if v, ok := parseNum(m[1]); ok {
			return &threshold{Metric: "annual_fees", Operator: ">", Value: v, Unit: "INR", Kind: "trigger"}
		}
	}
	if m := reYears.FindStringSubmatch(sentence); m != nil {
		if v, ok := parseNum(m[1]); ok {
			return &threshold{Metric: "retention_period", Operator: ">=", Value: v, Unit: "years", Kind: "requirement"}
		}
	}
	return nil
}

// parseNum parses an integer that may contain grouping commas.
func parseNum(s string) (float64, bool) {
	clean := strings.ReplaceAll(s, ",", "")
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
