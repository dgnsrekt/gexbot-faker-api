package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dgnsrekt/gexbot-downloader/internal/api/generated"
)

var dateDirRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type setupOpts struct {
	date     string
	dataDir  string
	noDocker bool
	timeout  time.Duration
}

func setupCmd() *cobra.Command {
	var o setupOpts
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Zero->ready bootstrap: find/start a faker, load a date, verify, print ready state",
		Long: "setup discovers a running faker (or brings one up via docker compose), ensures a\n" +
			"date is loaded — unpacking an existing on-disk EOD archive with no API key — verifies\n" +
			"with a sample pull, and prints the ready state as JSON on stdout. Progress lines go to\n" +
			"stderr. It never downloads without GEXBOT_API_KEY and never hangs silently.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if o.dataDir == "" {
				o.dataDir = envOr("DATA_DIR", "./data")
			}
			return runSetup(cmd.Context(), o)
		},
	}
	cmd.Flags().StringVar(&o.date, "date", "", "date to load (default: currently loaded, else newest available/archived)")
	cmd.Flags().StringVar(&o.dataDir, "data-dir", "", "local data dir to scan for EOD archives (env DATA_DIR, else ./data)")
	cmd.Flags().BoolVar(&o.noDocker, "no-docker", false, "never try to start docker; only use an already-running faker")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 90*time.Second, "how long to wait for the faker to become healthy")
	return cmd
}

// health is the subset of /health the bootstrap reasons about.
type health struct {
	Status    string `json:"status"`
	DataDate  string `json:"data_date"`
	DataMode  string `json:"data_mode"`
	CacheMode string `json:"cache_mode"`
}

func runSetup(ctx context.Context, o setupOpts) error {
	c := newClient()

	// 1. Discover a running faker.
	progress("discover", "probing "+c.base+"/health")
	h, err := probeHealth(ctx, c)
	if err != nil {
		// 2. Bring one up.
		if o.noDocker {
			return fail(&apiError{Msg: "no faker reachable and --no-docker set", Hint: "start it with `just serve-gex-faker` or `docker compose up -d gex-faker-api`"})
		}
		if err := bringUpDocker(ctx, o.timeout, c); err != nil {
			return fail(err)
		}
		if h, err = probeHealth(ctx, c); err != nil {
			return fail(&apiError{Msg: "faker started but /health is not answering: " + err.Error()})
		}
	}
	progress("discover", "faker is up", "data_date", h.DataDate, "cache_mode", h.CacheMode)

	// 3. Ensure a date is loaded.
	loaded, err := ensureDate(ctx, c, o, h)
	if err != nil {
		return fail(err)
	}

	// 4. Verify with a real sample pull, then reset the cursor so the agent starts
	// clean. A failed verification means the faker is up but not usable for a
	// standard pull, so setup must fail (nonzero exit) rather than report ready.
	tickers, err := verifyPull(ctx, c)
	if err != nil {
		return fail(err)
	}

	// Re-read status for the authoritative loaded date / cache mode.
	if h2, err := probeHealth(ctx, c); err == nil {
		h = h2
	}
	if loaded == "" {
		loaded = h.DataDate
	}

	progress("ready", "faker ready", "loaded_date", loaded, "verified", true)
	out, _ := json.Marshal(map[string]any{
		"base_url":    c.base,
		"key":         c.key,
		"loaded_date": loaded,
		"cache_mode":  h.CacheMode,
		"data_mode":   h.DataMode,
		"tickers":     tickers,
		"verified":    true,
	})
	return emit(out)
}

func probeHealth(ctx context.Context, c *apiClient) (health, error) {
	raw, err := c.get(ctx, "/health", false, nil)
	if err != nil {
		return health{}, err
	}
	var h health
	if err := json.Unmarshal(raw, &h); err != nil {
		return health{}, &apiError{Msg: "unparseable /health response: " + err.Error()}
	}
	return h, nil
}

// bringUpDocker starts the API container and polls /health until healthy.
func bringUpDocker(ctx context.Context, timeout time.Duration, c *apiClient) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return &apiError{Msg: "docker not found on PATH", Hint: "start the faker yourself with `just serve-gex-faker`"}
	}
	if !composeFilePresent() {
		return &apiError{Msg: "no docker-compose file in this directory", Hint: "run setup from the repo root, or start the faker with `just serve-gex-faker`"}
	}
	progress("bring-up", "docker compose up -d gex-faker-api")
	cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d", "gex-faker-api")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr // container build/pull noise → stderr, keeps stdout clean
	if err := cmd.Run(); err != nil {
		return &apiError{Msg: "docker compose up failed: " + err.Error()}
	}
	return waitHealthy(ctx, c, timeout)
}

func waitHealthy(ctx context.Context, c *apiClient, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	progress("bring-up", "waiting for /health", "timeout", timeout.String())
	for {
		if _, err := probeHealth(ctx, c); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return &apiError{Msg: "timed out waiting for the faker to become healthy", Hint: "check `docker compose logs gex-faker-api`"}
		}
		select {
		case <-ctx.Done():
			return &apiError{Msg: ctx.Err().Error()}
		case <-time.After(1500 * time.Millisecond):
		}
	}
}

func composeFilePresent() bool {
	for _, f := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	return false
}

// ensureDate makes sure a date is loaded and returns it. It prefers the already-
// loaded date, then a chosen date loaded via the keyless /reload-date path.
func ensureDate(ctx context.Context, c *apiClient, o setupOpts, h health) (string, error) {
	// Already loaded and no explicit override → nothing to do.
	if o.date == "" {
		if cur := currentDate(ctx, c); cur != "" {
			progress("ensure-date", "a date is already loaded", "date", cur)
			return cur, nil
		}
	}

	date := o.date
	if date == "" {
		date = pickDate(ctx, c, o.dataDir)
	}
	if date == "" {
		// Nothing on disk to load. Guardrail: only downloads need a key.
		if os.Getenv("GEXBOT_API_KEY") == "" {
			return "", &apiError{
				Msg:  "no date is loaded and no EOD archive is available to load",
				Hint: "set GEXBOT_API_KEY to fetch one, or drop an archive in " + filepath.Join(o.dataDir, "eod") + "/<date>/",
			}
		}
		return "", &apiError{
			Msg:  "no local archive found; automated download needs an explicit --date",
			Hint: "re-run with --date YYYY-MM-DD (GEXBOT_API_KEY is set), or download via the Studio/downloader first",
		}
	}

	progress("ensure-date", "loading date via /reload-date (materializes if needed)", "date", date)
	body := generated.ReloadDateRequest{Date: date}
	if _, err := c.postJSON(ctx, "/reload-date", false, body); err != nil {
		return "", err
	}
	return date, nil
}

func currentDate(ctx context.Context, c *apiClient) string {
	raw, err := c.get(ctx, "/current-date", false, nil)
	if err != nil {
		return ""
	}
	var m generated.CurrentDateResponse
	if json.Unmarshal(raw, &m) == nil && m.CurrentDate != nil {
		return *m.CurrentDate
	}
	return ""
}

// pickDate chooses a date to load: newest from /available-dates, else newest EOD
// archive directory under <dataDir>/eod.
func pickDate(ctx context.Context, c *apiClient, dataDir string) string {
	if raw, err := c.get(ctx, "/available-dates", false, nil); err == nil {
		var r generated.AvailableDatesResponse
		if json.Unmarshal(raw, &r) == nil && r.Dates != nil && len(*r.Dates) > 0 {
			d := append([]string{}, *r.Dates...)
			sort.Strings(d)
			return d[len(d)-1]
		}
	}
	return newestArchive(filepath.Join(dataDir, "eod"))
}

func newestArchive(eodDir string) string {
	entries, err := os.ReadDir(eodDir)
	if err != nil {
		return ""
	}
	var dates []string
	for _, e := range entries {
		if e.IsDir() && dateDirRe.MatchString(e.Name()) {
			dates = append(dates, e.Name())
		}
	}
	if len(dates) == 0 {
		return ""
	}
	sort.Strings(dates)
	return dates[len(dates)-1]
}

// verifyPull confirms end-to-end health with a real authenticated data pull, then
// rewinds that key's cursor so the agent starts clean. It returns the ticker list
// and a structured error if the pull did not return data.
func verifyPull(ctx context.Context, c *apiClient) ([]string, error) {
	tickers := indexTickers(ctx, c)
	sample := "SPX"
	if len(tickers) > 0 {
		sample = tickers[0]
	}
	progress("verify", "sample pull", "endpoint", "/"+sample+"/classic/gex_zero")
	if _, err := c.get(ctx, "/"+sample+"/classic/gex_zero", true, nil); err != nil {
		var ae *apiError
		if !errors.As(err, &ae) {
			ae = &apiError{Msg: err.Error()}
		}
		if ae.Hint == "" {
			ae.Hint = "faker is up but returned no classic/gex_zero for " + sample +
				" — check `gexfakercli available <date>`"
		}
		return tickers, ae
	}
	// The sample pull advanced this key's cursor; rewind it so the agent really
	// starts at index 0. If the rewind fails we must not report a clean ready
	// state — propagate the error.
	if _, err := c.postJSON(ctx, "/reset-cache?key="+url.QueryEscape(c.key), false, nil); err != nil {
		var ae *apiError
		if !errors.As(err, &ae) {
			ae = &apiError{Msg: err.Error()}
		}
		if ae.Hint == "" {
			ae.Hint = "sample pull succeeded but the cursor rewind failed — run `gexfakercli reset` before pulling"
		}
		return tickers, ae
	}
	return tickers, nil
}

func indexTickers(ctx context.Context, c *apiClient) []string {
	raw, err := c.get(ctx, "/tickers", false, nil)
	if err != nil {
		return nil
	}
	var r generated.TickersResponse
	if json.Unmarshal(raw, &r) != nil {
		return nil
	}
	var out []string
	if r.Indexes != nil {
		out = append(out, *r.Indexes...)
	}
	if r.Stocks != nil {
		out = append(out, *r.Stocks...)
	}
	if r.Futures != nil {
		out = append(out, *r.Futures...)
	}
	return out
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
