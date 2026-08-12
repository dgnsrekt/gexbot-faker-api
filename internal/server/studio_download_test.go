package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/download"
)

func TestDownloadJobState(t *testing.T) {
	// All succeeded → done.
	if s := downloadJobState(&download.BatchResult{Total: 3, Success: 3}, nil); s != "done" {
		t.Errorf("all-success → %q, want done", s)
	}
	// A 404 is NotFound, not Failed — must be "partial", not green "done".
	if s := downloadJobState(&download.BatchResult{Total: 3, Success: 2, NotFound: 1}, nil); s != "partial" {
		t.Errorf("mixed success+not_found → %q, want partial", s)
	}
	// Hard failure → error.
	if s := downloadJobState(&download.BatchResult{Total: 3, Success: 2, Failed: 1}, errors.New("1 failed")); s != "error" {
		t.Errorf("with error → %q, want error", s)
	}
}

// Download options reflect the effective YAML — its tickers, its enabled packages
// (including orderflow), and its curated category SUBSET (not expanded to all).
func TestDownloadOptionsFromYAML(t *testing.T) {
	t.Setenv("GEXBOT_API_KEY", "testkey")
	yaml := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(yaml, []byte(`
tickers:
  - SPX
  - NDX
packages:
  state:
    enabled: true
    categories: [gex_zero, gamma_zero]
  classic:
    enabled: false
  orderflow:
    enabled: true
`), 0o600); err != nil {
		t.Fatal(err)
	}
	dl := newDownloadManager(t.TempDir(), yaml, zap.NewNop())
	if !dl.enabled() {
		t.Skip("downloader config unavailable in this environment")
	}
	o := dl.options()
	if !o.Enabled {
		t.Fatal("options should be enabled with a key + valid YAML")
	}
	if o.ConfigPath != yaml {
		t.Errorf("config_path = %q, want %q", o.ConfigPath, yaml)
	}
	if len(o.Tickers) != 2 || o.Tickers[0] != "SPX" || o.Tickers[1] != "NDX" {
		t.Errorf("tickers = %v, want [SPX NDX]", o.Tickers)
	}
	// state (curated subset) + orderflow enabled; classic disabled.
	names := map[string][]string{}
	for _, p := range o.Packages {
		names[p.Name] = p.Categories
	}
	if _, ok := names["classic"]; ok {
		t.Error("classic is disabled and must not appear")
	}
	if got := names["state"]; len(got) != 2 || got[0] != "gex_zero" || got[1] != "gamma_zero" {
		t.Errorf("state categories = %v, want the YAML subset [gex_zero gamma_zero] (not expanded)", got)
	}
	if _, ok := names["orderflow"]; !ok {
		t.Error("orderflow is enabled in YAML and must appear")
	}
}

// An invalid downloader YAML (bad ticker/category) disables Studio downloads instead
// of serving unconfigured coverage as authoritative — the same bar the daemon holds.
func TestDownloadDisabledOnInvalidYAML(t *testing.T) {
	t.Setenv("GEXBOT_API_KEY", "testkey") // key is valid; the COVERAGE is not
	yaml := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(yaml, []byte("tickers: [NOTATICKER]\npackages:\n  classic:\n    enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dl := newDownloadManager(t.TempDir(), yaml, zap.NewNop())
	if dl.enabled() {
		t.Error("an invalid-ticker YAML must leave Studio downloads disabled")
	}
	if o := dl.options(); o.Enabled || o.Message == "" {
		t.Errorf("disabled options should carry a message: %+v", o)
	}
}

// A download request's tickers/packages are IGNORED — coverage is the server's YAML.
// A bogus/traversal ticker in the body can't create an archive with that coverage.
func TestDownloadIgnoresClientCoverage(t *testing.T) {
	t.Setenv("GEXBOT_API_KEY", "testkey")
	yaml := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(yaml, []byte("tickers: [SPX]\npackages:\n  classic:\n    enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dl := newDownloadManager(t.TempDir(), yaml, zap.NewNop())
	if !dl.enabled() {
		t.Skip("downloader config unavailable in this environment")
	}
	// The worker isn't started for this assertion; we only check that the request
	// contract accepts date-only and doesn't reject/act on client coverage fields.
	c := dl.cfgForDownload()
	if len(c.Tickers) != 1 || c.Tickers[0] != "SPX" {
		t.Errorf("cfgForDownload tickers = %v, want the YAML's [SPX] regardless of any request", c.Tickers)
	}
	if !c.Packages.Classic.Enabled || c.Packages.State.Enabled {
		t.Errorf("cfgForDownload packages = %+v, want only classic (from YAML)", c.Packages)
	}
}

func TestStudioCalendar(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir())
	body := getStudio(t, h, "/studio/api/calendar?month=2026-08")
	var resp struct {
		Days []calendarDay `json:"days"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Days) != 31 {
		t.Fatalf("August has 31 days, got %d", len(resp.Days))
	}
	// 2026-08-08 is a Saturday → not a market day, no state.
	var sat, weekday *calendarDay
	for i := range resp.Days {
		switch resp.Days[i].Date {
		case "2026-08-08":
			sat = &resp.Days[i]
		case "2026-08-10": // Monday
			weekday = &resp.Days[i]
		}
	}
	if sat == nil || sat.MarketDay {
		t.Errorf("2026-08-08 should be a non-market Saturday: %+v", sat)
	}
	if weekday == nil || !weekday.MarketDay {
		t.Errorf("2026-08-10 should be a market Monday: %+v", weekday)
	}
	// No archives in the temp dir → market days are "missing".
	if weekday != nil && weekday.State != "missing" {
		t.Errorf("empty data dir → market day state %q, want missing", weekday.State)
	}
}

func TestStudioCalendarBadMonth(t *testing.T) {
	h := newStudioTestServer(t, t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/studio/api/calendar?month=nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad month → %d, want 400", rec.Code)
	}
}

func TestStudioDownloadDisabledWithoutKey(t *testing.T) {
	// The test server's config.Load finds no GEXBOT_API_KEY → downloads disabled.
	h := newStudioTestServer(t, t.TempDir())

	var opts struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(getStudio(t, h, "/studio/api/download/options"), &opts); err != nil {
		t.Fatal(err)
	}
	if opts.Enabled {
		t.Error("downloads should be disabled without GEXBOT_API_KEY")
	}

	// POST download → 400 (disabled).
	req := httptest.NewRequest(http.MethodPost, "/studio/api/download",
		strings.NewReader(`{"dates":["2026-08-10"],"tickers":["SPX"],"packages":["classic"]}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("download while disabled → %d, want 400", rec.Code)
	}
}
