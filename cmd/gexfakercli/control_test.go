package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A plain `reset` must scope to the active --key; `reset --all` clears everyone.
func TestResetKeyScope(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	run := func(args ...string) {
		c := rootCmd()
		c.SetArgs(args)
		c.SetOut(io.Discard)
		c.SetErr(io.Discard)
		if err := c.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
	}

	run("--url", srv.URL, "--key", "agent-a", "reset")
	if gotQuery != "key=agent-a" {
		t.Fatalf("default reset query = %q, want key=agent-a", gotQuery)
	}

	run("--url", srv.URL, "--key", "agent-a", "reset", "--all")
	if gotQuery != "" {
		t.Fatalf("--all reset query = %q, want empty (reset every key)", gotQuery)
	}
}

// setup must exit nonzero when its end-to-end verification pull fails, rather than
// reporting a ready faker.
func TestSetupFailsWhenVerifyFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			_, _ = w.Write([]byte(`{"status":"ok","data_date":"2026-08-10","cache_mode":"exhaust","data_mode":"stream"}`))
		case r.URL.Path == "/current-load":
			_, _ = w.Write([]byte(`{"dates":["2026-08-10"],"from":"2026-08-10","to":"2026-08-10"}`))
		case r.URL.Path == "/tickers":
			_, _ = w.Write([]byte(`{"stocks":["QQQ"]}`))
		case strings.HasSuffix(r.URL.Path, "/classic/gex_zero"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"No more data available"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	flagURL, flagKey = srv.URL, "k"
	prevQuiet := flagQuiet
	flagQuiet = true
	t.Cleanup(func() { flagQuiet = prevQuiet })

	if err := runSetup(context.Background(), setupOpts{dataDir: t.TempDir()}); err == nil {
		t.Fatal("expected setup to fail when the verification pull fails")
	}
}

// setup must also fail when the post-verify cursor rewind fails, so a reported
// ready state always leaves the playback key at index 0.
func TestSetupFailsWhenRewindFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			_, _ = w.Write([]byte(`{"status":"ok","data_date":"2026-08-10","cache_mode":"exhaust","data_mode":"stream"}`))
		case r.URL.Path == "/current-load":
			_, _ = w.Write([]byte(`{"dates":["2026-08-10"],"from":"2026-08-10","to":"2026-08-10"}`))
		case r.URL.Path == "/tickers":
			_, _ = w.Write([]byte(`{"stocks":["QQQ"]}`))
		case strings.HasSuffix(r.URL.Path, "/classic/gex_zero"):
			_, _ = w.Write([]byte(`{"timestamp":1,"ticker":"QQQ"}`)) // pull succeeds
		case r.URL.Path == "/reset":
			w.WriteHeader(http.StatusInternalServerError) // rewind fails
			_, _ = w.Write([]byte(`{"error":"reset failed"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	flagURL, flagKey = srv.URL, "k"
	prevQuiet := flagQuiet
	flagQuiet = true
	t.Cleanup(func() { flagQuiet = prevQuiet })

	if err := runSetup(context.Background(), setupOpts{dataDir: t.TempDir()}); err == nil {
		t.Fatal("expected setup to fail when the post-verify rewind fails")
	}
}
