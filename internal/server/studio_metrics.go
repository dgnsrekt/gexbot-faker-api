package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// handleMetricsQuery proxies an instant PromQL query to Prometheus's
// /api/v1/query, so the Studio Monitoring screen can render metrics natively
// without the browser ever talking to Prometheus — the same server-side proxy
// model the Logs screen uses for Loki.
func (h *StudioHandlers) handleMetricsQuery(w http.ResponseWriter, r *http.Request) {
	h.proxyPrometheus(w, r, "/api/v1/query", "query", "time")
}

// handleMetricsRange proxies a range PromQL query to Prometheus's
// /api/v1/query_range for time-series panels.
func (h *StudioHandlers) handleMetricsRange(w http.ResponseWriter, r *http.Request) {
	h.proxyPrometheus(w, r, "/api/v1/query_range", "query", "start", "end", "step")
}

// proxyPrometheus forwards the given query params to a read-only Prometheus API
// path and streams back its JSON. It degrades with a JSON error (never a hang)
// when Prometheus isn't configured or is unreachable, mirroring the Loki proxy.
func (h *StudioHandlers) proxyPrometheus(w http.ResponseWriter, r *http.Request, path string, pass ...string) {
	w.Header().Set("Content-Type", "application/json")
	base := strings.TrimRight(h.server.config.PrometheusURL, "/")
	if base == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Metrics need Prometheus (PROMETHEUS_URL is unset). Start the observability stack.",
		})
		return
	}
	if r.URL.Query().Get("query") == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "query is required"})
		return
	}

	q := url.Values{}
	for _, k := range pass {
		if v := r.URL.Query().Get(k); v != "" {
			q.Set(k, v)
		}
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, base+path+"?"+q.Encode(), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "failed to build query"})
		return
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		h.server.logger.Warn("studio metrics: Prometheus unreachable", zap.Error(err))
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Prometheus is unreachable. Is the observability stack running?",
		})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
