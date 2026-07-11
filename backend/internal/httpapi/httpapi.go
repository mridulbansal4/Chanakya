// Package httpapi wires the chi router, shared middleware, and HTTP handlers
// for CHANAKYA's REST surface. As of Phase 3 it serves /health, /version, and
// the read-only obligation graph API under /api (list, detail, graph, posture),
// every data endpoint honouring an as-of date.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

// Store is the query surface the handlers need. *store.Store satisfies it.
type Store interface {
	Health(ctx context.Context) error
	FirstCircularID(ctx context.Context) (string, error)
	ListObligations(ctx context.Context, q store.ObligationQuery) ([]store.ObligationView, error)
	GetObligation(ctx context.Context, id string) (store.ObligationDetail, error)
	PostureAsOf(ctx context.Context, asOf time.Time) (store.Posture, error)
	GraphAsOf(ctx context.Context, circularID string, asOf time.Time) (store.Graph, error)
	ListClauses(ctx context.Context, circularID string, asOf time.Time) ([]domain.Clause, error)
	BlastRadius(ctx context.Context, circularID, clauseRef, amendedText string, threshold float64, asOf time.Time) (store.BlastRadius, error)
	EvidenceMap(ctx context.Context, asOf time.Time) (store.EvidenceMapping, error)
	ListTickets(ctx context.Context, asOf time.Time) ([]store.TicketView, error)
	ReviewQueue(ctx context.Context, asOf time.Time) ([]store.ObligationView, error)
	GetObligationDomain(ctx context.Context, id string) (domain.Obligation, error)
	SetObligationStatus(ctx context.Context, id string, status domain.ObligationStatus) error
	ApplyObligationCorrection(ctx context.Context, id string, c store.ObligationCorrection) error
	UpsertSignoff(ctx context.Context, rec store.SignoffRecord, validFrom, txFrom string) error
	GetSignoff(ctx context.Context, obligationID string) (store.SignoffRecord, bool, error)
	ListPolicyCandidates(ctx context.Context, asOf time.Time) ([]store.PolicyCandidate, error)
	GetPolicy(ctx context.Context, obligationID string) (store.PolicyRecord, bool, error)
	UpsertPolicy(ctx context.Context, p store.PolicyRecord, validFrom, txFrom string) error
	SetPolicyStage(ctx context.Context, obligationID string, stage domain.PolicyStage) error
	UpsertPolicyEval(ctx context.Context, e store.PolicyEvalRecord, validFrom, txFrom string) error
	GetPolicyEval(ctx context.Context, obligationID string) (store.PolicyEvalRecord, bool, error)
	FirmState(ctx context.Context, asOf time.Time) (map[string]any, error)
	Lineage(ctx context.Context, circularID string, asOf time.Time) (store.Lineage, error)
	RegulatorFeed(ctx context.Context, circularID string, asOf time.Time) (store.RegulatorFeed, error)
}

// Signer is the Ed25519 signing capability the sign-off handler needs.
type Signer interface {
	Sign(o domain.Obligation) (hashHex, sigB64 string, err error)
	PublicKeyB64() string
}

// FeedValidator validates the regulator feed payload against its schema.
type FeedValidator interface {
	Validate(payload []byte) error
}

// Options configures the router.
type Options struct {
	Store         Store
	Signer        Signer
	FeedValidator FeedValidator
	CORSOrigins   []string
	Version       string
}

// NewRouter builds the fully-configured chi router.
func NewRouter(opts Options) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   opts.CORSOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	h := &handlers{store: opts.Store, signer: opts.Signer, feedValidator: opts.FeedValidator, version: opts.Version}

	r.Get("/health", h.health)
	r.Get("/version", h.versionInfo)

	r.Route("/api", func(api chi.Router) {
		// Per-IP rate limit on the API surface (returns 429 with Retry-After).
		api.Use(httprate.LimitByIP(240, time.Minute))

		api.Get("/obligations", h.listObligations)
		// Obligation ids contain '/' (they embed the circular id), so detail is
		// a query-param endpoint rather than a path param.
		api.Get("/obligation", h.getObligation)
		api.Get("/graph", h.graph)
		api.Get("/posture", h.posture)
		api.Get("/clauses", h.listClauses)
		api.Post("/amendments/blast-radius", h.blastRadius)
		api.Get("/evidence", h.evidenceMap)
		api.Get("/tickets", h.tickets)
		api.Get("/review-queue", h.reviewQueue)
		api.Get("/signoff", h.getSignoff)
		api.Post("/signoff", h.postSignoff)
		api.Get("/policies", h.listPolicies)
		api.Get("/policy", h.getPolicy)
		api.Post("/policy/compile", h.compilePolicy)
		api.Post("/policy/stage", h.setPolicyStage)
		api.Post("/policy/evaluate", h.evaluatePolicy)
		api.Get("/firm-state", h.firmState)
		api.Get("/lineage", h.lineage)
		api.Get("/feed", h.regulatorFeed)
		api.Get("/feed/schema", h.feedSchema)
	})

	return r
}

// handlers holds dependencies shared by the HTTP handlers.
type handlers struct {
	store         Store
	signer        Signer
	feedValidator FeedValidator
	version       string
}

// health reports liveness plus database reachability.
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	dbOK := true
	dbErr := ""
	if err := h.store.Health(r.Context()); err != nil {
		dbOK = false
		dbErr = err.Error()
	}
	status := http.StatusOK
	overall := "ok"
	if !dbOK {
		status = http.StatusServiceUnavailable
		overall = "degraded"
	}
	writeJSON(w, status, map[string]any{
		"status":  overall,
		"version": h.version,
		"checks": map[string]any{
			"database": map[string]any{"ok": dbOK, "error": dbErr},
		},
	})
}

func (h *handlers) versionInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"version": h.version})
}

// writeJSON serialises v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error envelope.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

// notFound reports whether err is a store not-found.
func notFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
