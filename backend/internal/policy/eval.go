package policy

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// EvalResult is the deterministic outcome of evaluating firm state against a
// compiled policy, plus the OPA evaluation trace.
type EvalResult struct {
	Compliant  bool     `json:"compliant"`
	Applicable bool     `json:"applicable"`
	Denies     []string `json:"denies"`
	Trace      string   `json:"trace"`
}

// Evaluate runs the compiled Rego module against firm-state input using the
// embedded OPA engine, returning the decision and a pretty-printed trace. Pure
// and deterministic: the same module + input always yields the same result.
func Evaluate(ctx context.Context, module string, input any) (EvalResult, error) {
	query, err := rego.New(
		rego.Query("data."+PackageName+".result"),
		rego.Module("chanakya_policy.rego", module),
	).PrepareForEval(ctx)
	if err != nil {
		return EvalResult{}, fmt.Errorf("prepare policy: %w", err)
	}

	tracer := topdown.NewBufferTracer()
	rs, err := query.Eval(ctx, rego.EvalInput(input), rego.EvalQueryTracer(tracer))
	if err != nil {
		return EvalResult{}, fmt.Errorf("evaluate policy: %w", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return EvalResult{}, fmt.Errorf("policy produced no result")
	}

	m, ok := rs[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return EvalResult{}, fmt.Errorf("unexpected result shape %T", rs[0].Expressions[0].Value)
	}

	res := EvalResult{Denies: []string{}}
	if c, ok := m["compliant"].(bool); ok {
		res.Compliant = c
	}
	if a, ok := m["applicable"].(bool); ok {
		res.Applicable = a
	}
	if denies, ok := m["deny"].([]any); ok {
		for _, d := range denies {
			if s, ok := d.(string); ok {
				res.Denies = append(res.Denies, s)
			}
		}
	}

	var sb strings.Builder
	topdown.PrettyTrace(&sb, *tracer)
	res.Trace = sb.String()
	return res, nil
}
