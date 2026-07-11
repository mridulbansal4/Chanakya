// Command compile runs the Regulation Compiler over the clauses seeded in
// chanakya.db: extracts typed, cited obligations (schema-validated, embedded),
// wires the firm controls/evidence layer, detects gaps, and drafts remediation
// tickets. By default it uses the deterministic offline extractor; if
// CHANAKYA_LLM_API_KEY is set, it uses the real Anthropic extractor instead.
//
//	go run ./backend/cmd/compile
//
// Note: `go run ./backend/cmd/api` self-seeds AND compiles on first run (offline
// extractor), so this command is only needed to (re)compile explicitly or to
// use the real LLM extractor.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"chanakya/internal/bootstrap"
	"chanakya/internal/compiler"
	"chanakya/internal/config"
	"chanakya/internal/llm"
	"chanakya/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("compile: fatal: %v", err)
	}
}

func run() error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = st.Close() }()

	extractor, err := chooseExtractor()
	if err != nil {
		return err
	}

	res, err := bootstrap.Compile(ctx, st, extractor, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("compile: extractor = %s\n", res.Extractor)
	fmt.Printf("compile: created %d obligations (%d pending, %d needs_review), %d rejected\n",
		res.Created, res.Pending, res.NeedsReview, res.Rejected)
	fmt.Printf("compile: wired %d controls, %d evidence, %d obligation→control links\n",
		res.Controls, res.Evidence, res.Links)
	fmt.Printf("compile: evidence mapping — %d satisfied, %d gaps; drafted %d remediation tickets (state=draft)\n",
		res.Satisfied, res.Gaps, res.Tickets)
	return nil
}

// chooseExtractor picks the Anthropic extractor when a key is present, else the
// offline one.
func chooseExtractor() (llm.Extractor, error) {
	key := os.Getenv("CHANAKYA_LLM_API_KEY")
	if key == "" {
		return llm.NewOfflineExtractor(), nil
	}
	ex, err := llm.NewAnthropicExtractor(llm.AnthropicConfig{
		APIKey: key,
		Model:  os.Getenv("CHANAKYA_LLM_MODEL"),
		Schema: compiler.SchemaJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("build anthropic extractor: %w", err)
	}
	return ex, nil
}
