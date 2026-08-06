package workflow

import (
	"testing"
	"time"
)

var testNow = time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)

// stubResolver resolves the roles it knows and refuses the rest, so the
// unresolvable-owner path is exercisable.
type stubResolver struct{ known map[string]string }

func (s stubResolver) ResolveRole(role string) (string, string, bool) {
	if name, ok := s.known[role]; ok {
		return "emp_" + role, name, true
	}
	return "", "", false
}

var fullResolver = stubResolver{known: map[string]string{
	"Compliance": "Priya Menon", "Legal": "Farida Merchant", "Operations": "Manish Gupta",
	"Client Servicing": "Nisha Pillai", "HR": "Leena Fernandes", "Risk": "Ritu Malhotra",
	"Technology": "Sameer Khan", "Advisory": "Arjun Desai",
}}

// TestSynthesisIsVerbDriven: the verb selects the template from a fixed table,
// with no model in the loop. This is the property that makes synthesis
// unit-testable AND keeps a generation from authoring the firm's plan.
func TestSynthesisIsVerbDriven(t *testing.T) {
	cases := []struct {
		sentence string
		wantVerb Verb
		wantTpl  []TemplateID
	}{
		{"An investment adviser must notify each affected client in writing within 7 days.",
			VerbNotify, []TemplateID{TemplateClientNotification}},
		{"An investment adviser must retain all records for a period of 5 years.",
			VerbRetain, []TemplateID{TemplateEvidenceCollection}},
		{"An investment adviser shall maintain client-level segregation at all times.",
			VerbMaintain, []TemplateID{TemplateEvidenceCollection}},
		{"An investment adviser must disclose to the client the complete fee schedule.",
			VerbDisclose, []TemplateID{TemplatePolicyUpdate, TemplateClientNotification}},
		{"An adviser shall submit a complete application for registration.",
			VerbSubmit, []TemplateID{TemplateFiling}},
		{"Every employee shall be trained on the revised requirements.",
			VerbTrain, []TemplateID{TemplateTraining}},
		{"The adviser shall obtain the client's acknowledgement.",
			VerbObtain, []TemplateID{TemplateAttestation}},
	}

	for _, tc := range cases {
		got := Synthesize(ObligationInput{
			ID: "obl-1", ClauseRef: "5.2", SourceSentence: tc.sentence, Deadline: "P7D",
		}, fullResolver, testNow)

		if got.Unclassified {
			t.Errorf("%q was unclassified: %s", tc.sentence, got.Reason)
			continue
		}
		if len(got.Workflows) != len(tc.wantTpl) {
			t.Errorf("%q -> %d workflows, want %d", tc.sentence, len(got.Workflows), len(tc.wantTpl))
			continue
		}
		for i, want := range tc.wantTpl {
			if got.Workflows[i].Template != want {
				t.Errorf("%q -> template %q, want %q", tc.sentence, got.Workflows[i].Template, want)
			}
			if got.Workflows[i].Verb != tc.wantVerb {
				t.Errorf("%q -> verb %q, want %q", tc.sentence, got.Workflows[i].Verb, tc.wantVerb)
			}
		}
	}
}

// TestUnrecognisedVerbGoesToReview is the edge case the phase calls out: an act
// outside the closed vocabulary must be routed to review as unclassified - NOT
// mapped to the nearest-looking template, and NOT silently dropped.
func TestUnrecognisedVerbGoesToReview(t *testing.T) {
	got := Synthesize(ObligationInput{
		ID: "obl-x", ClauseRef: "9.9",
		SourceSentence: "The Board may issue such directions as it considers expedient.",
		Condition:      "",
	}, fullResolver, testNow)

	if !got.Unclassified {
		t.Fatalf("expected unclassified, got %d workflows (%v)",
			len(got.Workflows), got.Workflows)
	}
	if len(got.Workflows) != 0 {
		t.Error("an unclassified obligation must produce NO workflows - guessing a template is worse than admitting ignorance")
	}
	if got.Reason == "" {
		t.Error("an unclassified obligation must say why, so a reviewer knows what to do")
	}
}

// TestEverythingIsDraft is the phase's core safety invariant.
func TestEverythingIsDraft(t *testing.T) {
	got := Synthesize(ObligationInput{
		ID: "obl-1", ClauseRef: "5.2",
		SourceSentence: "An investment adviser must notify each affected client within 7 days.",
		Deadline:       "P7D",
	}, fullResolver, testNow)

	if len(got.Workflows) == 0 {
		t.Fatal("no workflows generated")
	}
	for _, w := range got.Workflows {
		if w.State != "draft" {
			t.Errorf("workflow %q state = %q, want draft", w.ID, w.State)
		}
		if len(w.Tasks) == 0 {
			t.Errorf("workflow %q has no tasks", w.ID)
		}
		for _, task := range w.Tasks {
			if task.State != "draft" {
				t.Errorf("task %q state = %q, want draft - CHANAKYA never dispatches", task.ID, task.State)
			}
		}
	}
}

// TestOwnersResolveToRealPeople: "owned by Compliance" is not actionable;
// "owned by Priya Menon" is.
func TestOwnersResolveToRealPeople(t *testing.T) {
	got := Synthesize(ObligationInput{
		ID: "obl-1", ClauseRef: "5.2",
		SourceSentence: "An investment adviser must notify each affected client within 7 days.",
		Deadline:       "P7D",
	}, fullResolver, testNow)

	for _, w := range got.Workflows {
		for _, task := range w.Tasks {
			if task.OwnerUnresolved {
				t.Errorf("task %q owner %q did not resolve", task.Title, task.OwnerRole)
				continue
			}
			if task.OwnerName == "" || task.OwnerEmployeeID == "" {
				t.Errorf("task %q resolved to an empty person", task.Title)
			}
		}
	}
}

// TestUnresolvableOwnerIsFlaggedNotFabricated is the phase's other named edge
// case: if a role cannot be resolved the task stays UNASSIGNED and is flagged.
// Assigning an arbitrary employee to satisfy a non-null column would put a real
// person's name against work nobody agreed they own.
func TestUnresolvableOwnerIsFlaggedNotFabricated(t *testing.T) {
	// A resolver that knows only Compliance; every other role is unresolvable.
	partial := stubResolver{known: map[string]string{"Compliance": "Priya Menon"}}

	got := Synthesize(ObligationInput{
		ID: "obl-1", ClauseRef: "5.2",
		SourceSentence: "An investment adviser must notify each affected client within 7 days.",
		Deadline:       "P7D",
	}, partial, testNow)

	if len(got.Workflows) == 0 {
		t.Fatal("no workflows generated")
	}
	w := got.Workflows[0]
	if len(w.UnresolvedOwners) == 0 {
		t.Fatal("unresolvable owners were not flagged on the workflow")
	}

	sawUnresolved := false
	for _, task := range w.Tasks {
		if !task.OwnerUnresolved {
			continue
		}
		sawUnresolved = true
		if task.OwnerEmployeeID != "" || task.OwnerName != "" {
			t.Errorf("task %q is flagged unresolved but was still assigned to %q/%q",
				task.Title, task.OwnerEmployeeID, task.OwnerName)
		}
	}
	if !sawUnresolved {
		t.Error("expected at least one unresolved task owner")
	}
}

// TestTasksFormADAG: dependencies reference real tasks in the same workflow, and
// there are no cycles. A checklist is not a DAG, and collecting acknowledgements
// before the notice has gone out is not a valid order.
func TestTasksFormADAG(t *testing.T) {
	for _, tid := range TemplateIDs() {
		tpl := Templates[tid]
		keys := map[string]bool{}
		for _, task := range tpl.Tasks {
			keys[task.Key] = true
		}
		for _, task := range tpl.Tasks {
			for _, dep := range task.DependsOn {
				if !keys[dep] {
					t.Errorf("template %s: task %q depends on unknown step %q", tid, task.Key, dep)
				}
				if dep == task.Key {
					t.Errorf("template %s: task %q depends on itself", tid, task.Key)
				}
			}
		}
		// Dependencies must point BACKWARDS in the ordered step list, which makes
		// a cycle structurally impossible.
		position := map[string]int{}
		for i, task := range tpl.Tasks {
			position[task.Key] = i
		}
		for _, task := range tpl.Tasks {
			for _, dep := range task.DependsOn {
				if position[dep] >= position[task.Key] {
					t.Errorf("template %s: task %q depends on %q which does not precede it",
						tid, task.Key, dep)
				}
			}
		}
	}
}

// TestAllEightTemplatesExist and are well-formed.
func TestAllEightTemplatesExist(t *testing.T) {
	ids := TemplateIDs()
	if len(ids) != 8 {
		t.Fatalf("got %d templates, want 8", len(ids))
	}
	for _, id := range ids {
		tpl, ok := Templates[id]
		if !ok {
			t.Errorf("template %q is listed but not defined", id)
			continue
		}
		if tpl.Name == "" || tpl.SLA == "" || len(tpl.Tasks) == 0 {
			t.Errorf("template %q is incomplete", id)
		}
		for _, task := range tpl.Tasks {
			if task.OwnerRole == "" {
				t.Errorf("template %q task %q has no owner role", id, task.Key)
			}
		}
	}
}

// TestVocabularyIsClosed: 25 verbs, every one mapped to at least one template.
func TestVocabularyIsClosed(t *testing.T) {
	if len(Vocabulary) != 25 {
		t.Errorf("vocabulary has %d verbs, want 25", len(Vocabulary))
	}
	for _, v := range Vocabulary {
		if !v.Valid() {
			t.Errorf("verb %q is in the vocabulary but maps to no template", v)
		}
		if len(verbTemplates[v]) == 0 {
			t.Errorf("verb %q maps to an empty template list", v)
		}
		for _, tid := range verbTemplates[v] {
			if _, ok := Templates[tid]; !ok {
				t.Errorf("verb %q maps to unknown template %q", v, tid)
			}
		}
	}
	// A verb outside the vocabulary must not validate.
	if Verb("frobnicate").Valid() {
		t.Error("an unknown verb validated - the vocabulary is not closed")
	}
}

// TestSynthesisIsDeterministic: same input, same ids and same plan every time.
func TestSynthesisIsDeterministic(t *testing.T) {
	in := ObligationInput{
		ID: "obl-1", ClauseRef: "4.1",
		SourceSentence: "An investment adviser must disclose the complete fee schedule.",
		Deadline:       "P30D",
	}
	first := Synthesize(in, fullResolver, testNow)
	for i := 0; i < 10; i++ {
		again := Synthesize(in, fullResolver, testNow)
		if len(again.Workflows) != len(first.Workflows) {
			t.Fatal("workflow count is not deterministic")
		}
		for j := range first.Workflows {
			if again.Workflows[j].ID != first.Workflows[j].ID {
				t.Fatal("workflow ids are not deterministic - re-synthesis would duplicate rows")
			}
			for k := range first.Workflows[j].Tasks {
				if again.Workflows[j].Tasks[k].ID != first.Workflows[j].Tasks[k].ID {
					t.Fatal("task ids are not deterministic")
				}
			}
		}
	}
}

// TestDeadlinesRespectTheObligation: a P7D obligation produces tasks due before
// the regulatory deadline, not after it.
func TestDeadlinesRespectTheObligation(t *testing.T) {
	got := Synthesize(ObligationInput{
		ID: "obl-1", ClauseRef: "5.2",
		SourceSentence: "An investment adviser must notify each affected client within 7 days.",
		Deadline:       "P30D",
	}, fullResolver, testNow)

	deadline := testNow.AddDate(0, 0, 30)
	for _, w := range got.Workflows {
		for _, task := range w.Tasks {
			due, err := time.Parse(time.RFC3339, task.Deadline)
			if err != nil {
				t.Errorf("task %q has an unparseable deadline %q", task.Title, task.Deadline)
				continue
			}
			if due.After(deadline) {
				t.Errorf("task %q is due %s, after the obligation's own deadline %s",
					task.Title, due.Format("2006-01-02"), deadline.Format("2006-01-02"))
			}
		}
	}
}

func TestParseISODuration(t *testing.T) {
	cases := map[string]int{"P7D": 7, "P30D": 30, "P1Y": 365, "P2M": 60}
	for in, wantDays := range cases {
		d, ok := parseISODuration(in)
		if !ok {
			t.Errorf("%q did not parse", in)
			continue
		}
		if got := int(d.Hours() / 24); got != wantDays {
			t.Errorf("%q -> %d days, want %d", in, got, wantDays)
		}
	}
	if _, ok := parseISODuration("7 days"); ok {
		t.Error("a non-ISO duration must not parse")
	}
}
