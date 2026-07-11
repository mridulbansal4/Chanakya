package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/policy"
	"chanakya/internal/store"
)

// listPolicies: GET /api/policies?as_of= — approved obligations + policy status.
func (h *handlers) listPolicies(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	list, err := h.store.ListPolicyCandidates(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list policies")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":      domain.RFC3339UTC(asOf),
		"candidates": list,
	})
}

// firmState: GET /api/firm-state?as_of= — suggested evaluation input.
func (h *handlers) firmState(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	fs, err := h.store.FirmState(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build firm state")
		return
	}
	writeJSON(w, http.StatusOK, fs)
}

// getPolicy: GET /api/policy?obligation_id= — the compiled policy + latest eval.
func (h *handlers) getPolicy(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("obligation_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "obligation_id is required")
		return
	}
	p, found, err := h.store.GetPolicy(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load policy")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "no policy compiled for this obligation")
		return
	}
	out := map[string]any{"policy": p}
	if ev, evFound, err := h.store.GetPolicyEval(r.Context(), id); err == nil && evFound {
		out["eval"] = ev
	}
	writeJSON(w, http.StatusOK, out)
}

type compilePolicyInput struct {
	ObligationID string `json:"obligation_id"`
}

// compilePolicy: POST /api/policy/compile — SAFETY GATE: only an approved
// (signed) obligation can be compiled into an enforceable policy.
func (h *handlers) compilePolicy(w http.ResponseWriter, r *http.Request) {
	var in compilePolicyInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	in.ObligationID = strings.TrimSpace(in.ObligationID)
	if in.ObligationID == "" {
		writeError(w, http.StatusBadRequest, "obligation_id is required")
		return
	}
	ctx := r.Context()
	ob, err := h.store.GetObligationDomain(ctx, in.ObligationID)
	if err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "obligation not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load obligation")
		return
	}
	// Gate: must be approved, and a valid approve sign-off must exist.
	if ob.Status != domain.StatusApproved {
		writeError(w, http.StatusConflict, "obligation must be signed off (approved) before a policy can be compiled")
		return
	}
	if so, found, err := h.store.GetSignoff(ctx, in.ObligationID); err != nil || !found || so.Action != "approve" {
		writeError(w, http.StatusConflict, "no approving sign-off found for this obligation")
		return
	}

	rego, err := policy.Compile(ob)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compile policy")
		return
	}
	rec := store.PolicyRecord{
		ID: "pol:" + in.ObligationID, ObligationID: in.ObligationID,
		PackageName: policy.PackageName, Rego: rego, Stage: string(domain.StageAudit),
	}
	// A policy exists in world time from when it is compiled (now) — the audit
	// lineage as-of a date before compilation shows the obligation un-enforced.
	now := domain.RFC3339UTC(time.Now())
	if err := h.store.UpsertPolicy(ctx, rec, now, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store policy")
		return
	}
	p, _, _ := h.store.GetPolicy(ctx, in.ObligationID)
	writeJSON(w, http.StatusOK, map[string]any{"policy": p})
}

type stageInput struct {
	ObligationID string `json:"obligation_id"`
	Stage        string `json:"stage"`
}

// setPolicyStage: POST /api/policy/stage — promote/demote audit|soft|hard.
func (h *handlers) setPolicyStage(w http.ResponseWriter, r *http.Request) {
	var in stageInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	stage := domain.PolicyStage(strings.TrimSpace(in.Stage))
	if !stage.Valid() {
		writeError(w, http.StatusBadRequest, "stage must be audit, soft or hard")
		return
	}
	if err := h.store.SetPolicyStage(r.Context(), strings.TrimSpace(in.ObligationID), stage); err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "no policy for this obligation")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to set stage")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"obligation_id": in.ObligationID, "stage": string(stage)})
}

type evaluateInput struct {
	ObligationID string         `json:"obligation_id"`
	Input        map[string]any `json:"input"`
	Stage        string         `json:"stage,omitempty"`
}

// evaluatePolicy: POST /api/policy/evaluate — deterministically evaluate firm
// state against the compiled policy, record the result, and (only at stage
// hard) mark it blocked.
func (h *handlers) evaluatePolicy(w http.ResponseWriter, r *http.Request) {
	var in evaluateInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	in.ObligationID = strings.TrimSpace(in.ObligationID)
	if in.ObligationID == "" {
		writeError(w, http.StatusBadRequest, "obligation_id is required")
		return
	}
	if in.Input == nil {
		writeError(w, http.StatusBadRequest, "input is required")
		return
	}
	ctx := r.Context()
	p, found, err := h.store.GetPolicy(ctx, in.ObligationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load policy")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "compile a policy for this obligation first")
		return
	}

	stage := p.Stage
	if in.Stage != "" {
		if !domain.PolicyStage(in.Stage).Valid() {
			writeError(w, http.StatusBadRequest, "stage must be audit, soft or hard")
			return
		}
		stage = in.Stage
	}

	res, err := policy.Evaluate(ctx, p.Rego, in.Input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to evaluate policy")
		return
	}
	// Staged enforcement: only 'hard' blocks, and only when non-compliant.
	blocked := stage == string(domain.StageHard) && !res.Compliant

	inputJSON, _ := json.Marshal(in.Input)
	now := domain.RFC3339UTC(time.Now())
	rec := store.PolicyEvalRecord{
		ID: "ev:" + in.ObligationID, PolicyID: p.ID, ObligationID: in.ObligationID,
		InputJSON: string(inputJSON), Compliant: res.Compliant, Applicable: res.Applicable,
		Denies: res.Denies, Stage: stage, Blocked: blocked, Trace: res.Trace,
	}
	if err := h.store.UpsertPolicyEval(ctx, rec, now, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record evaluation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"obligation_id": in.ObligationID,
		"stage":         stage,
		"compliant":     res.Compliant,
		"applicable":    res.Applicable,
		"denies":        res.Denies,
		"blocked":       blocked,
		"trace":         res.Trace,
	})
}

// decodeJSON reads a JSON body with a 1 MiB cap and unknown-field rejection.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
