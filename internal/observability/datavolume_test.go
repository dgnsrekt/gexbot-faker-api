package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDataVolumeCollector(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(dataVolumeCollector{path: t.TempDir()})
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, mf := range mfs {
		if len(mf.Metric) > 0 && mf.Metric[0].Gauge != nil {
			got[mf.GetName()] = mf.Metric[0].Gauge.GetValue()
		}
	}
	free := got["faker_data_volume_free_bytes"]
	total := got["faker_data_volume_total_bytes"]
	if free <= 0 {
		t.Errorf("free_bytes = %v, want > 0 for a real temp dir", free)
	}
	if total < free {
		t.Errorf("total_bytes (%v) must be >= free_bytes (%v)", total, free)
	}
	if _, ok := got["faker_data_volume_free_inodes"]; !ok {
		t.Error("expected faker_data_volume_free_inodes to be exported")
	}
	if got["faker_data_volume_statfs_success"] != 1 {
		t.Errorf("statfs_success = %v, want 1 on a real dir", got["faker_data_volume_statfs_success"])
	}
}

func TestDataVolumeCollectorStatfsFailure(t *testing.T) {
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(dataVolumeCollector{path: "/no/such/path/at/all"})
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, mf := range mfs {
		if len(mf.Metric) > 0 && mf.Metric[0].Gauge != nil {
			got[mf.GetName()] = mf.Metric[0].Gauge.GetValue()
		}
	}
	// The success flag must still be exported (=0); the byte/inode series must be omitted.
	if v, ok := got["faker_data_volume_statfs_success"]; !ok || v != 0 {
		t.Errorf("statfs_success on a bad path = %v (present=%v), want 0", v, ok)
	}
	if _, ok := got["faker_data_volume_free_bytes"]; ok {
		t.Error("free_bytes must be omitted when statfs fails (no misleading zero)")
	}
}
