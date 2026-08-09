package config

import (
	_ "embed"
	"encoding/json"
)

// tickers.json is the ticker source of truth, synced from GEXbot's /tickers
// endpoint by `go run ./cmd/synctickers` (a daily CI job checks it for drift —
// see .github/workflows/tickers-check.yml). Do not hand-edit it.
//
//go:embed tickers.json
var tickersJSON []byte

type tickerUniverse struct {
	Stocks  []string `json:"stocks"`
	Indexes []string `json:"indexes"`
	Futures []string `json:"futures"`
}

var (
	// ValidTickers is every supported ticker (stocks + indexes + futures).
	ValidTickers = map[string]bool{}
	// IndexTickers are the tickers GEXbot reports under "indexes".
	IndexTickers = map[string]bool{}
	// FutureTickers are the tickers GEXbot reports under "futures".
	FutureTickers = map[string]bool{}
)

func init() {
	var u tickerUniverse
	if err := json.Unmarshal(tickersJSON, &u); err != nil {
		panic("config: invalid embedded tickers.json: " + err.Error())
	}
	for _, s := range u.Stocks {
		ValidTickers[s] = true
	}
	for _, s := range u.Indexes {
		ValidTickers[s] = true
		IndexTickers[s] = true
	}
	for _, s := range u.Futures {
		ValidTickers[s] = true
		FutureTickers[s] = true
	}
	if len(ValidTickers) == 0 {
		panic("config: embedded tickers.json produced no tickers")
	}
}
