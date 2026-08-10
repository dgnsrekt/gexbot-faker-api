package downloadjob

import (
	"testing"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
)

func TestGenerateTasksForDate(t *testing.T) {
	cfg := &config.Config{Tickers: []string{"SPX", "NDX"}}
	cfg.Packages.Classic.Enabled = true
	cfg.Packages.Classic.Categories = []string{"gex_zero", "gex_full"}
	cfg.Packages.Orderflow.Enabled = true
	cfg.Packages.Orderflow.Categories = []string{"orderflow"}

	tasks := GenerateTasksForDate(cfg, "2026-08-07")

	// 2 tickers × (2 classic + 1 orderflow) = 6.
	if len(tasks) != 6 {
		t.Fatalf("got %d tasks, want 6", len(tasks))
	}
	for _, tk := range tasks {
		if tk.Date != "2026-08-07" {
			t.Errorf("task has wrong date: %+v", tk)
		}
		if tk.Ticker != "SPX" && tk.Ticker != "NDX" {
			t.Errorf("unexpected ticker: %+v", tk)
		}
	}
}

func TestGenerateTasksDefaultTickers(t *testing.T) {
	cfg := &config.Config{} // no tickers → falls back to DefaultTickers
	cfg.Packages.State.Enabled = true
	cfg.Packages.State.Categories = []string{"gex_full"}
	tasks := GenerateTasksForDate(cfg, "2026-08-07")
	if len(tasks) != len(config.DefaultTickers()) {
		t.Errorf("got %d tasks, want one per default ticker (%d)", len(tasks), len(config.DefaultTickers()))
	}
}

func TestGenerateTasksNoPackages(t *testing.T) {
	cfg := &config.Config{Tickers: []string{"SPX"}} // no packages enabled
	if tasks := GenerateTasksForDate(cfg, "2026-08-07"); len(tasks) != 0 {
		t.Errorf("expected 0 tasks with no packages, got %d", len(tasks))
	}
}
