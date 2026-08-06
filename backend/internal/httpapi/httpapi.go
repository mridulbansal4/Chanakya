// Package httpapi wires the chi router, shared middleware, and HTTP handlers
// for CHANAKYA's REST surface. As of Phase 3 it serves /health, /version, and
// the read-only obligation graph API under /api (list, detail, graph, posture),
// every data endpoint honouring an as-of date.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/enterprise"
	"chanakya/internal/ingest"
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

	// Ingestion (Phase 2). Nothing here writes to the regulatory graph except
	// ApproveIngestRun, which is the human gate.
	PutDocumentBlob(ctx context.Context, b store.DocumentBlob) error
	CreateIngestRun(ctx context.Context, sha, filename, jobID string) (store.IngestRun, bool, error)
	GetIngestRun(ctx context.Context, id string) (store.IngestRun, error)
	ListIngestRuns(ctx context.Context, limit int) ([]store.IngestRun, error)
	ApproveIngestRun(ctx context.Context, id, approvedBy string) (store.IngestRun, error)
	DiscardIngestRun(ctx context.Context, id string) error
	UpdateProposal(ctx context.Context, id string, p ingest.Proposal) error
	EnqueueJob(ctx context.Context, id, kind, payloadJSON string) error

	// Enterprise graph (Phase 3). Strictly read-only: the projection layer
	// infers, it never writes to a firm system.
	EnterpriseSummaryAsOf(ctx context.Context, asOf time.Time) (store.EnterpriseSummary, error)
	OrgChart(ctx context.Context, asOf time.Time) ([]store.EmployeeView, error)
	ListClients(ctx context.Context, q store.ClientQuery) ([]store.ClientView, error)
	ListDocuments(ctx context.Context, asOf time.Time, staleOnly bool) ([]store.DocumentView, error)

	// Workflows (Phase 4). Generated tasks are draft-only; approval records a
	// decision and dispatches nothing.
	ListWorkflows(ctx context.Context, asOf time.Time) ([]store.WorkflowView, error)
	GetWorkflow(ctx context.Context, id string, asOf time.Time) (store.WorkflowView, error)
	ApproveWorkflow(ctx context.Context, id, approver, note string, now time.Time) (store.WorkflowView, error)
	RegulatoryFeedItems(ctx context.Context, asOf time.Time) ([]store.RegulatoryFeedItem, error)
}

// Projector computes an obligation's projection onto the firm.
// *enterprise.Projector satisfies it.
type Projector interface {
	ImpactOf(ctx context.Context, obligationID string, asOf time.Time) (enterprise.Impact, error)
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
	// Pool is the background worker pool. Nil disables the ingestion endpoints
	// (they answer 503) rather than panicking - useful in tests that only need
	// the read-only surface.
	Pool        Pool
	Projector   Projector
	CORSOrigins []string
	Version     string
}

// NewRouter builds the fully-configured chi router.
func NewRouter(opts Options) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
	// The 30s request timeout is right for every handler EXCEPT the SSE progress
	// stream, which is long-lived by design: an ingestion run is 40-150s. Applying
	// it there would cancel the request context mid-run and cut the stream off
	// while the pipeline was still working.
	r.Use(timeoutExceptStreams(30 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  originChecker(opts.CORSOrigins),
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	h := &handlers{
		store:         opts.Store,
		signer:        opts.Signer,
		feedValidator: opts.FeedValidator,
		pool:          opts.Pool,
		projector:     opts.Projector,
		version:       opts.Version,
	}

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

		// Ingestion. The upload returns 202 immediately; the pipeline runs on a
		// worker and nothing enters the graph until /approve.
		api.Post("/ingest", h.postIngest)
		api.Get("/ingest", h.listIngest)
		api.Get("/ingest/{id}", h.getIngest)
		api.Get("/ingest/{id}/events", h.ingestEvents)
		api.Get("/ingest/{id}/preview", h.ingestPreview)
		api.Post("/ingest/{id}/approve", h.approveIngest)
		api.Delete("/ingest/{id}", h.discardIngest)

		// Enterprise graph + projection.
		api.Get("/enterprise/summary", h.enterpriseSummary)
		api.Get("/enterprise/impact", h.enterpriseImpact)
		api.Get("/clients", h.listEnterpriseClients)
		api.Get("/documents", h.listEnterpriseDocuments)

		// Workflow synthesis + the read-only connector registry.
		api.Get("/workflows", h.listWorkflows)
		api.Get("/workflows/{id}/tasks", h.getWorkflowTasks)
		api.Post("/workflows/{id}/approve", h.approveWorkflow)
		api.Get("/connectors", h.listConnectors)
		api.Get("/regulatory-feed", h.regulatoryFeedItems)
	})

	return r
}

// timeoutExceptStreams applies a request timeout to everything except the SSE
// progress stream, which must stay open for the life of an ingestion run.
func timeoutExceptStreams(d time.Duration) func(http.Handler) http.Handler {
	timed := middleware.Timeout(d)
	return func(next http.Handler) http.Handler {
		timedNext := timed(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/events") {
				next.ServeHTTP(w, r)
				return
			}
			timedNext.ServeHTTP(w, r)
		})
	}
}

// originChecker builds the CORS origin predicate from the configured allowlist.
//
// Safety: an empty allowlist means "local dev", where the developer has not
// stated which origin may talk to the API. We keep today's permissive behaviour
// so `go run ./backend/cmd/api` still works with no configuration, but we log
// the fact ONCE at startup rather than per request - a silently open CORS
// policy is exactly the kind of thing that survives into production unnoticed.
//
// Matching is exact on the full origin (scheme + host + port), the form browsers
// actually send. No trailing-slash stripping and no case folding: any such
// normalisation would widen the allowlist beyond what was configured.
func originChecker(allowed []string) func(*http.Request, string) bool {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		if o = strings.TrimSpace(o); o != "" {
			set[o] = struct{}{}
		}
	}
	if len(set) == 0 {
		log.Printf("chanakya: WARNING CORS allowlist is empty - every origin is accepted. Set CHANAKYA_CORS_ORIGINS before deploying.")
		return func(*http.Request, string) bool { return true }
	}
	return func(_ *http.Request, origin string) bool {
		_, ok := set[origin]
		return ok
	}
}

// handlers holds dependencies shared by the HTTP handlers.
type handlers struct {
	store         Store
	signer        Signer
	feedValidator FeedValidator
	pool          Pool
	projector     Projector
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
