package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func extract(t *testing.T, ref, text string) extraction {
	t.Helper()
	raw, err := NewOfflineExtractor().Extract(context.Background(), ExtractionRequest{ClauseRef: ref, Text: text})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var doc extraction
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return doc
}

func TestOfflineDeonticClassification(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    string
		wantAny bool
	}{
		{"must", "An adviser must disclose fees.", "MUST", true},
		{"shall", "An adviser shall maintain records.", "MUST", true},
		{"must not", "An adviser must not mix advisory and distribution.", "MUST_NOT", true},
		{"shall not", "An adviser shall not do this.", "MUST_NOT", true},
		{"must notify is not must-not", "An adviser must notify the client within 7 days.", "MUST", true},
		{"may", "An adviser may charge a fee.", "MAY", true},
		{"no modal", "This clause defines the term client.", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := extract(t, "1", tc.text)
			if !tc.wantAny {
				if len(doc.Obligations) != 0 {
					t.Fatalf("expected no obligations, got %d", len(doc.Obligations))
				}
				return
			}
			if len(doc.Obligations) != 1 {
				t.Fatalf("expected 1 obligation, got %d", len(doc.Obligations))
			}
			if doc.Obligations[0].DeonticType != tc.want {
				t.Errorf("deontic = %q, want %q", doc.Obligations[0].DeonticType, tc.want)
			}
		})
	}
}

// TestOfflineCitationIsVerbatim is the key property: the emitted source_sentence
// must be an exact substring of the clause text, so it passes the compiler's
// citation check.
func TestOfflineCitationIsVerbatim(t *testing.T) {
	text := "An investment adviser must retain records for 5 years. It may also keep backups."
	doc := extract(t, "5.1", text)
	if len(doc.Obligations) == 0 {
		t.Fatal("expected obligations")
	}
	for _, o := range doc.Obligations {
		if !strings.Contains(text, o.SourceSentence) {
			t.Errorf("source_sentence %q is not a substring of the clause text", o.SourceSentence)
		}
		if o.SourceClauseRef != "5.1" {
			t.Errorf("source_clause_ref = %q, want 5.1", o.SourceClauseRef)
		}
	}
}

func TestOfflineThresholdAndDeadline(t *testing.T) {
	doc := extract(t, "3.1", "A person providing advice to 300 or more clients must apply within 30 days.")
	if len(doc.Obligations) != 1 {
		t.Fatalf("expected 1 obligation, got %d", len(doc.Obligations))
	}
	o := doc.Obligations[0]
	if o.Threshold == nil || o.Threshold.Value != 300 || o.Threshold.Metric != "clients" {
		t.Errorf("threshold = %+v, want clients=300", o.Threshold)
	}
	if o.Deadline != "P30D" {
		t.Errorf("deadline = %q, want P30D", o.Deadline)
	}
	// threshold + deadline should push confidence above the base 0.70.
	if o.Confidence <= 0.70 {
		t.Errorf("confidence = %.2f, want > 0.70", o.Confidence)
	}
}

// TestOfflineOutputAlwaysValidJSON ensures a definitions clause yields an empty
// but well-formed obligations array.
func TestOfflineOutputAlwaysValidJSON(t *testing.T) {
	raw, err := NewOfflineExtractor().Extract(context.Background(),
		ExtractionRequest{ClauseRef: "1.2", Text: "'client' means a person who receives advice."})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.Contains(string(raw), `"obligations"`) {
		t.Errorf("output missing obligations key: %s", raw)
	}
}
