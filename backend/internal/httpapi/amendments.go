package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"chanakya/internal/domain"
)

// clauseView is the API shape for a clause in the picker.
type clauseView struct {
	ID        string `json:"id"`
	ClauseRef string `json:"clause_ref"`
	Heading   string `json:"heading"`
	Text      string `json:"text"`
	ParentID  string `json:"parent_id,omitempty"`
}

// listClauses: GET /api/clauses?as_of=&circular=
func (h *handlers) listClauses(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	circular := r.URL.Query().Get("circular")
	if circular == "" {
		id, err := h.store.FirstCircularID(r.Context())
		if err != nil {
			if notFound(err) {
				writeJSON(w, http.StatusOK, map[string]any{"clauses": []clauseView{}})
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to resolve circular")
			return
		}
		circular = id
	}
	clauses, err := h.store.ListClauses(r.Context(), circular, asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list clauses")
		return
	}
	out := make([]clauseView, 0, len(clauses))
	for _, c := range clauses {
		out = append(out, clauseView{
			ID: c.ID, ClauseRef: c.ClauseRef, Heading: c.Heading, Text: c.Text, ParentID: c.ParentID,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":    domain.RFC3339UTC(asOf),
		"circular": circular,
		"clauses":  out,
	})
}

// blastRadiusRequest is the POST body for the amendment preview.
type blastRadiusRequest struct {
	ClauseRef   string   `json:"clause_ref"`
	AmendedText string   `json:"amended_text"`
	Threshold   *float64 `json:"threshold,omitempty"`
	AsOf        string   `json:"as_of,omitempty"`
}

const defaultBlastThreshold = 0.30

// blastRadius: POST /api/amendments/blast-radius
//
// Computes the downstream impact of an amendment as a WHAT-IF preview. Nothing
// is persisted and nothing is enforced — this is a read-only simulation.
func (h *handlers) blastRadius(w http.ResponseWriter, r *http.Request) {
	var req blastRadiusRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20)) // 1 MiB cap
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.ClauseRef = strings.TrimSpace(req.ClauseRef)
	if req.ClauseRef == "" {
		writeError(w, http.StatusBadRequest, "clause_ref is required")
		return
	}
	if strings.TrimSpace(req.AmendedText) == "" {
		writeError(w, http.StatusBadRequest, "amended_text is required")
		return
	}
	threshold := defaultBlastThreshold
	if req.Threshold != nil {
		if *req.Threshold < 0 || *req.Threshold > 1 {
			writeError(w, http.StatusBadRequest, "threshold must be in [0,1]")
			return
		}
		threshold = *req.Threshold
	}

	// as_of via body (fallback to now).
	asOf, ok := parseAsOfValue(req.AsOf)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}

	circular, err := h.store.FirstCircularID(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve circular")
		return
	}
	br, err := h.store.BlastRadius(r.Context(), circular, req.ClauseRef, req.AmendedText, threshold, asOf)
	if err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "clause not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to compute blast radius")
		return
	}
	writeJSON(w, http.StatusOK, br)
}
