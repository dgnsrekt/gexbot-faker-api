package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

// loadedRange describes the span a load job loaded.
type loadedRange struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Dates []string `json:"dates"`
}

// rangeLoadJob is the status of one asynchronous /load request (a day or a span).
type rangeLoadJob struct {
	ID          string       `json:"job_id"`
	Dates       []string     `json:"dates"`
	State       string       `json:"state"` // "queued" | "running" | "done" | "error"
	Done        int          `json:"done"`  // days materialized so far
	Total       int          `json:"total"` // days in the span
	LoadedRange *loadedRange `json:"loaded_range,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// rangeLoadManager runs load jobs in the background so the client never blocks on the
// multi-minute materialize of a span. Work is serialized: one worker processes a job at a time, so
// each date's slow unpack runs sequentially (progress = days materialized / total). After all days
// are on disk it swaps the whole span in via ReloadManager.ReloadRange. Jobs are keyed by an opaque
// id; MaterializeDate is idempotent, so a job re-materializing an already-present day is a no-op.
type rangeLoadManager struct {
	mu        sync.Mutex
	jobs      map[string]*rangeLoadJob
	queue     chan string
	seq       atomic.Uint64
	dataDir   string
	reloadMgr *ReloadManager
	logger    *zap.Logger
}

func newRangeLoadManager(dataDir string, reloadMgr *ReloadManager, logger *zap.Logger) *rangeLoadManager {
	m := &rangeLoadManager{
		jobs:      map[string]*rangeLoadJob{},
		queue:     make(chan string, 256),
		dataDir:   dataDir,
		reloadMgr: reloadMgr,
		logger:    logger,
	}
	go m.worker()
	return m
}

func (m *rangeLoadManager) nextID() string {
	return fmt.Sprintf("range-%d", m.seq.Add(1))
}

// worker processes queued jobs one at a time.
func (m *rangeLoadManager) worker() {
	for id := range m.queue {
		m.mu.Lock()
		job := m.jobs[id]
		if job == nil || job.State != "queued" {
			m.mu.Unlock()
			continue
		}
		job.State = "running"
		dates := append([]string(nil), job.Dates...)
		m.mu.Unlock()

		var failErr error
		// Materialize each missing day, advancing progress. Detached from any request context so a
		// client disconnect can't abort a long unpack.
		for i, d := range dates {
			datePath := filepath.Join(m.dataDir, d)
			if _, err := os.Stat(datePath); os.IsNotExist(err) {
				if err := eod.MaterializeDate(m.dataDir, d, m.logger); err != nil {
					failErr = fmt.Errorf("materialize %s: %w", d, err)
					break
				}
			}
			m.mu.Lock()
			job.Done = i + 1
			m.mu.Unlock()
		}

		// All days on disk → build the cross-day loader and swap it in (ReloadRange re-checks/
		// re-materializes, but every day is already present so it's a fast no-op).
		if failErr == nil {
			if _, err := m.reloadMgr.ReloadRange(context.Background(), dates); err != nil {
				failErr = err
			}
		}

		m.mu.Lock()
		if failErr != nil {
			job.State = "error"
			job.Error = failErr.Error()
			m.logger.Warn("load job failed", zap.String("job", id), zap.Error(failErr))
		} else {
			job.State = "done"
			job.LoadedRange = &loadedRange{From: dates[0], To: dates[len(dates)-1], Dates: dates}
		}
		m.mu.Unlock()
	}
}

// start enqueues a load job for the (already normalized, non-empty) dates and returns its
// initial status.
func (m *rangeLoadManager) start(dates []string) rangeLoadJob {
	id := m.nextID()
	job := &rangeLoadJob{ID: id, Dates: dates, State: "queued", Total: len(dates)}
	m.mu.Lock()
	m.jobs[id] = job
	out := *job
	m.mu.Unlock()

	m.queue <- id
	return out
}

// status returns the job for id (ok=false if unknown).
func (m *rangeLoadManager) status(id string) (rangeLoadJob, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok {
		return *j, true
	}
	return rangeLoadJob{}, false
}
