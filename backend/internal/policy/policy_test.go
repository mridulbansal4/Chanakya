package policy

import (
	"context"
	"strings"
	"testing"

	"chanakya/internal/domain"
)

func thresholdObligation() domain.Obligation {
	return domain.Obligation{
		ID: "o_3_1", ClauseID: "C#3.1", Bearer: "investment adviser",
		DeonticType: domain.DeonticMust, SourceClauseRef: "3.1",
		SourceSentence: "A person providing advice to 300 or more clients must apply for registration.",
		ThresholdJSON:  `{"metric":"clients","operator":">=","value":300,"unit":"count"}`,
		Confidence:     0.85, Status: domain.StatusApproved,
	}
}

func TestCompileProducesValidRego(t *testing.T) {
	mod, err := Compile(thresholdObligation())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for _, want := range []string{"package chanakya.policy", "input.metrics[\"clients\"] >= 300", "input.attestations[\"o_3_1\"]", "result :="} {
		if !strings.Contains(mod, want) {
			t.Errorf("compiled module missing %q\n---\n%s", want, mod)
		}
	}
}

func TestEvaluateDeterministicPassFail(t *testing.T) {
	ctx := context.Background()
	mod, err := Compile(thresholdObligation())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	tests := []struct {
		name           string
		input          map[string]any
		wantCompliant  bool
		wantApplicable bool
		wantDenies     bool
	}{
		{
			name:           "triggered and attested -> compliant",
			input:          map[string]any{"metrics": map[string]any{"clients": 412}, "attestations": map[string]any{"o_3_1": true}},
			wantCompliant:  true,
			wantApplicable: true,
			wantDenies:     false,
		},
		{
			name:           "triggered and NOT attested -> non-compliant with deny",
			input:          map[string]any{"metrics": map[string]any{"clients": 412}, "attestations": map[string]any{"o_3_1": false}},
			wantCompliant:  false,
			wantApplicable: true,
			wantDenies:     true,
		},
		{
			name:           "below threshold -> not applicable, compliant",
			input:          map[string]any{"metrics": map[string]any{"clients": 100}, "attestations": map[string]any{}},
			wantCompliant:  true,
			wantApplicable: false,
			wantDenies:     false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Determinism: evaluate twice, expect identical decisions.
			var first EvalResult
			for i := 0; i < 2; i++ {
				res, err := Evaluate(ctx, mod, tc.input)
				if err != nil {
					t.Fatalf("Evaluate: %v", err)
				}
				if i == 0 {
					first = res
				} else if res.Compliant != first.Compliant || res.Applicable != first.Applicable {
					t.Fatalf("non-deterministic: %+v vs %+v", first, res)
				}
				if res.Compliant != tc.wantCompliant {
					t.Errorf("compliant = %v, want %v", res.Compliant, tc.wantCompliant)
				}
				if res.Applicable != tc.wantApplicable {
					t.Errorf("applicable = %v, want %v", res.Applicable, tc.wantApplicable)
				}
				if (len(res.Denies) > 0) != tc.wantDenies {
					t.Errorf("denies = %v, want present=%v", res.Denies, tc.wantDenies)
				}
				if res.Trace == "" {
					t.Errorf("expected a non-empty evaluation trace")
				}
			}
		})
	}
}

func TestCompileNoThresholdAlwaysApplies(t *testing.T) {
	ctx := context.Background()
	o := thresholdObligation()
	o.ThresholdJSON = "{}"
	o.SourceClauseRef = "4.1"
	mod, err := Compile(o)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(mod, "applicable := true") {
		t.Errorf("no-threshold policy should always apply:\n%s", mod)
	}
	res, err := Evaluate(ctx, mod, map[string]any{"attestations": map[string]any{"o_3_1": true}})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !res.Applicable || !res.Compliant {
		t.Errorf("attested no-threshold obligation should be applicable+compliant, got %+v", res)
	}
}

// TestRequirementThreshold verifies a requirement-type threshold ("retain >=5
// years") is compliant only when the firm's metric meets it — NOT reported
// compliant when the metric is below the requirement.
func TestRequirementThreshold(t *testing.T) {
	ctx := context.Background()
	o := domain.Obligation{
		ID: "o_5_1", ClauseID: "C#5.1", Bearer: "investment adviser",
		DeonticType: domain.DeonticMust, SourceClauseRef: "5.1",
		SourceSentence: "An adviser must retain records for 5 years.",
		ThresholdJSON:  `{"metric":"retention_period","operator":">=","value":5,"unit":"years","kind":"requirement"}`,
		Status:         domain.StatusApproved,
	}
	mod, err := Compile(o)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	cases := []struct {
		name          string
		retention     int
		wantCompliant bool
	}{
		{"meets requirement", 5, true},
		{"exceeds requirement", 7, true},
		{"below requirement -> NON-compliant", 3, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Evaluate(ctx, mod, map[string]any{"metrics": map[string]any{"retention_period": tc.retention}})
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if res.Compliant != tc.wantCompliant {
				t.Errorf("retention=%d compliant=%v, want %v (denies=%v)", tc.retention, res.Compliant, tc.wantCompliant, res.Denies)
			}
			if !res.Applicable {
				t.Errorf("requirement policy should always be applicable")
			}
			if tc.wantCompliant == (len(res.Denies) > 0) {
				t.Errorf("denies should be present iff non-compliant; compliant=%v denies=%v", res.Compliant, res.Denies)
			}
		})
	}
}
