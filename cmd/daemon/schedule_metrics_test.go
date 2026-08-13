package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/dgnsrekt/gexbot-downloader/internal/observability"
)

func TestUpdateScheduleMetrics(t *testing.T) {
	s := NewScheduler(17, 5, "America/New_York")
	loc := s.Location()
	// Mon 2026-08-10 18:00: a market day, past the 17:05 slot → expected latest is today.
	s.now = func() time.Time { return time.Date(2026, 8, 10, 18, 0, 0, 0, loc) }
	stateFile := filepath.Join(t.TempDir(), "last.txt")
	tr := NewDownloadTracker(stateFile)

	// An active retry backoff must win over the next scheduled market-day slot.
	retry := time.Date(2026, 8, 10, 18, 5, 0, 0, loc)
	updateScheduleMetrics(s, tr, retry)
	if got := int64(testutil.ToFloat64(observability.DaemonNextRun)); got != retry.Unix() {
		t.Errorf("next-run with backoff = %d, want the retry time %d", got, retry.Unix())
	}
	// With no backoff, next-run is the next scheduled slot (strictly after now → Tue).
	updateScheduleMetrics(s, tr, time.Time{})
	if got := int64(testutil.ToFloat64(observability.DaemonNextRun)); got <= s.now().Unix() {
		t.Errorf("next-run without backoff = %d, must be in the future", got)
	}

	// Fail safe: empty/garbage/behind state → overdue=1; up-to-date → 0. A lexical compare
	// would wrongly report "9999-99-99" as up-to-date.
	for _, c := range []struct {
		last string
		want float64
	}{
		{"", 1}, {"garbage", 1}, {"9999-99-99", 1}, {"2026-08-07", 1}, {"2026-08-10", 0},
	} {
		if err := os.WriteFile(stateFile, []byte(c.last+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		updateScheduleMetrics(s, tr, time.Time{})
		if got := testutil.ToFloat64(observability.DaemonDownloadOverdue); got != c.want {
			t.Errorf("overdue(last=%q) = %v, want %v", c.last, got, c.want)
		}
	}
}
