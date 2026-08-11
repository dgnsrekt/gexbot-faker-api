package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// serveControl runs a path through controlAuthMiddleware(token) with the given Authorization
// header and returns the status code (200 = passed through to the handler).
func serveControl(token, path, authHeader string) int {
	h := controlAuthMiddleware(token)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestIsMutatingControlPath(t *testing.T) {
	mutating := []string{"/load-range", "/reload-date", "/reset-cache", "load-range"}
	for _, p := range mutating {
		if !isMutatingControlPath(p) {
			t.Errorf("isMutatingControlPath(%q) = false, want true", p)
		}
	}
	open := []string{
		"/current-range", "/range-coverage", "/available-dates", "/current-date",
		"/load-range/status/range-1", "/seek-to-timestamp", "/health", "/SPX/classic/gex_zero",
	}
	for _, p := range open {
		if isMutatingControlPath(p) {
			t.Errorf("isMutatingControlPath(%q) = true, want false", p)
		}
	}
}

func TestControlAuthMiddleware(t *testing.T) {
	// No token configured → mutating routes are open (local dev).
	if c := serveControl("", "/load-range", ""); c != http.StatusOK {
		t.Errorf("no token, /load-range → %d, want 200", c)
	}

	// Token set, no/incorrect credential → 401 on a mutating route.
	if c := serveControl("secret", "/load-range", ""); c != http.StatusUnauthorized {
		t.Errorf("token set, no auth, /load-range → %d, want 401", c)
	}
	if c := serveControl("secret", "/reload-date", "Bearer nope"); c != http.StatusUnauthorized {
		t.Errorf("token set, wrong bearer, /reload-date → %d, want 401", c)
	}

	// Token set, correct Bearer/Basic → passes.
	if c := serveControl("secret", "/load-range", "Bearer secret"); c != http.StatusOK {
		t.Errorf("token set, correct bearer, /load-range → %d, want 200", c)
	}
	if c := serveControl("secret", "/reset-cache", basic("anyuser", "secret")); c != http.StatusOK {
		t.Errorf("token set, correct basic, /reset-cache → %d, want 200", c)
	}

	// Token set, but a read-only / non-mutating route is never gated (even without a credential).
	for _, p := range []string{"/current-range", "/range-coverage", "/available-dates", "/load-range/status/range-1", "/seek-to-timestamp"} {
		if c := serveControl("secret", p, ""); c != http.StatusOK {
			t.Errorf("token set, open route %s → %d, want 200 (not gated)", p, c)
		}
	}
}
