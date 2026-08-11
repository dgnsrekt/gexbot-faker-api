package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestDaemonSnapshotMetrics(t *testing.T) {
	RegisterDaemon() // idempotent; must not panic registering the new series

	DaemonSnapshots.WithLabelValues("SPX").Set(17844)
	if v := testutil.ToFloat64(DaemonSnapshots.WithLabelValues("SPX")); v != 17844 {
		t.Errorf("snapshots gauge = %v, want 17844", v)
	}

	before := testutil.ToFloat64(DaemonCoverageFindings.WithLabelValues("low-snapshots"))
	DaemonCoverageFindings.WithLabelValues("low-snapshots").Inc()
	if v := testutil.ToFloat64(DaemonCoverageFindings.WithLabelValues("low-snapshots")); v != before+1 {
		t.Errorf("findings counter = %v, want %v", v, before+1)
	}
}

func TestSetDaemonSnapshotsClearsStaleTickers(t *testing.T) {
	SetDaemonSnapshots(map[string]int{"SPX": 100, "SPY": 200})
	if c := testutil.CollectAndCount(DaemonSnapshots, "faker_daemon_snapshots"); c != 2 {
		t.Fatalf("after first publish, series = %d, want 2", c)
	}
	// SPY leaves the set; the next publish must not keep exporting it.
	SetDaemonSnapshots(map[string]int{"SPX": 110, "NDX": 300})
	if c := testutil.CollectAndCount(DaemonSnapshots, "faker_daemon_snapshots"); c != 2 {
		t.Errorf("after second publish, series = %d, want 2 (stale SPY must be cleared)", c)
	}
	if v := testutil.ToFloat64(DaemonSnapshots.WithLabelValues("SPX")); v != 110 {
		t.Errorf("SPX = %v, want the latest 110", v)
	}
}
