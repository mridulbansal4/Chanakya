package ingest

import (
	"regexp"
	"strings"
)

// Structural levels. L0 is a chapter/part/schedule/annexure heading; deeper
// levels nest under it.
const (
	levelDivision = 0 // CHAPTER / PART / SCHEDULE / ANNEXURE
	levelOne      = 1 // "3." or "3 "
	levelTwo      = 2 // "3.1"
	levelThree    = 3 // "3.1.1" or "(1)"
	levelAlpha    = 4 // "(a)"
	levelRoman    = 5 // "(ii)"
	levelUnnumber = -1
)

// numbering patterns, applied deepest-first. The prompt lists them in
// document-reading order; applied literally in that order, `^\d+\.` would claim
// "3.1" before the two-part pattern ever ran. Deepest-first is the same
// precedence expressed as evaluation order, and is what makes "3.1.1" a level-3
// node rather than a level-1 node called "3".
var (
	reDivision  = regexp.MustCompile(`(?i)^(chapter|part|schedule|annexure|annex)\b`)
	reThreePart = regexp.MustCompile(`^(\d+\.\d+\.\d+)\.?$`)
	reTwoPart   = regexp.MustCompile(`^(\d+\.\d+)\.?$`)
	reOnePart   = regexp.MustCompile(`^(\d+)\.$`)
	reBareNum   = regexp.MustCompile(`^(\d+)$`)
	reParenNum  = regexp.MustCompile(`^\((\d+)\)$`)
	reParenAlfa = regexp.MustCompile(`^\(([a-z]{1,3})\)$`)
	reRomanOnly = regexp.MustCompile(`^[ivxlcdm]+$`)
)

// numbering is what the lexer recognised at the start of a line.
type numbering struct {
	Token      string  // the raw leading token, e.g. "3.1" or "(a)"
	Ref        string  // the canonical reference fragment, e.g. "3.1" or "a"
	Level      int     // levelDivision..levelRoman, or levelUnnumber
	Rest       string  // the line text with the numbering token removed
	Confidence float64 // < 1 when the classification was ambiguous
}

// lexNumbering classifies a line's leading token.
//
// prevSibling is the token of the most recent list item seen at the alpha/roman
// depth, used only to resolve the genuine ambiguity of "(i)": it is both the
// ninth letter and the first roman numeral.
func lexNumbering(line string, prevSibling string) numbering {
	line = strings.TrimSpace(line)
	if line == "" {
		return numbering{Level: levelUnnumber, Confidence: 1}
	}

	if reDivision.MatchString(line) {
		return numbering{Token: firstToken(line), Ref: line, Level: levelDivision, Rest: line, Confidence: 1}
	}

	tok := firstToken(line)
	rest := strings.TrimSpace(strings.TrimPrefix(line, tok))

	switch {
	case reThreePart.MatchString(tok):
		return numbering{Token: tok, Ref: reThreePart.FindStringSubmatch(tok)[1], Level: levelThree, Rest: rest, Confidence: 1}
	case reTwoPart.MatchString(tok):
		return numbering{Token: tok, Ref: reTwoPart.FindStringSubmatch(tok)[1], Level: levelTwo, Rest: rest, Confidence: 1}
	case reOnePart.MatchString(tok):
		return numbering{Token: tok, Ref: reOnePart.FindStringSubmatch(tok)[1], Level: levelOne, Rest: rest, Confidence: 1}
	case reBareNum.MatchString(tok) && rest != "":
		// "3 Registration" - a number followed by text on the same line.
		return numbering{Token: tok, Ref: tok, Level: levelOne, Rest: rest, Confidence: 1}
	case reParenNum.MatchString(tok):
		return numbering{Token: tok, Ref: "(" + reParenNum.FindStringSubmatch(tok)[1] + ")", Level: levelThree, Rest: rest, Confidence: 1}
	case reParenAlfa.MatchString(tok):
		letters := reParenAlfa.FindStringSubmatch(tok)[1]
		lvl, conf := alphaOrRoman(letters, prevSibling)
		return numbering{Token: tok, Ref: "(" + letters + ")", Level: lvl, Rest: rest, Confidence: conf}
	}

	return numbering{Level: levelUnnumber, Rest: line, Confidence: 1}
}

// alphaOrRoman resolves whether a parenthesised letter run is an alphabetic or
// a roman list marker, and how confident that verdict is.
//
// Only "i", "v" and "x" are genuinely ambiguous - every other single letter is
// unambiguously alphabetic, and every multi-letter roman run ("ii", "iv") is
// unambiguously roman because no alphabetic list uses multi-letter markers.
//
// Sequence continuity decides the ambiguous cases: after "(h)" a "(i)" is the
// next letter; after "(iii)" it would be roman. With NO preceding sibling there
// is nothing to continue, so the answer is alpha at reduced confidence, and
// Stage 2 records that confidence rather than pretending to be sure. Guessing
// from the surrounding prose would be inventing evidence.
func alphaOrRoman(letters, prevSibling string) (level int, confidence float64) {
	if len(letters) > 1 && reRomanOnly.MatchString(letters) {
		return levelRoman, 1
	}
	if len(letters) > 1 {
		return levelAlpha, 0.6 // e.g. "(aa)" - unusual but alphabetic
	}

	switch letters {
	case "i", "v", "x":
		prev := strings.Trim(prevSibling, "()")
		if prev == "" {
			return levelAlpha, 0.5 // first item in its list: nothing to continue
		}
		if len(prev) == 1 && prev == predecessorLetter(letters) {
			return levelAlpha, 1 // "(h)" then "(i)": an alphabetic sequence
		}
		if reRomanOnly.MatchString(prev) {
			return levelRoman, 1 // continuing a roman run
		}
		return levelAlpha, 0.6
	default:
		return levelAlpha, 1
	}
}

// predecessorLetter returns the letter preceding s in the alphabet.
func predecessorLetter(s string) string {
	if len(s) != 1 || s[0] <= 'a' {
		return ""
	}
	return string(rune(s[0] - 1))
}

// firstToken returns the leading whitespace-delimited token of a line.
func firstToken(s string) string {
	if i := strings.IndexAny(s, " \t"); i > 0 {
		return s[:i]
	}
	return s
}
