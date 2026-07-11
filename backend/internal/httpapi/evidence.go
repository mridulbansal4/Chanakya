package httpapi

import (
	"net/http"

	"chanakya/internal/domain"
)

// evidenceMap: GET /api/evidence?as_of=
func (h *handlers) evidenceMap(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	em, err := h.store.EvidenceMap(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build evidence map")
		return
	}
	writeJSON(w, http.StatusOK, em)
}

// tickets: GET /api/tickets?as_of=
func (h *handlers) tickets(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	list, err := h.store.ListTickets(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tickets")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":   domain.RFC3339UTC(asOf),
		"count":   len(list),
		"tickets": list,
	})
}
