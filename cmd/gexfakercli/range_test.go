package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// runCLI executes the root command with the given args, discarding output. It fails the test on
// any command error.
func runCLI(t *testing.T, args ...string) {
	t.Helper()
	c := rootCmd()
	c.SetArgs(args)
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	if err := c.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
}

// load posts the span, presents --token as Bearer on the mutating route, and polls the job to a
// terminal state.
func TestLoadWaitsAndSendsToken(t *testing.T) {
	var postAuth, statusHits, postBody = "", 0, ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/load":
			postAuth = r.Header.Get("Authorization")
			b, _ := io.ReadAll(r.Body)
			postBody = string(b)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"range-1","state":"queued","total":2,"done":0}`))
		case r.URL.Path == "/load/status/range-1":
			statusHits++
			_, _ = w.Write([]byte(`{"job_id":"range-1","state":"done","total":2,"done":2,"loaded_range":{"from":"2026-08-06","to":"2026-08-07"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	runCLI(t, "--url", srv.URL, "--token", "tok", "--quiet", "load", "--dates", "2026-08-06,2026-08-07")

	if postAuth != "Bearer tok" {
		t.Errorf("load Authorization = %q, want %q", postAuth, "Bearer tok")
	}
	if statusHits == 0 {
		t.Error("load did not poll the job status")
	}
	if postBody == "" || postBody[0] != '{' {
		t.Errorf("load body = %q, want a JSON object with dates", postBody)
	}
}

// --no-wait returns the accepted job without polling.
func TestLoadNoWait(t *testing.T) {
	statusHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"range-9","state":"queued"}`))
			return
		}
		statusHits++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	runCLI(t, "--url", srv.URL, "--quiet", "load", "--from", "2026-08-06", "--to", "2026-08-07", "--no-wait")
	if statusHits != 0 {
		t.Errorf("--no-wait polled the status %d time(s), want 0", statusHits)
	}
}

// A positional single date loads via the same async job.
func TestLoadSingleDate(t *testing.T) {
	var postBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/load":
			b, _ := io.ReadAll(r.Body)
			postBody = string(b)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"j1","state":"queued","total":1}`))
		case r.URL.Path == "/load/status/j1":
			_, _ = w.Write([]byte(`{"job_id":"j1","state":"done","total":1,"done":1}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	runCLI(t, "--url", srv.URL, "--quiet", "load", "2026-08-07")
	if !strings.Contains(postBody, `"date":"2026-08-07"`) {
		t.Errorf("load body = %q, want a date field", postBody)
	}
}

// current-load and coverage are read-only GETs; coverage passes from/to as query params.
func TestCurrentLoadAndCoverage(t *testing.T) {
	var coverageQuery string
	var currentHit, coverageHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/current-load":
			currentHit = true
			if r.Header.Get("Authorization") != "" {
				t.Error("current-load should not send an Authorization header (open route)")
			}
			_, _ = w.Write([]byte(`{"dates":["2026-08-06"],"from":"2026-08-06","to":"2026-08-06"}`))
		case "/coverage":
			coverageHit = true
			coverageQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"days":[],"union":[],"intersection":[]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	runCLI(t, "--url", srv.URL, "--quiet", "current-load")
	runCLI(t, "--url", srv.URL, "--quiet", "coverage", "--from", "2026-08-06", "--to", "2026-08-10")

	if !currentHit {
		t.Error("current-load did not hit /current-load")
	}
	if !coverageHit {
		t.Error("coverage did not hit /coverage")
	}
	if coverageQuery != "from=2026-08-06&to=2026-08-10" {
		t.Errorf("coverage query = %q, want from=2026-08-06&to=2026-08-10", coverageQuery)
	}
}

// A job that ends in the error state must exit nonzero (not emit + exit 0).
func TestLoadErrorStateFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"range-1","state":"queued","total":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"job_id":"range-1","state":"error","error":"no archived dates in range"}`))
	}))
	defer srv.Close()

	c := rootCmd()
	c.SetArgs([]string{"--url", srv.URL, "--quiet", "load", "--dates", "2026-08-06"})
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	if err := c.Execute(); err == nil {
		t.Fatal("load whose job ends in error should exit nonzero")
	}
}

// A non-positive --timeout is rejected up front, before any request is made.
func TestLoadRejectsNonPositiveTimeout(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := rootCmd()
	c.SetArgs([]string{"--url", srv.URL, "--quiet", "load", "--dates", "2026-08-06", "--timeout", "0"})
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	if err := c.Execute(); err == nil {
		t.Fatal("--timeout 0 should error")
	}
	if hits != 0 {
		t.Errorf("made %d request(s) with an invalid --timeout, want 0 (validated before the POST)", hits)
	}
}

// setup's internal load/rewind steps must present the control token, so `setup --token` works
// against a token-gated faker (both /load and /reset are gated on the server).
func TestSetupSendsControlToken(t *testing.T) {
	var loadAuth, resetAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			_, _ = w.Write([]byte(`{"status":"ok","data_date":"","cache_mode":"exhaust","data_mode":"stream"}`))
		case r.URL.Path == "/dates":
			_, _ = w.Write([]byte(`{"dates":["2026-08-10"]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/load":
			loadAuth = r.Header.Get("Authorization")
			if loadAuth == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"control route requires the Studio auth token"}`))
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"s1","state":"queued","total":1}`))
		case r.URL.Path == "/load/status/s1":
			_, _ = w.Write([]byte(`{"job_id":"s1","state":"done","total":1,"done":1}`))
		case r.URL.Path == "/current-load":
			_, _ = w.Write([]byte(`{"dates":["2026-08-10"],"from":"2026-08-10","to":"2026-08-10"}`))
		case r.URL.Path == "/tickers":
			_, _ = w.Write([]byte(`{"stocks":["QQQ"]}`))
		case strings.HasSuffix(r.URL.Path, "/classic/gex_zero"):
			_, _ = w.Write([]byte(`{"timestamp":1,"ticker":"QQQ"}`))
		case r.URL.Path == "/reset":
			resetAuth = r.Header.Get("Authorization")
			if resetAuth == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"control route requires the Studio auth token"}`))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	prevURL, prevKey, prevToken, prevQuiet := flagURL, flagKey, flagToken, flagQuiet
	flagURL, flagKey, flagToken, flagQuiet = srv.URL, "k", "tok", true
	t.Cleanup(func() { flagURL, flagKey, flagToken, flagQuiet = prevURL, prevKey, prevToken, prevQuiet })

	if err := runSetup(context.Background(), setupOpts{dataDir: t.TempDir(), date: "2026-08-10"}); err != nil {
		t.Fatalf("setup with --token against a gated faker failed: %v", err)
	}
	if loadAuth != "Bearer tok" {
		t.Errorf("/load Authorization = %q, want %q", loadAuth, "Bearer tok")
	}
	if resetAuth != "Bearer tok" {
		t.Errorf("/reset Authorization = %q, want %q", resetAuth, "Bearer tok")
	}
}

// Conflicting selectors (a positional date AND range flags) fail without hitting the network.
func TestLoadRejectsConflictingSelectors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("load with conflicting selectors should not make a request")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := rootCmd()
	c.SetArgs([]string{"--url", srv.URL, "--quiet", "load", "2026-08-07", "--from", "2026-08-06"})
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	if err := c.Execute(); err == nil {
		t.Fatal("a positional date together with --from should error")
	}
}

// A job that never reaches a terminal state must make loadAndWait (setup's loader) give up with an
// error, not hang forever — so setup never blocks silently.
func TestLoadAndWaitTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/load" {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"stuck","state":"queued","total":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"job_id":"stuck","state":"queued"}`)) // never terminal
	}))
	defer srv.Close()

	prevURL, prevKey := flagURL, flagKey
	flagURL, flagKey = srv.URL, "k"
	t.Cleanup(func() { flagURL, flagKey = prevURL, prevKey })

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	if _, err := loadAndWait(ctx, newClient(), map[string]any{"date": "2026-08-07"}); err == nil {
		t.Fatal("loadAndWait should error on a job that never reaches a terminal state")
	}
}

// A missing target (no date, no --from/--to, no --dates) fails without hitting the network.
func TestLoadRequiresTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("load with no target should not make a request")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := rootCmd()
	c.SetArgs([]string{"--url", srv.URL, "--quiet", "load"})
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	if err := c.Execute(); err == nil {
		t.Fatal("load with no date/--from/--to/--dates should error")
	}
}
