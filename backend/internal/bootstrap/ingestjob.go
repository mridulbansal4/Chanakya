package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"

	"chanakya/internal/compiler"
	"chanakya/internal/domain"
	"chanakya/internal/ingest"
	"chanakya/internal/jobs"
	"chanakya/internal/llm"
	"chanakya/internal/store"
)

// JobKindIngest is the job kind the ingestion pipeline runs under.
const JobKindIngest = "ingest"

// IngestPayload is the job payload: the content address of an already-stored
// document plus the run it belongs to. The BYTES are not in the payload - they
// are in document_blob, addressed by sha256, so a job row stays small and the
// document has exactly one copy.
type IngestPayload struct {
	RunID    string `json:"run_id"`
	SHA256   string `json:"sha256"`
	Filename string `json:"filename"`
}

// compilerAdapter bridges *compiler.Compiler to ingest.ClauseCompiler, keeping
// the ingest package free of a dependency on the compiler package.
type compilerAdapter struct {
	c    *compiler.Compiler
	pool *jobs.Pool
}

func (a compilerAdapter) ExtractorName() string { return a.c.ExtractorName() }

func (a compilerAdapter) CompileClause(ctx context.Context, clause domain.Clause) (ingest.ClauseCompileResult, error) {
	// Extraction may call a real model. The pool's semaphore bounds how many of
	// those are in flight across every worker at once, so a 40-clause circular
	// does not open 40 simultaneous connections to a rate-limited API.
	if a.pool != nil {
		if err := a.pool.AcquireLLM(ctx); err != nil {
			return ingest.ClauseCompileResult{}, err
		}
		defer a.pool.ReleaseLLM()
	}
	res, err := a.c.CompileClause(ctx, clause)
	if err != nil {
		return ingest.ClauseCompileResult{}, err
	}
	out := ingest.ClauseCompileResult{Obligations: res.Obligations}
	for _, r := range res.Rejections {
		out.Rejections = append(out.Rejections, r.Reason)
	}
	return out, nil
}

// NewIngestHandler builds the job handler that runs the ingestion pipeline.
//
// It writes the PROPOSAL and nothing else. No circular, clause or obligation
// reaches the graph here - that happens only in store.ApproveIngestRun, called
// by a human.
func NewIngestHandler(st *store.Store, pool *jobs.Pool) (jobs.Handler, error) {
	extractor, err := llm.SelectExtractor(json.RawMessage(compiler.SchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("select extractor: %w", err)
	}
	comp, err := compiler.New(extractor, 0)
	if err != nil {
		return nil, fmt.Errorf("build compiler: %w", err)
	}
	completer := ingest.SelectMetaCompleter()

	return func(ctx context.Context, job store.Job, report func(jobs.Progress)) error {
		var payload IngestPayload
		if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
			return fmt.Errorf("decode ingest payload for job %q: %w", job.ID, err)
		}

		blob, err := st.GetDocumentBlob(ctx, payload.SHA256)
		if err != nil {
			return fmt.Errorf("load document %q: %w", payload.SHA256, err)
		}
		doc, err := ingest.Intake(blob.Bytes, blob.Filename)
		if err != nil {
			_ = st.FailIngestRun(ctx, payload.RunID, ingest.StageIntake, err.Error())
			return fmt.Errorf("intake %q: %w", blob.Filename, err)
		}

		if err := st.SetIngestRunStage(ctx, payload.RunID, store.RunRunning, ingest.StageIntake); err != nil {
			return fmt.Errorf("mark run %q running: %w", payload.RunID, err)
		}

		stageIndex := map[string]int{}
		for i, s := range ingest.Stages {
			stageIndex[s] = i + 1
		}

		proposal, stage, err := ingest.RunPipeline(ctx, doc, ingest.Options{
			Completer: completer,
			Compiler:  compilerAdapter{c: comp, pool: pool},
			// Stage 9 diffs an incoming document against the clauses already in
			// the graph for the circulars it claims to supersede, amend, or be a
			// new version of.
			Corpus: func(ctx context.Context, circularRefs []string) ([]ingest.ExistingClause, error) {
				return st.CorpusClausesFor(ctx, circularRefs)
			},
			Progress: func(stage string, done, total int, detail string) {
				report(jobs.Progress{
					Stage: stage, Done: done, Total: total, Detail: detail,
					Index: stageIndex[stage], Count: len(ingest.Stages),
				})
				_ = st.SetIngestRunStage(ctx, payload.RunID, store.RunRunning, stage)
			},
		})
		if err != nil {
			_ = st.FailIngestRun(ctx, payload.RunID, stage, err.Error())
			return fmt.Errorf("pipeline stage %q: %w", stage, err)
		}

		if err := st.SaveProposal(ctx, payload.RunID, proposal); err != nil {
			return fmt.Errorf("save proposal for run %q: %w", payload.RunID, err)
		}
		return nil
	}, nil
}
