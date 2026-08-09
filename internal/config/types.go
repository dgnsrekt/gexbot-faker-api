package config

// Package represents a data package type
type Package string

const (
	PackageState     Package = "state"
	PackageClassic   Package = "classic"
	PackageOrderflow Package = "orderflow"
)

// ValidCategories returns valid categories for each package
var ValidCategories = map[Package][]string{
	PackageState: {
		"gex_full", "gex_zero", "gex_one",
		"delta_zero", "delta_one",
		"gamma_zero", "gamma_one",
		"vanna_zero", "vanna_one",
		"charm_zero", "charm_one",
	},
	PackageClassic:   {"gex_full", "gex_zero", "gex_one"},
	PackageOrderflow: {"orderflow"},
}

// DefaultTickers returns a default set of common tickers to download when the
// config doesn't specify any. This is a curated convenience default, not the
// full universe — see tickers.json / ValidTickers for the complete set.
func DefaultTickers() []string {
	return []string{"SPX", "NDX", "RUT", "SPY", "QQQ", "IWM"}
}

// ValidTickers, IndexTickers, and FutureTickers are synced from GEXbot's
// /tickers endpoint and live in tickers.go (backed by the embedded tickers.json).
