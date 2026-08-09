package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dgnsrekt/gexbot-downloader/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestPanicHandlerRecordedInMetricsAndLog covers the PR #29 review note: a
// handler panic unwinds past zapLoggerMiddleware to the outer Recoverer. The
// instrumentation must still count and log the resulting 5xx (it runs in a defer
// and re-panics so Recoverer still writes the 500).
func TestPanicHandlerRecordedInMetricsAndLog(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	before := testutil.ToFloat64(observability.HTTPRequests.WithLabelValues("GET", "/panic", "5xx"))

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)        // outer, as in production wiring
	r.Use(zapLoggerMiddleware(logger)) // inner: instrumentation under test
	r.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/panic")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (Recoverer must still handle the panic)", resp.StatusCode)
	}

	after := testutil.ToFloat64(observability.HTTPRequests.WithLabelValues("GET", "/panic", "5xx"))
	if after-before != 1 {
		t.Errorf("5xx request counter delta = %v, want 1", after-before)
	}

	found := false
	for _, e := range logs.FilterMessage("request completed").All() {
		for _, f := range e.Context {
			if f.Key == "status" && f.Integer == int64(http.StatusInternalServerError) {
				found = true
			}
		}
	}
	if !found {
		t.Error("panic request was not recorded in the access log with status 500")
	}
}
