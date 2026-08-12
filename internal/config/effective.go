package config

// This file is the single source of truth for a config's *effective* download
// coverage — what a download WOULD fetch after defaults/fallbacks are applied.
// GenerateTasksForDate (the actual downloader), the Studio's read-only download
// options, and the daemon's /status all derive coverage from here, so the three
// can never disagree.

// EffectivePackage is an enabled package paired with its effective category list.
type EffectivePackage struct {
	Name       string
	Categories []string
}

// EffectiveTickers returns the tickers a download would use: the config's list,
// or DefaultTickers() when the config leaves it empty.
func EffectiveTickers(c *Config) []string {
	if len(c.Tickers) == 0 {
		return DefaultTickers()
	}
	return c.Tickers
}

// EffectivePackages returns the enabled packages with their effective category
// lists in a stable order (state, classic, orderflow). A package enabled with no
// configured categories expands to all valid categories for that package.
func EffectivePackages(c *Config) []EffectivePackage {
	out := make([]EffectivePackage, 0, 3)
	add := func(name Package, pc PackageConfig) {
		if !pc.Enabled {
			return
		}
		cats := pc.Categories
		if len(cats) == 0 {
			cats = ValidCategories[name]
		}
		out = append(out, EffectivePackage{Name: string(name), Categories: cats})
	}
	add(PackageState, c.Packages.State)
	add(PackageClassic, c.Packages.Classic)
	add(PackageOrderflow, c.Packages.Orderflow)
	return out
}
