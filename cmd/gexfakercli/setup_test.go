package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultURL(t *testing.T) {
	// Save and restore the env we mutate.
	for _, k := range []string{"GEXFAKER_URL", "HOST_BIND", "HOST_PORT"} {
		t.Setenv(k, "")
	}

	t.Run("explicit GEXFAKER_URL wins", func(t *testing.T) {
		t.Setenv("GEXFAKER_URL", "http://example:1234")
		if got := defaultURL(); got != "http://example:1234" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("falls back to HOST_BIND/HOST_PORT", func(t *testing.T) {
		t.Setenv("GEXFAKER_URL", "")
		t.Setenv("HOST_BIND", "10.0.0.5")
		t.Setenv("HOST_PORT", "9000")
		if got := defaultURL(); got != "http://10.0.0.5:9000" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("0.0.0.0 bind maps to loopback", func(t *testing.T) {
		t.Setenv("GEXFAKER_URL", "")
		t.Setenv("HOST_BIND", "0.0.0.0")
		t.Setenv("HOST_PORT", "")
		if got := defaultURL(); got != "http://127.0.0.1:8080" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestNewestArchive(t *testing.T) {
	dir := t.TempDir()
	eod := filepath.Join(dir, "eod")
	for _, name := range []string{"2026-07-17", "2026-08-03", "2026-08-10", "not-a-date", "staging.tmp"} {
		if err := os.MkdirAll(filepath.Join(eod, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := newestArchive(eod); got != "2026-08-10" {
		t.Fatalf("newestArchive = %q, want 2026-08-10", got)
	}

	if got := newestArchive(filepath.Join(dir, "missing")); got != "" {
		t.Fatalf("newestArchive(missing) = %q, want empty", got)
	}
}

// pickDate prefers the newest date from /dates over local archives.
func TestPickDateNewest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dates" {
			_, _ = w.Write([]byte(`{"dates":["2026-08-03","2026-08-10","2026-08-05"],"count":3}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	flagURL, flagKey = srv.URL, "k"
	got := pickDate(context.Background(), newClient(), t.TempDir())
	if got != "2026-08-10" {
		t.Fatalf("pickDate = %q, want 2026-08-10", got)
	}
}

func TestContains(t *testing.T) {
	if !contains(classicAggs, "gex_zero") {
		t.Fatal("expected gex_zero in classicAggs")
	}
	if contains(classicAggs, "bogus") {
		t.Fatal("bogus should not be a valid aggregation")
	}
}
