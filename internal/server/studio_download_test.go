package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
)

func TestCleanSelection(t *testing.T) {
	valid := func(s string) bool { return s == "SPX" || s == "NDX" }
	// Dedupe.
	if got, ok := cleanSelection([]string{"SPX", "NDX", "SPX"}, valid); !ok || len(got) != 2 {
		t.Errorf("dedupe: got %v ok=%v, want [SPX NDX]", got, ok)
	}
	// Traversal / unknown → rejected.
	for _, bad := range [][]string{{"../evil"}, {"SPX", "BOGUS"}, {"SPX/../x"}} {
		if _, ok := cleanSelection(bad, valid); ok {
			t.Errorf("cleanSelection(%v) accepted, want rejected", bad)
		}
	}
}

func TestStudioDownloadRejectsBadSelection(t *testing.T) {
	t.Setenv("GEXBOT_API_KEY", "testkey") // enable downloads so validation is reached
	dl := newDownloadManager(t.TempDir(), zap.NewNop())
	if !dl.enabled() {
		t.Skip("downloader config unavailable in this environment")
	}
	h := &StudioHandlers{
		server: &Server{config: &config.ServerConfig{DataDir: t.TempDir()}, logger: zap.NewNop()},
		dl:     dl,
	}
	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/studio/api/download", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.handleDownload(rec, req)
		return rec.Code
	}
	// Traversal / unknown ticker and unknown package must be rejected (400) — no
	// job is created and nothing reaches the filesystem.
	for _, bad := range []string{
		`{"dates":["2026-08-10"],"tickers":["../evil"],"packages":["classic"]}`,
		`{"dates":["2026-08-10"],"tickers":["BOGUS"],"packages":["classic"]}`,
		`{"dates":["2026-08-10"],"tickers":["SPX"],"packages":["evil"]}`,
	} {
		if c := post(bad); c != http.StatusBadRequest {
			t.Errorf("download %s → %d, want 400", bad, c)
		}
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
