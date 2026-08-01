// Package policy compiles a SIGNED obligation into a deterministic Rego policy
// and evaluates firm-state input against it with the embedded OPA engine.
//
// SAFETY MODEL: enforcement is done ONLY by this deterministic engine, and a
// policy is only ever compiled for an obligation that a human has approved +
// signed (the caller enforces that gate). The LLM never enforces anything.
// Enforcement is staged audit → soft → hard; "hard" (blocking) is a decision
// the caller records, never applied before a sign-off exists.
//
// INJECTION SAFETY: obligation fields are DATA, never code. Every value that
// reaches the generated module does so either as a JSON-encoded Rego string
// literal (regoString) - which cannot break out of its quotes for any input -
// or as a sanitized single-line comment (regoComment). No obligation field is
// ever interpolated into Rego's code structure. As a final backstop, Compile
// verifies the generated module actually parses and prepares for evaluation
// before returning it, so a caller can never persist an un-evaluable module.
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chanakya/internal/domain"

	"github.com/open-policy-agent/opa/v1/rego"
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
// operators default to ">=" (a threshold is a floor by convention here). The
// return value is ALWAYS one of a fixed set of safe operator tokens, so it is
// safe to emit directly as Rego code.
func opSymbol(op string) string {
	switch op {
	case ">", ">=", "<", "<=", "==":
		return op
	default:
		return ">="
	}
}

// regoString renders s as a Rego string literal. Rego string syntax follows
// JSON, so json.Marshal produces a literal that cannot break out of its quotes
// for ANY input: quotes, backslashes, and control characters (including the
// newlines an attacker would need to inject a new rule) are all escaped. This
// is the single mechanism that makes obligation data safe to embed.
func regoString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		// json.Marshal never fails for a plain string; fall back to empty.
		return `""`
	}
	return string(b)
}

// commentUnsafe matches control characters that must not survive into a
// single-line `#` comment - most importantly newlines, which would terminate
// the comment and let the remainder of the value be parsed as Rego code.
var commentUnsafe = regexp.MustCompile(`[\x00-\x1f\x7f]+`)

// regoComment sanitizes s for safe inclusion in a single-line comment by
// collapsing any run of control characters to a single space. Comments are
// cosmetic, so lossy sanitization here is acceptable and strictly safer than
// escaping.
func regoComment(s string) string {
	return strings.TrimSpace(commentUnsafe.ReplaceAllString(s, " "))
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
// applicable, deny}. Every obligation-derived value is emitted via regoString /
// regoComment, so no input can alter the module's structure.
func Compile(o domain.Obligation) (string, error) {
	ref := o.SourceClauseRef
	if ref == "" {
		ref = o.ClauseID
	}
	deontic := string(o.DeonticType)

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
	fmt.Fprintf(&b, "# CHANAKYA compiled policy - deterministic, generated from a SIGNED obligation.\n")
	fmt.Fprintf(&b, "# obligation: %s\n# clause: %s\n# deontic: %s\n\n",
		regoComment(o.ID), regoComment(ref), regoComment(deontic))
	fmt.Fprintf(&b, "package %s\n\n", PackageName)
	fmt.Fprintf(&b, "default compliant := false\n\n")

	switch {
	case isRequirement:
		// The threshold IS the duty: always applies; compliant iff the firm's
		// metric meets it. No attestation - the metric is the check. The metric
		// name is a JSON-encoded string map key and the value is a numeric
		// literal; the operator is one of the fixed safe tokens from opSymbol.
		op := opSymbol(th.Operator)
		val := strconv.FormatFloat(th.Value, 'f', -1, 64)
		fmt.Fprintf(&b, "# Requirement: the threshold from clause %s is the duty the firm must meet.\n", regoComment(ref))
		fmt.Fprintf(&b, "applicable := true\n\n")
		fmt.Fprintf(&b, "compliant if {\n\tinput.metrics[%s] %s %s\n}\n\n", regoString(th.Metric), op, val)
		// Every dynamic part of the message is a sprintf ARGUMENT (pure data),
		// never part of the constant format string - so no value can change the
		// message's (or module's) structure.
		fmt.Fprintf(&b, "deny contains msg if {\n\tnot compliant\n")
		fmt.Fprintf(&b, "\tmsg := sprintf(\"clause %%s (%%s): requirement on %%s (%%s %%s) not met\", [%s, %s, %s, %s, %s])\n",
			regoString(ref), regoString(deontic), regoString(th.Metric), regoString(op), regoString(val))
		fmt.Fprintf(&b, "}\n\n")

	case hasThreshold:
		// Trigger: the threshold gates applicability; attestation is compliance.
		op := opSymbol(th.Operator)
		val := strconv.FormatFloat(th.Value, 'f', -1, 64)
		fmt.Fprintf(&b, "# Applicability: the regulatory trigger threshold from clause %s.\n", regoComment(ref))
		fmt.Fprintf(&b, "default applicable := false\n")
		fmt.Fprintf(&b, "applicable if {\n\tinput.metrics[%s] %s %s\n}\n\n", regoString(th.Metric), op, val)
		writeAttestationCompliance(&b, o.ID, ref, deontic, true)

	default:
		// No threshold: always applies; attestation is compliance.
		fmt.Fprintf(&b, "# No numeric threshold - the obligation always applies.\n")
		fmt.Fprintf(&b, "applicable := true\n\n")
		writeAttestationCompliance(&b, o.ID, ref, deontic, false)
	}

	fmt.Fprintf(&b, "result := {\n\t\"compliant\": compliant,\n\t\"applicable\": applicable,\n\t\"deny\": deny,\n}\n")
	module := b.String()

	// Backstop: never hand back a module that will not parse/prepare. This
	// closes the class of persisted-but-un-evaluable policies at the source.
	if err := validatePrepares(module); err != nil {
		return "", fmt.Errorf("generated policy for obligation %q is invalid: %w", o.ID, err)
	}
	return module, nil
}

// writeAttestationCompliance emits the attestation-based compliance rules used by
// trigger and no-threshold policies. Attestations are keyed by the OBLIGATION ID
// so two obligations on one clause never collide. gated=true adds the
// "not applicable -> compliant" branch (only meaningful when applicability can
// be false). obligationID and ref are emitted as data (regoString), never code.
func writeAttestationCompliance(b *strings.Builder, obligationID, ref, deontic string, gated bool) {
	fmt.Fprintf(b, "# Compliant when the firm attests this obligation is satisfied.\n")
	if gated {
		fmt.Fprintf(b, "compliant if { not applicable }\n")
	}
	fmt.Fprintf(b, "compliant if {\n\tapplicable\n\tinput.attestations[%s] == true\n}\n\n", regoString(obligationID))
	fmt.Fprintf(b, "deny contains msg if {\n\tapplicable\n\tnot input.attestations[%s]\n", regoString(obligationID))
	fmt.Fprintf(b, "\tmsg := sprintf(\"clause %%s (%%s): applies but is not attested as satisfied\", [%s, %s])\n",
		regoString(ref), regoString(deontic))
	fmt.Fprintf(b, "}\n")
}

// validatePrepares confirms the generated module parses and compiles for
// evaluation with the same query the evaluator uses. It performs no I/O.
func validatePrepares(module string) error {
	_, err := rego.New(
		rego.Query("data."+PackageName+".result"),
		rego.Module("chanakya_policy.rego", module),
	).PrepareForEval(context.Background())
	return err
}
