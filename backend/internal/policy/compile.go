// Package policy compiles a SIGNED obligation into a deterministic Rego policy
// and evaluates firm-state input against it with the embedded OPA engine.
//
// SAFETY MODEL: enforcement is done ONLY by this deterministic engine, and a
// policy is only ever compiled for an obligation that a human has approved +
// signed (the caller enforces that gate). The LLM never enforces anything.
// Enforcement is staged audit → soft → hard; "hard" (blocking) is a decision
// the caller records, never applied before a sign-off exists.
package policy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"chanakya/internal/domain"
)

// PackageName is the Rego package every compiled policy lives in.
const PackageName = "chanakya.policy"

// threshold mirrors the obligation's structured threshold, when present.
type threshold struct {
	Metric   string  `json:"metric"`
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Kind     string  `json:"kind"` // "trigger" (default) | "requirement"
}

// opSymbol maps an extracted operator to a Rego comparison. Unknown/empty
// operators default to ">=" (a threshold is a floor by convention here).
func opSymbol(op string) string {
	switch op {
	case ">", ">=", "<", "<=", "==":
		return op
	default:
		return ">="
	}
}

// Compile deterministically generates the Rego module for an obligation. It has
// three shapes so a threshold's meaning is enforced correctly:
//
//   - TRIGGER threshold (">=300 clients -> must register"): the threshold gates
//     applicability; compliance is the firm's attestation. Not applicable =>
//     compliant.
//   - REQUIREMENT threshold ("retain records for >=5 years"): the threshold IS
//     the duty; always applies and compliant iff the firm's metric meets it.
//   - No threshold (fee disclosure): always applies; attestation is compliance.
//
// Attestations are keyed by OBLIGATION ID so obligations on the same clause do
// not collide. The module exposes a single `result` object: {compliant,
// applicable, deny}.
func Compile(o domain.Obligation) (string, error) {
	ref := o.SourceClauseRef
	if ref == "" {
		ref = o.ClauseID
	}

	var th threshold
	hasThreshold := false
	if t := strings.TrimSpace(o.ThresholdJSON); t != "" && t != "{}" && t != "null" {
		if err := json.Unmarshal([]byte(t), &th); err != nil {
			return "", fmt.Errorf("parse threshold for obligation %q: %w", o.ID, err)
		}
		hasThreshold = th.Metric != ""
	}
	isRequirement := hasThreshold && th.Kind == "requirement"

	var b strings.Builder
	fmt.Fprintf(&b, "# CHANAKYA compiled policy — deterministic, generated from a SIGNED obligation.\n")
	fmt.Fprintf(&b, "# obligation: %s\n# clause: %s\n# deontic: %s\n\n", o.ID, ref, o.DeonticType)
	fmt.Fprintf(&b, "package %s\n\n", PackageName)
	fmt.Fprintf(&b, "default compliant := false\n\n")

	switch {
	case isRequirement:
		// The threshold IS the duty: always applies; compliant iff the firm's
		// metric meets it. No attestation — the metric is the check.
		val := strconv.FormatFloat(th.Value, 'f', -1, 64)
		fmt.Fprintf(&b, "# Requirement: the threshold from clause %s is the duty the firm must meet.\n", ref)
		fmt.Fprintf(&b, "applicable := true\n\n")
		fmt.Fprintf(&b, "compliant if {\n\tinput.metrics[%q] %s %s\n}\n\n", th.Metric, opSymbol(th.Operator), val)
		fmt.Fprintf(&b, "deny contains msg if {\n\tnot compliant\n")
		fmt.Fprintf(&b, "\tmsg := sprintf(\"clause %%s (%%s): requirement on %s (%s %s) not met\", [%q, %q])\n",
			th.Metric, opSymbol(th.Operator), val, ref, string(o.DeonticType))
		fmt.Fprintf(&b, "}\n\n")

	case hasThreshold:
		// Trigger: the threshold gates applicability; attestation is compliance.
		val := strconv.FormatFloat(th.Value, 'f', -1, 64)
		fmt.Fprintf(&b, "# Applicability: the regulatory trigger threshold from clause %s.\n", ref)
		fmt.Fprintf(&b, "default applicable := false\n")
		fmt.Fprintf(&b, "applicable if {\n\tinput.metrics[%q] %s %s\n}\n\n", th.Metric, opSymbol(th.Operator), val)
		writeAttestationCompliance(&b, o.ID, ref, string(o.DeonticType), true)

	default:
		// No threshold: always applies; attestation is compliance.
		fmt.Fprintf(&b, "# No numeric threshold — the obligation always applies.\n")
		fmt.Fprintf(&b, "applicable := true\n\n")
		writeAttestationCompliance(&b, o.ID, ref, string(o.DeonticType), false)
	}

	fmt.Fprintf(&b, "result := {\n\t\"compliant\": compliant,\n\t\"applicable\": applicable,\n\t\"deny\": deny,\n}\n")
	return b.String(), nil
}

// writeAttestationCompliance emits the attestation-based compliance rules used by
// trigger and no-threshold policies. Attestations are keyed by the OBLIGATION ID
// so two obligations on one clause never collide. gated=true adds the
// "not applicable -> compliant" branch (only meaningful when applicability can
// be false).
func writeAttestationCompliance(b *strings.Builder, obligationID, ref, deontic string, gated bool) {
	fmt.Fprintf(b, "# Compliant when the firm attests this obligation is satisfied.\n")
	if gated {
		fmt.Fprintf(b, "compliant if { not applicable }\n")
	}
	fmt.Fprintf(b, "compliant if {\n\tapplicable\n\tinput.attestations[%q] == true\n}\n\n", obligationID)
	fmt.Fprintf(b, "deny contains msg if {\n\tapplicable\n\tnot input.attestations[%q]\n", obligationID)
	fmt.Fprintf(b, "\tmsg := sprintf(\"clause %%s (%%s): applies but is not attested as satisfied\", [%q, %q])\n", ref, deontic)
	fmt.Fprintf(b, "}\n")
}
