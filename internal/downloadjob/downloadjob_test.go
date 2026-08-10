package downloadjob

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgnsrekt/gexbot-downloader/internal/config"
	"github.com/dgnsrekt/gexbot-downloader/internal/eod"
)

func writeCat(t *testing.T, root, date, ticker, pkg, cat string) {
	t.Helper()
	p := filepath.Join(root, date, ticker, pkg, cat+".jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{\"timestamp\":1}\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func cfgWith(root, ticker string, pkgCats map[string][]string) *config.Config {
	c := &config.Config{Tickers: []string{ticker}}
	c.Output.Directory = root
	if cats, ok := pkgCats["classic"]; ok {
		c.Packages.Classic.Enabled = true
		c.Packages.Classic.Categories = cats
	}
	if cats, ok := pkgCats["state"]; ok {
		c.Packages.State.Enabled = true
		c.Packages.State.Categories = cats
	}
	return c
}

func manifestPackages(t *testing.T, root, date, ticker string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(eod.ManifestPath(eod.ArchivePath(root, date, ticker)))
	if err != nil {
		t.Fatal(err)
	}
	var man eod.Manifest
	if err := json.Unmarshal(data, &man); err != nil {
		t.Fatal(err)
	}
	pkgs := map[string]bool{}
	for _, m := range man.Members {
		pkgs[m.Package] = true
	}
	return pkgs
}

// TestPackMissingArchivesRebuildsOnAddedPackage covers the review note: after
// packing a package subset, a later request adding another package must rebuild
// the archive so the new package reaches the Library.
func TestPackMissingArchivesRebuildsOnAddedPackage(t *testing.T) {
	root := t.TempDir()
	date, ticker := "2026-08-07", "SPX"

	// First request: classic only.
	writeCat(t, root, date, ticker, "classic", "gex_full")
	if err := PackMissingArchives(cfgWith(root, ticker, map[string][]string{"classic": {"gex_full"}}), date); err != nil {
		t.Fatal(err)
	}
	if p := manifestPackages(t, root, date, ticker); p["state"] || !p["classic"] {
		t.Fatalf("after classic pack, manifest packages = %v, want classic only", p)
	}

	// Second request adds state — the existing archive must be rebuilt to include it.
	writeCat(t, root, date, ticker, "state", "gex_full")
	if err := PackMissingArchives(cfgWith(root, ticker, map[string][]string{"classic": {"gex_full"}, "state": {"gex_full"}}), date); err != nil {
		t.Fatal(err)
	}
	if p := manifestPackages(t, root, date, ticker); !p["state"] || !p["classic"] {
		t.Errorf("after adding state, manifest packages = %v, want classic+state", p)
	}
}

// TestPackMissingArchivesNoChurnOnPartial covers the completeness note: when a
// requested feed 404s (never lands on disk), a re-pack must NOT rebuild the
// archive every call — coverage is judged against on-disk JSONL, not the config.
func TestPackMissingArchivesNoChurnOnPartial(t *testing.T) {
	root := t.TempDir()
	date, ticker := "2026-08-07", "SPX"
	writeCat(t, root, date, ticker, "classic", "gex_full") // classic landed; state 404'd (absent)

	// Request both classic+state, but only classic is on disk.
	cfg := cfgWith(root, ticker, map[string][]string{"classic": {"gex_full"}, "state": {"gex_full"}})
	if err := PackMissingArchives(cfg, date); err != nil {
		t.Fatal(err)
	}
	archive := eod.ArchivePath(root, date, ticker)
	info1, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}

	// A second identical call must be a no-op (state still absent → manifest
	// already covers on-disk JSONL). The archive file must not be rewritten.
	if err := PackMissingArchives(cfg, date); err != nil {
		t.Fatal(err)
	}
	info2, err := os.Stat(archive)
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Errorf("partial archive was re-packed on a no-op call (mtime changed)")
	}
	if p := manifestPackages(t, root, date, ticker); p["state"] {
		t.Errorf("manifest should not claim state (it 404'd): %v", p)
	}
}

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
