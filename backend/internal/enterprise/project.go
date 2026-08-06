// Package enterprise projects regulatory obligations onto the firm.
//
// SAFETY ROLE. Everything this package produces is INFERENCE, and it is written
// as inference: every binding carries a confidence and a human-confirmation flag,
// and nothing here is presented as a fact the regulator asserted. That
// distinction is the reason the regulatory and enterprise graphs are separate
// namespaces at all - a guess about which internal policy governs a clause must
// never become indistinguishable from the clause itself.
//
// Nothing here enforces anything, and no firm system is written to.
package enterprise

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"chanakya/internal/store"
)

// Target types a binding may point at.
const (
	TargetDocument      = "document"
	TargetRegister      = "register"
	TargetAgreement     = "agreement"
	TargetSystem        = "system"
	TargetClientSegment = "client_segment"
)

// minBindingConfidence is the floor below which a proposed binding is not
// recorded at all. A very weak keyword overlap is noise, and recording it would
// bury the real bindings in a reviewer's queue.
const minBindingConfidence = 0.25

// Binding is a proposed obligation → firm-object edge.
type Binding struct {
	ObligationID string  `json:"obligation_id"`
	TargetType   string  `json:"target_type"`
	TargetID     string  `json:"target_id"`
	TargetLabel  string  `json:"target_label"`
	Confidence   float64 `json:"confidence"`
	// HumanConfirmed is false for everything this package produces. Only a person
	// can set it, and until they do the binding is a proposal.
	HumanConfirmed bool   `json:"human_confirmed"`
	Rationale      string `json:"rationale"`
}

// Projector computes obligation → firm bindings.
type Projector struct {
	store *store.Store
}

// NewProjector builds a Projector.
func NewProjector(st *store.Store) *Projector { return &Projector{store: st} }

// topicKeywords maps a firm-side topic to the words that signal it in clause
// text. This is deliberately a small, readable, auditable table rather than an
// embedding lookup: a reviewer must be able to see WHY a binding was proposed,
// and "cosine 0.41" does not explain that to a compliance officer.
var topicKeywords = map[string][]string{
	"fee":         {"fee", "fees", "charge", "charges", "consideration", "billing"},
	"agreement":   {"agreement", "terms and conditions", "mitc", "engagement", "contract"},
	"record":      {"record", "records", "retain", "retention", "maintain", "preserve"},
	"segregation": {"segregation", "segregate", "distribution", "arm's length", "conflict"},
	// "register" is deliberately absent: every register's label ends in the word,
	// so including it bound a registration clause to all seven registers at once
	// and buried the one that mattered.
	"registration": {"registration", "certificate of registration", "threshold"},
	"notification": {"notify", "notification", "inform", "communicate", "intimate"},
	"complaint":    {"complaint", "grievance", "redress", "redressal"},
	"training":     {"training", "certification", "certified", "qualification"},
	"risk":         {"risk profile", "risk profiling", "suitability"},
	"cyber":        {"cyber", "security", "data", "breach", "resilience"},
	"kyc":          {"kyc", "know your client", "identity", "anti-money"},
	"disclosure":   {"disclose", "disclosure", "transparent"},
	"audit":        {"audit", "inspection", "board"},
}

// Project proposes bindings for one obligation.
//
// It reads the obligation's clause text and its own condition/source sentence,
// scores each firm object against the topic table, and returns the survivors
// above minBindingConfidence. It WRITES NOTHING - see Persist.
func (p *Projector) Project(ctx context.Context, obligationID string, asOf time.Time) ([]Binding, error) {
	ob, err := p.store.GetObligation(ctx, obligationID)
	if err != nil {
		return nil, fmt.Errorf("load obligation %q: %w", obligationID, err)
	}

	// The obligation's own words plus the clause it came from: the clause gives
	// context the extracted fields lose.
	text := strings.ToLower(strings.Join([]string{
		ob.SourceSentence, ob.Condition, ob.ClauseHeading, ob.ClauseText,
	}, " "))
	topics := matchedTopics(text)
	if len(topics) == 0 {
		return nil, nil
	}

	var out []Binding

	docs, err := p.store.ListDocuments(ctx, asOf, false)
	if err != nil {
		return nil, fmt.Errorf("list documents for projection: %w", err)
	}
	for _, d := range docs {
		if conf, why := score(topics, strings.ToLower(d.Title)); conf >= minBindingConfidence {
			out = append(out, Binding{
				ObligationID: obligationID, TargetType: TargetDocument, TargetID: d.ID,
				TargetLabel: d.Title, Confidence: conf,
				Rationale: fmt.Sprintf("clause and document both concern %s", why),
			})
		}
	}

	registers, err := p.store.ListRegisters(ctx, asOf)
	if err != nil {
		return nil, fmt.Errorf("list registers for projection: %w", err)
	}
	for _, r := range registers {
		if conf, why := score(topics, strings.ToLower(r.Kind+" register")); conf >= minBindingConfidence {
			out = append(out, Binding{
				ObligationID: obligationID, TargetType: TargetRegister, TargetID: r.ID,
				TargetLabel: r.Kind + " register", Confidence: conf,
				Rationale: fmt.Sprintf("clause and register both concern %s", why),
			})
		}
	}

	systems, err := p.store.ListSystems(ctx, asOf)
	if err != nil {
		return nil, fmt.Errorf("list systems for projection: %w", err)
	}
	for _, sy := range systems {
		if conf, why := score(topics, strings.ToLower(sy.Kind+" "+sy.Vendor)); conf >= minBindingConfidence {
			out = append(out, Binding{
				ObligationID: obligationID, TargetType: TargetSystem, TargetID: sy.ID,
				TargetLabel: sy.Vendor, Confidence: conf,
				Rationale: fmt.Sprintf("clause and system both concern %s", why),
			})
		}
	}

	// An obligation about the client agreement binds to the AGREEMENT POPULATION
	// rather than to 140 individual rows: the duty is on the template, and the
	// per-client impact is what ImpactOf computes.
	if topics["agreement"] {
		out = append(out, Binding{
			ObligationID: obligationID, TargetType: TargetClientSegment, TargetID: "all_clients",
			TargetLabel: "All advised clients", Confidence: 0.8,
			Rationale: "the obligation governs the client agreement, which every client holds",
		})
	}

	// Deterministic order: strongest first, then by id so the output is stable
	// across runs and diffable in a test.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].TargetType != out[j].TargetType {
			return out[i].TargetType < out[j].TargetType
		}
		return out[i].TargetID < out[j].TargetID
	})
	return out, nil
}

// matchedTopics returns the topics whose vocabulary appears in the text.
func matchedTopics(text string) map[string]bool {
	out := map[string]bool{}
	for topic, words := range topicKeywords {
		for _, w := range words {
			if strings.Contains(text, w) {
				out[topic] = true
				break
			}
		}
	}
	return out
}

// score rates a target's label against the obligation's topics, returning the
// confidence and the topic that produced it (for the rationale).
func score(topics map[string]bool, label string) (float64, string) {
	best, bestTopic := 0.0, ""
	for topic := range topics {
		for _, w := range topicKeywords[topic] {
			if !strings.Contains(label, w) {
				continue
			}
			// A longer, more specific word matching is stronger evidence than a
			// short generic one.
			c := 0.45 + float64(len(w))*0.02
			if c > 0.9 {
				c = 0.9
			}
			if c > best {
				best, bestTopic = c, topic
			}
		}
	}
	return best, bestTopic
}

// Persist upserts the proposed bindings, deduping on
// (obligation_id, target_type, target_id) so re-projecting the same obligation
// updates the existing edge instead of adding a parallel one.
//
// human_confirmed is never set here. It is only ever set by a person, and it
// stays 0 for everything this package writes.
func (p *Projector) Persist(ctx context.Context, bindings []Binding, validFrom, txFrom string) error {
	for _, b := range bindings {
		if err := p.store.UpsertBinding(ctx, store.BindingRecord{
			ObligationID: b.ObligationID,
			TargetType:   b.TargetType,
			TargetID:     b.TargetID,
			Confidence:   b.Confidence,
			Rationale:    b.Rationale,
		}, validFrom, txFrom); err != nil {
			return fmt.Errorf("persist binding %s→%s: %w", b.ObligationID, b.TargetID, err)
		}
	}
	return nil
}
