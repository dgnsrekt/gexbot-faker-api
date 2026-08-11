package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/data"
)

func studioRouterWithProm(t *testing.T, promURL string) http.Handler {
	t.Helper()
	t.Setenv("GEXBOT_API_KEY", "") // keep the download manager disabled
	srv := &Server{
		loader:   stubLoader{},
		cache:    data.NewIndexCache(data.CacheModeExhaust),
		config:   &config.ServerConfig{DataDir: t.TempDir(), PrometheusURL: promURL, WSStreamInterval: time.Second},
		logger:   zap.NewNop(),
		loadedAt: time.Now(),
	}
	r := chi.NewRouter()
	RegisterStudioRoutes(r, srv, nil)
	return r
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestMetricsDegradesWithoutPrometheus(t *testing.T) {
	h := studioRouterWithProm(t, "") // PROMETHEUS_URL unset
	for _, p := range []string{"/studio/api/metrics/alerts", "/studio/api/metrics/query?query=up", "/studio/api/metrics/range?query=up"} {
		if rec := get(t, h, p); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s without Prometheus → %d, want 503", p, rec.Code)
		}
	}
}

func TestMetricsQueryRequiresQuery(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("Prometheus must not be hit when query is missing")
	}))
	defer prom.Close()
	h := studioRouterWithProm(t, prom.URL)
	if rec := get(t, h, "/studio/api/metrics/query"); rec.Code != http.StatusBadRequest {
		t.Errorf("query without param → %d, want 400", rec.Code)
	}
}

func TestMetricsAlertsProxies(t *testing.T) {
	var gotPath string
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"alerts":[{"labels":{"alertname":"FakerTargetDown"},"state":"firing"}]}}`))
	}))
	defer prom.Close()

	rec := get(t, studioRouterWithProm(t, prom.URL), "/studio/api/metrics/alerts")
	if rec.Code != http.StatusOK {
		t.Fatalf("alerts → %d, want 200", rec.Code)
	}
	if gotPath != "/api/v1/alerts" {
		t.Errorf("proxied to %q, want /api/v1/alerts", gotPath)
	}
	if !strings.Contains(rec.Body.String(), "FakerTargetDown") {
		t.Errorf("alert body not proxied through: %s", rec.Body.String())
	}
}
