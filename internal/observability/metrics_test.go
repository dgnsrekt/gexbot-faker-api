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
