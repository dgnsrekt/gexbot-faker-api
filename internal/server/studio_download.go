package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/download"
	"github.com/dgnsrekt/gexbot-downloader/internal/downloadjob"
	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

// cleanSelection dedupes vals and returns ok=false if any value fails valid —
// used to reject unknown/traversal tickers and unknown packages before they
// reach any filesystem path.
func cleanSelection(vals []string, valid func(string) bool) ([]string, bool) {
	seen := map[string]bool{}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if !valid(v) {
			return nil, false
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out, true
}

// downloadJob is the status of a background download of one date.
type downloadJob struct {
	Date     string `json:"date"`
	State    string `json:"state"` // queued | running | done | error
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Success  int    `json:"success"`
	Skipped  int    `json:"skipped"`
	Failed   int    `json:"failed"`
	NotFound int    `json:"not_found"`
	Error    string `json:"error,omitempty"`
}

type downloadReq struct {
	date     string
	tickers  []string
	packages []string
}

// downloadManager fetches market days from the GEXbot API in the background so
// the Studio never blocks on the network. Work is serialized (a single worker
// processes one date at a time) so many queued dates don't hammer the upstream
// API in parallel. Disabled (enabled=false) when no GEXBOT_API_KEY is configured.
type downloadManager struct {
	mu      sync.Mutex
	jobs    map[string]*downloadJob
	queue   chan downloadReq
	baseCfg *config.Config // nil when downloads are disabled (no API key)
	dataDir string
	logger  *zap.Logger
}

func newDownloadManager(dataDir string, logger *zap.Logger) *downloadManager {
	m := &downloadManager{
		jobs:    map[string]*downloadJob{},
		queue:   make(chan downloadReq, 512),
		dataDir: dataDir,
		logger:  logger,
	}
	// Load the downloader config (API key from GEXBOT_API_KEY). Guarded: a missing
	// key leaves downloads disabled, and the UI degrades with a clear message.
	if cfg, err := config.Load(""); err == nil {
		cfg.Output.Directory = dataDir // land downloads where the server serves them
		m.baseCfg = cfg
		go m.worker()
	} else {
		logger.Info("studio downloads disabled (set GEXBOT_API_KEY to enable)", zap.Error(err))
	}
	return m
}

func (m *downloadManager) enabled() bool { return m.baseCfg != nil }

// downloadJobState maps a finished batch to a UI state. A 404 is NotFound (the
// requested feed doesn't exist upstream), not Failed — the available data is
// still packed, but the job is marked "partial" (not green "done") so an
// incomplete archive never looks complete. Hard failures (err) are "error".
func downloadJobState(res *download.BatchResult, err error) string {
	if err != nil {
		return "error"
	}
	if res != nil && res.NotFound > 0 {
		return "partial"
	}
	return "done"
}

// cfgFor clones the base config with the request's ticker/package selection.
func (m *downloadManager) cfgFor(tickers, packages []string) *config.Config {
	c := *m.baseCfg // shallow copy; nested API/Download/Output are value structs
	c.Tickers = tickers
	c.Output.Directory = m.dataDir
	c.Packages = config.PackagesConfig{}
	for _, p := range packages {
		switch p {
		case "state":
			c.Packages.State.Enabled = true
		case "classic":
			c.Packages.Classic.Enabled = true
		case "orderflow":
			c.Packages.Orderflow.Enabled = true
		}
	}
	return &c
}

func (m *downloadManager) worker() {
	for req := range m.queue {
		m.mu.Lock()
		job := m.jobs[req.date]
		if job == nil || job.State != "queued" {
			m.mu.Unlock()
			continue
		}
		job.State = "running"
		m.mu.Unlock()

		cfg := m.cfgFor(req.tickers, req.packages)
		var res *download.BatchResult
		// Hold the cross-process per-date lock across download→pack so a concurrent
		// daemon run for the same date can't corrupt shared staging/tmp/archive
		// files. Only pack a complete batch: a partial failure must not publish an
		// incomplete archive that looks successful.
		err := downloadjob.WithDateLock(m.dataDir, req.date, func() error {
			var e error
			res, e = downloadjob.ExecuteDownload(context.Background(), cfg, req.date, m.logger, func(done, total int) {
				m.mu.Lock()
				job.Done, job.Total = done, total
				m.mu.Unlock()
			})
			if e != nil {
				return e
			}
			if res != nil && res.Failed > 0 {
				return fmt.Errorf("%d of %d files failed", res.Failed, res.Total)
			}
			// Pack the archive and mark each ticker materialized: the download's
			// auto-convert already left the JSONL on disk, so the date is "ready"
			// immediately (no redundant re-unpack). Shared with the daemon so the
			// two individual-download paths can't drift.
			return downloadjob.PackAndMark(cfg, req.date)
		})

		m.mu.Lock()
		if res != nil {
			job.Success, job.Skipped, job.Failed, job.NotFound = res.Success, res.Skipped, res.Failed, res.NotFound
		}
		job.State = downloadJobState(res, err)
		if err != nil {
			job.Error = err.Error()
			m.logger.Warn("studio download failed", zap.String("date", req.date), zap.Error(err))
		}
		m.mu.Unlock()
	}
}

// enqueue starts (or joins) a background download for date with the given
// selection. Returns a copy of the job.
func (m *downloadManager) enqueue(date string, tickers, packages []string) downloadJob {
	m.mu.Lock()
	if j, ok := m.jobs[date]; ok && (j.State == "queued" || j.State == "running") {
		out := *j
		m.mu.Unlock()
		return out
	}
	job := &downloadJob{Date: date, State: "queued"}
	m.jobs[date] = job
	out := *job
	m.mu.Unlock()
	m.queue <- downloadReq{date: date, tickers: tickers, packages: packages}
	return out
}

func (m *downloadManager) all() []downloadJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]downloadJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, *j)
	}
	return out
}

// --- handlers ---

// handleDownloadOptions reports whether downloads are enabled and the ticker
// universe + packages the UI offers for selection.
func (h *StudioHandlers) handleDownloadOptions(w http.ResponseWriter, _ *http.Request) {
	type pkg struct {
		Name       string   `json:"name"`
		Categories []string `json:"categories"`
	}
	writeStudioJSON(w, map[string]any{
		"enabled": h.dl.enabled(),
		"tickers": config.DefaultTickers(),
		"packages": []pkg{
			{Name: "state", Categories: config.ValidCategories[config.PackageState]},
			{Name: "classic", Categories: config.ValidCategories[config.PackageClassic]},
			{Name: "orderflow", Categories: config.ValidCategories[config.PackageOrderflow]},
		},
	})
}

// handleDownload enqueues background downloads. Body:
// {"dates":["YYYY-MM-DD",...], "tickers":[...], "packages":["state",...]}.
func (h *StudioHandlers) handleDownload(w http.ResponseWriter, r *http.Request) {
	if !h.dl.enabled() {
		http.Error(w, `{"error":"downloads are disabled — set GEXBOT_API_KEY on the server"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Dates    []string `json:"dates"`
		Tickers  []string `json:"tickers"`
		Packages []string `json:"packages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Dates) == 0 {
		http.Error(w, `{"error":"dates is required"}`, http.StatusBadRequest)
		return
	}
	// Validate + dedupe the caller-controlled dimensions: unknown/traversal
	// tickers would otherwise flow into filepath.Join(base, date, ticker, ...) and
	// let a request write outside the intended directory.
	tickers, ok := cleanSelection(body.Tickers, func(t string) bool { return config.ValidTickers[t] })
	if !ok || len(tickers) == 0 {
		http.Error(w, `{"error":"unknown or empty ticker selection"}`, http.StatusBadRequest)
		return
	}
	packages, ok := cleanSelection(body.Packages, func(p string) bool {
		return p == "state" || p == "classic" || p == "orderflow"
	})
	if !ok || len(packages) == 0 {
		http.Error(w, `{"error":"unknown or empty package selection"}`, http.StatusBadRequest)
		return
	}
	jobs := []downloadJob{}
	for _, d := range body.Dates {
		if !studioDateRe.MatchString(d) || !isMarketDayStr(d) {
			continue // skip malformed / non-market days
		}
		jobs = append(jobs, h.dl.enqueue(d, tickers, packages))
	}
	if len(jobs) == 0 {
		http.Error(w, `{"error":"no valid market days in request"}`, http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	writeStudioJSON(w, jobs)
}

func (h *StudioHandlers) handleDownloadStatus(w http.ResponseWriter, _ *http.Request) {
	writeStudioJSON(w, h.dl.all())
}

// calendarDay is one cell of the Download screen's month grid.
type calendarDay struct {
	Date      string `json:"date"`
	Day       int    `json:"day"`
	Weekday   int    `json:"weekday"` // 0=Sun..6=Sat
	MarketDay bool   `json:"market_day"`
	Holiday   bool   `json:"holiday"`
	State     string `json:"state"` // loaded|ready|archived|missing|"" (non-market)
}

// handleCalendar returns the days of ?month=YYYY-MM with their market-day and
// on-disk status, for the Download calendar.
func (h *StudioHandlers) handleCalendar(w http.ResponseWriter, r *http.Request) {
	monthStr := r.URL.Query().Get("month")
	first, err := time.Parse("2006-01", monthStr)
	if err != nil {
		http.Error(w, `{"error":"month must be YYYY-MM"}`, http.StatusBadRequest)
		return
	}

	// Archive state per date (materialized/total) + currently-loaded date.
	archived := map[string]eod.ArchiveInfo{}
	if list, err := eod.ListArchives(h.server.config.DataDir); err == nil {
		for _, a := range list {
			archived[a.Date] = a
		}
	}
	loaded := h.currentDate()

	days := []calendarDay{}
	for d := first; d.Month() == first.Month(); d = d.AddDate(0, 0, 1) {
		iso := d.Format("2006-01-02")
		market := isMarketDay(d)
		state := ""
		if a, ok := archived[iso]; ok {
			switch {
			case iso == loaded:
				state = "loaded"
			case len(a.Tickers) > 0 && a.Materialized == len(a.Tickers):
				state = "ready"
			default:
				state = "archived"
			}
		} else if market {
			state = "missing"
		}
		days = append(days, calendarDay{
			Date:      iso,
			Day:       d.Day(),
			Weekday:   int(d.Weekday()),
			MarketDay: market,
			Holiday:   !market && d.Weekday() != time.Saturday && d.Weekday() != time.Sunday,
			State:     state,
		})
	}
	writeStudioJSON(w, map[string]any{"month": monthStr, "days": days})
}

// isMarketDayStr parses a YYYY-MM-DD and reports whether it is a trading day.
func isMarketDayStr(s string) bool {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return false
	}
	return isMarketDay(d)
}
