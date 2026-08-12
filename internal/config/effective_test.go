package config

import "testing"

func TestEffectiveTickers(t *testing.T) {
	if got := EffectiveTickers(&Config{Tickers: []string{"SPX"}}); len(got) != 1 || got[0] != "SPX" {
		t.Errorf("configured tickers = %v, want [SPX]", got)
	}
	// Empty → DefaultTickers fallback (mirrors GenerateTasksForDate).
	if got := EffectiveTickers(&Config{}); len(got) != len(DefaultTickers()) {
		t.Errorf("empty tickers = %v, want DefaultTickers()", got)
	}
}

func TestEffectivePackages(t *testing.T) {
	c := &Config{}
	c.Packages.State.Enabled = true
	c.Packages.State.Categories = []string{"gex_zero"} // curated subset preserved
	c.Packages.Classic.Enabled = true                  // no categories → all valid
	c.Packages.Orderflow.Enabled = false               // disabled → omitted

	got := EffectivePackages(c)
	if len(got) != 2 {
		t.Fatalf("got %d packages, want 2 (state, classic)", len(got))
	}
	// Stable order: state, classic.
	if got[0].Name != "state" || len(got[0].Categories) != 1 || got[0].Categories[0] != "gex_zero" {
		t.Errorf("state = %+v, want the subset [gex_zero]", got[0])
	}
	if got[1].Name != "classic" || len(got[1].Categories) != len(ValidCategories[PackageClassic]) {
		t.Errorf("classic = %+v, want all valid classic categories", got[1])
	}
}
