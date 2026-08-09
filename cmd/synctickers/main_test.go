package main

import (
	"strings"
	"testing"
)

func TestNormalizeSortsAndIsDeterministic(t *testing.T) {
	b, err := normalize(tickers{
		Stocks:  []string{"TSLA", "AAPL"},
		Indexes: []string{"VIX", "SPX"},
		Futures: []string{"NQ_NDX", "ES_SPX"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)

	if !strings.HasSuffix(got, "}\n") {
		t.Errorf("expected trailing newline, got:\n%q", got)
	}
	if strings.Index(got, "AAPL") > strings.Index(got, "TSLA") {
		t.Error("stocks not sorted")
	}
	if strings.Index(got, "SPX") > strings.Index(got, "VIX") {
		t.Error("indexes not sorted")
	}

	// Same content in a different input order must produce identical bytes,
	// so a clean fetch always byte-matches the committed file for --check.
	b2, _ := normalize(tickers{
		Stocks:  []string{"AAPL", "TSLA"},
		Indexes: []string{"SPX", "VIX"},
		Futures: []string{"ES_SPX", "NQ_NDX"},
	})
	if string(b2) != got {
		t.Error("normalize is not deterministic across input order")
	}
}
