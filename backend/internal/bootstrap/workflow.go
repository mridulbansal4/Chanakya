package bootstrap

import (
	"context"
	"fmt"
	"time"

	"chanakya/internal/store"
	"chanakya/internal/workflow"
)

// WorkflowResult summarises a synthesis run.
type WorkflowResult struct {
	Workflows int
	Tasks     int
	// Unclassified counts obligations whose act was not in the closed verb
	// vocabulary. They are NOT dropped: they need a human to classify them, and
	// reporting the number is how that becomes visible rather than silent.
	Unclassified []string
	Unresolved   int
}

// GenerateWorkflows synthesises draft workflows for every current obligation.
//
// Everything it writes is state='draft'. Nothing is dispatched.
func GenerateWorkflows(ctx context.Context, st *store.Store, now time.Time) (WorkflowResult, error) {
	obligations, err := st.ListObligations(ctx, store.ObligationQuery{AsOf: now})
	if err != nil {
		return WorkflowResult{}, fmt.Errorf("list obligations for synthesis: %w", err)
	}

	// Owners resolve to REAL EMPLOYEES from the enterprise graph, not to a
	// placeholder role string.
	resolver, err := workflow.NewGraphOwnerResolver(ctx, st)
	if err != nil {
		return WorkflowResult{}, fmt.Errorf("build owner resolver: %w", err)
	}

	var (
		res   WorkflowResult
		specs []workflow.WorkflowSpec
	)
	for _, ob := range obligations {
		out := workflow.Synthesize(workflow.ObligationInput{
			ID:             ob.ID,
			ClauseRef:      ob.ClauseRef,
			Bearer:         ob.Bearer,
			DeonticType:    ob.DeonticType,
			Condition:      ob.Condition,
			SourceSentence: ob.SourceSentence,
			Deadline:       ob.Deadline,
			ValidFrom:      ob.ValidFrom,
		}, resolver, now)

		if out.Unclassified {
			res.Unclassified = append(res.Unclassified, ob.ClauseRef)
			continue
		}
		for _, spec := range out.Workflows {
			res.Tasks += len(spec.Tasks)
			res.Unresolved += len(spec.UnresolvedOwners)
		}
		specs = append(specs, out.Workflows...)
	}
	res.Workflows = len(specs)

	if len(specs) > 0 {
		at := now.UTC().Format(time.RFC3339)
		if err := st.SaveWorkflows(ctx, specs, at, at); err != nil {
			return res, err
		}
	}
	return res, nil
}
