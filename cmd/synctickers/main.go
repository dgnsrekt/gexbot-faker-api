// Command synctickers refreshes internal/config/tickers.json from GEXbot's
// /tickers endpoint — the source of truth for the ticker universe that
// internal/config embeds into ValidTickers / IndexTickers / FutureTickers.
//
// Run from the repo root:
//
//	go run ./cmd/synctickers            # fetch + rewrite internal/config/tickers.json
//	go run ./cmd/synctickers --check    # fetch + diff; exit 1 if drifted (used by CI)
//
// The /tickers endpoint is unauthenticated, so no API key is needed.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"
)

const defaultURL = "https://api.gex.bot/v2/tickers"

type tickers struct {
	Stocks  []string `json:"stocks"`
	Indexes []string `json:"indexes"`
	Futures []string `json:"futures"`
}

// normalize renders tickers with fixed key order, sorted symbols, 2-space
// indent, and a trailing newline, so a clean fetch byte-matches the committed
// file and --check has a stable comparison.
func normalize(t tickers) ([]byte, error) {
	sort.Strings(t.Stocks)
	sort.Strings(t.Indexes)
	sort.Strings(t.Futures)
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func main() {
	check := flag.Bool("check", false, "fetch and diff against the committed file; exit 1 on drift")
	url := flag.String("url", defaultURL, "GEXbot /tickers endpoint")
	out := flag.String("o", "internal/config/tickers.json", "output path (relative to repo root)")
	flag.Parse()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(*url)
	if err != nil {
		fatal("GET %s: %v", *url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		fatal("GET %s -> %d", *url, resp.StatusCode)
	}

	var t tickers
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		fatal("decode /tickers: %v", err)
	}
	if len(t.Stocks)+len(t.Indexes)+len(t.Futures) == 0 {
		fatal("empty ticker list from %s", *url)
	}

	live, err := normalize(t)
	if err != nil {
		fatal("normalize: %v", err)
	}

	if *check {
		have, err := os.ReadFile(*out)
		if err != nil {
			fatal("read %s: %v", *out, err)
		}
		if !bytes.Equal(have, live) {
			fmt.Fprintf(os.Stderr, "%s is STALE — GEXbot's ticker list changed.\nRun `go run ./cmd/synctickers` and commit %s.\n", *out, *out)
			os.Exit(1)
		}
		fmt.Printf("%s is current\n", *out)
		return
	}

	if err := os.WriteFile(*out, live, 0o644); err != nil {
		fatal("write %s: %v", *out, err)
	}
	fmt.Printf("wrote %s (%d stocks, %d indexes, %d futures)\n", *out, len(t.Stocks), len(t.Indexes), len(t.Futures))
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
