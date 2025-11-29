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

// DefaultTickers returns a default set of common tickers
func DefaultTickers() []string {
	return []string{"SPX", "NDX", "RUT", "SPY", "QQQ", "IWM"}
}

// ValidTickers lists all supported tickers (41 total)
var ValidTickers = map[string]bool{
	// Indices
	"SPX": true, "ES_SPX": true, "NDX": true, "NQ_NDX": true, "RUT": true, "VIX": true,
	// ETFs
	"SPY": true, "QQQ": true, "TQQQ": true, "UVXY": true, "IWM": true, "TLT": true,
	"GLD": true, "USO": true, "SLV": true, "HYG": true, "IBIT": true,
	// Stocks
	"AAPL": true, "TSLA": true, "MSFT": true, "AMZN": true, "NVDA": true, "META": true,
	"NFLX": true, "AVGO": true, "MSTR": true, "GOOG": true, "GOOGL": true, "AMD": true,
	"SMCI": true, "COIN": true, "PLTR": true, "APP": true, "BABA": true, "SNOW": true,
	"IONQ": true, "HOOD": true, "CRWD": true, "MU": true, "CRWV": true, "INTC": true,
	"UNH": true, "VALE": true, "SOFI": true, "GME": true, "TSM": true, "ORCL": true, "RDDT": true,
}
