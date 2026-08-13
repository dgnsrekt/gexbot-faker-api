package server

import (
	"sync"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
	"github.com/dgnsrekt/gexbot-downloader/internal/observability"
)

// materializeJob is the status of a materialization of one date.
type materializeJob struct {
	Date  string `json:"date"`
	State string `json:"state"` // "queued" | "running" | "done" | "error"
	Error string `json:"error,omitempty"`
}

// materializeManager runs date materializations in the background so the Studio
// UI never blocks on the multi-minute unpack. Work is **serialized**: a single
// worker goroutine processes one date at a time, so clicking Materialize on many
// rows queues them rather than spawning many simultaneous multi-minute disk/CPU
// jobs. Jobs are keyed by date and idempotent — enqueuing a date that's already
// queued or running returns the in-flight job. Per-ticker progress is read from
// the .eod-materialized markers on disk (via eod.ListArchives), so no progress
// plumbing into MaterializeDate is needed.
type materializeManager struct {
	mu      sync.Mutex
	jobs    map[string]*materializeJob
	queue   chan string
	dataDir string
	logger  *zap.Logger
}

func newMaterializeManager(dataDir string, logger *zap.Logger) *materializeManager {
	m := &materializeManager{
		jobs:    map[string]*materializeJob{},
		queue:   make(chan string, 1024), // >> plausible number of archived dates
		dataDir: dataDir,
		logger:  logger,
	}
	go m.worker()
	return m
}

// worker processes queued dates one at a time (global serialization).
func (m *materializeManager) worker() {
	for date := range m.queue {
		m.mu.Lock()
		job := m.jobs[date]
		if job == nil || job.State != "queued" {
			m.mu.Unlock()
			continue
		}
		job.State = "running"
		m.mu.Unlock()

		// MaterializeDate is idempotent and marker-gated; detached from any
		// request context so a client disconnect can't abort a long unpack.
		done := observability.TrackJob("materialize")
		err := eod.MaterializeDate(m.dataDir, date, m.logger)
		done(err)

		m.mu.Lock()
		if err != nil {
			job.State = "error"
			job.Error = err.Error()
			m.logger.Warn("studio materialize failed", zap.String("date", date), zap.Error(err))
		} else {
			job.State = "done"
		}
		m.mu.Unlock()
	}
}

// start enqueues date for materialization (or returns the in-flight job). The
// returned job is a copy safe to read without the lock.
func (m *materializeManager) start(date string) materializeJob {
	m.mu.Lock()
	if j, ok := m.jobs[date]; ok && (j.State == "queued" || j.State == "running") {
		out := *j
		m.mu.Unlock()
		return out
	}
	job := &materializeJob{Date: date, State: "queued"}
	m.jobs[date] = job
	out := *job
	m.mu.Unlock()

	m.queue <- date // buffered well beyond the number of real archive dates
	return out
}

// inProgress reports whether date is queued or running.
func (m *materializeManager) inProgress(date string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[date]
	return ok && (j.State == "queued" || j.State == "running")
}

// status returns the job for date (ok=false if none has ever run this process).
func (m *materializeManager) status(date string) (materializeJob, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[date]; ok {
		return *j, true
	}
	return materializeJob{}, false
}

// all returns a snapshot of every job seen this process.
func (m *materializeManager) all() []materializeJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]materializeJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, *j)
	}
	return out
}
