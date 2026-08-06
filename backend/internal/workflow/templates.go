// Package workflow turns an approved obligation into a DRAFT task DAG.
//
// SAFETY ROLE. Everything this package generates is `state='draft'`, exactly as
// store.GenerateDraftTickets already does for tickets, and it stays draft until
// a human approves the workflow. Nothing is dispatched: no email is sent, no
// ticket is filed into a customer system, no calendar invite goes out. "Dispatch"
// is logged, never performed.
//
// The second safety property is that synthesis is VERB-DRIVEN, not free
// generation. A closed vocabulary of regulatory verbs selects the template; a
// model may only fill in the parameters of a template a human already reviewed.
// Letting a model invent the task list would mean the firm's operational plan
// came from an unvalidated generation, which is exactly what the rest of the
// system is built to prevent.
package workflow

// TemplateID names one of the eight task shapes.
type TemplateID string

const (
	TemplatePolicyUpdate       TemplateID = "policy_update"
	TemplateClientNotification TemplateID = "client_notification"
	TemplateEvidenceCollection TemplateID = "evidence_collection"
	TemplateTraining           TemplateID = "training"
	TemplateBoardApproval      TemplateID = "board_approval"
	TemplateFiling             TemplateID = "filing"
	TemplateRemediation        TemplateID = "remediation"
	TemplateAttestation        TemplateID = "attestation"
)

// TaskSpec is one step of a template, before parameterisation.
type TaskSpec struct {
	Key       string
	Title     string
	Detail    string
	OwnerRole string
	// DependsOn lists the Keys of steps that must complete first. This is what
	// makes the output a DAG rather than a checklist: collecting acknowledgements
	// before the notification has gone out is not a valid order.
	DependsOn []string
	// OffsetDays positions the step's deadline relative to the obligation's own.
	// Negative means "before the regulatory deadline".
	OffsetDays int
}

// Template is a named, reviewed task shape.
type Template struct {
	ID          TemplateID
	Name        string
	Description string
	SLA         string
	Tasks       []TaskSpec
}

// Templates is the closed set. Adding one is a deliberate act with a reviewer,
// not something synthesis can do at runtime.
var Templates = map[TemplateID]Template{
	TemplatePolicyUpdate: {
		ID:          TemplatePolicyUpdate,
		Name:        "Policy update",
		Description: "Amend the governing internal policy and route it for approval.",
		SLA:         "P30D",
		Tasks: []TaskSpec{
			{Key: "draft", Title: "Draft the policy amendment", Detail: "Revise the governing policy to reflect the obligation.", OwnerRole: "Compliance", OffsetDays: -21},
			{Key: "legal", Title: "Legal review of the amendment", Detail: "Confirm the drafted change is consistent with the regulation and existing contracts.", OwnerRole: "Legal", DependsOn: []string{"draft"}, OffsetDays: -14},
			{Key: "approve", Title: "Approve and publish the revised policy", Detail: "Record the approval and publish the new version to the document store.", OwnerRole: "Compliance", DependsOn: []string{"legal"}, OffsetDays: -7},
			{Key: "communicate", Title: "Communicate the change internally", Detail: "Notify affected departments that the policy version has changed.", OwnerRole: "HR", DependsOn: []string{"approve"}, OffsetDays: 0},
		},
	},
	TemplateClientNotification: {
		ID:          TemplateClientNotification,
		Name:        "Client notification",
		Description: "Inform affected clients and retain evidence of the communication.",
		SLA:         "P30D",
		Tasks: []TaskSpec{
			{Key: "identify", Title: "Identify the affected client population", Detail: "Resolve the exact client list from the client register and their in-force agreements.", OwnerRole: "Operations", OffsetDays: -21},
			{Key: "draft", Title: "Draft the client communication", Detail: "Prepare the notice text; it must state what changed and what the client must do.", OwnerRole: "Compliance", DependsOn: []string{"identify"}, OffsetDays: -14},
			{Key: "approve", Title: "Approve the communication", Detail: "Compliance sign-off before anything reaches a client.", OwnerRole: "Compliance", DependsOn: []string{"draft"}, OffsetDays: -10},
			// Draft-only, forever: CHANAKYA prepares the send, a person performs it.
			{Key: "send", Title: "Send the notice to affected clients (DRAFT - not dispatched)", Detail: "CHANAKYA drafts the notification and never sends it. A person dispatches from the firm's own email system.", OwnerRole: "Client Servicing", DependsOn: []string{"approve"}, OffsetDays: -3},
			{Key: "acknowledge", Title: "Collect and file client acknowledgements", Detail: "Record each acknowledgement against the client's file.", OwnerRole: "Client Servicing", DependsOn: []string{"send"}, OffsetDays: 0},
		},
	},
	TemplateEvidenceCollection: {
		ID:          TemplateEvidenceCollection,
		Name:        "Evidence collection",
		Description: "Establish and retain the records the obligation requires.",
		SLA:         "P45D",
		Tasks: []TaskSpec{
			{Key: "define", Title: "Define the evidence the obligation requires", Detail: "State exactly which artefact demonstrates compliance, and for how long it must be kept.", OwnerRole: "Compliance", OffsetDays: -30},
			{Key: "locate", Title: "Locate the source system holding it", Detail: "Identify the system of record and confirm read access.", OwnerRole: "Technology", DependsOn: []string{"define"}, OffsetDays: -21},
			{Key: "retention", Title: "Configure the retention period", Detail: "Set the archive retention to at least the mandated period.", OwnerRole: "Operations", DependsOn: []string{"locate"}, OffsetDays: -10},
			{Key: "verify", Title: "Verify a sample can be retrieved", Detail: "Retrieve a sample record end to end to prove the evidence chain works.", OwnerRole: "Risk", DependsOn: []string{"retention"}, OffsetDays: 0},
		},
	},
	TemplateTraining: {
		ID:          TemplateTraining,
		Name:        "Training",
		Description: "Train and certify the staff the obligation applies to.",
		SLA:         "P60D",
		Tasks: []TaskSpec{
			{Key: "scope", Title: "Identify the staff in scope", Detail: "Determine which roles the obligation applies to.", OwnerRole: "HR", OffsetDays: -45},
			{Key: "material", Title: "Prepare the training material", Detail: "Build the content covering the new requirement.", OwnerRole: "Compliance", DependsOn: []string{"scope"}, OffsetDays: -30},
			{Key: "deliver", Title: "Deliver the training", Detail: "Run the session and record attendance.", OwnerRole: "HR", DependsOn: []string{"material"}, OffsetDays: -10},
			{Key: "certify", Title: "Record completion and certificates", Detail: "File each completion record against the employee.", OwnerRole: "HR", DependsOn: []string{"deliver"}, OffsetDays: 0},
		},
	},
	TemplateBoardApproval: {
		ID:          TemplateBoardApproval,
		Name:        "Board approval",
		Description: "Take the matter to the board and minute the decision.",
		SLA:         "P90D",
		Tasks: []TaskSpec{
			{Key: "pack", Title: "Prepare the board paper", Detail: "Summarise the obligation, the firm's exposure and the proposed response.", OwnerRole: "Compliance", OffsetDays: -30},
			{Key: "schedule", Title: "Schedule the board agenda item", Detail: "Place the item on the next board agenda.", OwnerRole: "Compliance", DependsOn: []string{"pack"}, OffsetDays: -21},
			{Key: "minute", Title: "Record the board decision in the minutes", Detail: "Minute the decision and any conditions attached to it.", OwnerRole: "Legal", DependsOn: []string{"schedule"}, OffsetDays: 0},
		},
	},
	TemplateFiling: {
		ID:          TemplateFiling,
		Name:        "Regulatory filing",
		Description: "Prepare, review and submit the required filing.",
		SLA:         "P30D",
		Tasks: []TaskSpec{
			{Key: "prepare", Title: "Prepare the filing", Detail: "Assemble the data the filing requires.", OwnerRole: "Compliance", OffsetDays: -21},
			{Key: "review", Title: "Review the filing for accuracy", Detail: "Second-person check before submission.", OwnerRole: "Risk", DependsOn: []string{"prepare"}, OffsetDays: -10},
			{Key: "submit", Title: "Submit to the regulator (DRAFT - not dispatched)", Detail: "CHANAKYA prepares the submission and never files it. A person submits through the regulator's own portal.", OwnerRole: "Compliance", DependsOn: []string{"review"}, OffsetDays: -2},
			{Key: "file", Title: "File the acknowledgement", Detail: "Retain the regulator's acknowledgement as evidence.", OwnerRole: "Compliance", DependsOn: []string{"submit"}, OffsetDays: 0},
		},
	},
	TemplateRemediation: {
		ID:          TemplateRemediation,
		Name:        "Remediation",
		Description: "Close a gap between the obligation and the firm's current state.",
		SLA:         "P45D",
		Tasks: []TaskSpec{
			{Key: "assess", Title: "Assess the gap", Detail: "Establish where current practice diverges from the obligation.", OwnerRole: "Risk", OffsetDays: -30},
			{Key: "plan", Title: "Agree the remediation plan", Detail: "Decide the corrective action and who owns it.", OwnerRole: "Compliance", DependsOn: []string{"assess"}, OffsetDays: -21},
			{Key: "execute", Title: "Execute the corrective action", Detail: "Carry out the agreed change.", OwnerRole: "Operations", DependsOn: []string{"plan"}, OffsetDays: -7},
			{Key: "close", Title: "Verify and close the finding", Detail: "Confirm the gap is closed and record the evidence.", OwnerRole: "Risk", DependsOn: []string{"execute"}, OffsetDays: 0},
		},
	},
	TemplateAttestation: {
		ID:          TemplateAttestation,
		Name:        "Attestation",
		Description: "Obtain a signed attestation that the obligation is being met.",
		SLA:         "P30D",
		Tasks: []TaskSpec{
			{Key: "prepare", Title: "Prepare the attestation statement", Detail: "Draft what is being attested to, in the obligation's own terms.", OwnerRole: "Compliance", OffsetDays: -21},
			{Key: "circulate", Title: "Circulate for attestation (DRAFT - not dispatched)", Detail: "CHANAKYA drafts the request and never sends it.", OwnerRole: "Compliance", DependsOn: []string{"prepare"}, OffsetDays: -10},
			{Key: "retain", Title: "Retain the signed attestations", Detail: "File each signed attestation as evidence.", OwnerRole: "Operations", DependsOn: []string{"circulate"}, OffsetDays: 0},
		},
	},
}

// TemplateIDs returns the template ids in a stable order.
func TemplateIDs() []TemplateID {
	return []TemplateID{
		TemplatePolicyUpdate, TemplateClientNotification, TemplateEvidenceCollection,
		TemplateTraining, TemplateBoardApproval, TemplateFiling,
		TemplateRemediation, TemplateAttestation,
	}
}
