package server

import (
	"sync"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

// materializeJob is the status of a background materialization of one date.
type materializeJob struct {
	Date  string `json:"date"`
	State string `json:"state"` // "running" | "done" | "error"
	Error string `json:"error,omitempty"`
}

// materializeManager runs date materializations in the background so the Studio
// UI never blocks on the multi-minute unpack. Jobs are keyed by date and are
// idempotent: starting a date that's already running returns the in-flight job.
// Per-ticker progress is read from the .eod-materialized markers on disk (via
// eod.ListArchives), so no progress plumbing into MaterializeDate is needed.
type materializeManager struct {
	mu      sync.Mutex
	jobs    map[string]*materializeJob
	dataDir string
	logger  *zap.Logger
}

func newMaterializeManager(dataDir string, logger *zap.Logger) *materializeManager {
	return &materializeManager{jobs: map[string]*materializeJob{}, dataDir: dataDir, logger: logger}
}

// start begins (or returns the running) materialization for date. The returned
// job is a copy safe to read without the lock.
func (m *materializeManager) start(date string) materializeJob {
	m.mu.Lock()
	if j, ok := m.jobs[date]; ok && j.State == "running" {
		out := *j
		m.mu.Unlock()
		return out
	}
	job := &materializeJob{Date: date, State: "running"}
	m.jobs[date] = job
	m.mu.Unlock()

	go func() {
		// Detached from the request context so a client disconnect can't abort a
		// long unpack; MaterializeDate is idempotent and marker-gated.
		err := eod.MaterializeDate(m.dataDir, date, m.logger)
		m.mu.Lock()
		if err != nil {
			job.State = "error"
			job.Error = err.Error()
		} else {
			job.State = "done"
		}
		m.mu.Unlock()
	}()

	return materializeJob{Date: date, State: "running"}
}

// running reports whether a materialization for date is in flight.
func (m *materializeManager) running(date string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[date]
	return ok && j.State == "running"
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
