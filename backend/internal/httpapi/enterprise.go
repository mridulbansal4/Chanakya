package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"chanakya/internal/domain"
	"chanakya/internal/store"
)

// enterpriseSummary: GET /api/enterprise/summary?as_of=
//
// The firm's posture reconstructed as of a date. Every gap it reports is the
// result of a query over the seeded graph, not a constant.
func (h *handlers) enterpriseSummary(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	summary, err := h.store.EnterpriseSummaryAsOf(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load the enterprise summary")
		return
	}
	org, err := h.store.OrgChart(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load the org chart")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":       domain.RFC3339UTC(asOf),
		"firm":        summary.Firm,
		"departments": summary.Departments,
		"counts":      summary.Counts,
		"gaps":        summary.Gaps,
		"systems":     summary.Systems,
		"registers":   summary.Registers,
		"org":         org,
	})
}

// enterpriseImpact: GET /api/enterprise/impact?obligation_id=&as_of=
//
// Returns NAMES - clients, documents, controls, owning departments - not counts.
func (h *handlers) enterpriseImpact(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("obligation_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "obligation_id is required")
		return
	}
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	if h.projector == nil {
		writeError(w, http.StatusServiceUnavailable, "the projection layer is not configured")
		return
	}
	impact, err := h.projector.ImpactOf(r.Context(), id, asOf)
	if err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "unknown obligation")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to compute the enterprise impact")
		return
	}
	writeJSON(w, http.StatusOK, impact)
}

// listEnterpriseClients: GET /api/clients?impacted_by=&segment=&adviser=&as_of=
func (h *handlers) listEnterpriseClients(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	q := r.URL.Query()

	// impacted_by resolves through the projection, so the answer is the same set
	// the impact endpoint reports rather than a separately-derived one that could
	// drift from it.
	if obligationID := strings.TrimSpace(q.Get("impacted_by")); obligationID != "" {
		if h.projector == nil {
			writeError(w, http.StatusServiceUnavailable, "the projection layer is not configured")
			return
		}
		impact, err := h.projector.ImpactOf(r.Context(), obligationID, asOf)
		if err != nil {
			if notFound(err) {
				writeError(w, http.StatusNotFound, "unknown obligation")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to resolve impacted clients")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"as_of": domain.RFC3339UTC(asOf), "impacted_by": obligationID,
			"count": len(impact.Clients), "clients": impact.Clients,
		})
		return
	}

	limit := 0
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 1000 {
			writeError(w, http.StatusBadRequest, "limit must be an integer between 0 and 1000")
			return
		}
		limit = n
	}

	clients, err := h.store.ListClients(r.Context(), store.ClientQuery{
		AsOf:            asOf,
		Segment:         strings.TrimSpace(q.Get("segment")),
		AdviserID:       strings.TrimSpace(q.Get("adviser")),
		TemplateVersion: strings.TrimSpace(q.Get("template")),
		Limit:           limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list clients")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of": domain.RFC3339UTC(asOf), "count": len(clients), "clients": clients,
	})
}

// regulatoryFeedItems: GET /api/regulatory-feed?as_of=
//
// The real state of CHANAKYA's regulatory corpus: which circulars it holds, how
// each one arrived (ingested upload vs seeded fixture), what they supersede or
// amend, and what an amendment actually changed. It replaces the scripted
// client-side simulation the screen used to render.
func (h *handlers) regulatoryFeedItems(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	items, err := h.store.RegulatoryFeedItems(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load the regulatory feed")
		return
	}
	runs, err := h.store.ListIngestRuns(r.Context(), 25)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load ingest history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":     domain.RFC3339UTC(asOf),
		"count":     len(items),
		"circulars": items,
		"runs":      runs,
		// Said in the payload so the screen renders the truth rather than a claim
		// someone has to remember to keep accurate.
		"monitoring_note": "CHANAKYA does not poll SEBI. Circulars enter this corpus when a " +
			"person uploads one and approves the result.",
	})
}

// listEnterpriseDocuments: GET /api/documents?stale=true&as_of=
func (h *handlers) listEnterpriseDocuments(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	staleParam := strings.TrimSpace(r.URL.Query().Get("stale"))
	if staleParam != "" && staleParam != "true" && staleParam != "false" {
		writeError(w, http.StatusBadRequest, "stale must be true or false")
		return
	}
	docs, err := h.store.ListDocuments(r.Context(), asOf, staleParam == "true")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of": domain.RFC3339UTC(asOf), "count": len(docs), "documents": docs,
	})
}
