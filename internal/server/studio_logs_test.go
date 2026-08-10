package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseLogLine(t *testing.T) {
	// Structured zap line.
	z := parseLogLine(`{"level":"warn","ts":1786312160.5,"caller":"x","msg":"disk full"}`, "gex-daemon", 1786312160500000000)
	if z.Level != "warn" || z.Msg != "disk full" || z.Service != "gex-daemon" || z.Time == "" {
		t.Errorf("zap parse wrong: %+v", z)
	}
	// Non-JSON line falls back to info + raw text, timestamp from Loki ns.
	r := parseLogLine("panic: boom", "gex-faker-api", 1786312160500000000)
	if r.Level != "info" || r.Msg != "panic: boom" || r.Time == "" {
		t.Errorf("raw parse wrong: %+v", r)
	}
}

func TestLokiQueryRange(t *testing.T) {
	loki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "query=") || !strings.Contains(r.URL.Path, "query_range") {
			t.Errorf("unexpected loki request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		// Two streams; values must be merged into one ascending timeline.
		_, _ = w.Write([]byte(`{"data":{"result":[
			{"stream":{"service":"gex-faker-api"},"values":[["200","{\"level\":\"info\",\"ts\":1,\"msg\":\"b\"}"]]},
			{"stream":{"service":"gex-daemon"},"values":[["100","{\"level\":\"error\",\"ts\":1,\"msg\":\"a\"}"]]}
		]}}`))
	}))
	defer loki.Close()

	c := &lokiClient{base: loki.URL, hc: loki.Client()}
	lines, maxNs, err := c.queryRange(context.Background(), 0, 1000, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// Ascending by ns: the daemon "a" (ns 100) before the api "b" (ns 200).
	if lines[0].Msg != "a" || lines[1].Msg != "b" {
		t.Errorf("lines not merged ascending: %+v", lines)
	}
	if maxNs != 200 {
		t.Errorf("maxNs = %d, want 200", maxNs)
	}
}

func TestHandleLogsWithoutLoki(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir()) // config.LokiURL is empty
	req := httptest.NewRequest(http.MethodGet, "/studio/api/logs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "data:") || !strings.Contains(body, "error") || !strings.Contains(body, "Loki") {
		t.Errorf("expected an SSE error event mentioning Loki, got: %q", body)
	}
}
