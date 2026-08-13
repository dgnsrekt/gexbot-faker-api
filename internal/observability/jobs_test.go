package observability

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestTrackJob(t *testing.T) {
	JobsInProgress.Reset()
	Jobs.Reset()

	// While running, in-progress is 1; after done, it's back to 0 and a success is counted.
	done := TrackJob("materialize")
	if got := testutil.ToFloat64(JobsInProgress.WithLabelValues("materialize")); got != 1 {
		t.Errorf("in-progress during job = %v, want 1", got)
	}
	done(nil)
	if got := testutil.ToFloat64(JobsInProgress.WithLabelValues("materialize")); got != 0 {
		t.Errorf("in-progress after job = %v, want 0", got)
	}
	if got := testutil.ToFloat64(Jobs.WithLabelValues("materialize", "success")); got != 1 {
		t.Errorf("success count = %v, want 1", got)
	}

	// A failing job increments the error result, not success.
	TrackJob("range_load")(errors.New("boom"))
	if got := testutil.ToFloat64(Jobs.WithLabelValues("range_load", "error")); got != 1 {
		t.Errorf("error count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(Jobs.WithLabelValues("range_load", "success")); got != 0 {
		t.Errorf("failed job must not count success, got %v", got)
	}
}
