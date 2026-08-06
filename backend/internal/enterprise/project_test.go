package enterprise

import (
	"testing"
)

// TestBindingsAreInferenceNotFact: every binding carries a confidence and is
// unconfirmed. This is the property that keeps a guess about which firm policy
// governs a clause distinguishable from the clause itself.
func TestBindingsAreInferenceNotFact(t *testing.T) {
	topics := matchedTopics("an investment adviser must disclose the complete fee schedule")
	if !topics["fee"] {
		t.Fatalf("expected the fee topic to match, got %v", topics)
	}

	conf, why := score(topics, "fee disclosure policy")
	if conf < minBindingConfidence {
		t.Errorf("a fee clause against a fee policy scored %.2f, below the %.2f floor", conf, minBindingConfidence)
	}
	if why == "" {
		t.Error("a binding must be able to explain itself - the rationale drives reviewer trust")
	}
	if conf > 1 {
		t.Errorf("confidence %.2f is out of range", conf)
	}
}

// TestUnrelatedTargetsScoreBelowFloor: weak keyword overlap must not become a
// binding, or the real ones drown in a reviewer's queue.
func TestUnrelatedTargetsScoreBelowFloor(t *testing.T) {
	topics := matchedTopics("an investment adviser must disclose the complete fee schedule")
	if conf, _ := score(topics, "business continuity plan"); conf >= minBindingConfidence {
		t.Errorf("an unrelated document scored %.2f, at or above the %.2f floor", conf, minBindingConfidence)
	}
}

// TestTopicVocabularyIsAuditable: the mapping is a readable table on purpose. A
// compliance officer must be able to see WHY a binding was proposed, which a
// cosine score does not tell them.
func TestTopicVocabularyIsAuditable(t *testing.T) {
	for topic, words := range topicKeywords {
		if len(words) == 0 {
			t.Errorf("topic %q has no vocabulary", topic)
		}
		for _, w := range words {
			if w == "" {
				t.Errorf("topic %q has an empty keyword", topic)
			}
		}
	}
	for _, required := range []string{"fee", "agreement", "record", "segregation", "notification"} {
		if _, ok := topicKeywords[required]; !ok {
			t.Errorf("the topic table is missing %q, which the seeded clauses rely on", required)
		}
	}
}
