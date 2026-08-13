package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_http_requests_total", Help: "HTTP requests handled.",
	}, []string{"method", "route", "status_class"})
	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "faker_http_request_duration_seconds", Help: "HTTP request duration.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
	HTTPInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_http_in_flight_requests", Help: "HTTP requests currently being handled.",
	})
	HTTPResponseBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_http_response_bytes_total", Help: "HTTP response bytes written.",
	}, []string{"route"})

	WSConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "faker_ws_connections", Help: "Active WebSocket connections.",
	}, []string{"hub"})
	WSConnectionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_ws_connections_total", Help: "WebSocket connections accepted.",
	}, []string{"hub"})
	WSActiveGroups = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "faker_ws_active_groups", Help: "Active WebSocket subscription groups.",
	}, []string{"hub"})
	WSMessagesSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_ws_messages_sent_total", Help: "WebSocket messages queued for clients.",
	}, []string{"hub", "format"})
	WSMessageBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_ws_message_bytes_total", Help: "WebSocket message bytes queued.",
	}, []string{"hub", "format"})
	WSSendErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_ws_send_errors_total", Help: "WebSocket messages rejected by a closed or full client buffer.",
	}, []string{"hub"})
	WSLastBroadcast = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "faker_ws_last_broadcast_timestamp_seconds", Help: "Unix timestamp of the last successful WebSocket broadcast.",
	}, []string{"hub"})

	DataLoadedTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_data_loaded_timestamp_seconds", Help: "Unix timestamp when the active dataset was loaded.",
	})
	DataDateTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_data_date_timestamp_seconds", Help: "Unix timestamp representing the active dataset date.",
	})
	Reloads = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_reloads_total", Help: "Dataset reload attempts.",
	}, []string{"result"})
	ReloadDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "faker_reload_duration_seconds", Help: "Dataset reload duration.",
		Buckets: []float64{.1, .5, 1, 2, 5, 10, 30},
	})
	ReloadInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_reload_in_progress", Help: "Whether a dataset reload is active.",
	})
	CacheReads = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_cache_reads_total", Help: "Playback cache reads.",
	}, []string{"result"})
	CacheResets = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "faker_cache_resets_total", Help: "Playback cache reset operations.",
	})
	SyncBroadcasts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_sync_broadcasts_total", Help: "Sync broadcast delivery attempts.",
	}, []string{"result"})

	DaemonRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_daemon_download_runs_total", Help: "Scheduled download runs.",
	}, []string{"result"})
	DaemonDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "faker_daemon_download_duration_seconds", Help: "Scheduled download duration.",
		Buckets: []float64{1, 10, 30, 60, 300, 900, 1800},
	})
	DaemonInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_daemon_download_in_progress", Help: "Whether a scheduled download is active.",
	})
	DaemonFiles = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_daemon_files_total", Help: "Files processed by scheduled downloads.",
	}, []string{"result"})
	DaemonLastSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_daemon_last_success_timestamp_seconds", Help: "Unix timestamp of the last successful scheduled download.",
	})
	DaemonNextRun = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_daemon_next_run_timestamp_seconds", Help: "Unix timestamp of the next scheduled download attempt (next market day at the configured time).",
	})
	// DaemonExpectedDate is the latest market day the daemon should already have EOD data
	// for (market-aware — does not advance over weekends/holidays). Compared against the
	// last successful download to derive DaemonDownloadOverdue.
	DaemonExpectedDate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_daemon_expected_date_timestamp_seconds", Help: "Unix timestamp of the latest market date the daemon should have downloaded.",
	})
	// DaemonDownloadOverdue is 1 when the last successful download is behind the expected
	// latest market date (a genuinely missed session), else 0 — a market-aware freshness
	// signal that, unlike a wall-clock threshold, does not false-alarm around closures.
	DaemonDownloadOverdue = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "faker_daemon_download_overdue", Help: "1 when the last successful download is behind the expected latest market date, else 0.",
	})
	// DaemonSnapshots is the latest download's per-ticker intraday snapshot count
	// (~23,400 ≈ a full 1/sec session). A sustained drop in a ticker's series is a
	// coverage regression — Prometheus/Studio can alert on deviation from a rolling median.
	DaemonSnapshots = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "faker_daemon_snapshots", Help: "Latest download's intraday snapshot count per ticker.",
	}, []string{"ticker"})
	// DaemonCoverageFindings counts coverage-check findings by kind
	// (low-snapshots, sparse-session, late-open, early-close, gap).
	DaemonCoverageFindings = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "faker_daemon_coverage_findings_total", Help: "Coverage-check findings by kind.",
	}, []string{"kind"})
)

var (
	registerServerOnce sync.Once
	registerDaemonOnce sync.Once
)

// SetDaemonSnapshots publishes the latest download's per-ticker snapshot counts,
// first resetting the vector so a ticker that left the set (e.g. removed from the
// config) stops exporting its stale value — otherwise the dashboard would keep
// presenting it as part of the current run.
func SetDaemonSnapshots(snaps map[string]int) {
	DaemonSnapshots.Reset()
	for ticker, n := range snaps {
		DaemonSnapshots.WithLabelValues(ticker).Set(float64(n))
	}
}

// RegisterServer registers only the metrics the API/server binary emits. Metrics
// are registered per binary (rather than a package init) so the daemon does not
// export server-only series as phantom zeros and vice versa — a bare `time() -
// <metric>` on an unset zero reads as ~56 years (the Unix epoch) on the wrong
// instance. Idempotent, so tests may call it more than once.
func RegisterServer() {
	registerServerOnce.Do(func() {
		prometheus.MustRegister(
			HTTPRequests, HTTPRequestDuration, HTTPInFlight, HTTPResponseBytes,
			WSConnections, WSConnectionsTotal, WSActiveGroups, WSMessagesSent, WSMessageBytes, WSSendErrors, WSLastBroadcast,
			DataLoadedTimestamp, DataDateTimestamp, Reloads, ReloadDuration, ReloadInProgress, CacheReads, CacheResets, SyncBroadcasts,
		)
	})
}

// RegisterDaemon registers only the metrics the daemon binary emits. See
// RegisterServer for why registration is split by binary. Idempotent.
func RegisterDaemon() {
	registerDaemonOnce.Do(func() {
		prometheus.MustRegister(
			DaemonRuns, DaemonDuration, DaemonInProgress, DaemonFiles, DaemonLastSuccess, DaemonNextRun,
			DaemonExpectedDate, DaemonDownloadOverdue, DaemonSnapshots, DaemonCoverageFindings,
		)
	})
}
