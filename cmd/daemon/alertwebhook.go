package main

import (
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/notify"
)

// alertmanagerPayload is the subset of Alertmanager's v4 webhook body we use.
type alertmanagerPayload struct {
	Alerts []struct {
		Status      string            `json:"status"` // "firing" | "resolved"
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	} `json:"alerts"`
}

// ntfyPriorityForSeverity maps an alert's severity label to an ntfy priority.
func ntfyPriorityForSeverity(sev string) string {
	switch sev {
	case "critical":
		return "urgent"
	case "warning":
		return "high"
	default:
		return "default"
	}
}

// alertWebhookHandler bridges Alertmanager → ntfy: it decodes the webhook payload and
// forwards each alert through the daemon's existing notifier, so Prometheus alerts reach
// the same ntfy destination as the download/coverage notifications (one channel, not two).
// A NoopNotifier (NTFY disabled) makes this a safe no-op.
//
// Security: like /metrics and /status, this lives on the UNAUTHENTICATED daemon diagnostics
// mux (:9091). The default Compose stack does not publish that port, so it stays on the
// internal network; if you run the daemon directly, do not expose :9091 (an open endpoint
// here could post arbitrary messages to the configured ntfy topic). See OBSERVABILITY.md.
func alertWebhookHandler(notifier notify.Notifier, logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload alertmanagerPayload
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
			logger.Warn("bad alertmanager webhook payload", zap.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		failed := 0
		for _, a := range payload.Alerts {
			name := a.Labels["alertname"]
			if name == "" {
				name = "alert"
			}
			msg := a.Annotations["summary"]
			if msg == "" {
				msg = a.Annotations["description"]
			}
			if msg == "" {
				msg = a.Status
			}
			title, priority := name, ntfyPriorityForSeverity(a.Labels["severity"])
			if a.Status == "resolved" {
				title, priority = "Resolved: "+name, "default"
			}
			if err := notifier.SendAlert(r.Context(), title, msg, priority); err != nil {
				failed++
				logger.Warn("failed to forward alert to ntfy", zap.String("alert", name), zap.Error(err))
			}
		}
		// Fail the webhook if any send failed so Alertmanager retries rather than
		// permanently losing the alert on a transient ntfy error. A retry may re-deliver
		// the already-succeeded alerts in the batch — acceptable versus losing one.
		if failed > 0 {
			http.Error(w, "some alerts failed to forward to ntfy", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
