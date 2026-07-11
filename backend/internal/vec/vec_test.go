package vec

import (
	"math"
	"testing"
)

func TestEmbedNormalised(t *testing.T) {
	v := Embed("An investment adviser must disclose its fees to every client.")
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	if len(v) != Dim {
		t.Fatalf("dim = %d, want %d", len(v), Dim)
	}
	if math.Abs(norm-1) > 1e-9 {
		t.Errorf("embedding not L2-normalised: norm^2 = %.6f", norm)
	}
}

func TestEmbedEmptyIsZero(t *testing.T) {
	v := Embed("the a an of to")
	for _, x := range v {
		if x != 0 {
			t.Fatalf("expected zero vector for all-stopword text")
		}
	}
}

func TestCosineSemantics(t *testing.T) {
	fee1 := Embed("The investment adviser must disclose the complete fee schedule before charging any fee.")
	fee2 := Embed("A person charging fees exceeding three crore in fees must apply for registration.")
	retention := Embed("An adviser must retain all records for a period of five years.")

	simFee := Cosine(fee1, fee2)
	simUnrelated := Cosine(fee1, retention)
	if simFee <= simUnrelated {
		t.Errorf("fee/fee similarity (%.3f) should exceed fee/retention (%.3f)", simFee, simUnrelated)
	}
	// identical text is maximally similar.
	if s := Cosine(fee1, fee1); math.Abs(s-1) > 1e-9 {
		t.Errorf("self-similarity = %.6f, want 1", s)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	v := Embed("client-level segregation of advisory and distribution")
	s, err := Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := Unmarshal(s)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got) != len(v) {
		t.Fatalf("round-trip length %d != %d", len(got), len(v))
	}
	if math.Abs(Cosine(v, got)-1) > 1e-9 {
		t.Errorf("round-trip changed the vector")
	}
	if _, err := Unmarshal("[]"); err != nil {
		t.Errorf("empty unmarshal errored: %v", err)
	}
}
