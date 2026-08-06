package ingest

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Stage 5 - semantic segmentation.
//
// A clause is rarely one normative statement. "An adviser shall apply within 30
// days, provided that an existing applicant need not reapply" carries a duty, a
// deadline and an exception, and treating it as one blob means the exception
// silently rides along with the duty. Splitting on discourse markers separates
// them.
//
// Every unit keeps CHARACTER OFFSETS into its parent clause text, so the
// citation gate still works at unit level: a unit is not a paraphrase, it is a
// slice, and the offsets prove it.

// UnitRole is what a semantic unit does inside its clause.
type UnitRole string

const (
	RoleNorm       UnitRole = "norm"
	RoleCondition  UnitRole = "condition"
	RoleException  UnitRole = "exception"
	RoleDeadline   UnitRole = "deadline"
	RolePenalty    UnitRole = "penalty"
	RoleDefinition UnitRole = "definition"
	RoleCrossRef   UnitRole = "cross_ref"
	RoleScope      UnitRole = "scope"
)

// Valid reports whether r is a known unit role.
func (r UnitRole) Valid() bool {
	switch r {
	case RoleNorm, RoleCondition, RoleException, RoleDeadline, RolePenalty,
		RoleDefinition, RoleCrossRef, RoleScope:
		return true
	default:
		return false
	}
}

// SemanticUnit is one atomic normative unit of a clause.
type SemanticUnit struct {
	ID          string   `json:"id"`
	ClauseID    string   `json:"clause_id"`
	Ordinal     int      `json:"ordinal"`
	Role        UnitRole `json:"role"`
	Text        string   `json:"text"`
	StartOffset int      `json:"start_offset"`
	EndOffset   int      `json:"end_offset"`
}

// discourseMarkers are the connectives that introduce a new normative unit.
// Each carries the role the FOLLOWING text plays.
var discourseMarkers = []struct {
	phrase string
	role   UnitRole
}{
	{"provided further that", RoleException},
	{"provided also that", RoleException},
	{"provided that", RoleException},
	{"save as otherwise", RoleException},
	{"save as", RoleException},
	{"notwithstanding", RoleScope},
	{"subject to", RoleCondition},
	{"in case of", RoleCondition},
	{"unless", RoleCondition},
	{"except", RoleException},
	{"where", RoleCondition},
}

var (
	reDeadlineCue = regexp.MustCompile(`(?i)\bwithin\s+\d+\s+(day|days|month|months|year|years)\b|\bon or before\b|\bno later than\b`)
	rePenaltyCue  = regexp.MustCompile(`(?i)\b(penalty|penalties|liable to|enforcement action|shall be punishable)\b`)
	reDefCue      = regexp.MustCompile(`(?i)\b(means|shall mean|is defined as|for the purposes of this)\b`)
	reCrossRefCue = regexp.MustCompile(`(?i)\b(clause|regulation|reg\.|annexure|schedule|the said circular)\b`)
	reScopeCue    = regexp.MustCompile(`(?i)\b(appl(?:y|ies|icable) to|shall apply)\b`)
)

// SegmentClause splits a clause into atomic normative units.
//
// The split is LEFT-TO-RIGHT, once per marker occurrence. Nested provisos are
// therefore under-split rather than speculatively decomposed: under-splitting is
// recoverable in review (a reviewer sees a longer unit), whereas a fabricated
// split silently invents a boundary the regulation does not have.
func SegmentClause(clauseID, text string) []SemanticUnit {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// Sentence boundaries first, then marker boundaries inside each sentence.
	var cuts []int
	for _, span := range sentenceSpans(text) {
		cuts = append(cuts, span.start)
		cuts = append(cuts, markerCuts(text, span.start, span.end)...)
	}
	cuts = append(cuts, len(text))
	sort.Ints(cuts)

	var (
		units   []SemanticUnit
		ordinal int
	)
	for i := 0; i < len(cuts)-1; i++ {
		start, end := cuts[i], cuts[i+1]
		if start >= end {
			continue
		}
		raw := text[start:end]
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		// Re-anchor the offsets onto the trimmed text so the recorded span is
		// exactly the unit's characters in the parent clause - the property the
		// citation gate needs.
		lead := strings.Index(raw, trimmed)
		s := start + lead
		e := s + len(trimmed)

		ordinal++
		units = append(units, SemanticUnit{
			ID:          fmt.Sprintf("%s/u%d", clauseID, ordinal),
			ClauseID:    clauseID,
			Ordinal:     ordinal,
			Role:        classifyRole(trimmed),
			Text:        trimmed,
			StartOffset: s,
			EndOffset:   e,
		})
	}
	return units
}

type span struct{ start, end int }

// sentenceSpans returns sentence boundaries. This upgrades
// llm.splitSentences, which splits on '.' alone: an abbreviation ("Rs." , "No.")
// or a decimal ("3.1", "0.5") would otherwise cut a sentence in half and hand
// the citation gate a fragment.
func sentenceSpans(text string) []span {
	var (
		out   []span
		start int
		runes = []rune(text)
	)
	// Offsets must be BYTE offsets (they index into the Go string), so track
	// both positions as we walk.
	byteAt := make([]int, len(runes)+1)
	b := 0
	for i, r := range runes {
		byteAt[i] = b
		b += len(string(r))
	}
	byteAt[len(runes)] = b

	for i := 0; i < len(runes); i++ {
		if runes[i] != '.' && runes[i] != ';' {
			continue
		}
		if runes[i] == '.' {
			// A '.' between digits is a decimal or a clause number, not a stop.
			if i > 0 && i+1 < len(runes) && isDigit(runes[i-1]) && isDigit(runes[i+1]) {
				continue
			}
			if isAbbreviationBefore(runes, i) {
				continue
			}
		}
		if i+1 < len(runes) && !isSpaceRune(runes[i+1]) {
			continue
		}
		out = append(out, span{start: byteAt[start], end: byteAt[i+1]})
		start = i + 1
	}
	if start < len(runes) {
		out = append(out, span{start: byteAt[start], end: byteAt[len(runes)]})
	}
	return out
}

// abbreviations that end in '.' without ending a sentence.
var abbreviations = []string{"rs", "no", "nos", "cl", "reg", "regs", "para", "vs", "viz", "etc", "i.e", "e.g", "ltd", "pvt", "co"}

func isAbbreviationBefore(runes []rune, dot int) bool {
	j := dot - 1
	for j >= 0 && !isSpaceRune(runes[j]) {
		j--
	}
	word := strings.ToLower(string(runes[j+1 : dot]))
	for _, a := range abbreviations {
		if word == a {
			return true
		}
	}
	return false
}

func isDigit(r rune) bool     { return r >= '0' && r <= '9' }
func isSpaceRune(r rune) bool { return r == ' ' || r == '\n' || r == '\t' || r == '\r' }

// markerCuts finds discourse-marker boundaries within [start,end).
//
// Each marker occurrence produces AT MOST ONE cut, and overlapping markers do
// not compound: once a position is claimed, a marker starting inside an
// already-claimed marker's phrase is ignored. That is what keeps "provided
// further that" from also cutting at the "provided that" inside it.
func markerCuts(text string, start, end int) []int {
	segment := strings.ToLower(text[start:end])
	claimed := make([]bool, len(segment))
	var cuts []int

	for _, m := range discourseMarkers {
		from := 0
		for {
			i := strings.Index(segment[from:], m.phrase)
			if i < 0 {
				break
			}
			pos := from + i
			from = pos + len(m.phrase)

			if claimed[pos] {
				continue
			}
			// A marker must start a word, not sit inside one ("wherever" is not
			// "where").
			if pos > 0 && !isSpaceRune(rune(segment[pos-1])) && segment[pos-1] != '(' && segment[pos-1] != ',' {
				continue
			}
			if e := pos + len(m.phrase); e < len(segment) && !isSpaceRune(rune(segment[e])) {
				continue
			}
			for k := pos; k < pos+len(m.phrase) && k < len(claimed); k++ {
				claimed[k] = true
			}
			if pos > 0 {
				cuts = append(cuts, start+pos)
			}
		}
	}
	sort.Ints(cuts)
	return cuts
}

// classifyRole tags a unit by the cues it contains.
//
// Order encodes specificity: an exception marker is a stronger signal than a
// deadline cue that happens to appear inside it.
func classifyRole(text string) UnitRole {
	lower := strings.ToLower(text)
	for _, m := range discourseMarkers {
		if strings.HasPrefix(lower, m.phrase) {
			return m.role
		}
	}
	switch {
	case rePenaltyCue.MatchString(text):
		return RolePenalty
	case reDeadlineCue.MatchString(text):
		return RoleDeadline
	case reDefCue.MatchString(text):
		return RoleDefinition
	case reScopeCue.MatchString(text):
		return RoleScope
	case reCrossRefCue.MatchString(text) && !strings.Contains(lower, "shall") && !strings.Contains(lower, "must"):
		return RoleCrossRef
	default:
		return RoleNorm
	}
}
