package policy

import (
	"context"
	"encoding/json"
	"testing"

	"chanakya/internal/domain"
)

// TestNoRegoInjectionViaThresholdMetric is the regression guard for C-1: a
// hostile threshold.metric must never be able to alter the compiled policy's
// verdict. The payload below is the one that, before the fix, made a
// non-compliant firm (retention_period=1 vs a >=5 requirement) report compliant
// by injecting an unconditional `compliant if { true }` rule.
func TestNoRegoInjectionViaThresholdMetric(t *testing.T) {
	metric := "X\", [\"a\",\"b\"])\n}\ncompliant if { true }\ndeny contains msg2 if {\nnot compliant\nmsg2 := sprintf(\"y %s %s"
	thb, _ := json.Marshal(map[string]any{
		"metric": metric, "operator": ">=", "value": 5, "kind": "requirement",
	})
	ob := domain.Obligation{
		ID: "obl-inj", ClauseID: "c1", Bearer: "ia",
		DeonticType: domain.DeonticMust, SourceClauseRef: "3.1", SourceSentence: "x",
		ThresholdJSON: string(thb),
	}

	mod, err := Compile(ob)
	if err != nil {
		// Rejecting the obligation outright is an acceptable safe outcome.
		return
	}
	// If it compiled, the injected rule must NOT be present as code, and the
	// ground-truth verdict must hold: retention_period=1 is NOT >= 5.
	res, err := Evaluate(context.Background(), mod,
		map[string]any{"metrics": map[string]any{"retention_period": 1}})
	if err != nil {
		t.Fatalf("compiled module failed to evaluate (should be well-formed): %v", err)
	}
	if res.Compliant {
		t.Fatalf("INJECTION: firm with retention_period=1 reported compliant against >=5 requirement\n%s", mod)
	}
}

// TestNoRegoInjectionViaSourceClauseRef guards the comment/message path: a
// newline-bearing source_clause_ref must not escape into executable Rego.
func TestNoRegoInjectionViaSourceClauseRef(t *testing.T) {
	ob := domain.Obligation{
		ID: "obl-2", ClauseID: "c1", Bearer: "ia",
		DeonticType: domain.DeonticMust,
		SourceClauseRef: "3.1\ninjected_rule := 42\n#",
		SourceSentence:  "x", ThresholdJSON: "{}",
	}
	mod, err := Compile(ob)
	if err != nil {
		return // safe rejection
	}
	// Must evaluate cleanly (no parse error) and behave as a normal
	// no-threshold, attestation-gated policy.
	res, err := Evaluate(context.Background(), mod,
		map[string]any{"attestations": map[string]any{"obl-2": true}})
	if err != nil {
		t.Fatalf("module should be well-formed and evaluable: %v", err)
	}
	if !res.Applicable || !res.Compliant {
		t.Fatalf("attested no-threshold obligation should be applicable+compliant, got %+v", res)
	}
}

// TestCompileNeverReturnsUnparseableModule is the regression guard for H-2:
// Compile must never hand back a module that fails to prepare for evaluation.
// A payload crafted to produce syntactically invalid Rego must cause Compile to
// error, not return a module that would later brick evaluation.
func TestCompileNeverReturnsUnparseableModule(t *testing.T) {
	// Unbalanced brace attempt via the metric.
	thb, _ := json.Marshal(map[string]any{
		"metric": `retention_period"] >= 5 }  bogus { {`, "operator": ">=", "value": 5, "kind": "requirement",
	})
	ob := domain.Obligation{
		ID: "obl-dos", ClauseID: "c1", Bearer: "ia",
		DeonticType: domain.DeonticMust, SourceClauseRef: "3.1", SourceSentence: "x",
		ThresholdJSON: string(thb),
	}
	mod, err := Compile(ob)
	if err != nil {
		return // rejected at compile time - good
	}
	// If Compile returned a module, it MUST be evaluable (validatePrepares ran).
	if _, err := Evaluate(context.Background(), mod,
		map[string]any{"metrics": map[string]any{"retention_period": 1}}); err != nil {
		t.Fatalf("Compile returned a module that does not evaluate (H-2 regression): %v", err)
	}
}
