package ingest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Stage 4 - normalization.
//
// THE RULE THAT GOVERNS THIS WHOLE FILE: the verbatim Clause.Text is NEVER
// touched. The citation gate proves an obligation is real by checking that its
// cited sentence is a substring of the clause text; rewriting that text would
// destroy the thing the proof is made of. Normalised output goes to a PARALLEL
// field, and downstream stages choose which one they need.

// Amount is a parsed Indian monetary or count quantity.
type Amount struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
	// Raw is the source text, kept so the normalisation is auditable back to
	// what the page actually said.
	Raw string `json:"raw"`
}

// Normalized is Stage 4's output for one clause.
type Normalized struct {
	ClauseRef string   `json:"clause_ref"`
	Text      string   `json:"text"`    // canonicalised, NOT the citation source
	Amounts   []Amount `json:"amounts"` // quantities found in the clause
}

var (
	// Indian numeric forms: "Rs. 3,00,00,000", "₹3,00,00,000", "INR 3 crore",
	// "₹3 crore", "Rupees three crore" (digits only - spelled-out numbers are
	// left alone rather than guessed).
	reAmountScaled = regexp.MustCompile(`(?i)(?:₹|rs\.?|inr|rupees)\s*([\d,]+(?:\.\d+)?)\s*(crore|crores|lakh|lakhs|thousand)?`)
	reScaledPlain  = regexp.MustCompile(`(?i)\b([\d,]+(?:\.\d+)?)\s+(crore|crores|lakh|lakhs)\b`)

	// Whitespace collapse must match compiler.containsNormalized EXACTLY. If
	// this were more permissive, text could normalise to something the citation
	// gate would not accept, and valid obligations would be rejected; if it were
	// stricter, the reverse. strings.Fields + join is the same operation.
	reMultiSpace = regexp.MustCompile(`\s+`)
)

var scaleFactors = map[string]float64{
	"crore": 1e7, "crores": 1e7,
	"lakh": 1e5, "lakhs": 1e5,
	"thousand": 1e3,
}

// NormalizeText canonicalises whitespace, quotes and dashes.
//
// The whitespace rule is deliberately identical to compiler.normalizeWS: collapse
// every run of whitespace to a single space. Nothing wider is applied.
func NormalizeText(s string) string {
	s = normalizeText(s) // NFKC + quote/dash folding, shared with Stage 1
	return strings.Join(strings.Fields(s), " ")
}

// NormalizeClause produces the parallel normalised view of one clause.
func NormalizeClause(clauseRef, text string) Normalized {
	return Normalized{
		ClauseRef: clauseRef,
		Text:      NormalizeText(text),
		Amounts:   ParseIndianAmounts(text),
	}
}

// ParseIndianAmounts extracts monetary quantities written in Indian forms.
//
// The Indian digit grouping (3,00,00,000) is NOT the international one
// (30,000,000): read with a Western thousands-grouping assumption, three crore
// becomes three hundred thousand, and a registration threshold would be off by
// two orders of magnitude. Commas are therefore stripped entirely rather than
// interpreted as group separators.
func ParseIndianAmounts(text string) []Amount {
	var out []Amount
	seen := map[string]bool{}

	add := func(raw, digits, scale string, unit string) {
		v, err := strconv.ParseFloat(strings.ReplaceAll(digits, ",", ""), 64)
		if err != nil {
			return
		}
		if f, ok := scaleFactors[strings.ToLower(scale)]; ok {
			v *= f
		}
		key := fmt.Sprintf("%s|%v|%s", raw, v, unit)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Amount{Value: v, Unit: unit, Raw: strings.TrimSpace(raw)})
	}

	for _, m := range reAmountScaled.FindAllStringSubmatch(text, -1) {
		add(m[0], m[1], m[2], "INR")
	}
	for _, m := range reScaledPlain.FindAllStringSubmatch(text, -1) {
		// Only record a bare "3 crore" if it was not already captured with its
		// currency marker, so "INR 3 crore" yields one amount and not two.
		if !reAmountScaled.MatchString(m[0]) {
			add(m[0], m[1], m[2], "count")
		}
	}
	return out
}
