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
}
