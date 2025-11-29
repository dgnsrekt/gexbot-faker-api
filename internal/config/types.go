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
	PackageClassic:   {"gex_full"},
	PackageOrderflow: {"orderflow"},
}

// DefaultTickers lists all supported tickers
var DefaultTickers = []string{
	"SPX", "NDX", "RUT", "SPY", "QQQ", "IWM",
	"VIX", "UVXY", "AAPL", "TSLA", "NVDA", "META",
	"AMZN", "GOOG", "GOOGL", "NFLX", "AMD", "ORCL", "BABA",
}
