package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type Diagnostics struct {
	server *http.Server
}

// DiagnosticsOption customizes the diagnostics mux at construction.
type DiagnosticsOption func(*http.ServeMux)

// WithStatus registers GET /status, JSON-encoding whatever provide() returns. The
// daemon uses it to expose sanitized effective config + runtime state; the API
// server leaves it off (its state is served by the main HTTP API). provide() must
// return only sanitized fields — never secrets (API keys, tokens, auth headers).
func WithStatus(provide func() any) DiagnosticsOption {
	return func(mux *http.ServeMux) {
		mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(provide())
		})
	}
}

// WithHandler registers an extra handler on the diagnostics mux (e.g. the daemon's
// Alertmanager→ntfy webhook bridge). The mux is internal-only (compose network), so these
// handlers are not exposed to the LAN.
func WithHandler(path string, h http.HandlerFunc) DiagnosticsOption {
	return func(mux *http.ServeMux) { mux.HandleFunc(path, h) }
}

func NewDiagnostics(addr string, ready func() bool, logger *zap.Logger, opts ...DiagnosticsOption) *Diagnostics {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/livez", statusHandler(func() bool { return true }))
	mux.HandleFunc("/readyz", statusHandler(ready))
	for _, opt := range opts {
		opt(mux)
	}
	return &Diagnostics{server: &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}}
}

func statusHandler(ok func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "ok"
		if !ok() {
			status = "unavailable"
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	}
}

func (d *Diagnostics) Start(logger *zap.Logger) {
	go func() {
		logger.Info("diagnostics server starting", zap.String("addr", d.server.Addr))
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("diagnostics server error", zap.Error(err))
		}
	}()
}

func (d *Diagnostics) Stop(ctx context.Context) error { return d.server.Shutdown(ctx) }
