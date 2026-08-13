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
// A NoopNotifier (NTFY disabled) makes this a safe no-op. Internal-only endpoint.
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
				logger.Warn("failed to forward alert to ntfy", zap.String("alert", name), zap.Error(err))
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
