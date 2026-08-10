package server

import (
	"context"
	"encoding/json"
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

// stubLoader implements data.DataLoader with a fixed set of loaded keys.
type stubLoader struct{ keys []string }

func (s stubLoader) GetAtIndex(context.Context, string, string, string, int) (*data.GexData, error) {
	return nil, nil
}
func (s stubLoader) GetRawAtIndex(context.Context, string, string, string, int) ([]byte, error) {
	return nil, nil
}
func (s stubLoader) GetLength(string, string, string) (int, error) { return 0, nil }
func (s stubLoader) Exists(string, string, string) bool            { return true }
func (s stubLoader) GetLoadedKeys() []string                       { return s.keys }
func (s stubLoader) FindIndexByTimestamp(context.Context, string, string, string, int64) (int, int64, error) {
	return 0, 0, nil
}
func (s stubLoader) Close() error { return nil }

func newStudioTestServer(t *testing.T, dataDir string) http.Handler {
	t.Helper()
	// Keep the download manager deterministically disabled (no upstream calls)
	// regardless of the developer's environment.
	t.Setenv("GEXBOT_API_KEY", "")
	cache := data.NewIndexCache(data.CacheModeExhaust)
	cache.SetIndex(data.CacheKey("SPX", "classic", "gex_zero", "supersecretkey"), 7)
	srv := &Server{
		loader:   stubLoader{keys: []string{"SPX/classic/gex_zero", "SPX/state/gex_full", "NDX/classic/gex_zero"}},
		cache:    cache,
		config:   &config.ServerConfig{DataDate: "2026-08-07", DataDir: dataDir, DataMode: "memory", CacheMode: "exhaust", EndpointCacheMode: "shared", WSGroupPrefix: "blue", WSStreamInterval: time.Second, WSEnabled: true, Port: "8080"},
		logger:   zap.NewNop(),
		loadedAt: time.Now(),
	}
	r := chi.NewRouter()
	RegisterStudioRoutes(r, srv, nil) // nil hubs → hubs endpoint returns []
	return r
}

func getStudio(t *testing.T, h http.Handler, path string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s → %d", path, rec.Code)
	}
	return rec.Body.Bytes()
}

func TestStudioStatus(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir())
	var st struct {
		Running     bool     `json:"running"`
		LoadedDate  string   `json:"loaded_date"`
		FilesLoaded int      `json:"files_loaded"`
		Tickers     []string `json:"tickers"`
		CacheMode   string   `json:"cache_mode"`
	}
	if err := json.Unmarshal(getStudio(t, h, "/studio/api/status"), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Running || st.LoadedDate != "2026-08-07" || st.FilesLoaded != 3 {
		t.Errorf("status = %+v", st)
	}
	if len(st.Tickers) != 2 { // SPX, NDX (distinct)
		t.Errorf("tickers = %v, want 2 distinct", st.Tickers)
	}
	if st.CacheMode != "exhaust" {
		t.Errorf("cache_mode = %q", st.CacheMode)
	}
}

func TestStudioConfigGroups(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir())
	var groups []settingGroup
	if err := json.Unmarshal(getStudio(t, h, "/studio/api/config"), &groups); err != nil {
		t.Fatal(err)
	}
	if len(groups) != 4 {
		t.Fatalf("expected 4 config groups, got %d", len(groups))
	}
	// Every row must carry an env var name.
	for _, g := range groups {
		for _, r := range g.Rows {
			if r.Env == "" {
				t.Errorf("group %q has a row with no env var: %+v", g.Title, r)
			}
		}
	}
}

func TestStudioHubsEmptyWhenNil(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir())
	body := getStudio(t, h, "/studio/api/hubs")
	var stats []hubStat
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatal(err)
	}
	if stats == nil || len(stats) != 0 {
		t.Errorf("hubs with nil WebSocketHubs should be [], got %s", body)
	}
}

func TestStudioKeysMasked(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir())
	var entries []keyEntry
	if err := json.Unmarshal(getStudio(t, h, "/studio/api/keys"), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 key, got %d", len(entries))
	}
	if entries[0].Key == "supersecretkey" || strings.Contains(entries[0].Key, "super") {
		t.Errorf("api key must not be exposed, got %q", entries[0].Key)
	}
	if !strings.HasPrefix(entries[0].Key, "client-") {
		t.Errorf("expected opaque client id, got %q", entries[0].Key)
	}
	if entries[0].Key != clientID("supersecretkey") {
		t.Errorf("client id not stable/deterministic: %q", entries[0].Key)
	}
	if len(entries[0].Streams) != 1 || entries[0].Streams[0].DataKey != "SPX/classic/gex_zero" {
		t.Errorf("stream wrong: %+v", entries[0].Streams)
	}
}

func TestStudioLibraryEmptyDir(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir())
	body := getStudio(t, h, "/studio/api/library")
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("library body not a JSON array: %s", body)
	}
	if len(rows) != 0 {
		t.Errorf("empty data dir should list no archives, got %d", len(rows))
	}
}

func TestStudioMaterializeRejectsBadDate(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir()) // temp dir has no archives
	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/studio/api/materialize", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	// Malformed / traversal shapes must be rejected before any path join.
	for _, bad := range []string{`{"date":"not-a-date"}`, `{"date":"2026-08-07/.."}`, `{"date":"../eod"}`, `{"date":""}`} {
		if code := post(bad); code != http.StatusBadRequest {
			t.Errorf("materialize %s → %d, want 400", bad, code)
		}
	}
	// Well-formed date but no archive on disk → 400 (not 202).
	if code := post(`{"date":"2026-08-07"}`); code != http.StatusBadRequest {
		t.Errorf("materialize valid-format-no-archive → %d, want 400", code)
	}
}

func TestStudioEndpointsNonEmpty(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir())
	var eps []endpointDoc
	if err := json.Unmarshal(getStudio(t, h, "/studio/api/endpoints"), &eps); err != nil {
		t.Fatal(err)
	}
	if len(eps) == 0 {
		t.Error("endpoint catalog should not be empty")
	}
}
