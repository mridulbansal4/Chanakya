package domain

import "testing"

func validObligation() Obligation {
	return Obligation{
		ID:              "o1",
		ClauseID:        "C#3.1",
		Bearer:          "investment adviser",
		DeonticType:     DeonticMust,
		SourceClauseRef: "3.1",
		SourceSentence:  "An adviser must apply for registration.",
		Confidence:      0.9,
		Status:          StatusPending,
	}
}

func TestObligationValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Obligation)
		wantErr bool
	}{
		{"valid", func(*Obligation) {}, false},
		{"missing clause id", func(o *Obligation) { o.ClauseID = "" }, true},
		{"missing bearer", func(o *Obligation) { o.Bearer = "" }, true},
		{"invalid deontic", func(o *Obligation) { o.DeonticType = "SHOULD" }, true},
		{"invalid status", func(o *Obligation) { o.Status = "maybe" }, true},
		{"missing source clause ref", func(o *Obligation) { o.SourceClauseRef = "" }, true},
		{"missing source sentence", func(o *Obligation) { o.SourceSentence = "" }, true},
		{"confidence too high", func(o *Obligation) { o.Confidence = 1.5 }, true},
		{"confidence negative", func(o *Obligation) { o.Confidence = -0.1 }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := validObligation()
			tc.mutate(&o)
			err := o.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestDeonticAndStatusValid(t *testing.T) {
	if !DeonticMustNot.Valid() || !DeonticMay.Valid() || !DeonticMust.Valid() {
		t.Error("expected all deontic constants valid")
	}
	if DeonticType("X").Valid() {
		t.Error("unexpected valid deontic")
	}
	if !StatusApproved.Valid() || StatusApproved != "approved" {
		t.Error("status approved check failed")
	}
}

func TestTicketStateValid(t *testing.T) {
	if !TicketDraft.Valid() || !TicketFiled.Valid() || !TicketResolved.Valid() {
		t.Error("expected draft/filed/resolved valid")
	}
	if TicketState("archived").Valid() {
		t.Error("unexpected valid ticket state")
	}
	if TicketDraft != "draft" {
		t.Error("draft constant mismatch")
	}
}
