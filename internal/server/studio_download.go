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
	date string
}

// downloadManager fetches market days from the GEXbot API in the background so
// the Studio never blocks on the network. Work is serialized (a single worker
// processes one date at a time) so many queued dates don't hammer the upstream
// API in parallel. Disabled (enabled=false) when no GEXBOT_API_KEY is configured.
//
// Coverage (tickers/packages/categories) is the YAML's authority, not the caller's:
// the worker downloads exactly what baseCfg selects, so a manual Studio download
// covers the same set as the scheduled daemon and a modified browser request can't
// create an archive with unconfigured coverage.
type downloadManager struct {
	mu         sync.Mutex
	jobs       map[string]*downloadJob
	queue      chan downloadReq
	baseCfg    *config.Config // nil when downloads are disabled (no API key)
	configPath string         // the downloader YAML path (empty = working-dir discovery)
	dataDir    string
	logger     *zap.Logger
}

func newDownloadManager(dataDir, configPath string, logger *zap.Logger) *downloadManager {
	m := &downloadManager{
		jobs:       map[string]*downloadJob{},
		queue:      make(chan downloadReq, 512),
		configPath: configPath,
		dataDir:    dataDir,
		logger:     logger,
	}
	// Load the downloader config (GEXBOT_DOWNLOADER_CONFIG path + API key from
	// GEXBOT_API_KEY). Guarded: a missing key/invalid config leaves downloads
	// disabled, and the UI degrades with a clear message. An explicit path makes the
	// server load the SAME YAML the daemon does (shared DAEMON_CONFIG_PATH).
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Info("studio downloads disabled (set GEXBOT_API_KEY / GEXBOT_DOWNLOADER_CONFIG to enable)", zap.Error(err))
		return m
	}
	// Validate the effective coverage the SAME way the daemon does
	// (config.ValidateDownloadConfig): an invalid ticker/category YAML must disable
	// manual downloads rather than serve unconfigured coverage as authoritative (#67).
	if verr := config.ValidateDownloadConfig(config.EffectiveTickers(cfg), cfg.Packages); verr != nil {
		logger.Warn("studio downloads disabled (invalid downloader config)", zap.Error(verr))
		return m
	}
	cfg.Output.Directory = dataDir // land downloads where the server serves them
	m.baseCfg = cfg
	go m.worker()
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

// cfgForDownload clones the base config for a download, landing output where the
// server serves it. The YAML's ticker/package/category selection is preserved
// verbatim (curated category subsets are not expanded), so manual and scheduled
// downloads produce identical coverage.
func (m *downloadManager) cfgForDownload() *config.Config {
	c := *m.baseCfg // shallow copy; nested API/Download/Output are value structs
	c.Output.Directory = m.dataDir
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

		cfg := m.cfgForDownload()
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

// enqueue starts (or joins) a background download for date. Coverage comes from the
// YAML config (baseCfg), not the caller. Returns a copy of the job.
func (m *downloadManager) enqueue(date string) downloadJob {
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
	m.queue <- downloadReq{date: date}
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

// downloadPkg is an enabled package and its effective category list (the YAML
// subset, or all valid categories when the YAML leaves it empty).
type downloadPkg struct {
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
}

// downloadOptions is the effective, YAML-authoritative download coverage the
// Studio renders read-only (the user chooses only dates).
type downloadOptions struct {
	Enabled    bool          `json:"enabled"`
	ConfigPath string        `json:"config_path"`
	Tickers    []string      `json:"tickers"`
	Packages   []downloadPkg `json:"packages"`
	Message    string        `json:"message,omitempty"`
}

// displayConfigPath names the YAML that governs download coverage, for the UI.
func (m *downloadManager) displayConfigPath() string {
	if m.configPath != "" {
		return m.configPath
	}
	return "auto-discovered ./configs/default.yaml"
}

// options reports the effective download coverage from the loaded YAML — exactly
// what a download WOULD fetch (tickers fall back to DefaultTickers, empty package
// categories fall back to all valid categories, mirroring GenerateTasksForDate).
func (m *downloadManager) options() downloadOptions {
	if !m.enabled() {
		return downloadOptions{
			Enabled:    false,
			ConfigPath: m.displayConfigPath(),
			Message:    "downloads are disabled — set GEXBOT_API_KEY (and GEXBOT_DOWNLOADER_CONFIG for the shared daemon YAML) on the server",
		}
	}
	pkgs := []downloadPkg{}
	for _, p := range config.EffectivePackages(m.baseCfg) {
		pkgs = append(pkgs, downloadPkg{Name: p.Name, Categories: p.Categories})
	}
	return downloadOptions{
		Enabled:    true,
		ConfigPath: m.displayConfigPath(),
		Tickers:    config.EffectiveTickers(m.baseCfg),
		Packages:   pkgs,
	}
}

// --- handlers ---

// handleDownloadOptions reports the effective YAML-configured download coverage
// (read-only in the UI) and whether downloads are enabled.
func (h *StudioHandlers) handleDownloadOptions(w http.ResponseWriter, _ *http.Request) {
	writeStudioJSON(w, h.dl.options())
}

// handleDownload enqueues background downloads. Body: {"dates":["YYYY-MM-DD",...]}.
// Coverage (tickers/packages/categories) is the server's effective YAML, NOT the
// caller's — any tickers/packages in the body are ignored, so a modified request
// cannot create an archive with unconfigured coverage.
func (h *StudioHandlers) handleDownload(w http.ResponseWriter, r *http.Request) {
	if !h.dl.enabled() {
		http.Error(w, `{"error":"downloads are disabled — set GEXBOT_API_KEY on the server"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Dates []string `json:"dates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Dates) == 0 {
		http.Error(w, `{"error":"dates is required"}`, http.StatusBadRequest)
		return
	}
	jobs := []downloadJob{}
	for _, d := range body.Dates {
		if !studioDateRe.MatchString(d) || !isMarketDayStr(d) {
			continue // skip malformed / non-market days
		}
		jobs = append(jobs, h.dl.enqueue(d))
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
