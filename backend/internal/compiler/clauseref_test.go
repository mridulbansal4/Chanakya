package compiler

import "testing"

func TestClauseRefMatchesToleratesLabels(t *testing.T) {
	match := []struct{ got, want string }{
		{"3", "3"},
		{"3.1", "3.1"},
		{"Clause 3", "3"},     // the Gemini case
		{"Clause 3.1", "3.1"}, // the Gemini case
		{"clause 3.1", "3.1"},
		{"§3.1", "3.1"},
		{"cl. 3.1", "3.1"},
		{"Regulation 3", "3"},
		{"Para 5.2", "5.2"},
	}
	for _, c := range match {
		if !clauseRefMatches(c.got, c.want) {
			t.Errorf("clauseRefMatches(%q, %q) = false, want true", c.got, c.want)
		}
	}

	noMatch := []struct{ got, want string }{
		{"Clause 5.2", "3.1"}, // genuinely different clause — must still reject
		{"4", "3"},
		{"3.2", "3.1"},
	}
	for _, c := range noMatch {
		if clauseRefMatches(c.got, c.want) {
			t.Errorf("clauseRefMatches(%q, %q) = true, want false", c.got, c.want)
		}
	}
}
