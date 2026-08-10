package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
	"github.com/dgnsrekt/gexbot-downloader/internal/ws"
)

// studioDateRe validates the exact YYYY-MM-DD shape accepted by the materialize
// endpoint (no separators/traversal).
var studioDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// StudioHandlers backs the GEX Faker Studio UI's read-only control-plane
// endpoints under /studio/api. They aggregate state the Server already holds
// (config, loader, cache, reload manager) plus the WebSocket hubs, so the SPA can
// render its screens in a few calls. All are unauthenticated, matching the other
// open control endpoints (/current-date, /available-dates, ...).
type StudioHandlers struct {
	server *Server
	hubs   *WebSocketHubs
	mat    *materializeManager
	dl     *downloadManager
}

// RegisterStudioRoutes wires the /studio/api/* JSON endpoints onto r.
func RegisterStudioRoutes(r chi.Router, s *Server, hubs *WebSocketHubs) {
	h := &StudioHandlers{
		server: s,
		hubs:   hubs,
		mat:    newMaterializeManager(s.config.DataDir, s.logger),
		dl:     newDownloadManager(s.config.DataDir, s.logger),
	}
	r.Get("/studio/api/status", h.handleStatus)
	r.Get("/studio/api/config", h.handleConfig)
	r.Get("/studio/api/hubs", h.handleHubs)
	r.Get("/studio/api/library", h.handleLibrary)
	r.Post("/studio/api/materialize", h.handleMaterialize)
	r.Get("/studio/api/materialize", h.handleMaterializeStatus)
	r.Get("/studio/api/calendar", h.handleCalendar)
	r.Get("/studio/api/download/options", h.handleDownloadOptions)
	r.Post("/studio/api/download", h.handleDownload)
	r.Get("/studio/api/download", h.handleDownloadStatus)
	r.Get("/studio/api/logs", h.handleLogs)
	r.Get("/studio/api/logs/volume", h.handleLogsVolume)
	r.Get("/studio/api/keys", h.handleKeys)
	r.Get("/studio/api/endpoints", h.handleEndpoints)
}

func writeStudioJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// currentDate returns the authoritative loaded date (reload manager if wired,
// else the startup config).
func (h *StudioHandlers) currentDate() string {
	if h.server.reloadManager != nil {
		if d := h.server.reloadManager.CurrentDate(); d != "" {
			return d
		}
	}
	return h.server.config.DataDate
}

type studioStatus struct {
	Running           bool     `json:"running"`
	LoadedDate        string   `json:"loaded_date"`
	LoadedAt          string   `json:"loaded_at"`
	FilesLoaded       int      `json:"files_loaded"`
	Tickers           []string `json:"tickers"`
	DataMode          string   `json:"data_mode"`
	CacheMode         string   `json:"cache_mode"`
	EndpointCacheMode string   `json:"endpoint_cache_mode"`
	IsReloading       bool     `json:"is_reloading"`
	DiskBytes         int64    `json:"disk_bytes"`
	GroupPrefix       string   `json:"group_prefix"`
	WSEnabled         bool     `json:"ws_enabled"`
	Port              string   `json:"port"`
}

func (h *StudioHandlers) handleStatus(w http.ResponseWriter, _ *http.Request) {
	cfg := h.server.config
	keys := h.server.loader.GetLoadedKeys()

	// Distinct tickers from loaded data keys (ticker/pkg/category).
	tickerSet := map[string]bool{}
	for _, k := range keys {
		if i := indexByte(k, '/'); i > 0 {
			tickerSet[k[:i]] = true
		}
	}
	tickers := make([]string, 0, len(tickerSet))
	for t := range tickerSet {
		tickers = append(tickers, t)
	}
	sort.Strings(tickers)

	loadedAt := h.server.loadedAt
	isReloading := false
	if h.server.reloadManager != nil {
		loadedAt = h.server.reloadManager.LoadedAt()
		isReloading = h.server.reloadManager.IsReloading()
	}

	writeStudioJSON(w, studioStatus{
		Running:           true,
		LoadedDate:        h.currentDate(),
		LoadedAt:          loadedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		FilesLoaded:       len(keys),
		Tickers:           tickers,
		DataMode:          cfg.DataMode,
		CacheMode:         cfg.CacheMode,
		EndpointCacheMode: cfg.EndpointCacheMode,
		IsReloading:       isReloading,
		DiskBytes:         cachedDiskSize(cfg.DataDir),
		GroupPrefix:       cfg.WSGroupPrefix,
		WSEnabled:         cfg.WSEnabled,
		Port:              cfg.Port,
	})
}

type settingRow struct {
	Label string `json:"label"`
	Help  string `json:"help"`
	Env   string `json:"env"`
	Value string `json:"value"`
}

type settingGroup struct {
	Title string       `json:"title"`
	Sub   string       `json:"sub"`
	Rows  []settingRow `json:"rows"`
}

func (h *StudioHandlers) handleConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := h.server.config
	onOff := func(b bool) string {
		if b {
			return "on"
		}
		return "off"
	}
	groups := []settingGroup{
		{Title: "Replay", Sub: "How the faker hands data to clients", Rows: []settingRow{
			{Label: "When a client reaches the end", Help: "Stop and return 404 (exhaust), or loop back to the open (rotation).", Env: "CACHE_MODE", Value: cfg.CacheMode},
			{Label: "Positions across endpoints", Help: "Share one position, or track each endpoint separately.", Env: "ENDPOINT_CACHE_MODE", Value: cfg.EndpointCacheMode},
			{Label: "Memory use", Help: "Load the whole day into RAM, or read from disk as you go.", Env: "DATA_MODE", Value: cfg.DataMode},
			{Label: "Date loaded", Help: "The market day currently being replayed.", Env: "DATA_DATE", Value: h.currentDate()},
		}},
		{Title: "Server", Sub: "Where clients reach it", Rows: []settingRow{
			{Label: "Port", Help: "", Env: "PORT", Value: cfg.Port},
			{Label: "Data folder", Help: "Where EOD archives and materialized JSONL live.", Env: "DATA_DIR", Value: cfg.DataDir},
		}},
		{Title: "Streaming", Sub: "WebSocket hubs", Rows: []settingRow{
			{Label: "WebSocket streaming", Help: "Turn the five hubs on or off.", Env: "WS_ENABLED", Value: onOff(cfg.WSEnabled)},
			{Label: "Broadcast every", Help: "How often each group gets a new record.", Env: "WS_STREAM_INTERVAL", Value: cfg.WSStreamInterval.String()},
			{Label: "Group prefix", Help: "Prepended to every group name.", Env: "WS_GROUP_PREFIX", Value: cfg.WSGroupPrefix},
		}},
		{Title: "Sync broadcast", Sub: "Let other services follow this one's market clock", Rows: []settingRow{
			{Label: "Broadcast positions", Help: "Publishes an SSE stream at /sync/stream.", Env: "SYNC_BROADCAST_SYSTEM_ENABLED", Value: onOff(cfg.SyncBroadcastSystemEnabled)},
			{Label: "Broadcaster name", Help: "", Env: "SYNC_BROADCAST_SYSTEM_ID", Value: cfg.SyncBroadcastSystemID},
			{Label: "Broadcast every", Help: "", Env: "SYNC_BROADCAST_SYSTEM_INTERVAL", Value: cfg.SyncBroadcastSystemInterval.String()},
		}},
	}
	writeStudioJSON(w, groups)
}

type hubStat struct {
	Name         string   `json:"name"`
	Clients      int      `json:"clients"`
	ActiveGroups []string `json:"active_groups"`
	Interval     string   `json:"interval"`
}

func (h *StudioHandlers) handleHubs(w http.ResponseWriter, _ *http.Request) {
	interval := h.server.config.WSStreamInterval.String()
	stats := []hubStat{}
	if h.hubs != nil {
		refs := []struct {
			name string
			hub  *ws.Hub
		}{
			{"orderflow", h.hubs.Orderflow},
			{"classic", h.hubs.Classic},
			{"state_gex", h.hubs.StateGex},
			{"state_greeks_zero", h.hubs.StateGreeksZero},
			{"state_greeks_one", h.hubs.StateGreeksOne},
		}
		for _, ref := range refs {
			if ref.hub == nil {
				continue
			}
			groups := ref.hub.GetActiveGroups()
			if groups == nil {
				groups = []string{}
			}
			sort.Strings(groups)
			stats = append(stats, hubStat{
				Name:         ref.name,
				Clients:      ref.hub.ClientCount(),
				ActiveGroups: groups,
				Interval:     interval,
			})
		}
	}
	writeStudioJSON(w, stats)
}

type libRow struct {
	eod.ArchiveInfo
	Loaded   bool   `json:"loaded"`
	Total    int    `json:"total"`               // archived tickers for the date
	State    string `json:"state"`               // loaded | ready | archived | materializing
	JobError string `json:"job_error,omitempty"` // last background-materialize failure, if any
}

func (h *StudioHandlers) handleLibrary(w http.ResponseWriter, _ *http.Request) {
	archives, err := eod.ListArchives(h.server.config.DataDir)
	if err != nil {
		http.Error(w, `{"error":"failed to list archives"}`, http.StatusInternalServerError)
		return
	}
	loaded := h.currentDate()
	rows := make([]libRow, 0, len(archives))
	for _, a := range archives {
		total := len(a.Tickers)
		state := studioLibraryState(a.Date == loaded, h.mat.inProgress(a.Date), a.Materialized, total)
		row := libRow{ArchiveInfo: a, Loaded: a.Date == loaded, Total: total, State: state}
		// Surface a terminal background failure so the UI doesn't silently revert
		// to a Materialize button and drop the error.
		if job, ok := h.mat.status(a.Date); ok && job.State == "error" {
			row.JobError = job.Error
		}
		rows = append(rows, row)
	}
	writeStudioJSON(w, rows)
}

// studioLibraryState maps a date's materialization to a UI state: "loaded" (being
// served), "materializing" (a background job is running), "ready" (every ticker
// materialized → Load is instant), else "archived" (needs Materialize first;
// includes a partially-materialized date with no running job).
func studioLibraryState(isLoaded, running bool, materialized, total int) string {
	switch {
	case isLoaded:
		return "loaded"
	case running:
		return "materializing"
	case total > 0 && materialized == total:
		return "ready"
	default:
		return "archived"
	}
}

// handleMaterialize starts (or joins) a background materialization for a date.
// Body: {"date":"YYYY-MM-DD"}. Returns the job (202 Accepted).
func (h *StudioHandlers) handleMaterialize(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Date == "" {
		http.Error(w, `{"error":"date is required"}`, http.StatusBadRequest)
		return
	}
	// Strict YYYY-MM-DD only — rejects path-like values (e.g. "2026-08-07/..")
	// before any path join, so an unauthenticated caller can't grow the job map
	// with lexical variants that resolve to real paths.
	if !studioDateRe.MatchString(body.Date) {
		http.Error(w, `{"error":"date must be YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}
	// And it must actually have an archive to unpack.
	if !eod.HasArchive(h.server.config.DataDir, body.Date) {
		http.Error(w, `{"error":"no archive for that date"}`, http.StatusBadRequest)
		return
	}
	job := h.mat.start(body.Date)
	w.WriteHeader(http.StatusAccepted)
	writeStudioJSON(w, job)
}

// handleMaterializeStatus returns the job for ?date=... (404 if none), or every
// job seen this process when no date is given.
func (h *StudioHandlers) handleMaterializeStatus(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		writeStudioJSON(w, h.mat.all())
		return
	}
	job, ok := h.mat.status(date)
	if !ok {
		http.Error(w, `{"error":"no job for that date"}`, http.StatusNotFound)
		return
	}
	writeStudioJSON(w, job)
}

type keyStream struct {
	DataKey string `json:"data_key"`
	Index   int    `json:"index"`
}

type keyEntry struct {
	Key     string      `json:"key"`
	Streams []keyStream `json:"streams"`
}

func (h *StudioHandlers) handleKeys(w http.ResponseWriter, _ *http.Request) {
	all := h.server.cache.AllPositions()
	entries := make([]keyEntry, 0, len(all))
	for apiKey, streams := range all {
		s := make([]keyStream, 0, len(streams))
		for dk, idx := range streams {
			s = append(s, keyStream{DataKey: dk, Index: idx})
		}
		sort.Slice(s, func(i, j int) bool { return s[i].DataKey < s[j].DataKey })
		entries = append(entries, keyEntry{Key: clientID(apiKey), Streams: s})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	writeStudioJSON(w, entries)
}

type endpointDoc struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Desc   string `json:"desc"`
}

func (h *StudioHandlers) handleEndpoints(w http.ResponseWriter, _ *http.Request) {
	writeStudioJSON(w, studioEndpointCatalog)
}

// studioEndpointCatalog is a curated, human-readable list of the endpoints a
// client points at, for the Server screen. Paths use the current (post-audit)
// gex_full/gex_zero/gex_one aggregation segments.
var studioEndpointCatalog = []endpointDoc{
	{"GET", "/{ticker}/classic/{aggregation}", "Classic GEX chain — advances one record per call"},
	{"GET", "/{ticker}/state/{type}", "State GEX profiles and greeks"},
	{"GET", "/{ticker}/orderflow/orderflow", "Latest orderflow metrics"},
	{"GET", "/options/{ticker}/expiries", "Option expiries over the loaded-date horizon"},
	{"GET", "/futures/conversion", "Front-month contract + conversion params"},
	{"GET", "/available-data/{date}", "What exists for a date"},
	{"POST", "/reload-date", "Swap the replay date without restarting"},
	{"POST", "/seek-to-timestamp", "Move every position to a market time"},
	{"POST", "/reset-cache", "Send positions back to the start"},
	{"GET", "/negotiate", "WebSocket URLs and group prefix"},
	{"GET", "/sync/stream", "SSE position broadcast for other services"},
}

// clientID returns a non-reversible, unique-per-key identifier for an API key.
// The /studio/api endpoints are unauthenticated and the server is often reachable
// beyond loopback, so no part of the credential is exposed; the hash still lets
// the UI tell clients apart and gives Svelte a stable, collision-free list key.
func clientID(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return "client-" + hex.EncodeToString(sum[:])[:8]
}

// diskSizeTTL bounds how often the whole data tree is walked for the "on disk"
// figure. Status is polled every few seconds per open Studio tab, so the walk
// must not run on the request path each time.
const diskSizeTTL = 60 * time.Second

var diskCache struct {
	mu        sync.Mutex
	root      string
	bytes     int64
	at        time.Time
	computing bool
}

// cachedDiskSize returns a recent total byte size for root without ever walking
// the tree on the calling goroutine: on a cache miss it kicks off one background
// walk and returns the last known value (0 until the first walk finishes). At
// most one walk runs per diskSizeTTL regardless of how many tabs are polling.
func cachedDiskSize(root string) int64 {
	diskCache.mu.Lock()
	fresh := diskCache.root == root && time.Since(diskCache.at) < diskSizeTTL
	if fresh || diskCache.computing {
		v := diskCache.bytes
		diskCache.mu.Unlock()
		return v
	}
	diskCache.computing = true
	last := diskCache.bytes
	diskCache.mu.Unlock()

	go func() {
		v := dirSize(root)
		diskCache.mu.Lock()
		diskCache.root = root
		diskCache.bytes = v
		diskCache.at = time.Now()
		diskCache.computing = false
		diskCache.mu.Unlock()
	}()
	return last
}

// dirSize returns the total byte size of all regular files under root (0 if the
// path is missing/unreadable). Used for the "on disk" figure.
func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries, keep walking
		}
		if d.IsDir() {
			return nil
		}
		if info, e := d.Info(); e == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func indexByte(s string, b byte) int { return strings.IndexByte(s, b) }
