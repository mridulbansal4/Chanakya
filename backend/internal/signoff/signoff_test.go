package signoff

import (
	"crypto/ed25519"
	"testing"

	"chanakya/internal/domain"
)

func testObligation() domain.Obligation {
	return domain.Obligation{
		ID: "o1", ClauseID: "C#3.1", Bearer: "investment adviser",
		DeonticType: domain.DeonticMust, Condition: "300 or more clients",
		ThresholdJSON: `{"metric":"clients","value":300}`, Deadline: "P30D",
		SourceClauseRef: "3.1", SourceSentence: "A person ... must apply for registration.",
		Confidence: 0.9, Status: domain.StatusPending,
		Temporal: domain.Temporal{ValidFrom: "2024-05-15T00:00:00Z"},
	}
}

func fixedSigner(t *testing.T) *Signer {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	return NewSigner(ed25519.NewKeyFromSeed(seed))
}

func TestSignAndVerify(t *testing.T) {
	s := fixedSigner(t)
	ob := testObligation()

	hash, sig, err := s.Sign(ob)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if hash == "" || sig == "" {
		t.Fatal("empty hash or signature")
	}

	valid, reason, err := Verify(s.PublicKeyB64(), ob, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !valid {
		t.Errorf("valid signature failed to verify: %s", reason)
	}
}

// TestVerifyFailsOnTamper is the core Phase 6 property: any change to the
// obligation's material content invalidates the signature.
func TestVerifyFailsOnTamper(t *testing.T) {
	s := fixedSigner(t)
	ob := testObligation()
	_, sig, err := s.Sign(ob)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	tampers := []struct {
		name   string
		mutate func(*domain.Obligation)
	}{
		{"deontic flipped", func(o *domain.Obligation) { o.DeonticType = domain.DeonticMustNot }},
		{"sentence edited", func(o *domain.Obligation) { o.SourceSentence = "a different sentence" }},
		{"threshold changed", func(o *domain.Obligation) { o.ThresholdJSON = `{"metric":"clients","value":500}` }},
		{"deadline changed", func(o *domain.Obligation) { o.Deadline = "P7D" }},
		{"bearer changed", func(o *domain.Obligation) { o.Bearer = "someone else" }},
	}
	for _, tc := range tampers {
		t.Run(tc.name, func(t *testing.T) {
			bad := testObligation()
			tc.mutate(&bad)
			valid, _, err := Verify(s.PublicKeyB64(), bad, sig)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if valid {
				t.Errorf("tampered obligation (%s) verified as valid — tampering not detected", tc.name)
			}
		})
	}
}

// TestStatusChangeDoesNotBreakSignature: flipping status to approved must NOT
// invalidate the signature (status is not part of the signed content).
func TestStatusChangeDoesNotBreakSignature(t *testing.T) {
	s := fixedSigner(t)
	ob := testObligation()
	_, sig, _ := s.Sign(ob)

	ob.Status = domain.StatusApproved
	valid, reason, err := Verify(s.PublicKeyB64(), ob, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !valid {
		t.Errorf("status change broke the signature: %s", reason)
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	s := fixedSigner(t)
	ob := testObligation()
	_, sig, _ := s.Sign(ob)

	otherSeed := make([]byte, ed25519.SeedSize)
	for i := range otherSeed {
		otherSeed[i] = byte(200 - i)
	}
	other := NewSigner(ed25519.NewKeyFromSeed(otherSeed))

	valid, _, err := Verify(other.PublicKeyB64(), ob, sig)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if valid {
		t.Error("signature verified under the wrong public key")
	}
}
