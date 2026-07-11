package feed

import "testing"

func TestValidatorAcceptsValidFeed(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	valid := `{
		"feed_version": "1.0",
		"source": "CHANAKYA SupTech feed",
		"regulator": "SEBI",
		"generated_as_of": "2026-07-12T00:00:00Z",
		"circular": {"id": "SEBI/IA/MC/2024", "title": "IA MC", "issued_on": "2024-05-15T00:00:00Z"},
		"obligations": [
			{
				"id": "o1", "clause_ref": "3.1", "bearer": "investment adviser",
				"deontic_type": "MUST", "threshold": {"metric": "clients", "value": 300},
				"status": "approved", "valid_from": "2024-05-15T00:00:00Z",
				"provenance": {
					"source_clause_ref": "3.1", "source_sentence": "…", "extractor_confidence": 0.85,
					"signoff": {"action": "approve", "signed_by": "V. Bansal", "obligation_hash": "abc", "signature": "sig", "public_key": "pk"}
				}
			}
		]
	}`
	if err := v.Validate([]byte(valid)); err != nil {
		t.Errorf("valid feed rejected: %v", err)
	}
}

func TestValidatorRejectsInvalidFeed(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	cases := map[string]string{
		"missing feed_version": `{"source":"x","regulator":"SEBI","generated_as_of":"t","circular":{"id":"a","title":"b","issued_on":"c"},"obligations":[]}`,
		"bad deontic enum":     `{"feed_version":"1.0","source":"x","regulator":"SEBI","generated_as_of":"t","circular":{"id":"a","title":"b","issued_on":"c"},"obligations":[{"id":"o","clause_ref":"3.1","bearer":"x","deontic_type":"SHOULD","threshold":{},"status":"approved","valid_from":"v","provenance":{"source_clause_ref":"3.1","source_sentence":"s","extractor_confidence":0.9}}]}`,
		"missing provenance":   `{"feed_version":"1.0","source":"x","regulator":"SEBI","generated_as_of":"t","circular":{"id":"a","title":"b","issued_on":"c"},"obligations":[{"id":"o","clause_ref":"3.1","bearer":"x","deontic_type":"MUST","threshold":{},"status":"approved","valid_from":"v"}]}`,
		"unknown top field":    `{"feed_version":"1.0","source":"x","regulator":"SEBI","generated_as_of":"t","circular":{"id":"a","title":"b","issued_on":"c"},"obligations":[],"rogue":1}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if err := v.Validate([]byte(payload)); err == nil {
				t.Errorf("expected %s to fail validation", name)
			}
		})
	}
}
