package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sys/unix"
)

// Disk exhaustion on the growing data volume (EOD archives + materialized JSONL + offset
// indexes, plus the Prometheus/Loki TSDBs on the same host) is this project's characteristic
// failure mode — one the default Go/process collectors cannot see. This collector reports the
// free/total bytes and free inodes of the filesystem holding the data directory, computed
// fresh at each scrape via statfs (no background goroutine, always current).
var (
	dataVolumeFreeBytes  = prometheus.NewDesc("faker_data_volume_free_bytes", "Free bytes on the filesystem holding the data directory.", nil, nil)
	dataVolumeTotalBytes = prometheus.NewDesc("faker_data_volume_total_bytes", "Total bytes of the filesystem holding the data directory.", nil, nil)
	dataVolumeFreeInodes = prometheus.NewDesc("faker_data_volume_free_inodes", "Free inodes on the filesystem holding the data directory.", nil, nil)
	dataVolumeStatfsOK   = prometheus.NewDesc("faker_data_volume_statfs_success", "1 if the last statfs of the data directory succeeded, else 0.", nil, nil)

	registerDataVolumeOnce sync.Once
)

type dataVolumeCollector struct{ path string }

func (c dataVolumeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dataVolumeFreeBytes
	ch <- dataVolumeTotalBytes
	ch <- dataVolumeFreeInodes
	ch <- dataVolumeStatfsOK
}

func (c dataVolumeCollector) Collect(ch chan<- prometheus.Metric) {
	var st unix.Statfs_t
	if err := unix.Statfs(c.path, &st); err != nil {
		// Omit the byte/inode series (no misleading zero) but ALWAYS export the success
		// flag as 0 — otherwise a failing statfs silently blanks disk monitoring while the
		// API target still scrapes green. An alert watches this flag.
		ch <- prometheus.MustNewConstMetric(dataVolumeStatfsOK, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(dataVolumeStatfsOK, prometheus.GaugeValue, 1)
	bsize := float64(st.Bsize)
	ch <- prometheus.MustNewConstMetric(dataVolumeFreeBytes, prometheus.GaugeValue, float64(st.Bavail)*bsize)
	ch <- prometheus.MustNewConstMetric(dataVolumeTotalBytes, prometheus.GaugeValue, float64(st.Blocks)*bsize)
	ch <- prometheus.MustNewConstMetric(dataVolumeFreeInodes, prometheus.GaugeValue, float64(st.Ffree))
}

// RegisterDataVolume adds the data-volume collector for dataDir to the default registry.
// Idempotent; a no-op path still registers (Collect just omits the series on statfs error).
func RegisterDataVolume(dataDir string) {
	registerDataVolumeOnce.Do(func() {
		prometheus.MustRegister(dataVolumeCollector{path: dataDir})
	})
}
