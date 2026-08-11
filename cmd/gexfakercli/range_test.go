package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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

// load-range posts the span, presents --token as Bearer on the mutating route, and polls the job to
// a terminal state.
func TestLoadRangeWaitsAndSendsToken(t *testing.T) {
	var postAuth, statusHits, postBody = "", 0, ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/load-range":
			postAuth = r.Header.Get("Authorization")
			b, _ := io.ReadAll(r.Body)
			postBody = string(b)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"range-1","state":"queued","total":2,"done":0}`))
		case r.URL.Path == "/load-range/status/range-1":
			statusHits++
			_, _ = w.Write([]byte(`{"job_id":"range-1","state":"done","total":2,"done":2,"loaded_range":{"from":"2026-08-06","to":"2026-08-07"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	runCLI(t, "--url", srv.URL, "--token", "tok", "--quiet", "load-range", "--dates", "2026-08-06,2026-08-07")

	if postAuth != "Bearer tok" {
		t.Errorf("load-range Authorization = %q, want %q", postAuth, "Bearer tok")
	}
	if statusHits == 0 {
		t.Error("load-range did not poll the job status")
	}
	if postBody == "" || postBody[0] != '{' {
		t.Errorf("load-range body = %q, want a JSON object with dates", postBody)
	}
}

// --no-wait returns the accepted job without polling.
func TestLoadRangeNoWait(t *testing.T) {
	statusHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load-range" {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"range-9","state":"queued"}`))
			return
		}
		statusHits++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	runCLI(t, "--url", srv.URL, "--quiet", "load-range", "--from", "2026-08-06", "--to", "2026-08-07", "--no-wait")
	if statusHits != 0 {
		t.Errorf("--no-wait polled the status %d time(s), want 0", statusHits)
	}
}

// current-range and coverage are read-only GETs; coverage passes from/to as query params.
func TestCurrentRangeAndCoverage(t *testing.T) {
	var coverageQuery string
	var currentHit, coverageHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/current-range":
			currentHit = true
			if r.Header.Get("Authorization") != "" {
				t.Error("current-range should not send an Authorization header (open route)")
			}
			_, _ = w.Write([]byte(`{"dates":["2026-08-06"],"from":"2026-08-06","to":"2026-08-06"}`))
		case "/range-coverage":
			coverageHit = true
			coverageQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"days":[],"union":[],"intersection":[]}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	runCLI(t, "--url", srv.URL, "--quiet", "current-range")
	runCLI(t, "--url", srv.URL, "--quiet", "coverage", "--from", "2026-08-06", "--to", "2026-08-10")

	if !currentHit {
		t.Error("current-range did not hit /current-range")
	}
	if !coverageHit {
		t.Error("coverage did not hit /range-coverage")
	}
	if coverageQuery != "from=2026-08-06&to=2026-08-10" {
		t.Errorf("coverage query = %q, want from=2026-08-06&to=2026-08-10", coverageQuery)
	}
}

// A missing span (no --from/--to and no --dates) fails without hitting the network.
func TestLoadRangeRequiresSpan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("load-range with no span should not make a request")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := rootCmd()
	c.SetArgs([]string{"--url", srv.URL, "--quiet", "load-range"})
	c.SetOut(io.Discard)
	c.SetErr(io.Discard)
	if err := c.Execute(); err == nil {
		t.Fatal("load-range with no --from/--to/--dates should error")
	}
}
