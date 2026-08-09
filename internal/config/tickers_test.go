package config

import "testing"

func TestTickerUniverseFromEmbed(t *testing.T) {
	if len(ValidTickers) == 0 {
		t.Fatal("ValidTickers empty — tickers.json embed failed to parse")
	}

	// Indexes and futures must be valid tickers and mutually exclusive.
	for tk := range IndexTickers {
		if !ValidTickers[tk] {
			t.Errorf("index %q missing from ValidTickers", tk)
		}
		if FutureTickers[tk] {
			t.Errorf("%q is classified as both index and future", tk)
		}
	}
	for tk := range FutureTickers {
		if !ValidTickers[tk] {
			t.Errorf("future %q missing from ValidTickers", tk)
		}
	}

	// Known members land in the right buckets.
	for _, tk := range []string{"SPX", "NDX", "RUT", "VIX"} {
		if !IndexTickers[tk] {
			t.Errorf("expected %q in IndexTickers", tk)
		}
	}
	for _, tk := range []string{"ES_SPX", "NQ_NDX"} {
		if !FutureTickers[tk] {
			t.Errorf("expected %q in FutureTickers", tk)
		}
	}
	if !ValidTickers["AAPL"] {
		t.Error("expected AAPL in ValidTickers")
	}
}
