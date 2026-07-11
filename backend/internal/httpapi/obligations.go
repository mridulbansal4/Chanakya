package httpapi

import (
	"net/http"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/store"
)

// parseAsOf reads the ?as_of query parameter. It accepts "YYYY-MM-DD" or a full
// RFC3339 timestamp; a date is interpreted as end-of-day UTC so an obligation
// issued that day is included. Missing/blank defaults to now. Returns an error
// only for a malformed value.
func parseAsOf(r *http.Request) (time.Time, bool) {
	return parseAsOfValue(r.URL.Query().Get("as_of"))
}

// parseAsOfValue parses an as-of string: "YYYY-MM-DD" (interpreted as end of
// that day UTC) or a full RFC3339 timestamp. Blank defaults to now.
func parseAsOfValue(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Now().UTC(), true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), true
	}
	if d, err := time.Parse("2006-01-02", raw); err == nil {
		return d.UTC().Add(24*time.Hour - time.Second), true
	}
	return time.Time{}, false
}

// validDeontic / validStatus guard filter inputs against the known domains.
func validDeontic(s string) bool {
	return s == "" || domain.DeonticType(s).Valid()
}
func validStatus(s string) bool {
	return s == "" || domain.ObligationStatus(s).Valid()
}

// listObligations: GET /api/obligations?as_of=&bearer=&deontic=&status=
func (h *handlers) listObligations(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of (use YYYY-MM-DD or RFC3339)")
		return
	}
	q := store.ObligationQuery{
		AsOf:    asOf,
		Bearer:  r.URL.Query().Get("bearer"),
		Deontic: r.URL.Query().Get("deontic"),
		Status:  r.URL.Query().Get("status"),
	}
	if !validDeontic(q.Deontic) {
		writeError(w, http.StatusBadRequest, "invalid deontic (MUST|MUST_NOT|MAY)")
		return
	}
	if !validStatus(q.Status) {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	items, err := h.store.ListObligations(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list obligations")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":       domain.RFC3339UTC(asOf),
		"count":       len(items),
		"obligations": items,
	})
}

// getObligation: GET /api/obligation?id=<obligation id>
func (h *handlers) getObligation(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	d, err := h.store.GetObligation(r.Context(), id)
	if err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "obligation not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load obligation")
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// posture: GET /api/posture?as_of=
func (h *handlers) posture(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	p, err := h.store.PostureAsOf(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute posture")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// graph: GET /api/graph?as_of=&circular=
func (h *handlers) graph(w http.ResponseWriter, r *http.Request) {
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
				writeJSON(w, http.StatusOK, store.Graph{
					AsOf:  domain.RFC3339UTC(asOf),
					Nodes: []store.GraphNode{},
					Edges: []store.GraphEdge{},
				})
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to resolve circular")
			return
		}
		circular = id
	}
	g, err := h.store.GraphAsOf(r.Context(), circular, asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build graph")
		return
	}
	writeJSON(w, http.StatusOK, g)
}
