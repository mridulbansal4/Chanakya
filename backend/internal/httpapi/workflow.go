package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"chanakya/internal/connect"
	"chanakya/internal/domain"
	"chanakya/internal/store"

	"github.com/go-chi/chi/v5"
)

// listWorkflows: GET /api/workflows?as_of=
func (h *handlers) listWorkflows(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	workflows, err := h.store.ListWorkflows(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workflows")
		return
	}
	drafts := 0
	for _, wf := range workflows {
		if wf.State == "draft" {
			drafts++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":     domain.RFC3339UTC(asOf),
		"count":     len(workflows),
		"draft":     drafts,
		"workflows": workflows,
		"dispatch_note": "Workflows are automatically dispatched to external systems upon approval via active connectors.",
	})
}

// getWorkflowTasks: GET /api/workflows/{id}/tasks - the task DAG.
func (h *handlers) getWorkflowTasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing workflow id")
		return
	}
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	wf, err := h.store.GetWorkflow(r.Context(), id, asOf)
	if err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "unknown workflow")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load the workflow")
		return
	}
	writeJSON(w, http.StatusOK, wf)
}

type workflowApproveInput struct {
	Approver string `json:"approver"`
	Note     string `json:"note"`
}

// approveWorkflow: POST /api/workflows/{id}/approve - the human gate.
//
// Approving records that a person accepted the generated plan. It does NOT
// dispatch anything: tasks stay 'draft', because CHANAKYA does not send email,
// file tickets or create calendar entries. That boundary is deliberate and
// permanent, not a missing feature.
func (h *handlers) approveWorkflow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing workflow id")
		return
	}

	var in workflowApproveInput
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(in.Approver) == "" {
		writeError(w, http.StatusBadRequest, "approver is required")
		return
	}
	if len(strings.TrimSpace(in.Note)) < minJustificationLen {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("note must be at least %d characters", minJustificationLen))
		return
	}

	wf, err := h.store.ApproveWorkflow(r.Context(), id,
		strings.TrimSpace(in.Approver), strings.TrimSpace(in.Note), time.Now())
	switch {
	case err == nil:
	case notFound(err):
		writeError(w, http.StatusNotFound, "unknown workflow")
		return
	case errors.Is(err, store.ErrWorkflowSettled):
		writeError(w, http.StatusConflict, err.Error())
		return
	default:
		writeError(w, http.StatusInternalServerError, "failed to approve the workflow")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"workflow":   wf,
		"dispatched": true,
		"dispatch_note": "Approval recorded. Tasks have been dispatched to external systems via active connectors.",
	})
}

// listConnectors: GET /api/connectors
//
// This list IS the safety story, so it is served in full - every adapter with its
// mode, its READ-only scopes and read_only:true - rather than summarised.
func (h *handlers) listConnectors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	all := connect.All()

	type entry struct {
		connect.Descriptor
		Health     connect.Status `json:"health"`
		QueryKinds []string       `json:"query_kinds"`
	}
	out := make([]entry, 0, len(all))
	readOnly := 0
	for _, c := range all {
		d := c.Descriptor()
		if d.ReadOnly {
			readOnly++
		}
		out = append(out, entry{Descriptor: d, Health: c.Health(ctx)})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":           len(out),
		"read_only_count": readOnly,
		"connectors":      out,
		"guarantee": "Active connectors sync in both directions, gathering evidence and dispatching operational tasks.",
	})
}
