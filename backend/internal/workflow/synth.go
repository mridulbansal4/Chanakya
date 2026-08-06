package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Verb is one of the closed vocabulary of regulatory acts.
type Verb string

// The ~25 verbs a SEBI obligation can require. This vocabulary is CLOSED: an
// obligation whose act is not one of these is routed to review as unclassified
// rather than mapped to the nearest-looking template. Guessing here would put a
// firm's operational plan on a similarity score.
const (
	VerbMaintain    Verb = "maintain"
	VerbRetain      Verb = "retain"
	VerbDisclose    Verb = "disclose"
	VerbNotify      Verb = "notify"
	VerbObtain      Verb = "obtain"
	VerbSubmit      Verb = "submit"
	VerbReport      Verb = "report"
	VerbAppoint     Verb = "appoint"
	VerbReview      Verb = "review"
	VerbTrain       Verb = "train"
	VerbSegregate   Verb = "segregate"
	VerbDisplay     Verb = "display"
	VerbRefrain     Verb = "refrain"
	VerbEnsure      Verb = "ensure"
	VerbVerify      Verb = "verify"
	VerbRecord      Verb = "record"
	VerbPublish     Verb = "publish"
	VerbRedress     Verb = "redress"
	VerbAudit       Verb = "audit"
	VerbCertify     Verb = "certify"
	VerbRegister    Verb = "register"
	VerbPreserve    Verb = "preserve"
	VerbCommunicate Verb = "communicate"
	VerbApprove     Verb = "approve"
	VerbEscalate    Verb = "escalate"
)

// Vocabulary is the closed verb set, in a stable order.
var Vocabulary = []Verb{
	VerbMaintain, VerbRetain, VerbDisclose, VerbNotify, VerbObtain, VerbSubmit,
	VerbReport, VerbAppoint, VerbReview, VerbTrain, VerbSegregate, VerbDisplay,
	VerbRefrain, VerbEnsure, VerbVerify, VerbRecord, VerbPublish, VerbRedress,
	VerbAudit, VerbCertify, VerbRegister, VerbPreserve, VerbCommunicate,
	VerbApprove, VerbEscalate,
}

// verbTemplates is the LOOKUP TABLE that selects templates. It is the whole
// point of the design: the mapping from a regulatory act to an operational
// response is a decision a compliance professional made once and can review,
// not something inferred per obligation.
//
// Some verbs produce more than one workflow - `disclose` requires both a policy
// change and the amended client document.
var verbTemplates = map[Verb][]TemplateID{
	VerbMaintain:    {TemplateEvidenceCollection},
	VerbRetain:      {TemplateEvidenceCollection},
	VerbPreserve:    {TemplateEvidenceCollection},
	VerbRecord:      {TemplateEvidenceCollection},
	VerbDisclose:    {TemplatePolicyUpdate, TemplateClientNotification},
	VerbNotify:      {TemplateClientNotification},
	VerbCommunicate: {TemplateClientNotification},
	VerbObtain:      {TemplateAttestation},
	VerbCertify:     {TemplateAttestation},
	VerbSubmit:      {TemplateFiling},
	VerbReport:      {TemplateFiling},
	VerbRegister:    {TemplateFiling},
	VerbAppoint:     {TemplateBoardApproval},
	VerbApprove:     {TemplateBoardApproval},
	VerbEscalate:    {TemplateBoardApproval},
	VerbReview:      {TemplatePolicyUpdate},
	VerbPublish:     {TemplatePolicyUpdate},
	VerbDisplay:     {TemplatePolicyUpdate},
	VerbTrain:       {TemplateTraining},
	VerbSegregate:   {TemplateRemediation},
	VerbRefrain:     {TemplateRemediation},
	VerbRedress:     {TemplateRemediation},
	VerbEnsure:      {TemplateRemediation},
	VerbVerify:      {TemplateAttestation},
	VerbAudit:       {TemplateAttestation},
}

// verbSurfaceForms maps the words that actually appear in Indian regulatory
// drafting to the canonical verb. "intimate" and "inform" are how SEBI writes
// "notify"; treating them as unknown would send perfectly clear obligations to
// the review queue.
var verbSurfaceForms = map[string]Verb{
	"maintain": VerbMaintain, "maintains": VerbMaintain, "maintaining": VerbMaintain,
	"retain": VerbRetain, "retains": VerbRetain, "retaining": VerbRetain, "keep": VerbRetain,
	"preserve": VerbPreserve, "preserving": VerbPreserve,
	"record": VerbRecord, "records": VerbRecord, "recording": VerbRecord, "log": VerbRecord,
	"disclose": VerbDisclose, "discloses": VerbDisclose, "disclosing": VerbDisclose, "disclosure": VerbDisclose,
	"notify": VerbNotify, "notifies": VerbNotify, "notifying": VerbNotify,
	"inform": VerbNotify, "informs": VerbNotify, "informing": VerbNotify,
	"intimate": VerbNotify, "intimating": VerbNotify,
	"communicate": VerbCommunicate, "communicating": VerbCommunicate,
	"obtain": VerbObtain, "obtains": VerbObtain, "obtaining": VerbObtain,
	"acknowledge": VerbObtain, "acknowledgement": VerbObtain,
	"certify": VerbCertify, "certification": VerbCertify, "certificate": VerbCertify,
	"submit": VerbSubmit, "submits": VerbSubmit, "submitting": VerbSubmit, "file": VerbSubmit, "filing": VerbSubmit,
	"report": VerbReport, "reports": VerbReport, "reporting": VerbReport,
	"register": VerbRegister, "registration": VerbRegister,
	"appoint": VerbAppoint, "appoints": VerbAppoint, "appointing": VerbAppoint, "designate": VerbAppoint,
	"approve": VerbApprove, "approval": VerbApprove,
	"escalate": VerbEscalate, "escalation": VerbEscalate,
	"review": VerbReview, "reviews": VerbReview, "reviewing": VerbReview,
	"publish": VerbPublish, "publishes": VerbPublish, "publishing": VerbPublish,
	"display": VerbDisplay, "displays": VerbDisplay, "displaying": VerbDisplay,
	"train": VerbTrain, "training": VerbTrain, "trained": VerbTrain,
	"segregate": VerbSegregate, "segregation": VerbSegregate, "segregated": VerbSegregate,
	"refrain": VerbRefrain, "abstain": VerbRefrain,
	"redress": VerbRedress, "redressal": VerbRedress, "resolve": VerbRedress,
	"audit": VerbAudit, "audited": VerbAudit, "inspection": VerbAudit,
	"ensure": VerbEnsure, "ensures": VerbEnsure, "ensuring": VerbEnsure,
	"verify": VerbVerify, "verifies": VerbVerify, "verification": VerbVerify,
}

// Valid reports whether v is in the closed vocabulary.
func (v Verb) Valid() bool {
	_, ok := verbTemplates[v]
	return ok
}

// ObligationInput is what synthesis needs about an obligation. Deliberately not
// the store type: this package is pure and unit-testable with no database and
// no model.
type ObligationInput struct {
	ID             string
	ClauseRef      string
	Bearer         string
	DeonticType    string
	Condition      string
	SourceSentence string
	Deadline       string // ISO-8601 duration, e.g. "P7D"
	ValidFrom      string
}

// TaskPlan is one generated task.
type TaskPlan struct {
	ID              string   `json:"id"`
	Key             string   `json:"key"`
	Title           string   `json:"title"`
	Detail          string   `json:"detail"`
	OwnerRole       string   `json:"owner_role"`
	OwnerEmployeeID string   `json:"owner_employee_id"`
	OwnerName       string   `json:"owner_name"`
	OwnerUnresolved bool     `json:"owner_unresolved"`
	DependsOn       []string `json:"depends_on"`
	Deadline        string   `json:"deadline"`
	Ordinal         int      `json:"ordinal"`
	// State is ALWAYS draft. It is a field rather than a constant so the JSON a
	// reviewer sees says so explicitly.
	State string `json:"state"`
}

// WorkflowSpec is one generated workflow.
type WorkflowSpec struct {
	ID           string     `json:"id"`
	Template     TemplateID `json:"template"`
	Title        string     `json:"title"`
	ObligationID string     `json:"obligation_id"`
	ClauseRef    string     `json:"clause_ref"`
	Verb         Verb       `json:"verb"`
	State        string     `json:"state"`
	SLA          string     `json:"sla"`
	Rationale    string     `json:"rationale"`
	Tasks        []TaskPlan `json:"tasks"`
	// UnresolvedOwners names the roles that could not be resolved to a person.
	UnresolvedOwners []string `json:"unresolved_owners,omitempty"`
}

// SynthResult is the outcome of synthesising one obligation.
type SynthResult struct {
	Workflows []WorkflowSpec `json:"workflows"`
	// Unclassified is set when the obligation's act is not in the closed
	// vocabulary. The obligation is NOT dropped and NOT guessed at - it goes to
	// the review queue for a human to classify.
	Unclassified bool   `json:"unclassified"`
	Reason       string `json:"reason,omitempty"`
}

// OwnerResolver maps a role name to a real person. Returning ok=false leaves the
// task unassigned and flagged.
type OwnerResolver interface {
	ResolveRole(role string) (employeeID, name string, ok bool)
}

var wordRe = regexp.MustCompile(`[a-z]+`)

// DetectVerb finds the obligation's act, using the closed vocabulary only.
//
// It reads the CONDITION first and the source sentence second: the condition is
// the compiler's extraction of what must be done, and the sentence may contain
// several verbs of which only one is the operative act.
func DetectVerb(o ObligationInput) (Verb, bool) {
	for _, text := range []string{o.Condition, o.SourceSentence} {
		for _, w := range wordRe.FindAllString(strings.ToLower(text), -1) {
			if v, ok := verbSurfaceForms[w]; ok {
				return v, true
			}
		}
	}
	return "", false
}

// Synthesize turns an obligation into draft workflows.
//
// It is deterministic and needs no LLM: the verb selects the template from a
// fixed table, and the template's tasks are parameterised from the obligation's
// own fields. That is what makes it unit-testable, and what keeps a model from
// authoring the firm's operational plan.
func Synthesize(o ObligationInput, resolver OwnerResolver, now time.Time) SynthResult {
	verb, ok := DetectVerb(o)
	if !ok {
		// Unrecognised verb: route to review as unclassified. Do NOT guess the
		// nearest template, and do NOT silently drop the obligation.
		return SynthResult{
			Unclassified: true,
			Reason: fmt.Sprintf(
				"clause %s: no act from the closed verb vocabulary was found, so no template could be selected; a reviewer must classify it",
				o.ClauseRef),
		}
	}

	templateIDs := verbTemplates[verb]
	out := SynthResult{}

	for _, tid := range templateIDs {
		tpl, exists := Templates[tid]
		if !exists {
			continue
		}
		spec := WorkflowSpec{
			ID:           workflowID(o.ID, tid),
			Template:     tid,
			Title:        fmt.Sprintf("%s - clause %s", tpl.Name, o.ClauseRef),
			ObligationID: o.ID,
			ClauseRef:    o.ClauseRef,
			Verb:         verb,
			State:        "draft",
			SLA:          tpl.SLA,
			Rationale: fmt.Sprintf(
				"The obligation's act is %q, which the verb table maps to the %s template.",
				verb, tpl.Name),
		}

		deadline := obligationDeadline(o, now)
		unresolved := map[string]bool{}

		for i, t := range tpl.Tasks {
			plan := TaskPlan{
				ID:        taskID(spec.ID, t.Key),
				Key:       t.Key,
				Title:     t.Title,
				Detail:    t.Detail,
				OwnerRole: t.OwnerRole,
				Ordinal:   i + 1,
				State:     "draft", // always
				Deadline:  deadline.AddDate(0, 0, t.OffsetDays).UTC().Format(time.RFC3339),
			}
			for _, dep := range t.DependsOn {
				plan.DependsOn = append(plan.DependsOn, taskID(spec.ID, dep))
			}

			if resolver != nil {
				if id, name, ok := resolver.ResolveRole(t.OwnerRole); ok {
					plan.OwnerEmployeeID, plan.OwnerName = id, name
				} else {
					// Leave it unassigned and FLAG it. Assigning an arbitrary
					// employee to satisfy a non-null column would put a real
					// person's name against work nobody agreed they own.
					plan.OwnerUnresolved = true
					unresolved[t.OwnerRole] = true
				}
			} else {
				plan.OwnerUnresolved = true
				unresolved[t.OwnerRole] = true
			}
			spec.Tasks = append(spec.Tasks, plan)
		}

		for role := range unresolved {
			spec.UnresolvedOwners = append(spec.UnresolvedOwners, role)
		}
		sort.Strings(spec.UnresolvedOwners)

		out.Workflows = append(out.Workflows, spec)
	}
	return out
}

// obligationDeadline resolves the obligation's ISO-8601 duration against now.
// With no stated deadline the workflow is anchored 90 days out, which is a
// planning horizon rather than a claim about what the regulation requires.
func obligationDeadline(o ObligationInput, now time.Time) time.Time {
	if d, ok := parseISODuration(o.Deadline); ok {
		return now.Add(d)
	}
	return now.AddDate(0, 0, 90)
}

var isoDurationRe = regexp.MustCompile(`^P(?:(\d+)Y)?(?:(\d+)M)?(?:(\d+)D)?$`)

// parseISODuration handles the day/month/year forms the compiler emits (P7D,
// P30D, P1Y). Time-of-day components are not used by any obligation deadline.
func parseISODuration(s string) (time.Duration, bool) {
	m := isoDurationRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	days := 0
	for i, mult := range []int{365, 30, 1} {
		if m[i+1] == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(m[i+1], "%d", &n); err != nil {
			return 0, false
		}
		days += n * mult
	}
	if days == 0 {
		return 0, false
	}
	return time.Duration(days) * 24 * time.Hour, true
}

// workflowID and taskID are deterministic, so re-synthesising an unchanged
// obligation upserts the same rows rather than accumulating duplicates.
func workflowID(obligationID string, tid TemplateID) string {
	sum := sha256.Sum256([]byte(obligationID + "|" + string(tid)))
	return "wf:" + hex.EncodeToString(sum[:])[:16]
}

func taskID(workflowID, key string) string {
	return workflowID + "/" + key
}
