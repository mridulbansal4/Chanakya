// Command api is the CHANAKYA HTTP server: it loads configuration, opens the
// SQLite system-of-record (applying embedded migrations in-process), and serves
// the REST API with graceful shutdown. No Docker, no external database.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chanakya/internal/bootstrap"
	"chanakya/internal/config"
	"chanakya/internal/feed"
	"chanakya/internal/httpapi"
	"chanakya/internal/signoff"
	"chanakya/internal/store"
)

// version is the build version, overridable at link time with -ldflags.
var version = "0.0.0-dev"

func main() {
	if err := run(); err != nil {
		log.Fatalf("chanakya: fatal: %v", err)
	}
}

// run performs startup, blocks serving requests, and returns on shutdown.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() {
		if cerr := st.Close(); cerr != nil {
			log.Printf("chanakya: closing store: %v", cerr)
		}
	}()
	log.Printf("chanakya: sqlite ready at %s (WAL, foreign_keys on)", cfg.DBPath)

	// Self-bootstrap: on an empty DB, seed the IA fixture and compile it
	// (offline extractor) so the two run commands (api + web) give a fully
	// working, seeded app with no manual seed step.
	if seeded, err := bootstrap.EnsureSeeded(ctx, st); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	} else if seeded {
		log.Printf("chanakya: bootstrapped demo data (IA fixture seeded + compiled)")
	}

	priv, err := signoff.LoadOrCreateKey(cfg.SigningKeyPath)
	if err != nil {
		return fmt.Errorf("load signing key: %w", err)
	}
	signer := signoff.NewSigner(priv)
	log.Printf("chanakya: sign-off key ready (pub %s…)", signer.PublicKeyB64()[:12])

	feedValidator, err := feed.NewValidator()
	if err != nil {
		return fmt.Errorf("compile feed schema: %w", err)
	}

	handler := httpapi.NewRouter(httpapi.Options{
		Store:         st,
		Signer:        signer,
		FeedValidator: feedValidator,
		CORSOrigins:   cfg.CORSOrigins,
		Version:       version,
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("chanakya: api listening on %s (version %s)", cfg.Addr, version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("listen: %w", err)
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
		log.Printf("chanakya: shutdown signal received")
	case err := <-serveErr:
		if err != nil {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Printf("chanakya: stopped cleanly")
	return nil
}
