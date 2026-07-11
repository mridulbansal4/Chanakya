package httpapi

import (
	"encoding/json"
	"net/http"

	"chanakya/internal/feed"
)

// resolveCircular returns the circular id from ?circular= or the first seeded
// circular. It writes an error response and returns ("", false) on failure.
func (h *handlers) resolveCircular(w http.ResponseWriter, r *http.Request) (string, bool) {
	if c := r.URL.Query().Get("circular"); c != "" {
		return c, true
	}
	id, err := h.store.FirstCircularID(r.Context())
	if err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "no circular loaded")
			return "", false
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve circular")
		return "", false
	}
	return id, true
}

// lineage: GET /api/lineage?as_of=&circular= — the bi-temporal audit lineage.
func (h *handlers) lineage(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	circular, ok := h.resolveCircular(w, r)
	if !ok {
		return
	}
	lin, err := h.store.Lineage(r.Context(), circular, asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reconstruct lineage")
		return
	}
	writeJSON(w, http.StatusOK, lin)
}

// regulatorFeed: GET /api/feed?as_of=&circular= — the machine-readable SupTech
// feed. It is validated against its schema before emission; a schema failure is
// a server error, so downstream consumers can trust the shape.
func (h *handlers) regulatorFeed(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	circular, ok := h.resolveCircular(w, r)
	if !ok {
		return
	}
	f, err := h.store.RegulatorFeed(r.Context(), circular, asOf)
	if err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "circular not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to build feed")
		return
	}
	payload, err := json.Marshal(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal feed")
		return
	}
	if h.feedValidator != nil {
		if err := h.feedValidator.Validate(payload); err != nil {
			writeError(w, http.StatusInternalServerError, "feed failed self-validation: "+err.Error())
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-CHANAKYA-Feed-Version", f.FeedVersion)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

// feedSchema: GET /api/feed/schema — the JSON schema the feed validates against.
func (h *handlers) feedSchema(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(feed.SchemaJSON)
}
