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
	if !requireQuery(w, r) {
		return
	}
	h.forwardPrometheus(w, r, "/api/v1/query", passthrough(r, "query", "time"))
}

// handleMetricsRange proxies a range PromQL query to Prometheus's
// /api/v1/query_range for time-series panels.
func (h *StudioHandlers) handleMetricsRange(w http.ResponseWriter, r *http.Request) {
	if !requireQuery(w, r) {
		return
	}
	h.forwardPrometheus(w, r, "/api/v1/query_range", passthrough(r, "query", "start", "end", "step"))
}

// handleMetricsAlerts proxies Prometheus's /api/v1/alerts so the Monitoring
// screen can show active (firing/pending) alerts from the rule set. No params.
func (h *StudioHandlers) handleMetricsAlerts(w http.ResponseWriter, r *http.Request) {
	h.forwardPrometheus(w, r, "/api/v1/alerts", url.Values{})
}

func requireQuery(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Query().Get("query") == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "query is required"})
		return false
	}
	return true
}

func passthrough(r *http.Request, keys ...string) url.Values {
	q := url.Values{}
	for _, k := range keys {
		if v := r.URL.Query().Get(k); v != "" {
			q.Set(k, v)
		}
	}
	return q
}

// forwardPrometheus GETs a read-only Prometheus API path with the given params
// and streams back its JSON. It degrades with a JSON error (never a hang) when
// Prometheus isn't configured or is unreachable, mirroring the Loki proxy.
func (h *StudioHandlers) forwardPrometheus(w http.ResponseWriter, r *http.Request, path string, q url.Values) {
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
	target := base + path
	if enc := q.Encode(); enc != "" {
		target += "?" + enc
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "failed to build request"})
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
