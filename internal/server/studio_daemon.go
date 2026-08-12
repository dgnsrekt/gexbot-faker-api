package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// handleDaemon proxies the daemon's sanitized status (GET :9091/status) to the
// Studio Settings screen — the same server-side proxy model as Prometheus/Loki, so
// the browser never talks to the daemon and port 9091 stays internal. Degrades
// (503/502) with a clear message when DAEMON_URL is unset or the daemon is down,
// keeping the rest of Studio usable.
func (h *StudioHandlers) handleDaemon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	base := strings.TrimRight(h.server.config.DaemonURL, "/")
	if base == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Daemon status needs DAEMON_URL (unset). It's set automatically in the compose stack.",
		})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, base+"/status", nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": "failed to build request"})
		return
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		h.server.logger.Warn("studio daemon: daemon unreachable", zap.Error(err))
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Daemon is unreachable. Is the gex-daemon container running?",
		})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
