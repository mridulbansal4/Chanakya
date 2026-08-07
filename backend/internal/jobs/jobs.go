// Package jobs is CHANAKYA's in-process background worker pool.
//
// SAFETY ROLE. Long-running ingestion must not run inside an HTTP request, and
// it must not be able to take the server down. Every stage runs under recover():
// a panic in one document's parse marks THAT job failed with the stage name and
// leaves every other in-flight job and the pool itself untouched. A worker
// crash-looping the process would take the audit trail offline with it.
//
// The pool also owns the concurrency budget. Pure-CPU stages fan out across
// workers; LLM-calling work is additionally capped by a semaphore so a large
// circular cannot open thirty simultaneous connections to a rate-limited API.
package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"chanakya/internal/store"
)

// DefaultLLMConcurrency caps simultaneous model calls. The retry/backoff shape
// in llm/gemini.go handles a 429 gracefully, but not asking for one is better.
const DefaultLLMConcurrency = 50

// pollInterval is how often an idle worker looks for new work. A queue in
// SQLite has no push notification, and polling this cheaply (one indexed UPDATE)
// costs far less than the complexity of a notification channel that would still
// need this fallback.
const pollInterval = 250 * time.Millisecond

// Handler executes one job kind. report publishes progress for the SSE stream.
type Handler func(ctx context.Context, job store.Job, report func(Progress)) error

// Progress is a snapshot of where a job is.
type Progress struct {
	Stage  string `json:"stage"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Detail string `json:"detail"`
	// Index/Count locate the stage within the whole pipeline, so a client can
	// render an overall bar without knowing the stage list.
	Index int `json:"index"`
	Count int `json:"count"`
}

// Store is the persistence surface the pool needs.
type Store interface {
	ClaimJob(ctx context.Context) (store.Job, error)
	SetJobProgress(ctx context.Context, id, progressJSON string) error
	FinishJob(ctx context.Context, id, state, errText string) error
}

// Pool runs queued jobs with a fixed number of workers.
type Pool struct {
	store    Store
	handlers map[string]Handler
	workers  int

	// llmSem bounds concurrent model calls across ALL workers, not per worker.
	llmSem chan struct{}

	mu        sync.RWMutex
	listeners map[string]map[chan Progress]struct{}
	latest    map[string]Progress

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// New builds a pool with N = min(4, NumCPU) workers.
func New(st Store, handlers map[string]Handler) *Pool {
	n := runtime.NumCPU()
	if n > 4 {
		n = 4
	}
	if n < 1 {
		n = 1
	}
	return &Pool{
		store:     st,
		handlers:  handlers,
		workers:   n,
		llmSem:    make(chan struct{}, DefaultLLMConcurrency),
		listeners: map[string]map[chan Progress]struct{}{},
		latest:    map[string]Progress{},
	}
}

// Workers reports the pool size.
func (p *Pool) Workers() int { return p.workers }

// Register adds a handler for a job kind. It exists so a handler that needs the
// pool itself (to acquire an LLM slot) can be built after the pool and attached,
// without a second throwaway pool holding a different semaphore.
func (p *Pool) Register(kind string, h Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handlers == nil {
		p.handlers = map[string]Handler{}
	}
	p.handlers[kind] = h
}

// handlerFor looks up a handler under the lock Register writes with.
func (p *Pool) handlerFor(kind string) (Handler, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	h, ok := p.handlers[kind]
	return h, ok
}

// AcquireLLM blocks until a model-call slot is free. Handlers that call an LLM
// wrap each call in AcquireLLM/ReleaseLLM.
func (p *Pool) AcquireLLM(ctx context.Context) error {
	select {
	case p.llmSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("acquire llm slot: %w", ctx.Err())
	}
}

// ReleaseLLM returns a model-call slot.
func (p *Pool) ReleaseLLM() {
	select {
	case <-p.llmSem:
	default:
	}
}

// Start launches the workers. Call Stop to shut them down.
func (p *Pool) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	log.Printf("chanakya: job pool started with %d workers (llm concurrency %d)",
		p.workers, cap(p.llmSem))
}

// Stop signals the workers and waits for them to drain.
func (p *Pool) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
}

// worker claims and runs jobs until the context is cancelled.
func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		job, err := p.store.ClaimJob(ctx)
		if err != nil {
			// An empty queue is the normal case, not an error worth logging.
			continue
		}
		p.run(ctx, job, id)
	}
}

// run executes one job under recover.
func (p *Pool) run(ctx context.Context, job store.Job, workerID int) {
	handler, ok := p.handlerFor(job.Kind)
	if !ok {
		_ = p.store.FinishJob(ctx, job.ID, store.JobFailed,
			fmt.Sprintf("no handler registered for job kind %q", job.Kind))
		return
	}

	// The stage a panic happened in is far more useful than the stack alone, so
	// track the last reported stage and name it in the recorded error.
	stage := "starting"
	report := func(pr Progress) {
		stage = pr.Stage
		p.publish(job.ID, pr)
		if raw, err := json.Marshal(pr); err == nil {
			_ = p.store.SetJobProgress(ctx, job.ID, string(raw))
		}
	}

	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				// A panic must not escape: it would kill the whole pool and
				// every other in-flight job with it.
				log.Printf("chanakya: worker %d recovered panic in job %s stage %s: %v\n%s",
					workerID, job.ID, stage, r, debug.Stack())
				err = fmt.Errorf("panic in stage %q: %v", stage, r)
			}
		}()
		return handler(ctx, job, report)
	}()

	if err != nil {
		_ = p.store.FinishJob(ctx, job.ID, store.JobFailed, err.Error())
		p.publish(job.ID, Progress{Stage: "failed", Detail: err.Error(), Done: 1, Total: 1})
		p.closeListeners(job.ID)
		return
	}
	_ = p.store.FinishJob(ctx, job.ID, store.JobSucceeded, "")
	p.publish(job.ID, Progress{Stage: "done", Detail: "awaiting human approval", Done: 1, Total: 1})
	p.closeListeners(job.ID)
}

// Subscribe returns a channel of progress events for a job, plus an unsubscribe
// function. The buffered channel and non-blocking publish below are what make a
// slow or vanished SSE client harmless: events are dropped for that client, and
// the pipeline never waits on it.
func (p *Pool) Subscribe(jobID string) (<-chan Progress, func()) {
	ch := make(chan Progress, 64)
	p.mu.Lock()
	if p.listeners[jobID] == nil {
		p.listeners[jobID] = map[chan Progress]struct{}{}
	}
	p.listeners[jobID][ch] = struct{}{}
	last, hasLast := p.latest[jobID]
	p.mu.Unlock()

	// Replay the last known state so a client that connects mid-run - or
	// reconnects after a dropped connection - immediately sees where things are
	// instead of waiting for the next event.
	if hasLast {
		ch <- last
	}

	return ch, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		if set, ok := p.listeners[jobID]; ok {
			if _, present := set[ch]; present {
				delete(set, ch)
				close(ch)
			}
			if len(set) == 0 {
				delete(p.listeners, jobID)
			}
		}
	}
}

// publish fans a progress event out to subscribers.
//
// Crucially it NEVER blocks on a subscriber. A browser tab that closed
// mid-ingestion must not be able to stall the pipeline: the job keeps running
// server-side and a reconnecting client catches up via GET /api/ingest/:id.
func (p *Pool) publish(jobID string, pr Progress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.latest[jobID] = pr
	for ch := range p.listeners[jobID] {
		select {
		case ch <- pr:
		default:
		}
	}
}

// closeListeners ends every stream for a finished job.
func (p *Pool) closeListeners(jobID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for ch := range p.listeners[jobID] {
		close(ch)
	}
	delete(p.listeners, jobID)
}

// LatestProgress returns the last known progress for a job.
func (p *Pool) LatestProgress(jobID string) (Progress, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pr, ok := p.latest[jobID]
	return pr, ok
}
