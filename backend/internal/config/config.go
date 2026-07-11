// Package config loads runtime configuration exclusively from environment
// variables (rule 4: no secrets in code). A local .env file, if present, is
// read as a convenience for development only; real deployments set real env
// vars. Nothing here has a hardcoded credential.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config is the fully-resolved runtime configuration for the API server.
type Config struct {
	// Addr is the TCP address the HTTP server listens on, e.g. ":8080".
	Addr string
	// DBPath is the path to the SQLite database file, created on first run.
	DBPath string
	// CORSOrigins is the list of allowed browser origins for the web app.
	CORSOrigins []string
	// SigningKeyPath is the Ed25519 seed file for sign-off (created on first use).
	SigningKeyPath string
}

// Load resolves configuration from the process environment, first attempting to
// hydrate os.Environ from a .env file in the working directory if one exists.
// It never returns an error for a missing .env; that file is optional.
func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	cfg := Config{
		Addr:           getenv("CHANAKYA_ADDR", ":8080"),
		DBPath:         getenv("CHANAKYA_DB_PATH", "./chanakya.db"),
		CORSOrigins:    splitAndTrim(getenv("CHANAKYA_CORS_ORIGINS", "http://localhost:3000")),
		SigningKeyPath: getenv("CHANAKYA_SIGNING_KEY_PATH", "./chanakya_signing.key"),
	}
	return cfg, nil
}

// getenv returns the environment value for key, or def when it is unset/empty.
func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// splitAndTrim splits a comma-separated list into trimmed, non-empty entries.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv reads KEY=VALUE lines from path into the environment, without
// overriding variables already set in the real environment. A missing file is
// not an error. Lines beginning with '#' and blank lines are ignored.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, val); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}
	return nil
}
