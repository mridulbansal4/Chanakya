package jobs

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"chanakya/internal/store"
)

// fakeStore is an in-memory stand-in for the job table, so the pool's semantics
// can be tested without a database.
type fakeStore struct {
	mu       sync.Mutex
	queued   []store.Job
	finished map[string]string // id -> terminal state
	errors   map[string]string
	progress map[string]string
}

func newFakeStore(jobs ...store.Job) *fakeStore {
	return &fakeStore{
		queued:   jobs,
		finished: map[string]string{},
		errors:   map[string]string{},
		progress: map[string]string{},
	}
}

func (f *fakeStore) ClaimJob(context.Context) (store.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.queued) == 0 {
		return store.Job{}, store.ErrNotFound
	}
	j := f.queued[0]
	f.queued = f.queued[1:]
	j.State = store.JobRunning
	return j, nil
}

func (f *fakeStore) SetJobProgress(_ context.Context, id, progressJSON string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progress[id] = progressJSON
	return nil
}

func (f *fakeStore) FinishJob(_ context.Context, id, state, errText string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finished[id] = state
	f.errors[id] = errText
	return nil
}

func (f *fakeStore) outcome(id string) (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.finished[id], f.errors[id]
}

// waitFor polls until cond is true or the deadline passes.
func waitFor(t *testing.T, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestWorkerPanicIsContained: a panic in one job must mark THAT job failed with
// the stage name, and must not crash the pool or affect other in-flight jobs.
func TestWorkerPanicIsContained(t *testing.T) {
	fs := newFakeStore(
		store.Job{ID: "boom", Kind: "test", PayloadJSON: "{}"},
		store.Job{ID: "fine", Kind: "test", PayloadJSON: "{}"},
	)
	pool := New(fs, map[string]Handler{
		"test": func(_ context.Context, job store.Job, report func(Progress)) error {
			report(Progress{Stage: "parsing", Done: 0, Total: 1})
			if job.ID == "boom" {
				panic("malformed content stream")
			}
			return nil
		},
	})
	pool.Start(context.Background())
	defer pool.Stop()

	if !waitFor(t, func() bool {
		a, _ := fs.outcome("boom")
		b, _ := fs.outcome("fine")
		return a != "" && b != ""
	}) {
		t.Fatal("jobs did not finish - the pool likely died with the panic")
	}

	state, errText := fs.outcome("boom")
	if state != store.JobFailed {
		t.Errorf("panicking job state = %q, want failed", state)
	}
	if !strings.Contains(errText, "parsing") {
		t.Errorf("recorded error %q does not name the stage the panic happened in", errText)
	}

	// The pool survived: the other job completed normally.
	if state, _ := fs.outcome("fine"); state != store.JobSucceeded {
		t.Errorf("the healthy job finished as %q - the panic took down the pool", state)
	}
}

// TestHandlerErrorIsRecorded: an ordinary error marks the job failed.
func TestHandlerErrorIsRecorded(t *testing.T) {
	fs := newFakeStore(store.Job{ID: "j1", Kind: "test"})
	pool := New(fs, map[string]Handler{
		"test": func(context.Context, store.Job, func(Progress)) error {
			return errors.New("intake rejected: encrypted")
		},
	})
	pool.Start(context.Background())
	defer pool.Stop()

	if !waitFor(t, func() bool { s, _ := fs.outcome("j1"); return s != "" }) {
		t.Fatal("job did not finish")
	}
	state, errText := fs.outcome("j1")
	if state != store.JobFailed || !strings.Contains(errText, "encrypted") {
		t.Errorf("state=%q err=%q", state, errText)
	}
}

// TestUnknownKindFailsCleanly: an unregistered job kind is a failure with a
// clear message, not a silently dropped job that stays 'running' forever.
func TestUnknownKindFailsCleanly(t *testing.T) {
	fs := newFakeStore(store.Job{ID: "j1", Kind: "nope"})
	pool := New(fs, map[string]Handler{})
	pool.Start(context.Background())
	defer pool.Stop()

	if !waitFor(t, func() bool { s, _ := fs.outcome("j1"); return s != "" }) {
		t.Fatal("job did not finish")
	}
	state, errText := fs.outcome("j1")
	if state != store.JobFailed || !strings.Contains(errText, "no handler") {
		t.Errorf("state=%q err=%q", state, errText)
	}
}

// TestSubscriberDisconnectDoesNotStallPipeline: a browser tab that closed
// mid-ingestion must not be able to block the worker. The publish path is
// non-blocking, so a full (or abandoned) subscriber channel drops events rather
// than back-pressuring the pipeline.
func TestSubscriberDisconnectDoesNotStallPipeline(t *testing.T) {
	fs := newFakeStore(store.Job{ID: "j1", Kind: "test"})
	pool := New(fs, map[string]Handler{
		"test": func(_ context.Context, _ store.Job, report func(Progress)) error {
			// Far more events than the subscriber buffer holds.
			for i := 0; i < 500; i++ {
				report(Progress{Stage: "compile", Done: i, Total: 500})
			}
			return nil
		},
	})

	// Subscribe and then never read, simulating a client that went away.
	_, unsubscribe := pool.Subscribe("j1")
	defer unsubscribe()

	pool.Start(context.Background())
	defer pool.Stop()

	if !waitFor(t, func() bool { s, _ := fs.outcome("j1"); return s != "" }) {
		t.Fatal("the pipeline stalled on an unread subscriber")
	}
	if state, _ := fs.outcome("j1"); state != store.JobSucceeded {
		t.Errorf("state = %q, want succeeded", state)
	}
}

// TestSubscribeReplaysLatest: a client connecting mid-run (or reconnecting after
// a dropped stream) immediately learns where things are.
func TestSubscribeReplaysLatest(t *testing.T) {
	fs := newFakeStore()
	pool := New(fs, map[string]Handler{})
	pool.publish("j1", Progress{Stage: "segment", Done: 3, Total: 10})

	ch, unsubscribe := pool.Subscribe("j1")
	defer unsubscribe()

	select {
	case pr := <-ch:
		if pr.Stage != "segment" || pr.Done != 3 {
			t.Errorf("replayed %+v, want the last known progress", pr)
		}
	case <-time.After(time.Second):
		t.Fatal("no replay of the last known state")
	}
}

// TestPoolSize: N = min(4, NumCPU).
func TestPoolSize(t *testing.T) {
	p := New(newFakeStore(), nil)
	if p.Workers() < 1 || p.Workers() > 4 {
		t.Errorf("workers = %d, want between 1 and 4", p.Workers())
	}
}

// TestLLMSemaphoreBoundsConcurrency: model calls are capped across ALL workers,
// so a 40-clause circular does not open 40 simultaneous connections.
func TestLLMSemaphoreBoundsConcurrency(t *testing.T) {
	p := New(newFakeStore(), nil)
	ctx := context.Background()
	for i := 0; i < DefaultLLMConcurrency; i++ {
		if err := p.AcquireLLM(ctx); err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
	}
	// The next acquire must block; prove it by cancelling.
	tight, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if err := p.AcquireLLM(tight); err == nil {
		t.Fatal("acquired more slots than the concurrency cap allows")
	}
	p.ReleaseLLM()
	if err := p.AcquireLLM(ctx); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}
