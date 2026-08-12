package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
)

func daemonHandler(daemonURL string) *StudioHandlers {
	return &StudioHandlers{
		server: &Server{config: &config.ServerConfig{DaemonURL: daemonURL}, logger: zap.NewNop()},
	}
}

func callDaemon(h *StudioHandlers) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/studio/api/daemon", nil)
	rec := httptest.NewRecorder()
	h.handleDaemon(rec, req)
	return rec
}

// The proxy streams the daemon's status through on success.
func TestStudioDaemonProxyOK(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Errorf("proxied to %q, want /status", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ready":true,"config_path":"/app/configs/custom.yaml"}`))
	}))
	defer up.Close()

	rec := callDaemon(daemonHandler(up.URL))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"config_path":"/app/configs/custom.yaml"`) {
		t.Errorf("body did not pass through: %s", rec.Body.String())
	}
}

// Unset DAEMON_URL degrades with 503 (not a hard failure).
func TestStudioDaemonProxyUnset(t *testing.T) {
	rec := callDaemon(daemonHandler(""))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unset DAEMON_URL → %d, want 503", rec.Code)
	}
}

// An unreachable daemon degrades with 502.
func TestStudioDaemonProxyUnreachable(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := up.URL
	up.Close() // now refusing connections

	rec := callDaemon(daemonHandler(url))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("unreachable daemon → %d, want 502", rec.Code)
	}
}
