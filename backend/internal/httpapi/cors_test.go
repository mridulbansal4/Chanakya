package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOriginChecker_Allowlist proves the configured allowlist is actually
// consulted. Before Phase 1 the predicate unconditionally returned true, so
// Options.CORSOrigins was decorative - this test is the guard against that
// regressing.
func TestOriginChecker_Allowlist(t *testing.T) {
	check := originChecker([]string{"http://localhost:3000", " https://app.example.com "})

	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:3000", true},
		{"https://app.example.com", true},  // surrounding whitespace is trimmed at config time
		{"http://evil.example.com", false}, // not on the list
		{"http://localhost:3001", false},   // different port is a different origin
		{"https://localhost:3000", false},  // different scheme is a different origin
		{"http://localhost:3000/", false},  // no implicit trailing-slash stripping
		{"http://LOCALHOST:3000", false},   // no implicit case folding
		{"", false},                        // no Origin header
	}
	for _, tc := range cases {
		if got := check(nil, tc.origin); got != tc.want {
			t.Errorf("originChecker(%q) = %v, want %v", tc.origin, got, tc.want)
		}
	}
}

// TestOriginChecker_EmptyAllowlistIsPermissive documents the local-dev fallback:
// with nothing configured we preserve the previous permissive behaviour (and log
// a warning once at construction, not per request).
func TestOriginChecker_EmptyAllowlistIsPermissive(t *testing.T) {
	check := originChecker(nil)
	if !check(nil, "http://anything.example") {
		t.Fatal("empty allowlist must stay permissive for local dev")
	}
}

// TestRouterCORSRejectsUnlistedOrigin exercises the predicate through the real
// chi/cors middleware: an allowed origin gets Access-Control-Allow-Origin back,
// a non-allowlisted one does not.
func TestRouterCORSRejectsUnlistedOrigin(t *testing.T) {
	r := NewRouter(Options{CORSOrigins: []string{"http://localhost:3000"}, Version: "test"})

	for _, tc := range []struct {
		origin    string
		wantAllow string
	}{
		{"http://localhost:3000", "http://localhost:3000"},
		{"http://evil.example.com", ""},
	} {
		req := httptest.NewRequest(http.MethodOptions, "/api/obligations", nil)
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.wantAllow {
			t.Errorf("origin %q: Access-Control-Allow-Origin = %q, want %q", tc.origin, got, tc.wantAllow)
		}
	}
}
