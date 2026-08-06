package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"chanakya/internal/bootstrap"
	"chanakya/internal/ingest"
	"chanakya/internal/jobs"
	"chanakya/internal/store"

	"github.com/go-chi/chi/v5"
)

// maxUploadBytes bounds a multipart upload. It matches ingest.MaxDocumentBytes
// so the two limits cannot drift apart and produce a confusing double rejection.
const maxUploadBytes = ingest.MaxDocumentBytes

// postIngest: POST /api/ingest - multipart PDF upload.
//
// Returns 202 with an ingest_id IMMEDIATELY. The pipeline is 40-150s with a live
// model; holding the request open for it would time the client out and, because
// the store pins a single connection, block every other request behind it.
func (h *handlers) postIngest(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "ingestion worker pool is not running")
		return
	}
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "expected a multipart form with a 'file' part")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' part")
		return
	}
	defer func() { _ = file.Close() }()

	raw, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read uploaded file")
		return
	}

	filename := header.Filename
	if filename == "" {
		filename = "upload.pdf"
	}

	// Stage 0 runs SYNCHRONOUSLY: its failures (not a PDF, encrypted, too large)
	// are answers the user needs right now, and they are cheap to determine.
	// Everything expensive happens on a worker.
	doc, err := ingest.Intake(raw, filename)
	if err != nil {
		writeError(w, intakeStatus(err), err.Error())
		return
	}

	ctx := r.Context()
	if err := h.store.PutDocumentBlob(ctx, store.DocumentBlob{
		SHA256: doc.SHA256, Bytes: doc.Bytes, Filename: doc.Filename,
		Size: len(doc.Bytes), PageCount: doc.PageCount,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store the document")
		return
	}

	jobID := "job:" + store.IngestRunID(doc.SHA256)
	run, created, err := h.store.CreateIngestRun(ctx, doc.SHA256, doc.Filename, jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create the ingest run")
		return
	}
	// Content addressing makes the duplicate case exact: the same bytes already
	// queued or running return the EXISTING run rather than starting a second
	// pipeline over an identical document.
	if !created {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"ingest_id": run.ID, "state": run.State, "duplicate": true,
			"sha256": doc.SHA256, "filename": doc.Filename,
		})
		return
	}

	payload, err := json.Marshal(bootstrap.IngestPayload{
		RunID: run.ID, SHA256: doc.SHA256, Filename: doc.Filename,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build the job payload")
		return
	}
	if err := h.store.EnqueueJob(ctx, jobID, bootstrap.JobKindIngest, string(payload)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue the ingestion job")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"ingest_id": run.ID, "job_id": jobID, "state": store.RunQueued,
		"duplicate": false, "sha256": doc.SHA256, "filename": doc.Filename,
		"page_count": doc.PageCount, "stages": ingest.Stages,
	})
}

// intakeStatus maps a Stage 0 rejection to an HTTP status. Each failure mode has
// its own status AND its own message: telling someone their encrypted PDF is
// "damaged" sends them to fix the wrong thing.
func intakeStatus(err error) int {
	switch {
	case errors.Is(err, ingest.ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, ingest.ErrNotPDF), errors.Is(err, ingest.ErrCorrupt):
		return http.StatusBadRequest
	case errors.Is(err, ingest.ErrEncrypted), errors.Is(err, ingest.ErrScanned):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusBadRequest
	}
}

// getIngest: GET /api/ingest/:id - status, current stage, counts, errors.
//
// This is also the RECONNECT path: a client whose SSE stream dropped calls this
// to learn current state, rather than restarting the stream or re-running
// finished stages.
func (h *handlers) getIngest(w http.ResponseWriter, r *http.Request) {
	run, ok := h.loadRun(w, r)
	if !ok {
		return
	}

	resp := map[string]any{
		"ingest_id": run.ID,
		"state":     run.State,
		"stage":     run.Stage,
		"filename":  run.Filename,
		"sha256":    run.SHA256,
		"doc_kind":  run.DocKind,
		"error":     run.Error,
		"stages":    ingest.Stages,
		"counts": map[string]int{
			"clauses":             len(run.Proposal.Clauses),
			"obligations":         len(run.Proposal.Obligations),
			"semantic_units":      len(run.Proposal.Units),
			"resolved_references": len(run.Proposal.ClauseRefs),
			"dangling_references": len(run.Proposal.Dangling),
			"rejected":            len(run.Proposal.Rejected),
		},
	}
	if h.pool != nil && run.JobID != "" {
		if pr, ok := h.pool.LatestProgress(run.JobID); ok {
			resp["progress"] = pr
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// listIngest: GET /api/ingest - recent runs.
func (h *handlers) listIngest(w http.ResponseWriter, r *http.Request) {
	runs, err := h.store.ListIngestRuns(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list ingest runs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(runs), "runs": runs})
}

// ingestEvents: GET /api/ingest/:id/events - SSE progress stream.
//
// A dropped client connection must NOT cancel the job. The stream therefore
// listens on the pool's broadcast and on r.Context().Done() only to stop
// WRITING; the pipeline runs to completion on its worker regardless, and a
// reconnecting client resumes from GET /api/ingest/:id.
func (h *handlers) ingestEvents(w http.ResponseWriter, r *http.Request) {
	run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported by this server")
		return
	}
	if h.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "ingestion worker pool is not running")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// A settled run has no further events; send its terminal state and close so
	// the client is not left holding an idle connection.
	if run.State == store.RunPreview || run.State == store.RunApproved ||
		run.State == store.RunDiscarded || run.State == store.RunFailed {
		writeSSE(w, flusher, "state", map[string]any{
			"ingest_id": run.ID, "state": run.State, "stage": run.Stage, "error": run.Error,
		})
		writeSSE(w, flusher, "done", map[string]any{"state": run.State})
		return
	}

	events, unsubscribe := h.pool.Subscribe(run.JobID)
	defer unsubscribe()

	writeSSE(w, flusher, "state", map[string]any{
		"ingest_id": run.ID, "state": run.State, "stage": run.Stage,
		"stages": ingest.Stages,
	})

	// Heartbeats keep intermediaries from closing an idle connection during a
	// slow stage (a 30-clause extraction can be quiet for a while).
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			// The client went away. The job keeps running - see the note above.
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case pr, open := <-events:
			if !open {
				writeSSE(w, flusher, "done", map[string]any{"ingest_id": run.ID})
				return
			}
			writeSSE(w, flusher, "progress", pr)
		}
	}
}

// writeSSE emits one named SSE event.
func writeSSE(w http.ResponseWriter, f http.Flusher, event string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw); err != nil {
		return
	}
	f.Flush()
}

// ingestPreview: GET /api/ingest/:id/preview - the proposal, BEFORE commit.
//
// Everything here is a proposal. The obligations carry their verbatim source
// sentence and the clause text it came from, so a reviewer can check every
// citation without anything having entered the graph.
func (h *handlers) ingestPreview(w http.ResponseWriter, r *http.Request) {
	run, ok := h.loadRun(w, r)
	if !ok {
		return
	}
	if run.State == store.RunQueued || run.State == store.RunRunning {
		writeError(w, http.StatusConflict, "this run is still processing; no proposal yet")
		return
	}
	if run.State == store.RunFailed {
		writeError(w, http.StatusConflict, "this run failed at stage "+run.Stage+": "+run.Error)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ingest_id": run.ID,
		"state":     run.State,
		"committed": run.State == store.RunApproved,
		"proposal":  run.Proposal,
	})
}

type ingestApproveInput struct {
	SignedBy      string `json:"signed_by"`
	Justification string `json:"justification"`
}

// approveIngest: POST /api/ingest/:id/approve - THE HUMAN GATE.
//
// This is the only code path that moves an ingested document into the regulatory
// graph, and it commits the whole run in one transaction. The same friction as
// obligation sign-off applies: a named person and a substantive justification,
// because accepting a document into the system of record is a decision someone
// must be answerable for.
func (h *handlers) approveIngest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing ingest id")
		return
	}

	var in ingestApproveInput
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(in.SignedBy) == "" {
		writeError(w, http.StatusBadRequest, "signed_by is required")
		return
	}
	if len(strings.TrimSpace(in.Justification)) < minJustificationLen {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("justification must be at least %d characters", minJustificationLen))
		return
	}

	run, err := h.store.ApproveIngestRun(r.Context(), id, strings.TrimSpace(in.SignedBy))
	switch {
	case err == nil:
	case notFound(err):
		writeError(w, http.StatusNotFound, "unknown ingest id")
		return
	case errors.Is(err, store.ErrRunSettled), errors.Is(err, store.ErrRunNotReady):
		// A second approve, an approve after discard, or an approve of a run
		// that has no proposal yet. Never a silent no-op, never a second commit,
		// and the message distinguishes the three.
		writeError(w, http.StatusConflict, err.Error())
		return
	default:
		writeError(w, http.StatusInternalServerError, "commit failed; nothing was written to the graph: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ingest_id":   run.ID,
		"state":       run.State,
		"circular_id": run.CircularID,
		"approved_by": run.ApprovedBy,
		"approved_at": run.ApprovedAt,
		"committed": map[string]int{
			"clauses":     len(run.Proposal.Clauses),
			"obligations": len(run.Proposal.Obligations),
			"units":       len(run.Proposal.Units),
			"relations":   len(run.Proposal.Relations),
			"dangling":    len(run.Proposal.Dangling),
		},
	})
}

// discardIngest: DELETE /api/ingest/:id - drop a proposal.
func (h *handlers) discardIngest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing ingest id")
		return
	}
	err := h.store.DiscardIngestRun(r.Context(), id)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"ingest_id": id, "state": store.RunDiscarded})
	case notFound(err):
		writeError(w, http.StatusNotFound, "unknown ingest id")
	case errors.Is(err, store.ErrRunSettled):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "failed to discard the run")
	}
}

// loadRun resolves the :id path param to a run, writing the error response
// itself when it cannot.
func (h *handlers) loadRun(w http.ResponseWriter, r *http.Request) (store.IngestRun, bool) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing ingest id")
		return store.IngestRun{}, false
	}
	run, err := h.store.GetIngestRun(r.Context(), id)
	if err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "unknown ingest id")
			return store.IngestRun{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load the ingest run")
		return store.IngestRun{}, false
	}
	return run, true
}

// Pool is the worker-pool surface the ingest handlers need.
type Pool interface {
	Subscribe(jobID string) (<-chan jobs.Progress, func())
	LatestProgress(jobID string) (jobs.Progress, bool)
}
