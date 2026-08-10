package server

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serveWithAuth(token, authHeader string) int {
	h := studioAuthMiddleware(token)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/studio/api/status", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func basic(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestStudioAuthMiddleware(t *testing.T) {
	// No token configured → always open.
	if c := serveWithAuth("", ""); c != http.StatusOK {
		t.Errorf("open studio (no token) → %d, want 200", c)
	}

	// Token configured → unauthenticated is rejected with a Basic challenge.
	rec := httptest.NewRecorder()
	studioAuthMiddleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/studio/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no auth → %d, want 401", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("missing WWW-Authenticate challenge")
	}

	// Correct Basic (any username, password = token) and correct Bearer pass.
	if c := serveWithAuth("secret", basic("admin", "secret")); c != http.StatusOK {
		t.Errorf("correct basic → %d, want 200", c)
	}
	if c := serveWithAuth("secret", basic("anything", "secret")); c != http.StatusOK {
		t.Errorf("basic ignores username → %d, want 200", c)
	}
	if c := serveWithAuth("secret", "Bearer secret"); c != http.StatusOK {
		t.Errorf("correct bearer → %d, want 200", c)
	}

	// Wrong password / wrong token → 401.
	if c := serveWithAuth("secret", basic("admin", "nope")); c != http.StatusUnauthorized {
		t.Errorf("wrong basic → %d, want 401", c)
	}
	if c := serveWithAuth("secret", "Bearer nope"); c != http.StatusUnauthorized {
		t.Errorf("wrong bearer → %d, want 401", c)
	}
}
